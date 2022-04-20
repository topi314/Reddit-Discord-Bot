package dbot

import (
	"github.com/disgoorg/snowflake"
)

type Config struct {
	DevMode         bool                  `json:"dev_mode"`
	DevGuildIDs     []snowflake.Snowflake `json:"dev_guild_ids"`
	SupportGuildID  snowflake.Snowflake   `json:"support_guild_id"`
	LogLevel        string                `json:"log_level"`
	ErrorLogWebhook LogWebhookConfig      `json:"error_log_webhook"`
	Token           string                `json:"token"`
	Database        DatabaseConfig        `json:"database"`
}

type LogWebhookConfig struct {
	ID    snowflake.Snowflake `json:"id"`
	Token string              `json:"token"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"db_name"`
	SSLMode  string `json:"ssl_mode"`
}
