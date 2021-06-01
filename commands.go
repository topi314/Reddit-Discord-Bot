package main

import (
	"fmt"

	"github.com/DisgoOrg/disgo/api"
	"github.com/DisgoOrg/disgo/api/events"
)

var states = map[api.Snowflake]*WebhookCreateState{}


func onSubredditAdd(event *events.CommandEvent) error {
	states[event.Interaction.ID] = &WebhookCreateState{
		Interaction: event.Interaction,
		Subreddit:   event.Option("subreddit").String(),
	}

	return event.ReplyCreate(api.NewInteractionResponseBuilder().
		SetEphemeral(true).
		SetContent("click [here](" + oauth2URL(event.Disgo().ApplicationID(), event.Interaction.ID.String(), redirectURL) + ") to add a new webhook").
		BuildData(),
	)
}

func onSubredditRemove(event *events.CommandEvent) error {
	/*go func() {
		database.Delete(&SubredditSubscription{}, "webhook_token = " + webhookClient.Token())
	}()*/
	return nil
}

func onSubredditList(event *events.CommandEvent) error {
	return nil
}

func oauth2URL(clientID api.Snowflake, state string, redirectURL string) string {
	return fmt.Sprintf("https://discord.com/oauth2/authorize?response_type=code&client_id=%s&state=%s&scope=webhook.incoming&redirect_uri=%s", clientID, state, redirectURL)
}
