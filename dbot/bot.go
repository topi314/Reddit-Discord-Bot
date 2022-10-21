package dbot

import (
	"context"
	"net/http"
	"regexp"

	"github.com/TopiSenpai/Reddit-Discord-Bot/db"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/httpserver"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/log"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const (
	CallbackURL            = "/subreddit/callback"
	SuccessURL             = "/subreddit/success"
	InteractionCallbackURL = "/interactions/callback"
)

var imageRegex = regexp.MustCompile(`.*\.(?:jpg|gif|png)`)

type Bot struct {
	Logger        log.Logger
	HTTPClient    *http.Client
	Mux           *http.ServeMux
	Client        bot.Client
	Reddit        *reddit.Client
	OAuth2Client  oauth2.Client
	WebhookStates map[string]oauth2State
	Subreddits    *Subreddits
	DB            db.DB
	Config        Config
	Version       string
}

func (b *Bot) Setup() (err error) {
	b.OAuth2Client = oauth2.New(b.Client.ApplicationID(), b.Config.Bot.Secret)
	b.WebhookStates = make(map[string]oauth2State)
	b.HTTPClient = &http.Client{Timeout: 20}
	b.Mux = http.NewServeMux()
	b.Mux.HandleFunc(CallbackURL, b.OnSubredditCreateHandler)
	b.Mux.HandleFunc(SuccessURL, b.OnSubredditCreateSuccessHandler)

	var opt bot.ConfigOpt
	if b.Config.Bot.PublicKey != "" {
		opt = bot.WithHTTPServerConfigOpts(b.Config.Bot.PublicKey,
			httpserver.WithServeMux(b.Mux),
			httpserver.WithAddress(b.Config.Bot.ServerAddress),
			httpserver.WithURL(InteractionCallbackURL),
		)
	} else {
		opt = bot.WithDefaultGateway()
	}
	b.Client, err = disgo.New(b.Config.Bot.Token,
		bot.WithLogger(b.Logger),
		opt,
		bot.WithRestClientConfigOpts(rest.WithHTTPClient(b.HTTPClient)),
		bot.WithEventListenerFunc(b.OnApplicationCommand),
	)
	return
}

func (b *Bot) RegisterCommands() {
	if b.Config.DevMode {
		for _, guildID := range b.Config.DevGuildIDs {
			if _, err := b.Client.Rest().SetGuildCommands(b.Client.ID(), guildID, commands); err != nil {
				b.Logger.Errorf("failed to register commands for guild %s: %s", guildID, err)
			}
		}
	}
	if _, err := b.Client.Rest().SetGlobalCommands(b.Client.ID(), commands); err != nil {
		b.Logger.Errorf("failed to register global commands: %s", err)
	}
}

func (b *Bot) Start() error {
	if b.Config.Bot.PublicKey != "" {
		return b.Client.OpenHTTPServer()
	}
	go func() {
		if err := http.ListenAndServe(b.Config.Bot.ServerAddress, b.Mux); err != nil && err != http.ErrServerClosed {
			b.Logger.Errorf("error running http server: %s", err)
		}
	}()
	return b.Client.OpenGateway(context.TODO())
}
