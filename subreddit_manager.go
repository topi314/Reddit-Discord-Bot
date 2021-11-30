package main

import (
	"net/http"
	"time"

	"github.com/DisgoOrg/disgo/discord"
	"github.com/DisgoOrg/disgo/rest"
	"github.com/DisgoOrg/disgo/webhook"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

var subreddits = map[string][]*webhook.Client{}
var subredditChannels = map[string]chan struct{}{}

func subscribeToSubreddit(subreddit string, webhookClient *webhook.Client) {
	logger.Debugf("subscribing to r/%s", subreddit)
	_, ok := subreddits[subreddit]
	if !ok {
		subreddits[subreddit] = []*webhook.Client{}
	}
	subreddits[subreddit] = append(subreddits[subreddit], webhookClient)

	_, ok = subredditChannels[subreddit]
	if !ok {
		quit := make(chan struct{})
		subredditChannels[subreddit] = quit
		go listenToSubreddit(subreddit, quit)
	}
}

func unsubscribeFromSubreddit(subreddit string, webhookID discord.Snowflake, deleteWebhook bool) {
	logger.Debugf("unsubcribing from r/%s", subreddit)
	_, ok := subreddits[subreddit]
	if !ok {
		return
	}
	for i, wc := range subreddits[subreddit] {
		if wc.ID == webhookID {
			subreddits[subreddit] = append(subreddits[subreddit][:i], subreddits[subreddit][i+1:]...)
			if deleteWebhook {
				err := wc.DeleteWebhook()
				if err != nil {
					logger.Errorf("error while deleting wehook: %s", err)
				}
			}
			database.Delete(&SubredditSubscription{}, "webhook_id = ?", webhookID)
			if len(subreddits[subreddit]) == 0 {
				delete(subreddits, subreddit)
				subredditChannels[subreddit] <- struct{}{}
			}
			return
		}
	}
	logger.Warnf("could not find webhook `%s` to remove", webhookID)
}

func listenToSubreddit(subreddit string, quit chan struct{}) {
	logger.Debugf("listening to r/%s", subreddit)
	posts, errs, closer := redditClient.Stream.Posts(subreddit, reddit.StreamInterval(time.Minute*2), reddit.StreamDiscardInitial)
	for {
		select {
		case <-quit:
			closer()
			logger.Debugf("stop listening to r/%s", subreddit)
			return

		case post := <-posts:
			description := post.Body
			if len(description) > 2048 {
				description = string([]rune(description)[0:2045]) + "..."
			}

			url := post.URL
			if !imageRegex.MatchString(url) {
				url = ""
			}

			webhookMessageCreate := discord.WebhookMessageCreate{
				Embeds: []discord.Embed{discord.NewEmbedBuilder().
					SetTitle(post.Title).
					SetURL("https://www.reddit.com"+post.Permalink).
					SetColor(0xff581a).
					SetAuthorName("New post on "+post.SubredditNamePrefixed).
					SetAuthorURL("https://www.reddit.com/"+post.SubredditNamePrefixed).
					SetDescription(description).
					SetImage(url).
					AddField("Author", post.Author, false).
					Build()},
			}

			for _, webhookClient := range subreddits[subreddit] {
				_, err := webhookClient.CreateMessage(webhookMessageCreate)
				if err != nil {
					if e, ok := err.(*rest.Error); ok {
						if e.Code == http.StatusNotFound {
							logger.Warnf("webhook `%s` not found, removing it", webhookClient.ID)
							unsubscribeFromSubreddit(subreddit, webhookClient.ID, true)
							continue
						}
						if e.RsBody != nil {
							logger.Errorf("error while sending post to webhook: %s, body: %s", err, string(e.RsBody))
							continue
						}
					}
					logger.Errorf("error while sending post to webhook: %s", err)
				}

			}

		case err := <-errs:
			logger.Errorf("received error from reddit post stream: %s", err)
		}
	}
}

func loadAllSubreddits() {
	var subredditSubscriptions []*SubredditSubscription
	_ = database.Find(&subredditSubscriptions)
	for _, subredditSubscription := range subredditSubscriptions {
		webhookClient := webhook.NewClient(subredditSubscription.WebhookID, subredditSubscription.WebhookToken,
			webhook.WithRestClientConfigOpts(rest.WithHTTPClient(httpClient)),
			webhook.WithLogger(logger),
		)
		subscribeToSubreddit(subredditSubscription.Subreddit, webhookClient)
	}
}
