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

const RedditColor = 0xff581a

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

func (b *Bot) RemoveSubscription(webhookID snowflake.ID, webhookToken string, err error) error {
	if err != nil {
		_, _ = b.Client.Rest().CreateWebhookMessage(webhookID, webhookToken, discord.WebhookMessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "Error",
					Timestamp:   json.Ptr(time.Now()),
					Color:       RedditColor,
					Description: fmt.Sprintf("An error occurred while trying to get posts from this subreddit: %s\nRemoving this webhook" + err.Error()),
				},
			},
		}, false, 0)
	}

	_ = b.Client.Rest().DeleteWebhookWithToken(webhookID, webhookToken, rest.WithReason("Removing webhook because of error: "+err.Error()))

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

func (b *Bot) waitTime() time.Duration {
	return time.Minute / time.Duration(b.Cfg.Reddit.RequestsPerMinute)
}

func (b *Bot) ListenSubreddits() {
	for {
		now := time.Now()
		subscriptions, err := b.DB.GetAllSubscriptionIDs()
		if err != nil {
			log.Error("error getting subscriptions:", err.Error())
			continue
		}
		log.Debugf("checking subreddits for %d subscriptions", len(subscriptions))

		for i := range subscriptions {
			subNow := time.Now()
			sub, err := b.DB.GetSubscription(subscriptions[i])
			if err == ErrSubscriptionNotFound {
				continue
			} else if err != nil {
				log.Errorf("error checking subscription for webhook %s: %s", subscriptions[i], err.Error())
				continue
			}

			b.checkSubscription(*sub)

			waitTime := b.waitTime() - time.Now().Sub(subNow)
			if waitTime > 0 {
				<-time.After(waitTime)
			}
		}

		duration := time.Now().Sub(now)
		if duration > time.Duration(len(subscriptions))*b.waitTime() {
			log.Debugf("took %s too long to check %d subreddits", duration.String(), len(subscriptions))
		}
	}
}

func (b *Bot) checkSubscription(sub Subscription) {
	if sub.SubredditIcon == "" {
		icon, err := b.Reddit.GetSubredditIcon(sub.Subreddit)
		if err != nil {
			if !errors.Is(err, ErrSubredditNotFound) && !errors.Is(err, ErrSubredditForbidden) {
				log.Errorf("error getting subreddit icon for subreddit %s: %s", sub.Subreddit, err.Error())
			}
		} else {
			if err = b.DB.UpdateSubscriptionSubredditIcon(sub.WebhookID, icon); err != nil {
				log.Errorf("error updating subreddit icon for webhook %s: %s", sub.WebhookID, err.Error())
			}
			sub.SubredditIcon = icon
		}
	}
	posts, before, err := b.Reddit.GetPosts(sub.Subreddit, sub.Type, sub.LastPost)
	if err != nil {
		log.Errorf("error getting posts for subreddit %s: %s", sub.Subreddit, err.Error())
		if errors.Is(err, ErrSubredditNotFound) || errors.Is(err, ErrSubredditForbidden) {
			if err = b.RemoveSubscription(sub.WebhookID, sub.WebhookToken, err); err != nil {
				log.Errorf("error removing sub for webhook %s: %s", sub.WebhookID, err.Error())
			}
		}
		return
	}
	log.Debugf("got %d posts for subreddit %s before: %s\n", len(posts), sub.Subreddit, sub.LastPost)

	if sub.LastPost != "" {
		for i := len(posts) - 1; i >= 0; i-- {
			b.sendPost(sub, posts[i])
		}
	}
	if before != "" {
		if err = b.DB.UpdateSubscriptionLastPost(sub.WebhookID, before); err != nil {
			log.Errorf("error updating last post for webhook %s: %s", sub.WebhookID, err.Error())
		}
	}
}

func (b *Bot) sendPost(sub Subscription, post RedditPost) {
	var webhookMessageCreate discord.WebhookMessageCreate
	switch sub.FormatType {
	case FormatTypeEmbed:
		embed := discord.Embed{
			Title:       cutString(post.Title, 256),
			Description: cutString(html.UnescapeString(post.Selftext), 4069),
			URL:         "https://reddit.com" + post.Permalink,
			Timestamp:   json.Ptr(time.Unix(int64(post.CreatedUtc), 0)),
			Color:       RedditColor,
			Author: &discord.EmbedAuthor{
				Name:    fmt.Sprintf("%s post in %s", strings.Title(sub.Type), post.SubredditNamePrefixed),
				URL:     "https://reddit.com/" + post.SubredditNamePrefixed,
				IconURL: sub.SubredditIcon,
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

		webhookMessageCreate = discord.WebhookMessageCreate{
			Embeds: []discord.Embed{embed},
		}
	case FormatTypeText:
		webhookMessageCreate = discord.WebhookMessageCreate{
			Content: fmt.Sprintf("## [%s](https://reddit.com%s)\n%s", post.Title, post.Permalink, cutString(quoteString(html.UnescapeString(post.Selftext)), 4000)),
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

	if _, err := b.Client.Rest().CreateWebhookMessage(sub.WebhookID, sub.WebhookToken, webhookMessageCreate, false, 0); err != nil {
		var restError rest.Error
		if errors.Is(err, &restError) && restError.Response.StatusCode == http.StatusNotFound {
			if err = b.RemoveSubscription(sub.WebhookID, sub.WebhookToken, nil); err != nil {
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

func quoteString(str string) string {
	return "> " + strings.ReplaceAll(str, "\n", "\n> ")
}
