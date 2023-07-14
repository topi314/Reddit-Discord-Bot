package redditbot

import (
	"database/sql"
	"errors"

	"github.com/disgoorg/snowflake/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

type Subscription struct {
	Subreddit    string       `db:"subreddit"`
	GuildID      snowflake.ID `db:"guild_id"`
	ChannelID    snowflake.ID `db:"channel_id"`
	WebhookID    snowflake.ID `db:"webhook_id"`
	WebhookToken string       `db:"webhook_token"`
}

func NewDB(cfg DatabaseConfig, schema string) (*DB, error) {
	var (
		driverName     string
		dataSourceName string
	)
	switch cfg.Type {
	case DatabaseTypePostgres:
		driverName = "pgx"
		dataSourceName = cfg.PostgresConfig.DataSourceName()
	case DatabaseTypeSQLite:
		driverName = "sqlite"
		dataSourceName = cfg.SQLite.DataSourceName()
	}

	dbx, err := sqlx.Connect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	// apply schema
	_, err = dbx.Exec(schema)
	if err != nil {
		return nil, err
	}

	return &DB{dbx}, nil
}

type DB struct {
	dbx *sqlx.DB
}

func (d *DB) Close() error {
	return d.dbx.Close()
}

func (d *DB) AddSubscription(sub Subscription) error {
	_, err := d.dbx.NamedExec(`INSERT INTO subscriptions (subreddit, guild_id, channel_id, webhook_id, webhook_token) VALUES (:subreddit, :guild_id, :channel_id, :webhook_id, :webhook_token)`, sub)
	return err
}

func (d *DB) RemoveSubscription(webhookID snowflake.ID) (*Subscription, error) {
	var sub Subscription
	if err := d.dbx.Get(&sub, `DELETE FROM subscriptions WHERE webhook_id = $1 RETURNING *`, webhookID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	return &sub, nil
}

func (d *DB) RemoveSubscriptionByGuildSubreddit(guildID snowflake.ID, subreddit string) (*Subscription, error) {
	var sub Subscription
	if err := d.dbx.Get(&sub, `DELETE FROM subscriptions WHERE guild_id = $1 AND subreddit = $2 RETURNING *`, guildID, subreddit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}

	return &sub, nil
}

func (d *DB) GetAllSubscriptions() ([]Subscription, error) {
	var subs []Subscription
	err := d.dbx.Select(&subs, `SELECT * FROM subscriptions`)
	return subs, err
}

func (d *DB) HasSubscription(webhookID snowflake.ID) (bool, error) {
	var count int
	err := d.dbx.Get(&count, `SELECT COUNT(*) FROM subscriptions WHERE webhook_id = $1`, webhookID)
	return count > 0, err
}

func (d *DB) GetSubscriptionsByGuild(guildID snowflake.ID) ([]Subscription, error) {
	var subs []Subscription
	err := d.dbx.Select(&subs, `SELECT * FROM subscriptions WHERE guild_id = $1`, guildID)
	return subs, err
}

func (d *DB) GetSubscriptionsByChannel(channelID snowflake.ID) ([]Subscription, error) {
	var subs []Subscription
	err := d.dbx.Select(&subs, `SELECT * FROM subscriptions WHERE channel_id = $1`, channelID)
	return subs, err
}
