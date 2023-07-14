package redditbot

import (
	"math/rand"
	"net/http"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/oauth2"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type SetupState struct {
	Subreddit   string
	Interaction discord.ApplicationCommandInteraction
}

type Bot struct {
	Cfg           Config
	RedditIcon    []byte
	Meter         metric.Meter
	Client        bot.Client
	Reddit        *Reddit
	DB            *DB
	Server        *http.Server
	DiscordConfig *oauth2.Config
	Rand          *rand.Rand

	States    map[string]SetupState
	LastPosts map[snowflake.ID]string

	PostsSentGauge    metric.Int64Counter
	SubredditsCounter metric.Int64UpDownCounter
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
		log.Fatalf("error starting server: %s", err.Error())
	}
}
