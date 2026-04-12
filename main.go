package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/topi314/reddit-discord-bot/v2/redditbot"
)

var (
	//go:embed sql/schema.sql
	schema string

	//go:embed reddit.png
	redditIcon []byte
)

func main() {
	version, commit := readVersionAndCommit()
	slog.Info("starting reddit-discord-bot...", slog.String("version", version), slog.String("commit", commit))
	cfg, err := redditbot.ReadConfig()
	if err != nil {
		slog.Error("error reading config", slog.Any("err", err))
		return
	}

	if err = setupLogger(cfg.Log); err != nil {
		slog.Error("error setting up logger", slog.Any("err", err))
		return
	}

	slog.Info("loaded config", slog.String("config", cfg.String()))
	if err = cfg.Validate(); err != nil {
		slog.Error("error validating config", slog.Any("err", err))
		return
	}

	client, err := disgo.New(cfg.Discord.Token,
		bot.WithDefaultGateway(),
	)
	if err != nil {
		slog.Error("error creating client", slog.Any("err", err))
		return
	}

	reddit, err := redditbot.NewReddit(cfg.Reddit, version)
	if err != nil {
		slog.Error("error creating reddit client", slog.Any("err", err))
		return
	}

	db, err := redditbot.NewDB(cfg.Database, schema)
	if err != nil {
		slog.Error("error creating database client", slog.Any("err", err))
	}

	b := redditbot.New(cfg, redditIcon, client, reddit, db)
	defer b.Close()

	if cfg.Metrics.Enabled {
		mux := http.NewServeMux()
		mux.Handle(cfg.Metrics.Endpoint, promhttp.Handler())
		b.MetricsServer = &http.Server{
			Addr:    cfg.Metrics.ListenAddr,
			Handler: mux,
		}
	}

	if cfg.Server.Enabled {
		mux := http.NewServeMux()
		mux.HandleFunc(cfg.Server.Endpoint, b.OnDiscordCallback)
		b.Server = &http.Server{
			Addr:    cfg.Server.ListenAddr,
			Handler: mux,
		}
	}

	b.Client.AddEventListeners(bot.NewListenerFunc(b.OnApplicationCommand))

	if cfg.Discord.SyncCommands {
		if _, err = client.Rest.SetGlobalCommands(client.ApplicationID, redditbot.Commands); err != nil {
			slog.Error("error setting global commands", slog.Any("err", err))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = client.OpenGateway(ctx); err != nil {
		slog.Error("error opening discord gateway", slog.Any("err", err))
	}

	go b.ListenSubreddits()

	if cfg.Server.Enabled {
		go b.ListenAndServe()
		defer func() {
			sCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer sCancel()
			if sErr := b.Server.Shutdown(sCtx); sErr != nil {
				slog.Error("error shutting down server", slog.Any("err", sErr))
			}
		}()
	}

	if cfg.Metrics.Enabled {
		go b.ListenAndServeMetrics()
		defer func() {
			mCtx, mCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer mCancel()
			if mErr := b.MetricsServer.Shutdown(mCtx); mErr != nil {
				slog.Error("error shutting down metrics server", slog.Any("err", mErr))
			}
		}()
	}

	defer slog.Info("stopping reddit-discord-bot...")

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	<-s
}

func readVersionAndCommit() (version string, commit string) {
	version = "(devel)"
	commit = "unknown"

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit
	}

	if bi.Main.Version != "" {
		version = bi.Main.Version
	}

	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			commit = s.Value
			break
		}
	}

	return version, commit
}

func setupLogger(cfg redditbot.LogConfig) error {
	options := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     cfg.Level,
	}

	var sHandler slog.Handler
	switch cfg.Format {
	case "json":
		sHandler = slog.NewJSONHandler(os.Stdout, options)
	case "text":
		sHandler = slog.NewTextHandler(os.Stdout, options)
	default:
		return fmt.Errorf("unknown log format: %s", cfg.Format)
	}
	slog.SetDefault(slog.New(sHandler))
	return nil
}
