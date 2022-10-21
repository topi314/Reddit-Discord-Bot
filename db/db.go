package db

import "github.com/disgoorg/snowflake/v2"

type Subscription struct {
	Subreddit    string
	GuildID      snowflake.ID
	WebhookID    snowflake.ID
	WebhookToken string
}

type DB interface {
	GetSubscriptions(guildID snowflake.ID) ([]Subscription, error)
	GetSubscription(guildID snowflake.ID, subreddit string) (Subscription, error)
	HasSubscription(guildID snowflake.ID, subreddit string) (bool, error)
	AddSubscription(guildID snowflake.ID, subreddit string, webhookID snowflake.ID, webhookToken string) error
	RemoveSubscription(guildID snowflake.ID, subreddit string) error
	RemoveAllSubscriptions(subreddit string) error
}
