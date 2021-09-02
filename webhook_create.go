package main

import (
	"net/http"
	"net/url"

	"github.com/DisgoOrg/disgo/api"
	"github.com/DisgoOrg/disgohook"
	wapi "github.com/DisgoOrg/disgohook/api"
	"github.com/DisgoOrg/restclient"
)

var tokenURL = restclient.NewCustomRoute(restclient.POST, "https://discord.com/api/oauth2/token")

type WebhookCreateState struct {
	Interaction *api.Interaction
	Subreddit   string
}

func webhookCreateHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := api.Snowflake(query.Get("state"))
	guildID := query.Get("guild_id")
	if code == "" || state == "" || guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing info<br />Retry or reach out <a href=\"https://discord.gg/sD3ABd5\" target=\"_blank\">here</a> for help"))
		return
	}

	webhookState, ok := states[state]
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("state not found or expired<br />Retry or reach out <a href=\"https://discord.gg/sD3ABd5\" target=\"_blank\">here</a> for help"))
		return
	}
	delete(states, state)

	compiledRoute, _ := tokenURL.Compile(nil)
	var rs *struct {
		*wapi.Webhook `json:"webhook"`
	}

	rq := url.Values{
		"client_id":     {dgo.ApplicationID().String()},
		"client_secret": {secret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURL},
	}
	var err error
	err = dgo.RestClient().Do(compiledRoute, rq, &rs)
	if err != nil {
		logger.Errorf("error while exchanging code: %s", err)
		writeError(w)
		return
	}

	webhookClient, err := disgohook.NewWebhookClientByIDToken(httpClient, logger, rs.Webhook.ID, *rs.Webhook.Token)
	if err != nil {
		logger.Errorf("error creating webhook client: %s", err)
		writeError(w)
		return
	}

	go func() {
		database.Create(&SubredditSubscription{
			Subreddit:    webhookState.Subreddit,
			GuildID:      *rs.Webhook.GuildID,
			ChannelID:    *rs.Webhook.ChannelID,
			WebhookID:    webhookClient.ID(),
			WebhookToken: webhookClient.Token(),
		})
	}()

	subscribeToSubreddit(webhookState.Subreddit, webhookClient)

	_, err = webhookClient.SendMessage(wapi.NewWebhookMessageCreateBuilder().
		SetContent("Webhook for [r/" + webhookState.Subreddit + "](https://www.reddit.com/r/" + webhookState.Subreddit + ") successfully created").
		Build(),
	)
	message := api.NewMessageCreateBuilder().SetEphemeral(true)
	if err != nil {
		logger.Errorf("error while tesing webhook: %s", err)
		message.SetContent("There was a problem setting up your webhook.\nRetry or reach out for help [here](https://discord.gg/sD3ABd5)")
	} else {
		message.SetContent("Successfully added webhook. Everything is ready to go")
	}

	_, err = webhookState.Interaction.SendFollowup(message.Build())
	if err != nil {
		logger.Errorf("error while sending followup: %s", err)
		writeError(w)
		return
	}

	http.Redirect(w, r, redirectURL + "/success", http.StatusSeeOther)
}

func webhookCreateSuccessHandler(w http.ResponseWriter, r *http.Request){
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`subreddit successfully created.<br />You can now close this site<br /><br />nFor further questions you can reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a>`))
}

func writeError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`There was a problem setting up your subreddit notifications<br />Retry or reach out <a href="https://discord.gg/sD3ABd5" target="_blank">here</a> for help`))
}
