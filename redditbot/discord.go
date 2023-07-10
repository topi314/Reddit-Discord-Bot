package redditbot

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
)

var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "subreddit",
		Description: "manage your subscribed subreddits",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "add",
				Description: "add a subreddit to your subscriptions",
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
				Description: "remove a subreddit from your subscriptions",
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
				Description: "list your subscribed subreddits",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionChannel{
						Name:        "channel",
						Description: "the channel to list the subreddits for",
						Required:    false,
					},
				},
			},
		},
	},
	discord.SlashCommandCreate{
		Name:        "info",
		Description: "get info about me",
	},
}

func (b *Bot) OnApplicationCommand(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	switch data.CommandName() {
	case "subreddit":
		switch *data.SubCommandName {
		case "add":
			b.OnSubredditAdd(data, event)
		case "remove":
			b.OnSubredditRemove(data, event)
		case "list":
			b.OnSubredditList(data, event)
		}
	case "info":
		b.OnInfo(event)
	}
}

func (b *Bot) OnSubredditAdd(data discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	subreddit := data.String("subreddit")

	if err := b.Reddit.CheckSubreddit(subreddit); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Invalid subreddit: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if b.Cfg.Server.Enabled {
		state := b.randomString(16)
		url := b.DiscordConfig.AuthCodeURL(state)

		b.States[state] = SetupState{
			Subreddit:   subreddit,
			Interaction: event.ApplicationCommandInteraction,
		}
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Click the button to add a webhook for the subreddit %s", subreddit),
			Components: []discord.ContainerComponent{
				discord.ActionRowComponent{
					discord.NewLinkButton("Add Webhook", url),
				},
			},
			Flags: discord.MessageFlagEphemeral,
		})
		return
	}

	webhook, err := b.Client.Rest().CreateWebhook(event.Channel().ID(), discord.WebhookCreate{
		Name:   subreddit,
		Avatar: discord.NewIconRaw(discord.IconTypePNG, b.RedditIcon),
	})
	if err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("failed to create webhook: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if err = b.DB.AddSubscription(Subscription{
		Subreddit:    subreddit,
		GuildID:      *event.GuildID(),
		ChannelID:    event.Channel().ID(),
		WebhookID:    webhook.ID(),
		WebhookToken: webhook.Token,
	}); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Failed to save subscription to the database: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if _, err = b.Client.Rest().CreateWebhookMessage(webhook.ID(), webhook.Token, discord.WebhookMessageCreate{
		Content: fmt.Sprintf("Added subscription for [r/%s](https://reddit.com/r/%s)", subreddit, subreddit),
	}, true, 0); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Failed to send test message to webhook: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	_ = event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Subscribed to [r/%s](https://reddit.com/r/%s)", subreddit, subreddit),
	})
}

func (b *Bot) OnSubredditRemove(data discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	subreddit := data.String("subreddit")

	sub, err := b.DB.RemoveSubscriptionByGuildSubreddit(*event.GuildID(), subreddit)
	if err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Something went wrong: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	_ = b.Client.Rest().DeleteWebhookWithToken(sub.WebhookID, sub.WebhookToken, rest.WithReason(fmt.Sprintf("Removed subreddit %s by %s", subreddit, event.User().Tag())))

	_ = event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Removed subreddit %s", subreddit),
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (b *Bot) OnSubredditList(data discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	var (
		subs []Subscription
		err  error
	)

	if channel, ok := data.OptChannel("channel"); ok {
		subs, err = b.DB.GetSubscriptionsByChannel(channel.ID)
	} else {
		subs, err = b.DB.GetSubscriptionsByGuild(*event.GuildID())
	}
	if err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Something went wrong: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if len(subs) == 0 {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "You don't have any subscriptions",
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	content := fmt.Sprintf("**Subscriptions(%d):**\n", len(subs))
	for _, sub := range subs {
		content += fmt.Sprintf("- %s\n", sub.Subreddit)
	}

	_ = event.CreateMessage(discord.MessageCreate{
		Content: content,
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (b *Bot) OnInfo(event *events.ApplicationCommandInteractionCreate) {
	_ = event.CreateMessage(discord.MessageCreate{
		Content: "I'm a bot that sends you reddit posts to discord.\nYou can add subreddits with `/subreddit add <subreddit>`\nYou can remove subreddits with `/subreddit remove <subreddit>`\nYou can list your subreddits with `/subreddit list`You can get help on [GitHub](https://github.com/topi314/Reddit-Discord-Bot)",
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (b *Bot) OnDiscordCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state := query.Get("state")
	code := query.Get("code")

	setupState, ok := b.States[state]
	if !ok {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	defer delete(b.States, state)
	token, err := b.DiscordConfig.Exchange(r.Context(), code)
	if err != nil {
		_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
			Content:    json.Ptr("Error while exchanging code: " + err.Error()),
			Components: &[]discord.ContainerComponent{},
		})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	webhookRaw := token.Extra("webhook")
	if webhookRaw == nil {
		err = errors.New("no webhook found in token response")
		_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
			Content:    json.Ptr("Failed to get webhook: " + err.Error()),
			Components: &[]discord.ContainerComponent{},
		})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wh := webhookRaw.(map[string]any)
	webhookID := snowflake.MustParse(wh["id"].(string))
	webhookToken := wh["token"].(string)

	if err = b.DB.AddSubscription(Subscription{
		Subreddit:    setupState.Subreddit,
		GuildID:      *setupState.Interaction.GuildID(),
		ChannelID:    setupState.Interaction.Channel().ID(),
		WebhookID:    webhookID,
		WebhookToken: webhookToken,
	}); err != nil {
		_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
			Content:    json.Ptr("Failed to save subscription to the database: " + err.Error()),
			Components: &[]discord.ContainerComponent{},
		})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = b.Client.Rest().CreateWebhookMessage(webhookID, webhookToken, discord.WebhookMessageCreate{
		Content: fmt.Sprintf("Added subscription for [r/%s](https://reddit.com/r/%s)", setupState.Subreddit, setupState.Subreddit),
	}, true, 0); err != nil {
		_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
			Content:    json.Ptr("Failed to send test message to webhook: " + err.Error()),
			Components: &[]discord.ContainerComponent{},
		})
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	delete(b.States, state)
	_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
		Content:    json.Ptr(fmt.Sprintf("Subscribed to [r/%s](https://reddit.com/r/%s)", setupState.Subreddit, setupState.Subreddit)),
		Components: &[]discord.ContainerComponent{},
	})
	w.Write([]byte("success, you can close this window now"))
}
