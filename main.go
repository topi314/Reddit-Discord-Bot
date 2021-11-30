package main

import (
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/DisgoOrg/disgo/core"
	"github.com/DisgoOrg/disgo/core/bot"
	"github.com/DisgoOrg/disgo/core/events"
	"github.com/DisgoOrg/disgo/discord"
	"github.com/DisgoOrg/disgo/httpserver"
	"github.com/DisgoOrg/disgo/oauth2"
	"github.com/DisgoOrg/disgo/rest"
	"github.com/DisgoOrg/dislog"
	"github.com/sirupsen/logrus"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const (
	InteractionCallbackURL = "/webhooks/interactions/callback"
	CreateCallbackURL      = "/webhooks/create/callback"
	SuccessURL             = "/success"
)

var (
	token = os.Getenv("token")

	logWebhookID      = discord.Snowflake(os.Getenv("log_webhook_id"))
	logWebhookToken   = os.Getenv("log_webhook_token")
	publicKey         = os.Getenv("public_key")
	secret            = os.Getenv("secret")
	baseURL           = os.Getenv("base_url")
	webhookServerPort = os.Getenv("webhook_server_port")
	loglevel, _       = logrus.ParseLevel(os.Getenv("log_level"))

	logger       = logrus.New()
	httpClient   = http.DefaultClient
	disgo        *core.Bot
	oauth2Client *oauth2.Client
	redditClient *reddit.Client

	imageRegex = regexp.MustCompile(`.*\.(?:jpg|gif|png)`)
)

func main() {
	logger.SetLevel(loglevel)

	if logWebhookID != "" && logWebhookToken != "" {
		dlog, err := dislog.New(
			dislog.WithWebhookIDToken(logWebhookID, logWebhookToken),
			dislog.WithLogLevels(dislog.InfoLevelAndAbove...),
		)
		if err != nil {
			logger.Errorf("error initializing dislog %s", err)
			return
		}
		defer dlog.Close()

		logger.AddHook(dlog)
	}

	logger.Infof("starting Reddit-Discord-Bot...")

	serveMux := http.NewServeMux()
	serveMux.HandleFunc(CreateCallbackURL, webhookCreateHandler)
	serveMux.HandleFunc(SuccessURL, webhookCreateSuccessHandler)

	var err error
	disgo, err = bot.New(token,
		bot.WithRestClientOpts(
			rest.WithHTTPClient(httpClient),
		),
		bot.WithLogger(logger),
		bot.WithCacheOpts(
			core.WithCacheFlags(core.CacheFlagsNone),
			core.WithMemberCachePolicy(core.MemberCachePolicyNone),
			core.WithMessageCachePolicy(core.MessageCachePolicyNone),
		),
		bot.WithHTTPServerOpts(
			httpserver.WithPort(webhookServerPort),
			httpserver.WithURL(InteractionCallbackURL),
			httpserver.WithPublicKey(publicKey),
			httpserver.WithServeMux(serveMux),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnSlashCommand: onSlashCommand,
		}),
	)
	if err != nil {
		logger.Fatal("error while building disgo instance: ", err)
		return
	}

	if _, err = disgo.SetCommands(commands); err != nil {
		logger.Error("error while setting commands: ", err)
	}

	oauth2Client = oauth2.New(disgo.ApplicationID, secret,
		oauth2.WithRestClientConfigOpts(
			rest.WithHTTPClient(httpClient),
		),
		oauth2.WithStateController()
	)

	if err = disgo.StartHTTPServer(); err != nil {
		logger.Fatal("error while starting http server: ", err)
		return
	}

	redditClient, err = reddit.NewReadonlyClient()
	if err != nil {
		logger.Panic("failed to init reddit client")
		return
	}

	connectToDatabase()
	loadAllSubreddits()

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}
