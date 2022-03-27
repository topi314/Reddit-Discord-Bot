package main

import (
	"context"
	"net/http"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/snowflake"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

func (b *RedditBot) subscribeToSubreddit(subreddit string, webhookClient webhook.Client) {
	b.Logger.Debugf("subscribing to r/%s", subreddit)
	_, ok := b.Subreddits[subreddit]
	if !ok {
		b.Subreddits[subreddit] = []webhook.Client{}
	}
	b.Subreddits[subreddit] = append(b.Subreddits[subreddit], webhookClient)

	_, ok = b.SubredditCancelFuncs[subreddit]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		b.SubredditCancelFuncs[subreddit] = cancel
		go b.listenToSubreddit(subreddit, ctx)
	}
}

func (b *RedditBot) unsubscribeFromSubreddit(subreddit string, webhookID snowflake.Snowflake, deleteWebhook bool) {
	b.Logger.Debugf("unsubscribing from r/%s", subreddit)
	_, ok := b.Subreddits[subreddit]
	if !ok {
		return
	}
	for i, wc := range b.Subreddits[subreddit] {
		if wc.ID() == webhookID {
			b.Subreddits[subreddit] = append(b.Subreddits[subreddit][:i], b.Subreddits[subreddit][i+1:]...)
			if deleteWebhook {
				err := wc.DeleteWebhook()
				if err != nil {
					b.Logger.Error("error while deleting webhook: ", err)
				}
			}
			if _, err := b.DB.NewDelete().Model((*Subscription)(nil)).Where("webhook_id = ?", webhookID).Exec(context.TODO()); err != nil {
				b.Logger.Error("error while deleting subscription: ", err)
			}
			if len(b.Subreddits[subreddit]) == 0 {
				delete(b.Subreddits, subreddit)
				b.SubredditCancelFuncs[subreddit]()
			}
			return
		}
	}
	b.Logger.Warnf("could not find webhook `%s` to remove", webhookID)
}

func (b *RedditBot) listenToSubreddit(subreddit string, ctx context.Context) {
	b.Logger.Debugf("listening to r/%s", subreddit)
	posts, errs, closer := b.RedditClient.Stream.Posts(subreddit, reddit.StreamInterval(time.Second*30), reddit.StreamDiscardInitial)
	for {
		select {
		case <-ctx.Done():
			closer()
			b.Logger.Debugf("stop listening to r/%s", subreddit)
			return

		case post := <-posts:
			description := post.Body
			if len(description) > 4096 {
				description = string([]rune(description)[0:4093]) + "..."
			}

			url := post.URL
			if !imageRegex.MatchString(url) {
				url = ""
			}
			title := post.Title
			if len(title) > 256 {
				title = string([]rune(title)[0:253]) + "..."
			}

			webhookMessageCreate := discord.WebhookMessageCreate{
				Embeds: []discord.Embed{discord.NewEmbedBuilder().
					SetTitle(title).
					SetURL("https://www.reddit.com"+post.Permalink).
					SetColor(0xff581a).
					SetAuthorName("New post on "+post.SubredditNamePrefixed).
					SetAuthorURL("https://www.reddit.com/"+post.SubredditNamePrefixed).
					SetDescription(description).
					SetImage(url).
					AddField("Author", post.Author, false).
					Build(),
				},
			}

			for _, webhookClient := range b.Subreddits[subreddit] {
				_, err := webhookClient.CreateMessage(webhookMessageCreate)
				if e, ok := err.(*rest.Error); ok {
					if e.Response.StatusCode == http.StatusNotFound {
						b.Logger.Warnf("webhook `%s` not found, removing it", webhookClient.ID)
						b.unsubscribeFromSubreddit(subreddit, webhookClient.ID(), true)
						continue
					}
					b.Logger.Errorf("error while sending post to webhook: %s, body: %s", err, string(e.RsBody))
				} else if err != nil {
					b.Logger.Error("error while sending post to webhook: ", err)
				} else {
					b.Logger.Debugf("sent post to webhook `%s`", webhookClient.ID())
				}
			}

		case err := <-errs:
			b.Logger.Error("received error from reddit post stream: ", err)
		}
	}
}

func (b *RedditBot) loadAllSubreddits() error {
	var subscriptions []Subscription
	if err := b.DB.NewSelect().Model(&subscriptions).Scan(context.TODO()); err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		webhookClient := webhook.NewClient(subscription.WebhookID, subscription.WebhookToken,
			webhook.WithRestClientConfigOpts(rest.WithHTTPClient(b.HTTPClient)),
			webhook.WithLogger(b.Logger),
		)
		b.subscribeToSubreddit(subscription.Subreddit, webhookClient)
	}
	return nil
}
