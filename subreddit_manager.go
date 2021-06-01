package main

import (
	"github.com/DisgoOrg/disgohook"
	wapi "github.com/DisgoOrg/disgohook/api"
)

var subreddits = map[string][]wapi.WebhookClient{}
var subredditChannels = map[string]chan struct{}{}

func subscribeToSubreddit(subreddit string, webhookClient wapi.WebhookClient) {
	logger.Infof("subcribing to r/%s", subreddit)
	_, ok := subreddits[subreddit]
	if !ok {
		subreddits[subreddit] = []wapi.WebhookClient{}
	}
	subreddits[subreddit] = append(subreddits[subreddit], webhookClient)

	_, ok = subredditChannels[subreddit]
	if !ok {
		quit := make(chan struct{})
		subredditChannels[subreddit] = quit
		go listenToSubreddit(subreddit, quit)
	}
}

func unsubscribeFromSubreddit(subreddit string, webhookClient wapi.WebhookClient) {
	logger.Infof("unsubcribing from r/%s", subreddit)
	_, ok := subreddits[subreddit]
	if !ok {
		return
	}
	for i, wc := range subreddits[subreddit] {
		if wc == webhookClient {
			subreddits[subreddit] = append(subreddits[subreddit][:i], subreddits[subreddit][i+1:]...)
			if len(subreddits[subreddit]) == 0 {
				delete(subreddits, subreddit)
				subredditChannels[subreddit] <- struct{}{}
			}
		}
	}
}

func listenToSubreddit(subreddit string, quit chan struct{}) {
	logger.Infof("listeninmg to r/%s", subreddit)
	postCount := 0
	posts, errs, _ := redditClient.Stream.Posts(subreddit)
	for {
		select {
		case <-quit:
			logger.Infof("shutting down reddit post stream...")
			return
		case post := <-posts:
			postCount++
			if postCount <= 100 {
				continue
			}
			description := post.Body
			if len(description) > 2048 {
				description = string([]rune(description)[0:2045]) + "..."
			}

			url := post.URL
			if !imageRegex.MatchString(url) {
				url = ""
			}

			embed := wapi.NewEmbedBuilder().
				SetTitle(post.Title).
				SetURL("https://www.reddit.com"+post.Permalink).
				SetColor(0xff581a).
				SetAuthorName("New post on "+post.SubredditNamePrefixed).
				SetAuthorURL("https://www.reddit.com/"+post.SubredditNamePrefixed).
				SetDescription(description).
				SetImage(url).
				AddField("Author", post.Author, false).
				Build()

			for _, webhookClient := range subreddits[subreddit] {
				_, err := webhookClient.SendEmbed(embed)
				if err != nil {
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
		webhookClient, err := disgohook.NewWebhookClientByIDToken(httpClient, logger, subredditSubscription.WebhookID, subredditSubscription.WebhookToken)
		if err != nil {
			logger.Errorf("error creating webhook client: %s", err)
			continue
		}
		subscribeToSubreddit(subredditSubscription.Subreddit, webhookClient)
	}
}
