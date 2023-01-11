package main

import (
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/log"
	xoauth2 "golang.org/x/oauth2"
	"net/http"
	"os"
)

var (
	discordToken        = os.Getenv("DISCORD_TOKEN")
	discordClientSecret = os.Getenv("DISCORD_CLIENT_SECRET")
	baseURL             = os.Getenv("BASE_URL")

	anilistClientID     = os.Getenv("ANILIST_CLIENT_ID")
	anilistClientSecret = os.Getenv("ANILIST_CLIENT_SECRET")

	letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

type Anicord struct {
	client  bot.Client
	oauth2  oauth2.Client
	anilist xoauth2.Config
}

func (a *Anicord) updateMetadata() error {
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

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting Anicord...")
	log.Infof("disgo %s", disgo.Version)

	client, err := disgo.New(discordToken)
	if err != nil {
		log.Panic(err)
	}
	oauth2Client := oauth2.New(client.ApplicationID(), discordClientSecret)

	a := &Anicord{
		client: client,
		oauth2: oauth2Client,
		anilist: xoauth2.Config{
			ClientID:     anilistClientID,
			ClientSecret: anilistClientSecret,
			Endpoint: xoauth2.Endpoint{
				AuthURL:  "https://anilist.co/api/v2/oauth/authorize",
				TokenURL: "https://anilist.co/api/v2/oauth/token",
			},
			RedirectURL: baseURL + "/anilist",
			Scopes:      []string{},
		},
	}

	if err = a.updateMetadata(); err != nil {
		log.Panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/verify", a.handleVerify)
	mux.HandleFunc("/discord", a.handleDiscord)
	mux.HandleFunc("/anilist", a.handleAnilist)
	_ = http.ListenAndServe("0.0.0.0:6969", mux)
}

type anilistResponse struct {
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
