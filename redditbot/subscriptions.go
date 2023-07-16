package redditbot

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/json"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// RequestsPerMinute is the number of requests per minute we can make to the reddit API. This is 600 requests per 10 minutes.
	RequestsPerMinute = 590 / 10

	// WaitTime is the time we wait between requests to the reddit API to avoid hitting the ratelimit.
	WaitTime = time.Minute / RequestsPerMinute
)

var (
	ErrSubredditNotFound  = errors.New("subreddit not found")
	ErrSubredditForbidden = errors.New("subreddit forbidden")
)

var imageRegex = regexp.MustCompile(`https://.*\.(?:jpg|gif|png)`)

func (b *Bot) AddSubscription(sub Subscription) error {
	if err := b.DB.AddSubscription(sub); err != nil {
		return err
	}

	subreddits.With(prometheus.Labels{
		"subreddit":  sub.Subreddit,
		"type":       sub.Type,
		"webhook_id": strconv.FormatInt(int64(sub.WebhookID), 10),
		"guild_id":   strconv.FormatInt(int64(sub.GuildID), 10),
		"channel_id": strconv.FormatInt(int64(sub.ChannelID), 10),
	}).Inc()

	return nil
}

func (b *Bot) RemoveSubscription(webhookID snowflake.ID) error {
	sub, err := b.DB.RemoveSubscription(webhookID)
	if err != nil {
		return err
	}

	subreddits.With(prometheus.Labels{
		"subreddit":  sub.Subreddit,
		"type":       sub.Type,
		"webhook_id": strconv.FormatInt(int64(sub.WebhookID), 10),
		"guild_id":   strconv.FormatInt(int64(sub.GuildID), 10),
		"channel_id": strconv.FormatInt(int64(sub.ChannelID), 10),
	}).Dec()

	return nil
}

func (b *Bot) RemoveSubscriptionByGuildSubreddit(guildID snowflake.ID, subreddit string, reason string) error {
	sub, err := b.DB.RemoveSubscriptionByGuildSubreddit(guildID, subreddit)
	if err != nil {
		return err
	}

	_ = b.Client.Rest().DeleteWebhookWithToken(sub.WebhookID, sub.WebhookToken, rest.WithReason(reason))

	subreddits.With(prometheus.Labels{
		"subreddit":  sub.Subreddit,
		"type":       sub.Type,
		"webhook_id": strconv.FormatInt(int64(sub.WebhookID), 10),
		"guild_id":   strconv.FormatInt(int64(sub.GuildID), 10),
		"channel_id": strconv.FormatInt(int64(sub.ChannelID), 10),
	}).Dec()

	return nil
}

func (b *Bot) ListenSubreddits() {
	for {
		now := time.Now()
		subscriptions, err := b.DB.GetAllSubscriptions()
		if err != nil {
			log.Error("error getting subscriptions:", err.Error())
			continue
		}
		log.Debugf("checking subreddits for %d subscriptions", len(subscriptions))

		for i := range subscriptions {
			subNow := time.Now()
			ok, err := b.DB.HasSubscription(subscriptions[i].WebhookID)
			if err != nil {
				log.Errorf("error checking subscription for webhook %s: %s", subscriptions[i].WebhookID, err.Error())
				continue
			}
			if !ok {
				continue
			}

			b.checkSubreddit(subscriptions[i])

			waitTime := WaitTime - time.Now().Sub(subNow)
			if waitTime > 0 {
				<-time.After(WaitTime)
			}
		}

		duration := time.Now().Sub(now)
		if duration > time.Duration(len(subscriptions))*WaitTime {
			log.Debugf("took %s too long to check %d subreddits", duration.String(), len(subscriptions))
		}
	}
}

func (b *Bot) checkSubreddit(sub Subscription) {
	lastPost := b.LastPosts[sub.WebhookID]
	posts, before, err := b.Reddit.GetPosts(b.Reddit, sub.Subreddit, sub.Type, lastPost)
	if err != nil {
		log.Errorf("error getting posts for subreddit %s: %s", sub.Subreddit, err.Error())
		if errors.Is(err, ErrSubredditNotFound) || errors.Is(err, ErrSubredditForbidden) {
			if err = b.RemoveSubscription(sub.WebhookID); err != nil {
				log.Errorf("error removing sub for webhook %s: %s", sub.WebhookID, err.Error())
			}
		}
		return
	}
	log.Debugf("got %d posts for subreddit %s before: %v\n", len(posts), sub.Subreddit, before)

	if lastPost != "" {
		for i := len(posts) - 1; i >= 0; i-- {
			b.sendPost(sub, posts[i])
		}
	}
	if before != "" {
		b.LastPosts[sub.WebhookID] = before
	}
}

func (b *Bot) sendPost(sub Subscription, post RedditPost) {
	embed := discord.Embed{
		Title:       cutString(post.Title, 256),
		Description: cutString(html.UnescapeString(post.Selftext), 4069),
		URL:         "https://reddit.com" + post.Permalink,
		Timestamp:   json.Ptr(time.Unix(int64(post.CreatedUtc), 0)),
		Color:       0xff581a,
		Author: &discord.EmbedAuthor{
			Name:    fmt.Sprintf("%s post in %s", strings.Title(sub.Type), post.SubredditNamePrefixed),
			URL:     "https://reddit.com/" + post.SubredditNamePrefixed,
			IconURL: "https://www.redditstatic.com/desktop2x/img/favicon/android-icon-192x192.png",
		},
		Footer: &discord.EmbedFooter{
			Text: "posted by " + post.Author,
		},
	}
	if imageRegex.MatchString(post.URL) {
		embed.Image = &discord.EmbedResource{
			URL: post.URL,
		}
	}

	postsSent.With(prometheus.Labels{
		"subreddit":  sub.Subreddit,
		"type":       sub.Type,
		"webhook_id": strconv.FormatUint(uint64(sub.WebhookID), 10),
		"guild_id":   strconv.FormatUint(uint64(sub.GuildID), 10),
		"channel_id": strconv.FormatUint(uint64(sub.ChannelID), 10),
	}).Inc()

	if b.Cfg.TestMode {
		log.Debugf("sending post to webhook %d: %s", sub.WebhookID, post.Title)
		return
	}

	if _, err := b.Client.Rest().CreateWebhookMessage(sub.WebhookID, sub.WebhookToken, discord.WebhookMessageCreate{Embeds: []discord.Embed{embed}}, false, 0); err != nil {
		var restError rest.Error
		if errors.Is(err, &restError) && restError.Response.StatusCode == http.StatusNotFound {
			if err = b.RemoveSubscription(sub.WebhookID); err != nil {
				log.Errorf("error removing sub for webhook %s: %s", sub.WebhookID, err.Error())
			}
			return
		}
		log.Errorf("error sending post to webhook %d: %s", sub.WebhookID, err.Error())
	}
}

func cutString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) > maxLen {
		return string(runes[0:maxLen-1]) + "…"
	}
	return string(runes)
}
