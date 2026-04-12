package redditbot

import (
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	redditColor        = 0xff581a
	defaultRedditProxy = "https://reddit.com"
)

var (
	ErrSubredditNotFound  = errors.New("subreddit not found")
	ErrSubredditForbidden = errors.New("subreddit forbidden")
)

var imageRegex = regexp.MustCompile(`https://.*\.(?:jpg|jpeg|gif|png)`)

var errUnknownFormatType = errors.New("unknown format type")

func (b *Bot) AddSubscription(sub Subscription) error {
	if err := b.db.AddSubscription(sub); err != nil {
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
		_, _ = b.Client.Rest.CreateWebhookMessage(webhookID, webhookToken, discord.WebhookMessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "Error",
					Timestamp:   json.Ptr(time.Now()),
					Color:       redditColor,
					Description: fmt.Sprintf("An error occurred while trying to get posts from this subreddit: %s\nRemoving this webhook" + err.Error()),
				},
			},
		}, rest.CreateWebhookMessageParams{})
	}

	errMessage := "unknown error"
	if err != nil {
		errMessage = err.Error()
	}
	_ = b.Client.Rest.DeleteWebhookWithToken(webhookID, webhookToken, rest.WithReason("Removing webhook because of error: "+errMessage))

	sub, err := b.db.RemoveSubscription(webhookID)
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
	sub, err := b.db.RemoveSubscriptionByGuildSubreddit(guildID, subreddit)
	if err != nil {
		return err
	}

	_ = b.Client.Rest.DeleteWebhookWithToken(sub.WebhookID, sub.WebhookToken, rest.WithReason(reason))

	subreddits.With(prometheus.Labels{
		"subreddit":  sub.Subreddit,
		"type":       sub.Type,
		"webhook_id": strconv.FormatInt(int64(sub.WebhookID), 10),
		"guild_id":   strconv.FormatInt(int64(sub.GuildID), 10),
		"channel_id": strconv.FormatInt(int64(sub.ChannelID), 10),
	}).Dec()

	return nil
}

func (b *Bot) targetTime() time.Duration {
	return time.Minute / time.Duration(b.cfg.Reddit.RequestsPerMinute)
}

func (b *Bot) ListenSubreddits() {
	for {
		now := time.Now()
		subscriptions, err := b.db.GetAllSubscriptionIDs()
		if err != nil {
			slog.Error("error getting subscriptions", slog.Any("err", err))
			continue
		}
		slog.Debug("checking subreddits for subscriptions", slog.Int("subscriptions", len(subscriptions)))

		for i := range subscriptions {
			subNow := time.Now()
			sub, err := b.db.GetSubscription(subscriptions[i])
			if errors.Is(err, ErrSubscriptionNotFound) {
				continue
			} else if err != nil {
				slog.Error("error checking subscription for webhook", slog.String("webhook_id", subscriptions[i].String()), slog.Any("err", err))
				continue
			}

			b.checkSubscription(*sub)

			waitTime := b.targetTime() - time.Now().Sub(subNow)
			if waitTime > 0 {
				slog.Debug("waiting before checking next sub", slog.String("wait_time", waitTime.String()))
				<-time.After(waitTime)
			}
		}

		duration := time.Now().Sub(now)
		if duration > time.Duration(len(subscriptions))*b.targetTime() {
			slog.Debug("took too long to check subreddits", slog.String("duration", duration.String()), slog.Int("subscriptions", len(subscriptions)))
		}

		time.Sleep(1 * time.Second)
	}
}

