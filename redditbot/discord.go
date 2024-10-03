package redditbot

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
)

var typeChoices = []discord.ApplicationCommandOptionChoiceString{
	{
		Name:  "New",
		Value: "new",
	},
	// This does not seem to work like it should
	// {
	// 	Name:  "Hot",
	// 	Value: "hot",
	// },
	// {
	// 	Name:  "Top",
	// 	Value: "top",
	// },
	// {
	// 	Name:  "Rising",
	// 	Value: "rising",
	// },
}

var formatTypeChoices = []discord.ApplicationCommandOptionChoiceString{
	{
		Name:  strings.Title(string(FormatTypeEmbed)),
		Value: string(FormatTypeEmbed),
	},
	{
		Name:  strings.Title(string(FormatTypeText)),
		Value: string(FormatTypeText),
	},
	{
		Name:  strings.Title(string(FormatTypeLink)),
		Value: string(FormatTypeLink),
	},
}

var Commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:        "reddit",
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
					discord.ApplicationCommandOptionString{
						Name:        "type",
						Description: "the type of posts to send",
						Required:    false,
						Choices:     typeChoices,
					},
					discord.ApplicationCommandOptionString{
						Name:        "format-type",
						Description: "how to format the subreddit posts",
						Required:    false,
						Choices:     formatTypeChoices,
					},
					discord.ApplicationCommandOptionRole{
						Name:        "role",
						Description: "the role to ping when a post is sent",
						Required:    false,
					},
					discord.ApplicationCommandOptionString{
						Name:        "proxy",
						Description: "the proxy to use for the post links such as: https://rxddit.com",
						Required:    false,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "update",
				Description: "update a subreddit in your subscriptions",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "subreddit",
						Description: "the subreddit to update",
						Required:    true,
					},
					discord.ApplicationCommandOptionString{
						Name:        "type",
						Description: "the type of posts to send",
						Required:    false,
						Choices:     typeChoices,
					},
					discord.ApplicationCommandOptionString{
						Name:        "format-type",
						Description: "how to format the subreddit posts",
						Required:    false,
						Choices:     formatTypeChoices,
					},
					discord.ApplicationCommandOptionRole{
						Name:        "role",
						Description: "the role to ping when a post is sent",
						Required:    false,
					},
					discord.ApplicationCommandOptionString{
						Name:        "proxy",
						Description: "the proxy to use for the post links such as: https://rxddit.com",
						Required:    false,
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
		DefaultMemberPermissions: json.NewNullablePtr(discord.PermissionManageGuild),
	},
	discord.SlashCommandCreate{
		Name:        "info",
		Description: "get info about me",
	},
}

func (b *Bot) OnApplicationCommand(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	switch data.CommandName() {
	case "reddit":
		switch *data.SubCommandName {
		case "add":
			b.OnSubredditAdd(data, event)
		case "update":
			b.OnSubredditUpdate(data, event)
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
	postType, ok := data.OptString("type")
	if !ok {
		postType = "new"
	}
	formatType, ok := data.OptString("format-type")
	if !ok {
		formatType = "embed"
	}
	roleID := data.Snowflake("role")
	proxy := data.String("proxy")

	ok, err := b.db.HasSubscriptionByGuildSubreddit(*event.GuildID(), subreddit)
	if err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Failed to check if you are already subscribed to this subreddit: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}
	if ok {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("You are already subscribed to `r/%s`", subreddit),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if err = b.reddit.CheckSubreddit(subreddit); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Invalid subreddit: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if b.cfg.Server.Enabled {
		state := b.randomString(16)
		url := b.discordConfig.AuthCodeURL(state)

		b.states[state] = SetupState{
			Subreddit:   subreddit,
			PostType:    postType,
			FormatType:  FormatType(formatType),
			RoleID:      roleID,
			RedditProxy: proxy,
			Interaction: event.ApplicationCommandInteraction,
		}
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Click the button to add a webhook for the subreddit `%s`", subreddit),
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
		Avatar: discord.NewIconRaw(discord.IconTypePNG, b.redditIcon),
	})
	if err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("failed to create webhook: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if _, err = b.Client.Rest().CreateWebhookMessage(webhook.ID(), webhook.Token, discord.WebhookMessageCreate{
		Content: fmt.Sprintf("Added subscription for %s", formatSubreddit(subreddit)),
	}, true, 0); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Failed to send test message to webhook: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if err = b.db.AddSubscription(Subscription{
		Subreddit:    subreddit,
		Type:         postType,
		FormatType:   FormatType(formatType),
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

	_ = event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Subscribed to %s)", formatSubreddit(subreddit)),
	})
}

