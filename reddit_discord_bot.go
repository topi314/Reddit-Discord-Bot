package main

import (
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	"github.com/DisgoOrg/disgo"
	"github.com/DisgoOrg/disgo/api"
	"github.com/DisgoOrg/disgommand"
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

	logWebhookToken      = os.Getenv("log_webhook_token")
	publicKey            = os.Getenv("public_key")
	secret               = os.Getenv("secret")
	baseURL              = os.Getenv("base_url")
	webhookServerPort, _ = strconv.Atoi(os.Getenv("webhook_server_port"))
	loglevel, _          = logrus.ParseLevel(os.Getenv("log_level"))

	logger       = logrus.New()
	httpClient   = http.DefaultClient
	dgo          api.Disgo
	redditClient *reddit.Client

	imageRegex = regexp.MustCompile(`.*\.(?:jpg|gif|png)`)
)

func main() {
	logger.SetLevel(loglevel)

	if logWebhookToken != "" {
		dlog, err := dislog.NewDisLogByToken(httpClient, logrus.InfoLevel, logWebhookToken, dislog.InfoLevelAndAbove...)
		if err != nil {
			logger.Errorf("error initializing dislog %s", err)
			return
		}
		defer dlog.Close()

		logger.AddHook(dlog)
	}

	logger.Infof("starting Reddit-Discord-Bot...")

	router := disgommand.NewRouter(logger, true)

	subredditOption := api.NewStringOption("subreddit", "the subreddit to add or remove").SetRequired(true)

	router.HandleFunc("subreddit", "lets you manage all your subreddits", nil, api.PermissionManageServer, api.PermissionsNone, nil)
	router.HandleFunc("subreddit/add", "adds a new subreddit", nil, api.PermissionManageServer, api.PermissionsNone, onSubredditAdd, subredditOption)
	router.HandleFunc("subreddit/remove", "removes a subreddit", nil, api.PermissionManageServer, api.PermissionsNone, onSubredditRemove, subredditOption)
	router.HandleFunc("subreddit/list", "lists all added subreddits", nil, api.PermissionManageServer, api.PermissionsNone, onSubredditList)

	var err error
	dgo, err = disgo.NewBuilder(token).
		SetHTTPClient(httpClient).
		SetLogger(logger).
		SetCacheFlags(api.CacheFlagsNone).
		SetMemberCachePolicy(api.MemberCachePolicyNone).
		SetMessageCachePolicy(api.MessageCachePolicyNone).
		SetWebhookServerProperties(InteractionCallbackURL, webhookServerPort, publicKey).
		AddEventListeners(router).
		Build()
	if err != nil {
		logger.Fatalf("error while building disgo instance: %s", err)
		return
	}

	_ = router.CreateGlobalCommands(dgo)

	dgo.Start()
	dgo.WebhookServer().Router().HandleFunc(CreateCallbackURL, webhookCreateHandler).Methods("GET")
	dgo.WebhookServer().Router().HandleFunc(SuccessURL, webhookCreateSuccessHandler).Methods("GET")

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
