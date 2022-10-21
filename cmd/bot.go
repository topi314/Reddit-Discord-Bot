package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/TopiSenpai/Reddit-Discord-Bot/dbot"
	"github.com/sirupsen/logrus"
)

const version = "dev"

func main() {
	var err error
	logger := logrus.New()

	bot := &dbot.Bot{
		Logger:  logger,
		Version: version,
	}
	bot.Logger.Infof("Starting dbot version: %s", version)

	if err = bot.Setup(); err != nil {
		bot.Logger.Errorf("Error setting up bot: %v", err)
		return
	}

	if err = bot.Start(); err != nil {
		bot.Logger.Errorf("Error starting bot: %v", err)
		return
	}

	bot.Logger.Info("Bot is running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}
