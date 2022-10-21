package dbot

import (
	"net/http"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
)

type oauth2State struct {
	Interaction   discord.Interaction
	SubredditName string
}

func (b *Bot) OnSubredditCreateHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	guildID := query.Get("guild_id")
	if code == "" || state == "" || guildID == "" {
		writeMessage(w, http.StatusBadRequest, `missing info<br />Retry or reach out <a href="`+b.Config.SupportInviteURL+`" target="_blank">here</a> for help`)
		return
	}

	webhookState, ok := b.WebhookStates[state]
	if !ok {
		writeMessage(w, http.StatusForbidden, `state not found or expired<br />Retry or reach out <a href="`+b.Config.SupportInviteURL+`" target="_blank">here</a> for help`)
		return
	}
	delete(b.WebhookStates, state)

	session, err := b.OAuth2Client.StartSession(code, state, "")
	if err != nil {
		b.Logger.Errorf("error while exchanging code: %s", err)
		b.writeError(w, err)
		b.updateError(webhookState.Interaction, err)
		return
	}

	webhookClient := webhook.New(session.Webhook().ID(), session.Webhook().Token,
		webhook.WithRestClientConfigOpts(
			rest.WithHTTPClient(b.HTTPClient),
		),
		webhook.WithLogger(b.Logger),
	)

	if _, err = webhookClient.CreateMessage(discord.NewWebhookMessageCreateBuilder().
		SetContentf("Webhook for [r/%s](https://www.reddit.com/r/%s) successfully created", webhookState.SubredditName, webhookState.SubredditName).
		Build(),
	); err != nil {
		b.Logger.Errorf("error while creating webhook message: %s", err)
		b.writeError(w, err)
		b.updateError(webhookState.Interaction, err)
		return
	}

	if err = b.DB.AddSubscription(*webhookState.Interaction.GuildID(), webhookState.SubredditName, session.Webhook().ID(), session.Webhook().Token); err != nil {
		b.Logger.Errorf("error while adding subscription: %s", err)
		b.writeError(w, err)
		b.updateError(webhookState.Interaction, err)
		return
	}

	// TODO: subscribe to subreddit

	http.Redirect(w, r, b.Config.Bot.RedirectURL+SuccessURL, http.StatusSeeOther)
}

func (b *Bot) OnSubredditCreateSuccessHandler(w http.ResponseWriter, r *http.Request) {

}

func (b *Bot) writeError(w http.ResponseWriter, err error) {
	writeMessage(w, http.StatusInternalServerError, `There was an error setting up your subreddit subscription<br />Error: `+err.Error()+`<br /><br />Retry or reach out <a href="`+b.Config.SupportInviteURL+`" target="_blank">here</a> for help`)
}

func writeMessage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}

func (b *Bot) updateError(interaction discord.Interaction, err error) {
	_, _ = b.Client.Rest().UpdateInteractionResponse(interaction.ApplicationID(), interaction.Token(), discord.NewMessageUpdateBuilder().
		SetContentf("There was an error setting up your subreddit subscription. Error: %s. Retry or reach out [here](%s) for help", err, b.Config.SupportInviteURL).
		Build(),
	)
}
