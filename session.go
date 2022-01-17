package main

import (
	"time"

	"github.com/DisgoOrg/disgo/discord"
	"github.com/DisgoOrg/disgo/oauth2"
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
