package main

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
)

var _ oauth2.SessionController = (*CustomSessionController)(nil)

type CustomSessionController struct{}

func (c *CustomSessionController) CreateSessionFromResponse(_ string, response discord.AccessTokenResponse) oauth2.Session {
	return c.CreateSession("", "", "", nil, "", time.Time{}, response.Webhook)
}

func (c *CustomSessionController) GetSession(_ string) oauth2.Session {
	return nil
}

func (c *CustomSessionController) CreateSession(_ string, _ string, _ string, _ []discord.OAuth2Scope, _ discord.TokenType, _ time.Time, webhook *discord.IncomingWebhook) oauth2.Session {
	return &Session{
		webhook: webhook,
	}
}

func (c *CustomSessionController) CreateSessionFromExchange(_ string, exchange discord.AccessTokenResponse) oauth2.Session {
	return c.CreateSession("", "", "", nil, "", time.Time{}, exchange.Webhook)
}