func (b *Bot) OnSubredditUpdate(data discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	subreddit := data.String("subreddit")
	postType := data.String("type")
	formatType := FormatType(data.String("format-type"))
	roleID := data.Snowflake("role")
	proxy := data.String("proxy")

	sub, err := b.db.GetSubscriptionsByGuildSubreddit(*event.GuildID(), subreddit)
	if errors.Is(err, ErrSubscriptionNotFound) {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("You are not subscribed to r/%s", subreddit),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}
	if err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Failed to get subscription from the database: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	if postType == "" {
		postType = sub.Type
	}
	if formatType == "" {
		formatType = sub.FormatType
	}
	if roleID != 0 {
		roleID = sub.RoleID
	}
	if proxy == "" {
		proxy = sub.RedditProxy
	}

	if err = b.db.UpdateSubscription(sub.WebhookID, postType, formatType, roleID, proxy); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: "Failed to update subscription: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

	_ = event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Updated subscription for %s", formatSubreddit(subreddit)),
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (b *Bot) OnSubredditRemove(data discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	subreddit := data.String("subreddit")

	if err := b.RemoveSubscriptionByGuildSubreddit(*event.GuildID(), subreddit, fmt.Sprintf("Removed subreddit %s by %s", subreddit, event.User().Tag())); err != nil {
		_ = event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Something went wrong: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
		return
	}

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
		subs, err = b.db.GetSubscriptionsByChannel(channel.ID)
	} else {
		subs, err = b.db.GetSubscriptionsByGuild(*event.GuildID())
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

	content := fmt.Sprintf("# Subscriptions(%d):\n", len(subs))
	for _, sub := range subs {
		role := "`none`"
		if sub.RoleID != 0 {
			role = fmt.Sprintf("<@%s>", sub.RoleID)
		}
		proxy := "`none`"
		if sub.RedditProxy != "" {
			proxy = fmt.Sprintf("`%s`", sub.RedditProxy)
		}
		content += fmt.Sprintf("- `%s` - `%s` - %s - %s- %s\n", strings.Title(sub.Type), strings.Title(string(sub.FormatType)), role, proxy, formatSubreddit(sub.Subreddit))
	}

	_ = event.CreateMessage(discord.MessageCreate{
		Content:         content,
		Flags:           discord.MessageFlagEphemeral,
		AllowedMentions: &discord.AllowedMentions{},
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

	setupState, ok := b.states[state]
	if !ok {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	defer delete(b.states, state)
	token, err := b.discordConfig.Exchange(r.Context(), code)
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

	if err = b.db.AddSubscription(Subscription{
		Subreddit:    setupState.Subreddit,
		Type:         setupState.PostType,
		FormatType:   setupState.FormatType,
		GuildID:      *setupState.Interaction.GuildID(),
		ChannelID:    setupState.Interaction.Channel().ID(),
		WebhookID:    webhookID,
		WebhookToken: webhookToken,
		RoleID:       setupState.RoleID,
		RedditProxy:  setupState.RedditProxy,
	}); err != nil {
		_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
			Content:    json.Ptr("Failed to save subscription to the database: " + err.Error()),
			Components: &[]discord.ContainerComponent{},
		})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = b.Client.Rest().CreateWebhookMessage(webhookID, webhookToken, discord.WebhookMessageCreate{
		Content: fmt.Sprintf("Added subscription for %s", formatSubreddit(setupState.Subreddit)),
	}, true, 0); err != nil {
		_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
			Content:    json.Ptr("Failed to send test message to webhook: " + err.Error()),
			Components: &[]discord.ContainerComponent{},
		})
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	delete(b.states, state)
	_, _ = b.Client.Rest().UpdateInteractionResponse(setupState.Interaction.ApplicationID(), setupState.Interaction.Token(), discord.MessageUpdate{
		Content:    json.Ptr(fmt.Sprintf("Subscribed to %s)", formatSubreddit(setupState.Subreddit))),
		Components: &[]discord.ContainerComponent{},
	})
	_, _ = w.Write([]byte("success, you can close this window now"))
}

func formatSubreddit(subreddit string) string {
	return fmt.Sprintf("[`r/%s`](https://reddit.com/r/%s)", subreddit, subreddit)
}
