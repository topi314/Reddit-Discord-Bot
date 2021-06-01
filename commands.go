package main

import (
	"fmt"
	"strings"

	"github.com/DisgoOrg/disgo/api"
	"github.com/DisgoOrg/disgo/api/events"
)

var states = map[api.Snowflake]*WebhookCreateState{}

func onSubredditAdd(event *events.CommandEvent) error {
	subreddit := strings.ToLower(event.Option("subreddit").String())

	var subredditSubscription *SubredditSubscription
	if err := database.Where("subreddit = ? AND guild_id = ?", subreddit, event.Interaction.GuildID).First(&subredditSubscription).Error; err == nil {
		return event.ReplyCreate(api.NewInteractionResponseBuilder().
			SetEphemeral(true).
			SetContentf("you already added r/%s to this server", subreddit).
			BuildData(),
		)
	}

	states[event.Interaction.ID] = &WebhookCreateState{
		Interaction: event.Interaction,
		Subreddit:   subreddit,
	}

	return event.ReplyCreate(api.NewInteractionResponseBuilder().
		SetEphemeral(true).
		SetContent("click [here](" + oauth2URL(event.Disgo().ApplicationID(), event.Interaction.ID.String(), redirectURL) + ") to add a new webhook").
		BuildData(),
	)
}

func onSubredditRemove(event *events.CommandEvent) error {
	subreddit := strings.ToLower(event.Option("subreddit").String())

	var subredditSubscription *SubredditSubscription
	if err := database.Where("subreddit = ? AND guild_id = ?", subreddit, event.Interaction.GuildID).First(&subredditSubscription).Error; err != nil {
		return event.ReplyCreate(api.NewInteractionResponseBuilder().
			SetEphemeral(true).
			SetContentf("could not find r/%s linked to any channel", subreddit).
			BuildData(),
		)
	}
	database.Delete(subredditSubscription)
	unsubscribeFromSubreddit(subreddit, subredditSubscription.WebhookID)
	return event.ReplyCreate(api.NewInteractionResponseBuilder().
		SetEphemeral(true).
		SetContentf("removed r/%s", subreddit).
		BuildData(),
	)
}

func onSubredditList(event *events.CommandEvent) error {
	var subredditSubscriptions []*SubredditSubscription
	db := database.Where("guild_id = ?", event.Interaction.GuildID).Find(&subredditSubscriptions)
	var message string
	if db.Error != nil {
		message = "no linked subreddits found"
	} else {
		message = "Following subreddits are linked:\n"
		for _, subredditSubscription := range subredditSubscriptions {
			message += fmt.Sprintf("• r/%s in <#%s>\n", subredditSubscription.Subreddit, subredditSubscription.ChannelID)
		}
	}
	return event.ReplyCreate(api.NewInteractionResponseBuilder().
		SetEphemeral(true).
		SetContentf(message).
		BuildData(),
	)
}

func oauth2URL(clientID api.Snowflake, state string, redirectURL string) string {
	return fmt.Sprintf("https://discord.com/oauth2/authorize?response_type=code&client_id=%s&state=%s&scope=webhook.incoming&redirect_uri=%s", clientID, state, redirectURL)
}
