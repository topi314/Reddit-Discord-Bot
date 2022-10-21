package dbot

import (
	"github.com/disgoorg/snowflake/v2"
)

type Config struct {
	DevMode          bool             `json:"dev_mode"`
	DevGuildIDs      []snowflake.ID   `json:"dev_guild_ids"`
	SupportGuildID   snowflake.ID     `json:"support_guild_id"`
	SupportInviteURL string           `json:"support_invite_url"`
	LogLevel         string           `json:"log_level"`
	ErrorLogWebhook  LogWebhookConfig `json:"error_log_webhook"`
	Bot              BotConfig        `json:"bot"`
	Reddit           RedditConfig     `json:"reddit"`
	Database         DatabaseConfig   `json:"database"`
}

type LogWebhookConfig struct {
	ID    snowflake.ID `json:"id"`
	Token string       `json:"token"`
}

type BotConfig struct {
	Token         string `json:"token"`
	PublicKey     string `json:"public_key"`
	Secret        string `json:"secret"`
	RedirectURL   string `json:"redirect_url"`
	ServerAddress string `json:"server_address"`
}

type RedditConfig struct {
	ID       string `json:"id"`
	Secret   string `json:"secret"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"db_name"`
	SSLMode  string `json:"ssl_mode"`
}
