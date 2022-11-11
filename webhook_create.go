package main

import (
	"context"
	"net/http"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
)

type WebhookCreateState struct {
	Interaction discord.ApplicationCommandInteraction
	Subreddit   string
}

var webhookCreateStates = map[string]WebhookCreateState{}

func (b *RedditBot) webhookCreateHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	guildID := query.Get("guild_id")
	if code == "" || state == "" || guildID == "" {
		writeMessage(w, http.StatusBadRequest, `missing info<br />Retry or reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a> for help`)
		return
	}

	webhookState, ok := webhookCreateStates[state]
	if !ok {
		writeMessage(w, http.StatusForbidden, `state not found or expired<br />Retry or reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a> for help`)
		return
	}
	delete(webhookCreateStates, state)

	session, err := b.OAuth2Client.StartSession(code, state, "")
	if err != nil {
		b.Logger.Errorf("error while exchanging code: %s", err)
		writeError(w, err)
		return
	}

	webhookClient := webhook.New(session.Webhook().ID(), session.Webhook().Token,
		webhook.WithRestClientConfigOpts(
			rest.WithHTTPClient(b.HTTPClient),
		),
		webhook.WithLogger(b.Logger),
	)
	if err != nil {
		b.Logger.Errorf("error creating webhook client: %s", err)
		writeError(w, err)
		return
	}

	go func() {
		if _, err = b.DB.NewInsert().Model(&Subscription{
			Subreddit:    webhookState.Subreddit,
			GuildID:      session.Webhook().GuildID,
			ChannelID:    session.Webhook().ChannelID,
			WebhookID:    session.Webhook().ID(),
			WebhookToken: session.Webhook().Token,
		}).Exec(context.TODO()); err != nil {
			b.Logger.Errorf("error inserting subscription: %s", err)
			return
		}
	}()

	b.subscribeToSubreddit(webhookState.Subreddit, webhookClient)

	_, err = webhookClient.CreateMessage(discord.NewWebhookMessageCreateBuilder().
		SetContent("Webhook for [r/" + webhookState.Subreddit + "](https://www.reddit.com/r/" + webhookState.Subreddit + ") successfully created").
		Build(),
	)
	message := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if err != nil {
		b.Logger.Errorf("error while testing webhook: %s", err)
		message.SetContentf("There was a problem setting up your webhook.\nError: %s\n\nRetry or reach out for help [here](https://discord.gg/sD3ABd5)", err)
	} else {
		message.SetContent("Successfully added webhook. Everything is ready to go")
	}

	if _, err = b.Client.Rest().CreateFollowupMessage(webhookState.Interaction.ApplicationID(), webhookState.Interaction.Token(), message.Build()); err != nil {
		b.Logger.Errorf("error while sending followup: %s", err)
		writeError(w, err)
		return
	}

	http.Redirect(w, r, baseURL+SuccessURL, http.StatusSeeOther)
}

func webhookCreateSuccessHandler(w http.ResponseWriter, _ *http.Request) {
	writeMessage(w, http.StatusOK, `subreddit successfully created.<br />You can now close this site<br /><br />For further questions you can reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a>`)
}

func writeError(w http.ResponseWriter, err error) {
	writeMessage(w, http.StatusInternalServerError, `There was a problem setting up your subreddit notifications<br />Error: `+err.Error()+`<br /><br />Retry or reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a> for help`)
}

func writeMessage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}
