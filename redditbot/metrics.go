package redditbot

import (
	"go.opentelemetry.io/otel/metric"
)

func (b *Bot) InitMetrics() error {
	if !b.Cfg.Otel.Enabled {
		return nil
	}

	var err error
	if b.PostsSentGauge, err = b.Meter.Int64Counter("redditbot_posts_sent",
		metric.WithDescription("The number of posts sent to Discord"),
	); err != nil {
		return err
	}

	if b.SubredditsCounter, err = b.Meter.Int64UpDownCounter("redditbot_subreddits",
		metric.WithDescription("The number of subreddits being checked"),
	); err != nil {
		return err
	}

	if b.Reddit.counter, err = b.Meter.Int64Counter("redditbot_reddit_requests",
		metric.WithDescription("The number of requests made to the Reddit API"),
	); err != nil {
		return err
	}

	return nil
}
