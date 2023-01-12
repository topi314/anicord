package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/httpserver"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/log"
	"github.com/robfig/cron/v3"
	xoauth2 "golang.org/x/oauth2"
)

var (
	discordToken        = os.Getenv("DISCORD_TOKEN")
	discordClientSecret = os.Getenv("DISCORD_CLIENT_SECRET")
	discordPublicKey    = os.Getenv("DISCORD_PUBLIC_KEY")
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

	mux := http.NewServeMux()

	client, err := disgo.New(discordToken,
		bot.WithHTTPServerConfigOpts(discordPublicKey,
			httpserver.WithServeMux(mux),
			httpserver.WithAddress(listenAddr),
		),
	)
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
		rateLimiter: &RateLimiter{
			Limit: 1,
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

	mux.HandleFunc("/verify", a.handleVerify)
	mux.HandleFunc("/discord", a.handleDiscord)
	mux.HandleFunc("/anilist", a.handleAnilist)
	if err = client.OpenHTTPServer(); err != nil {
		log.Panic(err)
	}

	log.Info("Anicord is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}
