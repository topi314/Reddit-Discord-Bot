package main

import (
	"context"
	"net/http"
	"sync"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/snowflake/v2"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const MaxRetries = 10

func (b *RedditBot) subscribeToSubreddit(subreddit string, webhookClient webhook.Client) {
	b.Logger.Debugf("subscribing to r/%s", subreddit)
	b.SubredditsMu.Lock()
	defer b.SubredditsMu.Unlock()
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

func (b *RedditBot) unsubscribeFromSubreddit(subreddit string, webhookID snowflake.ID) {
	b.Logger.Debugf("unsubscribing from r/%s", subreddit)
	b.SubredditsMu.Lock()
	defer b.SubredditsMu.Unlock()
	_, ok := b.Subreddits[subreddit]
	if !ok {
		return
	}
	for i, webhookClient := range b.Subreddits[subreddit] {
		if webhookClient.ID() == webhookID {
			b.Subreddits[subreddit] = append(b.Subreddits[subreddit][:i], b.Subreddits[subreddit][i+1:]...)
			err := webhookClient.DeleteWebhook()
			if err != nil {
				b.Logger.Error("error while deleting webhook: ", err)
			}

			if _, err = b.DB.NewDelete().Model((*Subscription)(nil)).Where("webhook_id = ?", webhookID).Exec(context.TODO()); err != nil {
				b.Logger.Error("error while deleting subscription: ", err)
			}

			// if there are no more webhooks for this subreddit, cancel the subscription
			if len(b.Subreddits[subreddit]) == 0 {
				delete(b.Subreddits, subreddit)
				// cancel the listen goroutine
				b.SubredditCancelFuncs[subreddit]()
			}
			return
		}
	}
	b.Logger.Infof("could not find webhook `%s` to remove", webhookID)
}

func (b *RedditBot) listenToSubreddit(subreddit string, ctx context.Context) {
	b.Logger.Debugf("listening to r/%s", subreddit)
	posts, errs, closer := b.RedditClient.Stream.Posts(subreddit, reddit.StreamInterval(streamInterval), reddit.StreamDiscardInitial)
	defer closer()
	defer b.Logger.Debugf("stop listening to r/%s", subreddit)
	for {
		select {
		case <-ctx.Done():
			return
		case post := <-posts:
			b.processPost(post, subreddit)
		case err := <-errs:
			b.processError(err, subreddit)
		}
	}
}

func (b *RedditBot) processPost(post *reddit.Post, subreddit string) {
	title := cutString(post.Title, 256)
	description := cutString(post.Body, 4096)

	url := post.URL
	if !imageRegex.MatchString(url) {
		url = ""
	}

	webhookMessageCreate := discord.WebhookMessageCreate{
		Embeds: []discord.Embed{discord.NewEmbedBuilder().
			SetTitle(title).
			SetURL("https://www.reddit.com"+post.Permalink).
			SetColor(EmbedColor).
			SetAuthorName("New post on "+post.SubredditNamePrefixed).
			SetAuthorURL("https://www.reddit.com/"+post.SubredditNamePrefixed).
			SetDescription(description).
			SetImage(url).
			AddField("Author", post.Author, false).
			Build(),
		},
	}

	b.SubredditsMu.RLock()
	defer b.SubredditsMu.RUnlock()
	var wg sync.WaitGroup
	for i := range b.Subreddits[subreddit] {
		webhookClient := b.Subreddits[subreddit][i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.sendPostToWebhook(webhookClient, webhookMessageCreate, subreddit)
		}()
	}
	wg.Wait()
}

func (b *RedditBot) sendPostToWebhook(webhookClient webhook.Client, messageCreate discord.WebhookMessageCreate, subreddit string) {
	_, err := webhookClient.CreateMessage(messageCreate)
	if err != nil {
		if e, ok := err.(*rest.Error); ok && e.Response.StatusCode == http.StatusNotFound {
			b.Logger.Warnf("webhook `%s` not found, removing it", webhookClient.ID())
			go b.unsubscribeFromSubreddit(subreddit, webhookClient.ID())
			return
		}
		b.Logger.Error("error while sending post to webhook: ", err)
		return
	}
	b.Logger.Debugf("sent post to webhook `%s`", webhookClient.ID())
}

func (b *RedditBot) processError(err error, subreddit string) {
	if redditErr, ok := err.(*reddit.ErrorResponse); ok && redditErr.Response != nil && redditErr.Response.StatusCode == http.StatusNotFound {
		b.Logger.Infof("subreddit `%s` not found, removing it", subreddit)
		b.SubredditsMu.RLock()
		defer b.SubredditsMu.RUnlock()
		for _, webhookClient := range b.Subreddits[subreddit] {
			if _, rErr := webhookClient.CreateMessage(discord.WebhookMessageCreate{
				Embeds: []discord.Embed{discord.NewEmbedBuilder().
					SetDescriptionf("The subreddit `%s` you were subscribed to seems to no longer exists.\nDeleting the webhook.", subreddit).
					SetColor(EmbedColor).
					Build(),
				},
			}); rErr != nil {
				b.Logger.Error("error while sending delete to webhook: ", rErr)
			}
			go b.unsubscribeFromSubreddit(subreddit, webhookClient.ID())
		}
		return
	}
	b.Logger.Error("received error from reddit post stream: ", err)
}

func (b *RedditBot) loadAllSubreddits() error {
	var subscriptions []Subscription
	if err := b.DB.NewSelect().Model(&subscriptions).Scan(context.TODO()); err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		webhookClient := webhook.New(subscription.WebhookID, subscription.WebhookToken,
			webhook.WithRestClient(b.RestClient),
			webhook.WithLogger(b.Logger),
		)
		b.subscribeToSubreddit(subscription.Subreddit, webhookClient)
	}
	return nil
}

func cutString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) > maxLen {
		return string(runes[0:maxLen-1]) + "…"
	}
	return string(runes)
}
