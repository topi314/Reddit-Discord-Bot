package redditbot

import (
	"context"
	"math/rand"
	"net/http"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/oauth2"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

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
	Interaction discord.ApplicationCommandInteraction
}

type Bot struct {
	Cfg           Config
	RedditIcon    []byte
	Client        bot.Client
	Reddit        *Reddit
	DB            *DB
	Server        *http.Server
	MetricsServer *http.Server
	DiscordConfig *oauth2.Config
	Rand          *rand.Rand

	States map[string]SetupState
}

func (b *Bot) randomString(length int) string {
	bb := make([]byte, length)
	for i := range bb {
		bb[i] = letters[b.Rand.Intn(len(letters))]
	}
	return string(bb)
}

func (b *Bot) ListenAndServe() {
	if err := b.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("error starting server:", err.Error())
	}
}

func (b *Bot) ListenAndServeMetrics() {
	if err := b.MetricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("error starting server:", err.Error())
	}
}

func (b *Bot) Close() {
	b.Client.Close(context.Background())
	_ = b.DB.Close()
	_ = b.Server.Shutdown(context.Background())
	if b.MetricsServer != nil {
		_ = b.MetricsServer.Shutdown(context.Background())
	}
}
