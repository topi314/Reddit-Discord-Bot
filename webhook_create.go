package main

import (
	"net/http"
	"net/url"
	"os"

	"github.com/DisgoOrg/disgo/api"
	"github.com/DisgoOrg/disgo/api/endpoints"
	"github.com/DisgoOrg/disgohook"
)

type WebhookCreate struct {
	Interaction *api.Interaction
	Subreddit   string
}

var tokenURL = endpoints.NewCustomRoute(endpoints.POST, "https://discord.com/api/oauth2/token")

func webhookCreateHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := api.Snowflake(query.Get("state"))
	guildID := query.Get("guild_id")
	if code == "" || state == "" || guildID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	webhookState, ok := states[state]
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	delete(states, state)

	compiledRoute, _ := tokenURL.Compile()
	var rs *struct {
		Webhook struct {
			ID            string `json:"id"`
			ApplicationID string `json:"application_id"`
			Name          string `json:"name"`
			Url           string `json:"url"`
			ChannelID     string `json:"channel_id"`
			Token         string `json:"token"`
			Type          int    `json:"type"`
			GuildID       string `json:"guild_id"`
		} `json:"webhook"`
	}

	rq := url.Values{
		"client_id":     {dgo.ApplicationID().String()},
		"client_secret": {os.Getenv("secret")},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURL},
	}
	err := dgo.RestClient().Request(compiledRoute, rq, &rs)
	if err != nil {
		logger.Errorf("error while exchanging code: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	webhookClient, err := disgohook.NewWebhookByIDToken(httpClient, logger, rs.Webhook.ID, rs.Webhook.Token)
	if err != nil {
		logger.Errorf("error creating webhook client: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	addSubreddit(webhookState.Subreddit, webhookClient)

	_, err = webhookState.Interaction.SendFollowup(api.NewFollowupMessageBuilder().
		SetEphemeral(true).
		SetContent("Successfully added webhook!").
		Build(),
	)
	if err != nil {
		logger.Errorf("error while sending followup: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
