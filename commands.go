package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/DisgoOrg/disgo/core"
	"github.com/DisgoOrg/disgo/core/events"
	"github.com/DisgoOrg/disgo/discord"
)

var subredditNamePattern = regexp.MustCompile(`\A[A-Za-z0-9][A-Za-z0-9_]{2,20}`)

var commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "subreddit",
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
		},
		DefaultPermission: true,
	},
}

func onSlashCommand(event *events.ApplicationCommandInteractionEvent) {
	data := event.SlashCommandInteractionData()
	var err error
	if data.CommandName == "subreddit" {
		if event.Member.InteractionPermissions().Missing(discord.PermissionManageServer) {
			err = event.Create(discord.MessageCreate{
				Content: "You don't have permission to manage this server",
				Flags:   discord.MessageFlagEphemeral,
			})
			return
		}
		switch *data.SubCommandName {
		case "add":
			err = onSubredditAdd(data, event)

		case "remove":
			err = onSubredditRemove(data, event)

		case "list":
			err = onSubredditList(event)
		}
	}
	if err != nil {
		logger.Error("error while processing command: ", err)
	}
}

func onSubredditAdd(data *core.SlashCommandInteractionData, event *events.ApplicationCommandInteractionEvent) error {
	name := *data.Options.String("subreddit")
	if !subredditNamePattern.MatchString(name) {
		return event.Create(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("`%s` is not a valid subreddit name. paste just the name.", name).
			Build(),
		)
	}
	subreddit := strings.ToLower(name)

	if _, resp, err := redditClient.Subreddit.Get(context.Background(), subreddit); err != nil {
		return err
	} else if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return event.Create(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("could not find `r/%s`.", name).
			Build(),
		)
	}

	var subredditSubscription *SubredditSubscription
	if err := database.Where("subreddit = ? AND guild_id = ?", subreddit, event.GuildID).First(&subredditSubscription).Error; err == nil {
		return event.Create(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("you already added `r/%s` to this server", subreddit).
			Build(),
		)
	}

	url, state := oauth2Client.GenerateAuthorizationURLState(baseURL+CreateCallbackURL, 0, *event.GuildID, false, discord.ApplicationScopeWebhookIncoming)

	webhookCreateStates[state] = WebhookCreateState{
		Interaction: event.ApplicationCommandInteraction,
		Subreddit:   subreddit,
	}

	return event.Create(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("click [here](%s) to add a new webhook", url).
		Build(),
	)
}

func onSubredditRemove(data *core.SlashCommandInteractionData, event *events.ApplicationCommandInteractionEvent) error {
	subreddit := strings.ToLower(*data.Options.String("subreddit"))

	var subredditSubscription *SubredditSubscription
	if err := database.Where("subreddit = ? AND guild_id = ?", subreddit, event.GuildID).First(&subredditSubscription).Error; err != nil {
		return event.Create(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("could not find r/%s linked to any channel", subreddit).
			Build(),
		)
	}
	unsubscribeFromSubreddit(subreddit, subredditSubscription.WebhookID, true)
	return event.Create(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("removed r/%s", subreddit).
		Build(),
	)
}

func onSubredditList(event *events.ApplicationCommandInteractionEvent) error {
	var subredditSubscriptions []*SubredditSubscription
	db := database.Where("guild_id = ?", event.GuildID).Find(&subredditSubscriptions)
	var message string
	if db.Error != nil {
		message = "no linked subreddits found"
	} else {
		message = "Following subreddits are linked:\n"
		for _, subredditSubscription := range subredditSubscriptions {
			message += fmt.Sprintf("• r/%s in <#%s>\n", subredditSubscription.Subreddit, subredditSubscription.ChannelID)
		}
	}
	return event.Create(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf(message).
		Build(),
	)
}
