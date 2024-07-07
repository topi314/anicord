package anicord

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/json"
	"github.com/lmittmann/tint"
	"github.com/robfig/cron/v3"
	xoauth2 "golang.org/x/oauth2"
)

func New(cfg Config, client bot.Client, oauth2Client oauth2.Client, db *DB, anilistQuery string) *Bot {
	return &Bot{
		cfg:    cfg,
		client: client,
		oauth2: oauth2Client,
		anilist: xoauth2.Config{
			ClientID:     cfg.Anilist.ClientID,
			ClientSecret: cfg.Anilist.ClientSecret,
			Endpoint: xoauth2.Endpoint{
				AuthURL:  "https://anilist.co/api/v2/oauth/authorize",
				TokenURL: "https://anilist.co/api/v2/oauth/token",
			},
			RedirectURL: cfg.Server.BaseURL + "/anilist",
			Scopes:      []string{},
		},
		sessions:     map[string]oauth2.Session{},
		rateLimiter:  &RateLimiter{Limit: 1},
		db:           db,
		c:            cron.New(),
		anilistQuery: anilistQuery,
	}
}

type Bot struct {
	cfg          Config
	client       bot.Client
	oauth2       oauth2.Client
	anilist      xoauth2.Config
	sessions     map[string]oauth2.Session
	rateLimiter  *RateLimiter
	db           *DB
	c            *cron.Cron
	anilistQuery string
}

func (b *Bot) SetupCron() error {
	if _, err := b.c.AddFunc("@every 12h", b.UpdateMetadata); err != nil {
		return fmt.Errorf("error while adding cron job: %w", err)
	}

	b.c.Start()
	return nil
}

func (b *Bot) Close() {
	cronCtx := b.c.Stop()
	select {
	case <-cronCtx.Done():
		slog.Info("cron stopped")

	case <-time.After(5 * time.Second):
		slog.Error("error while stopping cron", tint.Err(cronCtx.Err()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.client.Close(ctx)

	if err := b.db.Close(); err != nil {
		slog.Error("error while closing database", tint.Err(err))
		return
	}
}

func (b *Bot) UpdateApplicationMetadata() error {
	_, err := b.client.Rest().UpdateApplicationRoleConnectionMetadata(b.client.ApplicationID(), []discord.ApplicationRoleConnectionMetadata{
		{
			Type:        discord.ApplicationRoleConnectionMetadataTypeIntegerGreaterThanOrEqual,
			Key:         "anime_count",
			Name:        "Anime Watched",
			Description: "How many anime you have watched",
		},
		{
			Type:        discord.ApplicationRoleConnectionMetadataTypeIntegerGreaterThanOrEqual,
			Key:         "manga_count",
			Name:        "Manga Read",
			Description: "How many manga you have read",
		},
	})
	return err
}

func (b *Bot) UpdateMetadata() {
	slog.Info("updating metadata")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users, err := b.db.GetUsers(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error while getting users", tint.Err(err))
		return
	}
	for _, user := range users {
		if err = b.updateUserMetadata(ctx, user); err != nil {
			slog.ErrorContext(ctx, "error while updating user metadata", tint.Err(err))
			continue
		}
	}
}

func (b *Bot) updateUserMetadata(ctx context.Context, user User) error {
	httpClient := b.anilist.Client(ctx, &xoauth2.Token{
		AccessToken:  user.AnilistAccessToken,
		RefreshToken: user.AnilistRefreshToken,
		Expiry:       user.AnilistExpiry,
	})

	data, err := json.Marshal(map[string]any{
		"query": b.anilistQuery,
	})
	if err != nil {
		return fmt.Errorf("error while marshaling query: %w", err)
	}

	b.rateLimiter.Lock()

	rq, err := http.NewRequest(http.MethodPost, "https://graphql.anilist.co", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("error while creating gql request: %w", err)
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := httpClient.Do(rq)
	if err != nil {
		_ = b.rateLimiter.Unlock(nil)
		return fmt.Errorf("error while executing gql request: %w", err)
	}
	if err = b.rateLimiter.Unlock(rs); err != nil {
		b.client.Logger().Errorf("error while updating rate limiter: %s", err)
	}
	defer rs.Body.Close()

	var v anilistResponse
	if err = json.NewDecoder(rs.Body).Decode(&v); err != nil {
		return fmt.Errorf("error while decoding gql response: %w", err)
	}

	if len(v.Errors) > 0 {
		var errStr string
		for _, anilistErr := range v.Errors {
			if anilistErr.Message == "Invalid token" {
				return b.db.RemoveUser(ctx, user.DiscordID)
			}
			errStr += fmt.Sprintf("%d: %s, ", anilistErr.Status, anilistErr.Message)
		}
		return fmt.Errorf("error while executing gql request: %v", errStr)
	}

	session := user.Session()
	newSession, err := b.oauth2.VerifySession(session)
	if err != nil {
		return fmt.Errorf("error while verifying session: %w", err)
	}

	if newSession.AccessToken != session.AccessToken {
		user.DiscordAccessToken = newSession.AccessToken
		user.DiscordRefreshToken = newSession.RefreshToken
		user.DiscordExpiry = newSession.Expiration
		if err = b.db.AddUser(ctx, user); err != nil {
			return fmt.Errorf("error while updating user: %w", err)
		}
	}

	_, err = b.oauth2.UpdateApplicationRoleConnection(newSession, b.client.ApplicationID(), discord.ApplicationRoleConnectionUpdate{
		PlatformName:     json.Ptr("Anilist"),
		PlatformUsername: &v.Data.Viewer.Name,
		Metadata: &map[string]string{
			"anime_count": strconv.Itoa(v.Data.Viewer.Statistics.Anime.Count),
			"manga_count": strconv.Itoa(v.Data.Viewer.Statistics.Manga.Count),
		},
	})
	var restErr *rest.Error
	if errors.As(err, &restErr) && restErr.Response.StatusCode == http.StatusUnauthorized {
		return b.db.RemoveUser(ctx, user.DiscordID)
	}
	if err != nil {
		return fmt.Errorf("error while updating application role connection: %w", err)
	}
	return nil
}

func (b *Bot) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/verify", b.handleVerify)
	mux.HandleFunc("/discord", b.handleDiscord)
	mux.HandleFunc("/anilist", b.handleAnilist)
}

type anilistResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Status  int    `json:"status"`
	} `json:"errors"`
	Data struct {
		Viewer struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Statistics struct {
				Anime struct {
					Count int `json:"count"`
				} `json:"anime"`
				Manga struct {
					Count int `json:"count"`
				}
			}
		}
	}
}
