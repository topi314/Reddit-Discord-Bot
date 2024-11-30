package redditbot

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/oauth2"
)

const (
	letters  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	authURL  = "https://discord.com/api/oauth2/authorize"
	tokenURL = "https://discord.com/api/oauth2/token"
)

var postsSent = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "redditbot_posts_sent",
	Help: "The number of posts sent to Discord",
}, []string{"subreddit", "type", "webhook_id", "guild_id", "channel_id"})

var subreddits = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "redditbot_subreddits",
	Help: "The number of subreddits being monitored",
}, []string{"subreddit", "type", "webhook_id", "guild_id", "channel_id"})

var redditRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "redditbot_reddit_requests",
	Help: "The number of requests made to the Reddit API",
}, []string{"path", "method", "status", "important", "sleep", "used", "remaining", "reset"})

type SetupState struct {
	Subreddit   string
	PostType    string
	FormatType  FormatType
	RoleID      snowflake.ID
	RedditProxy string
	LinkButton  bool
	Interaction discord.ApplicationCommandInteraction
}

func New(cfg Config, redditIcon []byte, client bot.Client, reddit *Reddit, db *DB) *Bot {
	return &Bot{
		cfg:        cfg,
		redditIcon: redditIcon,
		Client:     client,
		reddit:     reddit,
		db:         db,
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
		discordConfig: &oauth2.Config{
			ClientID:     client.ApplicationID().String(),
			ClientSecret: cfg.Discord.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   authURL,
				TokenURL:  tokenURL,
				AuthStyle: oauth2.AuthStyleInParams,
			},
			RedirectURL: cfg.Server.RedirectURL,
			Scopes:      []string{string(discord.OAuth2ScopeWebhookIncoming)},
		},
		states: make(map[string]SetupState),
	}
}

type Bot struct {
	cfg           Config
	redditIcon    []byte
	Client        bot.Client
	reddit        *Reddit
	db            *DB
	Server        *http.Server
	MetricsServer *http.Server
	discordConfig *oauth2.Config
	rand          *rand.Rand

	states   map[string]SetupState
	statesMu sync.Mutex
}

func (b *Bot) randomString(length int) string {
	bb := make([]byte, length)
	for i := range bb {
		bb[i] = letters[b.rand.Intn(len(letters))]
	}
	return string(bb)
}

func (b *Bot) ListenAndServe() {
	if err := b.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("error starting server", slog.Any("err", err))
		os.Exit(-1)
	}
}

func (b *Bot) ListenAndServeMetrics() {
	if err := b.MetricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("error starting metrics server", slog.Any("err", err))
		os.Exit(-1)
	}
}

func (b *Bot) Close() {
	b.Client.Close(context.Background())
	_ = b.db.Close()
	_ = b.Server.Shutdown(context.Background())
	if b.MetricsServer != nil {
		_ = b.MetricsServer.Shutdown(context.Background())
	}
}
