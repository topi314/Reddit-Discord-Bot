package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var subredditNamePattern = regexp.MustCompile(`\A(r/)?(?P<name>[A-Za-z\d]\w{2,20})$`)

var commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		CommandName: "subreddit",
		Description: "lets you manage all your subreddits",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "add",
				Description: "adds a subreddit",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "subreddit",
						Description: "the subreddit to add",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "remove",
				Description: "removes a subreddit",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "subreddit",
						Description: "the subreddit to remove",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "list",
				Description: "lists all added subreddits",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "info",
				Description: "prints info about myself",
			},
		},
		DefaultPermission: true,
	},
}

func (b *RedditBot) onApplicationCommandInteraction(event *events.ApplicationCommandInteractionEvent) {
	data := event.SlashCommandInteractionData()
	var err error
	if data.CommandName() == "subreddit" {
		if event.Member().Permissions.Missing(discord.PermissionManageServer) {
			err = event.CreateMessage(discord.MessageCreate{
				Content: "You don't have permission to manage this server",
				Flags:   discord.MessageFlagEphemeral,
			})
			return
		}
		switch *data.SubCommandName {
		case "add":
			err = b.onSubredditAdd(event, data)

		case "remove":
			err = b.onSubredditRemove(event, data)

		case "list":
			err = b.onSubredditList(event)

		case "info":
			err = b.onSubredditInfo(event)
		}
	}
	if err != nil {
		b.Logger.Error("error while processing command: ", err)
	}
}

func parseSubredditName(name string) (string, bool) {
	match := subredditNamePattern.FindStringSubmatch(name)
	if len(match) == 0 {
		return "", false
	}
	return strings.ToLower(match[subredditNamePattern.SubexpIndex("name")]), true
}

func (b *RedditBot) onSubredditAdd(event *events.ApplicationCommandInteractionEvent, data discord.SlashCommandInteractionData) error {
	subreddit, ok := parseSubredditName(data.String("subreddit"))
	if !ok {
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("`%s` is not a valid subreddit name.", data.String("subreddit")).
			Build(),
		)
	}

	if _, resp, err := b.RedditClient.Subreddit.Get(context.Background(), subreddit); err != nil {
		return err
	} else if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("could not find `r/%s`.", subreddit).
			Build(),
		)
	}

	var subscription Subscription
	if err := b.DB.NewSelect().Model(&subscription).Where("subreddit = ? AND guild_id = ?", subreddit, *event.GuildID()).Scan(context.TODO()); err == nil {
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("you already added `r/%s` to this server", subreddit).
			Build(),
		)
	}

	url, state := b.OAuth2Client.GenerateAuthorizationURLState(baseURL+CreateCallbackURL, 0, *event.GuildID(), false, discord.ApplicationScopeWebhookIncoming)

	webhookCreateStates[state] = WebhookCreateState{
		Interaction: event.ApplicationCommandInteraction,
		Subreddit:   subreddit,
	}

	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("click [here](%s) to add a new webhook for the subreddit", url).
		Build(),
	)
}

func (b *RedditBot) onSubredditRemove(event *events.ApplicationCommandInteractionEvent, data discord.SlashCommandInteractionData) error {
	subreddit, ok := parseSubredditName(data.String("subreddit"))
	if !ok {
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("`%s` is not a valid subreddit name.", data.String("subreddit")).
			Build(),
		)
	}

	var subscription Subscription
	if err := b.DB.NewSelect().Model(&subscription).Where("subreddit = ? AND guild_id = ?", subreddit, *event.GuildID()).Scan(context.TODO()); err != nil {
		fmt.Printf("ERROR: %s", err)
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("could not find `r/%s` linked to any channels", subreddit).
			Build(),
		)
	}
	b.unsubscribeFromSubreddit(subreddit, subscription.WebhookID)
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("removed `r/%s`", subreddit).
		Build(),
	)
}

func (b *RedditBot) onSubredditList(event *events.ApplicationCommandInteractionEvent) error {
	var subscriptions []*Subscription
	var message string
	if err := b.DB.NewSelect().Model(&subscriptions).Where("guild_id = ?", *event.GuildID()).Scan(context.TODO()); err != nil {
		message = "There was an error retrieving your subreddits"
	} else {
		if len(subscriptions) == 0 {
			message = "no linked subreddits found"
		} else {
			message = "Following subreddits are linked:\n"
			for _, subscription := range subscriptions {
				message += fmt.Sprintf("• `r/%s` in <#%s>\n", subscription.Subreddit, subscription.ChannelID)
			}
		}
	}
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf(message).
		Build(),
	)
}

func (b *RedditBot) onSubredditInfo(event *events.ApplicationCommandInteractionEvent) error {
	b.SubredditsMu.Lock()
	defer b.SubredditsMu.Unlock()
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("not stuck").
		Build(),
	)
}
