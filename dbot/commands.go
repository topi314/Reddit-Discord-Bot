package dbot

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/json"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

var subredditNamePattern = regexp.MustCompile(`\A(r/)?(?P<name>[A-Za-z\d]\w{2,20})$`)

var commands = []discord.ApplicationCommandCreate{
	discord.SlashCommandCreate{
		Name:                     "subreddit",
		Description:              "Lets you manage all your subreddits\",",
		DefaultMemberPermissions: json.NewOptional(discord.PermissionManageServer),
		DMPermission:             json.NewPtr(false),
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "add",
				Description: "Lets you add a new subreddit",
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
				Description: "Lets you remove a subreddit",
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
				Description: "Lists all configured subreddits",
			},
		},
	},
}

func (b *Bot) OnApplicationCommand(e *events.ApplicationCommandInteractionCreate) {
	data := e.SlashCommandInteractionData()
	var err error
	if data.CommandName() == "subreddit" {
		switch *data.SubCommandName {
		case "add":
			err = b.onSubredditAdd(e, data)

		case "remove":
			err = b.onSubredditRemove(e, data)

		case "list":
			err = b.onSubredditList(e)
		}
	}
	if err != nil {
		b.Logger.Error("error while processing command: ", err)
	}
}

func (b *Bot) onSubredditAdd(e *events.ApplicationCommandInteractionCreate, data discord.SlashCommandInteractionData) error {
	subredditName, ok := parseSubredditName(data.String("subreddit"))
	if !ok {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("`%s` is not a valid subreddit subredditName.", data.String("subreddit")).
			Build(),
		)
	}
	has, err := b.DB.HasSubscription(*e.GuildID(), subredditName)
	if err != nil {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("error while checking if subreddit `%s` is already added: %s", subredditName, err).
			Build(),
		)
	}
	if has {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("The subreddit `%s` is already added.", subredditName).
			Build(),
		)
	}
	_, _, err = b.Reddit.Subreddit.Get(context.TODO(), subredditName)
	if httpErr, ok := err.(*reddit.ErrorResponse); ok && httpErr.Response.StatusCode == http.StatusNotFound {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("The subreddit `%s` does not exist.", subredditName).
			Build(),
		)
	} else if httpErr, ok = err.(*reddit.ErrorResponse); ok && httpErr.Response.StatusCode == http.StatusForbidden {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("I can't access this subreddit.", subredditName).
			Build(),
		)
	} else if err != nil {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("Error while fetching subreddit: %s", err).
			Build(),
		)
	}

	url, state := b.OAuth2Client.GenerateAuthorizationURLState(b.Config.Bot.RedirectURL+CallbackURL, 0, *e.GuildID(), false, discord.OAuth2ScopeWebhookIncoming)

	b.WebhookStates[state] = oauth2State{
		Interaction:   e.ApplicationCommandInteraction,
		SubredditName: subredditName,
	}

	return e.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("Click [here](%s) to add a new webhook for this subreddit", url).
		Build(),
	)
}

func (b *Bot) onSubredditRemove(e *events.ApplicationCommandInteractionCreate, data discord.SlashCommandInteractionData) error {
	subredditName, ok := parseSubredditName(data.String("subreddit"))
	if !ok {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("`%s` is not a valid subreddit name.", data.String("subreddit")).
			Build(),
		)
	}

	err := b.DB.RemoveSubscription(*e.GuildID(), subredditName)
	if err == sql.ErrNoRows {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("Subreddit with name `%s` not found.", subredditName).
			Build(),
		)
	} else if err != nil {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("Error while removing subreddit: %s", err).
			Build(),
		)
	}

	b.Subreddits.UnsubscribeFromSubreddit(*e.GuildID(), subredditName)

	return e.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContentf("Subreddit `%s` removed.", subredditName).
		Build(),
	)
}

func (b *Bot) onSubredditList(e *events.ApplicationCommandInteractionCreate) error {
	subscriptions, err := b.DB.GetSubscriptions(*e.GuildID())
	if err != nil {
		return e.CreateMessage(discord.NewMessageCreateBuilder().
			SetEphemeral(true).
			SetContentf("Error while fetching subscriptions: %s", err).
			Build(),
		)
	}

	var content string
	if len(subscriptions) == 0 {
		content = "No subreddits are configured."
	} else {
		content = "The following subreddits are configured:"
		for _, sub := range subscriptions {
			content += "\n" + sub.Subreddit
		}
	}

	return e.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(true).
		SetContent(content).
		Build(),
	)
}

func parseSubredditName(name string) (string, bool) {
	match := subredditNamePattern.FindStringSubmatch(name)
	if len(match) == 0 {
		return "", false
	}
	return strings.ToLower(match[subredditNamePattern.SubexpIndex("name")]), true
}
