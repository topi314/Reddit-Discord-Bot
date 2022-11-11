package main

import (
	"context"
	"database/sql"
	"os"
	"strconv"

	"github.com/disgoorg/snowflake/v2"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

var (
	devMode, _            = strconv.ParseBool(os.Getenv("dev_mode"))
	shouldSyncDBTables, _ = strconv.ParseBool(os.Getenv("should_sync_db"))

	dbUser     = os.Getenv("db_user")
	dbPassword = os.Getenv("db_password")
	dbAddress  = os.Getenv("db_address")
	dbName     = os.Getenv("db_name")
)

func (b *RedditBot) SetupDB() error {
	sqlDB := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithAddr(dbAddress),
		pgdriver.WithUser(dbUser),
		pgdriver.WithPassword(dbPassword),
		pgdriver.WithDatabase(dbName),
		pgdriver.WithInsecure(true),
	))
	b.DB = bun.NewDB(sqlDB, pgdialect.New())
	b.DB.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(devMode)))
	if shouldSyncDBTables {
		if err := b.DB.ResetModel(context.TODO(), (*Subscription)(nil)); err != nil {
			return err
		}
	}
	return nil
}

type Subscription struct {
	Subreddit    string       `bun:"subreddit,pk"`
	GuildID      snowflake.ID `bun:"guild_id,pk"`
	ChannelID    snowflake.ID `bun:"channel_id,pk"`
	WebhookID    snowflake.ID `bun:"webhook_id,notnull"`
	WebhookToken string       `bun:"webhook_token,notnull"`
}
