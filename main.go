package main

import (
	"context"
	_ "embed"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/httpserver"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/lmittmann/tint"
	"github.com/topi314/anicord/anicord"
)

var (
	//go:embed sql/schema.sql
	schema string

	//go:embed anilist.gql
	anilistQuery string
)

func main() {
	path := flag.String("config", "config.yml", "path to config.yml")
	flag.Parse()

	cfg, err := anicord.ReadConfig(*path)
	if err != nil {
		slog.Error("failed to read config", tint.Err(err))
		os.Exit(-1)
	}
	setupLogger(cfg.Log)

	slog.Info("starting Anicord...")
	slog.Info("disgo version: ", slog.String("version", disgo.Version))

	mux := http.NewServeMux()

	client, err := disgo.New(cfg.Discord.Token,
		bot.WithHTTPServerConfigOpts(cfg.Discord.PublicKey,
			httpserver.WithServeMux(mux),
			httpserver.WithAddress(cfg.Server.ListenAddr),
		),
	)
	if err != nil {
		slog.Error("error while creating disgo client", tint.Err(err))
		os.Exit(-1)
	}
	oauth2Client := oauth2.New(client.ApplicationID(), cfg.Discord.ClientSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := anicord.NewDB(ctx, cfg.DB, schema)
	if err != nil {
		slog.Error("error while creating database", tint.Err(err))
		os.Exit(-1)
	}

	b := anicord.New(cfg, client, oauth2Client, db, anilistQuery)
	if err = b.UpdateApplicationMetadata(); err != nil {
		slog.Error("error while updating application metadata", tint.Err(err))
		os.Exit(-1)
	}

	if err = b.SetupCron(); err != nil {
		slog.Error("error while setting up cron", tint.Err(err))
		os.Exit(-1)
	}

	b.SetupRoutes(mux)
	if err = client.OpenHTTPServer(); err != nil {
		slog.Error("error while opening http server", tint.Err(err))
		os.Exit(-1)
	}

	slog.Info("Anicord is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func setupLogger(cfg anicord.LogConfig) {
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: cfg.AddSource,
			Level:     cfg.Level,
		})
	} else {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			AddSource: cfg.AddSource,
			Level:     cfg.Level,
			NoColor:   cfg.NoColor,
		})
	}
	slog.SetDefault(slog.New(handler))
}
