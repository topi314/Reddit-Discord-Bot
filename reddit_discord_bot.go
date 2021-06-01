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
	"github.com/sirupsen/logrus"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

var token = os.Getenv("token")
var publicKey = os.Getenv("public_key")
var secret = os.Getenv("secret")
var redirectURL = os.Getenv("redirect_url")
var webhookServerPort, _ = strconv.Atoi(os.Getenv("webhook_server_port"))

var logger = logrus.New()
var httpClient *http.Client
var dgo api.Disgo
var redditClient *reddit.Client

var imageRegex = regexp.MustCompile(`.*\.(?:jpg|gif|png)`)

func main() {
	httpClient = http.DefaultClient

	logger.SetLevel(logrus.InfoLevel)
	logger.Infof("starting Reddit-Discord-Bot...")

	router := disgommand.NewRouter(logger, true)

	router.HandleFunc("subreddit", "lets you manage all your subreddits", nil, nil)
	router.HandleFunc("subreddit/add", "adds a new subreddit", nil, onSubredditAdd, api.NewStringOption("subreddit", "the subreddit to add").SetRequired(true))
	router.HandleFunc("subreddit/remove", "removes a subreddit", nil, onSubredditRemove, api.NewStringOption("subreddit", "the subreddit to remove").SetRequired(true))
	router.HandleFunc("subreddit/list", "lists all added subreddits", nil, onSubredditList)

	var err error
	dgo, err = disgo.NewBuilder(token).
		SetHTTPClient(httpClient).
		SetLogger(logger).
		SetCacheFlags(api.CacheFlagsNone).
		SetMemberCachePolicy(api.MemberCachePolicyNone).
		SetMessageCachePolicy(api.MessageCachePolicyNone).
		SetWebhookServerProperties("/webhooks/interactions/callback", webhookServerPort, publicKey).
		AddEventListeners(router).
		Build()
	if err != nil {
		logger.Fatalf("error while building disgo instance: %s", err)
		return
	}

	_ = router.CreateGlobalCommands(dgo)

	dgo.Start()
	dgo.WebhookServer().Router().HandleFunc("/webhooks/create/callback", webhookCreateHandler).Methods("GET")

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
