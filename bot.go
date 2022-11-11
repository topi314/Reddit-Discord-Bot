package main

import (
	"net/http"
	"sync"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/httpserver"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/log"
	"github.com/uptrace/bun"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

type RedditBot struct {
	Logger       log.Logger
	HTTPClient   *http.Client
	Client       bot.Client
	OAuth2Client oauth2.Client
	RedditClient *reddit.Client
	DB           *bun.DB

	Subreddits           map[string][]webhook.Client
	SubredditsMu         sync.RWMutex
	SubredditCancelFuncs map[string]func()
}

func (b *RedditBot) Setup() error {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc(CreateCallbackURL, b.webhookCreateHandler)
	serveMux.HandleFunc(SuccessURL, webhookCreateSuccessHandler)

	var err error
	b.Client, err = disgo.New(token,
		bot.WithLogger(b.Logger),
		bot.WithCacheConfigOpts(
			cache.WithCacheFlags(cache.FlagsNone),
			cache.WithMemberCachePolicy(cache.PolicyNone[discord.Member]),
			cache.WithMessageCachePolicy(cache.PolicyNone[discord.Message]),
		),
		bot.WithHTTPServerConfigOpts(publicKey,
			httpserver.WithAddress(webhookServerAddress),
			httpserver.WithURL(InteractionCallbackURL),
			httpserver.WithServeMux(serveMux),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnApplicationCommandInteraction: b.onApplicationCommandInteraction,
		}),
	)
	return err
}

func (b *RedditBot) Start() error {
	return b.Client.OpenHTTPServer()
}

func (b *RedditBot) SetupCommands() error {
	_, err := b.Client.Rest().SetGlobalCommands(b.Client.ApplicationID(), commands)
	return err
}

func (b *RedditBot) SetupOAuth2() {
	b.OAuth2Client = oauth2.New(b.Client.ApplicationID(), secret, oauth2.WithSessionController(&CustomSessionController{}))
}
