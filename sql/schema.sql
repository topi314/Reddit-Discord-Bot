CREATE TABLE IF NOT EXISTS subscriptions
(
	subreddit     VARCHAR   NOT NULL,
	type          VARCHAR   NOT NULL DEFAULT 'new',
	format_type   VARCHAR   NOT NULL DEFAULT 'embed',
	guild_id      BIGINT    NOT NULL,
	channel_id    BIGINT    NOT NULL,
	webhook_id    BIGINT    NOT NULL,
	webhook_token VARCHAR   NOT NULL,
	last_post     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	role_id       BIGINT    NOT NULL DEFAULT 0,
	reddit_proxy  VARCHAR   NOT NULL DEFAULT '',
	PRIMARY KEY (subreddit, guild_id)
)
