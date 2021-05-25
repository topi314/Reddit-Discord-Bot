package main

import (
	"fmt"

	"github.com/DisgoOrg/disgo/api"
	"github.com/DisgoOrg/disgo/api/events"
)

const redirectURL = "https://553f0ccaa2d9.ngrok.io/webhooks/create/callback"

var states = map[api.Snowflake]*WebhookCreate{}

var commands = []*api.CommandCreate{
	{
		Name:              "subreddit",
		Description:       "lets you manage all your subreddits",
		Options: []*api.CommandOption{
			{
				Type:        api.CommandOptionTypeSubCommand,
				Name:        "add",
				Description: "adds a new subreddit",
				Options: []*api.CommandOption{
					{
						Type:        api.CommandOptionTypeString,
						Name:        "subreddit",
						Description: "the subreddit to add",
						Required:    true,
					},
				},
			},
			{
				Type:        api.CommandOptionTypeSubCommand,
				Name:        "remove",
				Description: "removes a subreddit",
				Options: []*api.CommandOption{
					{
						Type:        api.CommandOptionTypeString,
						Name:        "subreddit",
						Description: "the subreddit to remove",
						Required:    true,
					},
				},
			},
			{
				Type:        api.CommandOptionTypeSubCommand,
				Name:        "list",
				Description: "lists all added subreddits",
				Options: []*api.CommandOption{
					{
						Type:        api.CommandOptionTypeChannel,
						Name:        "channel",
						Description: "the channel to list all subreddits from",
						Required:    false,
					},
				},
			},
		},
	},
}

func getListenerAdapter() *events.ListenerAdapter {
	return &events.ListenerAdapter{
		OnSlashCommand: onSlashCommand,
	}
}

func onSlashCommand(event *events.SlashCommandEvent) {
	switch event.CommandName {
	case "subreddit":
		switch *event.SubCommandName {
		case "add":
			states[event.Interaction.ID] = &WebhookCreate{
				Interaction: event.Interaction,
				Subreddit: event.Option("subreddit").String(),
			}
			_ = event.Reply(api.NewInteractionResponseBuilder().
				SetEphemeral(true).
				SetContent("click [here](" + oauth2URL(event.Disgo().ApplicationID(), event.Interaction.ID.String(), redirectURL) + ") to add a new webhook").
				Build(),
			)


		case "remove":

		case "list":

		}
	}
}

func initCommands() error {
	_, err := dgo.RestClient().SetGuildCommands(dgo.ApplicationID(), "608506410803658753", commands...)
	return err
}

func oauth2URL(clientID api.Snowflake, state string, redirectURL string) string {
	return fmt.Sprintf("https://discord.com/oauth2/authorize?response_type=code&client_id=%s&state=%s&scope=webhook.incoming&redirect_uri=%s", clientID, state, redirectURL)
}





