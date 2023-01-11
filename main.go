package main

import (
	"net/http"
	"os"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/log"
	"github.com/robfig/cron/v3"
	xoauth2 "golang.org/x/oauth2"
)

var (
	discordToken        = os.Getenv("DISCORD_TOKEN")
	discordClientSecret = os.Getenv("DISCORD_CLIENT_SECRET")
	baseURL             = os.Getenv("BASE_URL")
	listenAddr          = os.Getenv("LISTEN_ADDR")

	anilistClientID     = os.Getenv("ANILIST_CLIENT_ID")
	anilistClientSecret = os.Getenv("ANILIST_CLIENT_SECRET")

	letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting Anicord...")
	log.Infof("disgo %s", disgo.Version)

	client, err := disgo.New(discordToken)
	if err != nil {
		log.Panic(err)
	}
	oauth2Client := oauth2.New(client.ApplicationID(), discordClientSecret)

	db, err := NewDB()
	if err != nil {
		log.Panic(err)
	}

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
		db: db,
		c:  cron.New(),
	}

	if err = a.updateApplicationMetadata(); err != nil {
		log.Panic(err)
	}

	if _, err = a.c.AddFunc("@every 12h", a.updateMetadata); err != nil {
		log.Panic(err)
	}

	a.c.Start()

	mux := http.NewServeMux()
	mux.HandleFunc("/verify", a.handleVerify)
	mux.HandleFunc("/discord", a.handleDiscord)
	mux.HandleFunc("/anilist", a.handleAnilist)
	_ = http.ListenAndServe(listenAddr, mux)
}
