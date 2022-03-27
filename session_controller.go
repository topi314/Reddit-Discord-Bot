package main

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
)

var _ oauth2.SessionController = (*CustomSessionController)(nil)

type CustomSessionController struct{}

func (c *CustomSessionController) GetSession(identifier string) oauth2.Session { panic("implement me") }

func (c *CustomSessionController) CreateSession(_ string, _ string, _ string, _ []discord.ApplicationScope, _ discord.TokenType, _ time.Time, webhook *discord.IncomingWebhook) oauth2.Session {
	return &Session{
		webhook: webhook,
	}
}

func (c *CustomSessionController) CreateSessionFromExchange(_ string, exchange discord.AccessTokenExchange) oauth2.Session {
	return c.CreateSession("", "", "", nil, "", time.Time{}, exchange.Webhook)
}
