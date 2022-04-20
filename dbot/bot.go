package dbot

import (
	"context"

	"github.com/TopiSenpai/Reddit-Discord-Bot/reddit"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/log"
)

type Bot struct {
	Logger  log.Logger
	Client  bot.Client
	Reddit  *reddit.Client
	DB      DB
	Config  Config
	Version string
}

func (b *Bot) SetupBot() (err error) {
	b.Client, err = disgo.New(b.Config.Token,
		bot.WithLogger(b.Logger),
		bot.WithGatewayConfigOpts(gateway.WithGatewayIntents(discord.GatewayIntentGuilds, discord.GatewayIntentGuildVoiceStates)),
		bot.WithEventListeners(b.Commands, b.Paginator, b.Listeners),
	)
	return err
}

func (b *Bot) StartBot() (err error) {
	return b.Client.ConnectGateway(context.TODO())
}
