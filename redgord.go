package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/DisgoOrg/disgo"
	"github.com/DisgoOrg/disgo/api"
	wapi "github.com/DisgoOrg/disgohook/api"
	"github.com/sirupsen/logrus"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

var logger = logrus.New()
var httpClient *http.Client
var dgo api.Disgo
var subreddits = map[string][]wapi.Webhook{}
var redditClient *reddit.Client

func main() {
	httpClient = http.DefaultClient

	logger.SetLevel(logrus.DebugLevel)
	logger.Infof("starting Reddit-Discord-Bot...")

	var err error
	dgo, err = disgo.NewBuilder(os.Getenv("token")).
		SetHTTPClient(httpClient).
		SetLogger(logger).
		SetCacheFlags(api.CacheFlagsNone).
		SetMemberCachePolicy(api.MemberCachePolicyNone).
		SetMessageCachePolicy(api.MessageCachePolicyNone).
		SetWebhookServerProperties("/webhooks/interactions/callback", 80, os.Getenv("public_key")).
		AddEventListeners(getListenerAdapter()).
		Build()
	if err != nil {
		logger.Fatalf("error while building disgo instance: %s", err)
		return
	}

	dgo.Start()
	dgo.WebhookServer().Router().HandleFunc("/webhooks/create/callback", webhookCreateHandler).Methods("GET")

	if err = initCommands(); err != nil {
		logger.Panic("failed to init commands")
	}

	redditClient, err = reddit.NewReadonlyClient()
	if err != nil {
		logger.Panic("failed to init reddit client")
	}

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func addSubreddit(subreddit string, webhookClient wapi.Webhook) {
	subreddits[subreddit] = append(subreddits[subreddit], webhookClient)

	go func() {
		posts, errs, _ := redditClient.Stream.Posts(subreddit)
		for {
			select {
			case post := <-posts:
				description := post.Body
				if len(description) > 2048 {
					description = string([]rune(description)[0:2045]) + "..."
				}

				message := wapi.NewWebhookMessageBuilder().
					SetEmbeds(wapi.NewEmbedBuilder().
						SetTitle(post.Title).
						SetURL("https://www.reddit.com" + post.Permalink).
						SetColor(0xff581a).
						SetAuthorName("New post in "+ post.SubredditNamePrefixed).
						SetAuthorURL("https://www.reddit.com" + post.SubredditNamePrefixed).
						SetDescription(description).
						AddField("Author", post.Author, false).
						Build(),
					).
					Build()

				for _, webhookClient := range subreddits[subreddit] {
					_, err := webhookClient.SendMessage(message)
					if err != nil {
						logger.Errorf("error while sending post to webhook: %s", err)
					}
				}
			case err := <-errs:
				logger.Errorf("received error from reddit post stream: %s", err)
			}
		}
	}()
}
