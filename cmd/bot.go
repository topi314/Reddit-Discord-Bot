package main

import (
	"os"
	"regexp"

	"github.com/TopiSenpai/Reddit-Discord-Bot/dbot"
	"github.com/disgoorg/snowflake"
	"github.com/sirupsen/logrus"
)

var (
	token                = os.Getenv("token")
	publicKey            = os.Getenv("public_key")
	secret               = os.Getenv("secret")
	baseURL              = os.Getenv("base_url")
	webhookServerAddress = os.Getenv("webhook_server_address")

	loglevel, _     = logrus.ParseLevel(os.Getenv("log_level"))
	logWebhookID    = snowflake.GetSnowflakeEnv("log_webhook_id")
	logWebhookToken = os.Getenv("log_webhook_token")

	redditID       = os.Getenv("reddit_id")
	redditSecret   = os.Getenv("reddit_secret")
	redditUsername = os.Getenv("reddit_username")
	redditPassword = os.Getenv("reddit_password")

	imageRegex = regexp.MustCompile(`.*\.(?:jpg|gif|png)`)
)

func main() {
	var err error
	logger := logrus.New(log.Ldate | log.Ltime | log.Lshortfile)

	bot := &dbot.Bot{
		Logger:  logger,
		Version: version,
	}
	bot.Logger.Infof("Starting dbot version: %s", version)
	bot.Logger.Infof("Syncing commands? %v", *shouldSyncCommands)
	bot.Logger.Infof("Syncing DB tables? %v", *shouldSyncDBTables)
	bot.Logger.Infof("Exiting after syncing? %v", *exitAfterSync)
}