func (b *Bot) checkSubscription(sub Subscription) {
	posts, err := b.reddit.GetPostsUntil(sub.Subreddit, sub.Type, sub.LastPost, b.cfg.Reddit.MaxPages)
	if err != nil {
		slog.Error("error getting posts for subreddit", slog.String("subreddit", sub.Subreddit), slog.Any("err", err))
		if errors.Is(err, ErrSubredditNotFound) || errors.Is(err, ErrSubredditForbidden) {
			if err = b.RemoveSubscription(sub.WebhookID, sub.WebhookToken, err); err != nil {
				slog.Error("error removing sub for webhook", slog.String("webhook_id", sub.WebhookID.String()), slog.Any("err", err))
			}
		}
		return
	}
	slog.Debug("got posts for subreddit before:", slog.String("subreddit", sub.Subreddit), slog.Int("posts", len(posts)), slog.Time("last_post", sub.LastPost))

	for i := len(posts) - 1; i >= 0; i-- {
		if !b.sendPost(sub, posts[i]) {
			return
		}
	}

	if len(posts) > 0 {
		if err = b.db.UpdateSubscriptionLastPost(sub.WebhookID, time.Unix(int64(posts[0].CreatedUtc), 0)); err != nil {
			slog.Error("error updating last post for webhook", slog.String("webhook_id", sub.WebhookID.String()), slog.Any("err", err))
		}
	}
}

func (b *Bot) sendPost(sub Subscription, post RedditPost) bool {
	webhookMessageCreate, err := b.buildPostMessageCreate(sub, post)
	if err != nil {
		slog.Error("unknown format type", slog.String("format_type", string(sub.FormatType)))
		return true
	}

	postsSent.With(prometheus.Labels{
		"subreddit":  sub.Subreddit,
		"type":       sub.Type,
		"webhook_id": strconv.FormatUint(uint64(sub.WebhookID), 10),
		"guild_id":   strconv.FormatUint(uint64(sub.GuildID), 10),
		"channel_id": strconv.FormatUint(uint64(sub.ChannelID), 10),
	}).Inc()

	if b.cfg.TestMode {
		slog.Debug("sending post to webhook", slog.String("webhook_id", sub.WebhookID.String()), slog.Any("post", post))
		return true
	}

	if _, err = b.Client.Rest.CreateWebhookMessage(sub.WebhookID, sub.WebhookToken, webhookMessageCreate, rest.CreateWebhookMessageParams{}); err != nil {
		if restError, ok := errors.AsType[*rest.Error](err); ok && restError.Response.StatusCode == http.StatusNotFound {
			if err = b.RemoveSubscription(sub.WebhookID, sub.WebhookToken, nil); err != nil {
				slog.Error("error removing sub for webhook", slog.String("webhook_id", sub.WebhookID.String()), slog.Any("err", err))
			}
			return false
		}
		slog.Error("error sending post to webhook", slog.String("webhook_id", sub.WebhookID.String()), slog.Any("err", err))
	}

	return true
}

