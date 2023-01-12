package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/json"
	"github.com/disgoorg/log"
	"github.com/robfig/cron/v3"
	xoauth2 "golang.org/x/oauth2"
)

//go:embed anilist.gql
var anilistQuery string

type Anicord struct {
	client      bot.Client
	oauth2      oauth2.Client
	anilist     xoauth2.Config
	rateLimiter *RateLimiter
	db          *DB
	c           *cron.Cron
}

type RateLimiter struct {
	Limit     int
	Remaining int
	Reset     time.Time
	mu        sync.Mutex
}

func (r *RateLimiter) Lock() {
	r.mu.Lock()
	now := time.Now()
	if now.After(r.Reset) {
		r.Reset = now
		r.Remaining = r.Limit
	}

	if r.Remaining == 0 {
		time.Sleep(r.Reset.Sub(now))
	}
	r.Remaining--
}

func (r *RateLimiter) Unlock(rs *http.Response) error {
	defer r.mu.Unlock()
	if rs == nil {
		return nil
	}

	var (
		limit      = rs.Header.Get("X-RateLimit-Limit")
		remaining  = rs.Header.Get("X-RateLimit-Remaining")
		retryAfter = rs.Header.Get("Retry-After")
	)

	var err error
	r.Limit, err = strconv.Atoi(limit)
	if err != nil {
		return err
	}
	r.Remaining, err = strconv.Atoi(remaining)
	if err != nil {
		return err
	}
	if retryAfter != "" {
		var after int
		after, err = strconv.Atoi(retryAfter)
		if err != nil {
			return err
		}
		r.Reset = time.Now().Add(time.Duration(after) * time.Second)
	}

	return nil
}

func (a *Anicord) updateApplicationMetadata() error {
	_, err := a.client.Rest().UpdateApplicationRoleConnectionMetadata(a.client.ApplicationID(), []discord.ApplicationRoleConnectionMetadata{
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

func (a *Anicord) updateMetadata() {
	log.Info("updating metadata...")
	users, err := a.db.GetUsers()
	if err != nil {
		log.Error(err)
		return
	}
	for _, user := range users {
		if err = a.updateUserMetadata(context.Background(), user); err != nil {
			log.Error(err)
			continue
		}
	}
}

func (a *Anicord) updateUserMetadata(ctx context.Context, user User) error {
	httpClient := a.anilist.Client(ctx, &xoauth2.Token{
		AccessToken:  user.AnilistAccessToken,
		RefreshToken: user.AnilistRefreshToken,
		Expiry:       user.AnilistExpiry,
	})

	data, err := json.Marshal(map[string]any{
		"query": anilistQuery,
	})
	if err != nil {
		return fmt.Errorf("error while marshaling query: %w", err)
	}

	a.rateLimiter.Lock()

	rq, err := http.NewRequest(http.MethodPost, "https://graphql.anilist.co", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("error while creating gql request: %w", err)
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := httpClient.Do(rq)
	if err != nil {
		_ = a.rateLimiter.Unlock(nil)
		return fmt.Errorf("error while executing gql request: %w", err)
	}
	if err = a.rateLimiter.Unlock(rs); err != nil {
		a.client.Logger().Errorf("error while updating rate limiter: %s", err)
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
				return a.db.DeleteUser(user.DiscordID)
			}
			errStr += fmt.Sprintf("%d: %s, ", anilistErr.Status, anilistErr.Message)
		}
		return fmt.Errorf("error while executing gql request: %v", errStr)
	}

	_, err = a.oauth2.UpdateApplicationRoleConnection(user, a.client.ApplicationID(), discord.ApplicationRoleConnectionUpdate{
		PlatformName:     json.Ptr("Anilist"),
		PlatformUsername: &v.Data.Viewer.Name,
		Metadata: &map[string]string{
			"anime_count": strconv.Itoa(v.Data.Viewer.Statistics.Anime.Count),
			"manga_count": strconv.Itoa(v.Data.Viewer.Statistics.Manga.Count),
		},
	})
	if restErr, ok := err.(*rest.Error); ok && restErr.Response.StatusCode == http.StatusUnauthorized {
		return a.db.DeleteUser(user.DiscordID)
	}
	if err != nil {
		return fmt.Errorf("error while updating application role connection: %w", err)
	}
	return nil
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
