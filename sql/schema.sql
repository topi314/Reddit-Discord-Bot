CREATE TABLE IF NOT EXISTS subscriptions
(
	subreddit     VARCHAR NOT NULL,
	type          VARCHAR NOT NULL DEFAULT 'new',
	guild_id      BIGINT  NOT NULL,
	channel_id    BIGINT  NOT NULL,
	webhook_id    BIGINT  NOT NULL,
	webhook_token VARCHAR NOT NULL,
	PRIMARY KEY (subreddit, guild_id)
)