func (b *Bot) buildPostMessageCreate(sub Subscription, post RedditPost) (discord.WebhookMessageCreate, error) {
	proxy := defaultRedditProxy
	if sub.RedditProxy != "" {
		proxy = sub.RedditProxy
	}

	var (
		webhookMessageCreate discord.WebhookMessageCreate
		mentionContent       string
	)
	if sub.RoleID != 0 {
		var allowedMentions discord.AllowedMentions
		if sub.RoleID == sub.GuildID {
			mentionContent = "@everyone"
			allowedMentions = discord.AllowedMentions{
				Parse: []discord.AllowedMentionType{discord.AllowedMentionTypeEveryone},
			}
		} else {
			mentionContent = discord.RoleMention(sub.RoleID)
			allowedMentions = discord.AllowedMentions{
				Roles: []snowflake.ID{sub.RoleID},
			}
		}
		webhookMessageCreate.AllowedMentions = &allowedMentions
	}

	switch sub.FormatType {
	case FormatTypeEmbed:
		embed := discord.Embed{
			Title:       cutString(post.Title, 256),
			Description: cutString(html.UnescapeString(post.Selftext), 4069),
			URL:         "https://reddit.com" + post.Permalink,
			Timestamp:   json.Ptr(time.Unix(int64(post.CreatedUtc), 0)),
			Color:       redditColor,
			Author: &discord.EmbedAuthor{
				Name:    fmt.Sprintf("%s post in %s", cases.Title(language.Und).String(sub.Type), post.SubredditNamePrefixed),
				URL:     "https://reddit.com/" + post.SubredditNamePrefixed,
				IconURL: post.SrDetail.CommunityIcon,
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

		webhookMessageCreate.Embeds = []discord.Embed{embed}
	case FormatTypeText:
		webhookMessageCreate.Content = cutString(fmt.Sprintf("## [%s](%s%s)\n%s", post.Title, proxy, post.Permalink, quoteString(html.UnescapeString(post.Selftext))), 2000)
	case FormatTypeLink:
		webhookMessageCreate.Content = fmt.Sprintf("New [post](%s%s) in [`%s`](<%s>)", proxy, post.Permalink, post.SubredditNamePrefixed, "https://reddit.com/"+post.SubredditNamePrefixed)
	case FormatTypeLinkWithTitle:
		webhookMessageCreate.Content = fmt.Sprintf("[%s](%s%s)", post.Title, proxy, post.Permalink)
	case FormatTypeComponents:
		postURL := proxy + post.Permalink
		subredditURL := "https://reddit.com/" + post.SubredditNamePrefixed
		authorURL := "https://reddit.com/user/" + post.Author

		icon := post.SrDetail.CommunityIcon
		if icon == "" {
			icon = "https://raw.githubusercontent.com/topi314/Reddit-Discord-Bot/refs/heads/v2/reddit.png"
		}

		var containerComponents []discord.ContainerSubComponent
		if mentionContent != "" {
			containerComponents = append(containerComponents, discord.NewTextDisplay(mentionContent))
		}

		containerComponents = append(containerComponents, discord.NewSection(
			discord.NewTextDisplay(fmt.Sprintf("%s post in [%s](%s)", cases.Title(language.Und).String(sub.Type), post.SubredditNamePrefixed, subredditURL)),
			discord.NewTextDisplay(fmt.Sprintf("### [%s](%s)", cutString(post.Title, 256), postURL)),
		).WithAccessory(discord.NewThumbnail(icon)))

		if body := cutString(html.UnescapeString(post.Selftext), 2000); body != "" {
			containerComponents = append(containerComponents, discord.NewTextDisplay(body))
		}

		if imageRegex.MatchString(post.URL) {
			containerComponents = append(containerComponents, discord.NewMediaGallery(discord.MediaGalleryItem{
				Media: discord.UnfurledMediaItem{URL: post.URL},
			}))
		}

		containerComponents = append(containerComponents,
			discord.NewSmallSeparator(),
			discord.NewTextDisplay(fmt.Sprintf("-# posted by [%s](%s) on %s", post.Author, authorURL, time.Unix(int64(post.CreatedUtc), 0).Format("Jan 2 2006 at 15:04:05"))),
		)

		if sub.LinkButton {
			containerComponents = append(containerComponents,
				discord.NewActionRow(discord.NewLinkButton("Open Post", postURL)),
			)
		}

		webhookMessageCreate.Components = []discord.LayoutComponent{
			discord.NewContainer(containerComponents...).WithAccentColor(redditColor),
		}
		webhookMessageCreate.Flags = discord.MessageFlagIsComponentsV2
	default:
		return discord.WebhookMessageCreate{}, fmt.Errorf("%w: %s", errUnknownFormatType, sub.FormatType)
	}

	if sub.FormatType != FormatTypeComponents {
		if mentionContent != "" {
			webhookMessageCreate.Content = cutString(mentionContent+"\n"+webhookMessageCreate.Content, 2000)
		}

		if sub.LinkButton {
			webhookMessageCreate.Components = []discord.LayoutComponent{
				discord.NewActionRow(
					discord.NewLinkButton("Open Post", "https://reddit.com"+post.Permalink),
				),
			}
		}
	}

	return webhookMessageCreate, nil
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
