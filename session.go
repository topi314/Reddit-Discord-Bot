package main

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
)

var _ oauth2.Session = (*Session)(nil)

type Session struct {
	webhook *discord.IncomingWebhook
}

func (s *Session) AccessToken() string { panic("implement me") }

func (s *Session) RefreshToken() string { panic("implement me") }

func (s *Session) Scopes() []discord.ApplicationScope { panic("implement me") }

func (s *Session) TokenType() discord.TokenType { panic("implement me") }

func (s *Session) Expiration() time.Time { panic("implement me") }

func (s *Session) Webhook() *discord.IncomingWebhook {
	return s.webhook
}
