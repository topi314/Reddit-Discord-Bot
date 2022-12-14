package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/dislog"
	"github.com/disgoorg/snowflake/v2"
	"github.com/sirupsen/logrus"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const (
	InteractionCallbackURL = "/webhooks/interactions/callback"
	CreateCallbackURL      = "/webhooks/create/callback"
	SuccessURL             = "/success"

	EmbedColor = 0xff581a
)

var (
	token                = os.Getenv("token")
	logWebhookID         = snowflake.GetEnv("log_webhook_id")
	logWebhookToken      = os.Getenv("log_webhook_token")
	publicKey            = os.Getenv("public_key")
	secret               = os.Getenv("secret")
	baseURL              = os.Getenv("base_url")
	webhookServerAddress = os.Getenv("webhook_server_address")
	loglevel, _          = logrus.ParseLevel(os.Getenv("log_level"))

	shouldSyncCommands, _ = strconv.ParseBool(os.Getenv("should_sync_commands"))

	redditID       = os.Getenv("reddit_id")
	redditSecret   = os.Getenv("reddit_secret")
	redditUsername = os.Getenv("reddit_username")
	redditPassword = os.Getenv("reddit_password")

	imageRegex = regexp.MustCompile(`.*\.(?:jpg|gif|png)`)
)

func main() {
	logger := logrus.New()
	logger.SetLevel(loglevel)
	logger.SetReportCaller(true)

	if logWebhookID != 0 && logWebhookToken != "" {
		hook, err := dislog.New(
			dislog.WithWebhookIDToken(logWebhookID, logWebhookToken),
			dislog.WithLogLevels(dislog.InfoLevelAndAbove...),
		)
		if err != nil {
			logger.Errorf("error initializing dislog %s", err)
			return
		}
		defer hook.Close(context.TODO())

		logger.AddHook(hook)
	}

	logger.Infof("starting Reddit-Discord-Bot...")
	redditBot := &RedditBot{
		Logger:               logger,
		RestClient:           rest.NewClient("", rest.WithHTTPClient(&http.Client{Timeout: 10 * time.Second})),
		Subreddits:           map[string][]webhook.Client{},
		SubredditCancelFuncs: map[string]func(){},
	}

	var err error
	if err = redditBot.Setup(); err != nil {
		logger.Fatal("error setting up bot:", err)
	}

	if shouldSyncCommands {
		if err = redditBot.SetupCommands(); err != nil {
			logger.Error("error setting up bot:", err)
		}
	}

	redditBot.SetupOAuth2()

	if err = redditBot.Start(); err != nil {
		logger.Fatal("error while starting http server: ", err)
		return
	}

	if redditBot.RedditClient, err = reddit.NewClient(reddit.Credentials{
		ID:       redditID,
		Secret:   redditSecret,
		Username: redditUsername,
		Password: redditPassword,
	}); err != nil {
		logger.Fatal("failed to init reddit client: ", err)
		return
	}

	if err = redditBot.SetupDB(); err != nil {
		logger.Fatal("failed to setup database: ", err)
		return
	}
	if err = redditBot.loadAllSubreddits(); err != nil {
		logger.Fatal("failed to load subscriptions: ", err)
		return
	}

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}
