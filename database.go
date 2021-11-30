package main

import (
	"fmt"
	"os"
	"time"

	"github.com/DisgoOrg/disgo/discord"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dbUser = os.Getenv("db_user")
var dbPassword = os.Getenv("db_password")
var dbHost = os.Getenv("db_host")
var dbPort = os.Getenv("db_port")
var dbName = os.Getenv("db_name")

var database *gorm.DB

type SubredditSubscription struct {
	ID           uint              `gorm:"primarykey"`
	Subreddit    string            `gorm:"uniqueIndex:Subreddit_ChannelID"`
	GuildID      discord.Snowflake `gorm:"uniqueIndex:Subreddit_ChannelID"`
	ChannelID    discord.Snowflake
	WebhookID    discord.Snowflake
	WebhookToken string
}

func connectToDatabase() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", dbHost, dbUser, dbPassword, dbName, dbPort)
	var err error
	database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatalf("error while connecting to db: %s", err)
	}
	db, err := database.DB()
	if err != nil {
		logger.Fatalf("error getting db: %s", err)
	}
	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 10)

	err = database.AutoMigrate(&SubredditSubscription{})
	if err != nil {
		logger.Fatalf("failed to auto-migrate db: %s", err)
	}
}
