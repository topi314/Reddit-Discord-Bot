package main

import (
	"net/http"

	"github.com/DisgoOrg/disgo/core"
	"github.com/DisgoOrg/disgo/discord"
	"github.com/DisgoOrg/disgo/rest"
	"github.com/DisgoOrg/disgo/webhook"
)

type WebhookCreateState struct {
	Interaction *core.SlashCommandInteraction
	Subreddit   string
}

var webhookCreateStates = map[string]WebhookCreateState{}

func webhookCreateHandler(w http.ResponseWriter, r *http.Request) {
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

	session, err := oauth2Client.StartSession(code, state, "")
	if err != nil {
		logger.Errorf("error while exchanging code: %s", err)
		writeError(w)
		return
	}

	webhookClient := webhook.NewClient(session.Webhook().ID(), session.Webhook().Token,
		webhook.WithRestClientConfigOpts(
			rest.WithHTTPClient(httpClient),
		),
		webhook.WithLogger(logger),
	)
	if err != nil {
		logger.Errorf("error creating webhook client: %s", err)
		writeError(w)
		return
	}

	go func() {
		database.Create(&SubredditSubscription{
			Subreddit:    webhookState.Subreddit,
			GuildID:      session.Webhook().GuildID,
			ChannelID:    session.Webhook().ChannelID,
			WebhookID:    session.Webhook().ID(),
			WebhookToken: session.Webhook().Token,
		})
	}()

	subscribeToSubreddit(webhookState.Subreddit, webhookClient)

	_, err = webhookClient.CreateMessage(discord.NewWebhookMessageCreateBuilder().
		SetContent("Webhook for [r/" + webhookState.Subreddit + "](https://www.reddit.com/r/" + webhookState.Subreddit + ") successfully created").
		Build(),
	)
	message := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if err != nil {
		logger.Errorf("error while tesing webhook: %s", err)
		message.SetContent("There was a problem setting up your webhook.\nRetry or reach out for help [here](https://discord.gg/sD3ABd5)")
	} else {
		message.SetContent("Successfully added webhook. Everything is ready to go")
	}

	_, err = webhookState.Interaction.CreateFollowup(message.Build())
	if err != nil {
		logger.Errorf("error while sending followup: %s", err)
		writeError(w)
		return
	}

	http.Redirect(w, r, baseURL+SuccessURL, http.StatusSeeOther)
}

func webhookCreateSuccessHandler(w http.ResponseWriter, _ *http.Request) {
	writeMessage(w, http.StatusOK, `subreddit successfully created.<br />You can now close this site<br /><br />For further questions you can reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a>`)
}

func writeError(w http.ResponseWriter) {
	writeMessage(w, http.StatusInternalServerError, `There was a problem setting up your subreddit notifications<br />Retry or reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a> for help`)
}

func writeMessage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}
