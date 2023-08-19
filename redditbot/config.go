package redditbot

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/disgoorg/log"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func ReadConfig() (Config, error) {
	f := flag.NewFlagSet("config", flag.ExitOnError)
	path := f.String("config", "./config.yml", "Endpoint to config file (default: ./config.yml)")

	f.Bool("test_mode", false, "Test mode (default: false)")

	f.Int("log.level", 2, "Log level (0: trace, 1: debug, 2: info, 3: warn, 4: error, 5: fatal, 6: panic)")
	f.Bool("log.add_source", false, "Add source to log ")

	f.Bool("server.enabled", true, "Server enabled")
	f.String("server.listen_addr", "0.0.0.0:8080", "Server listen address")
	f.String("server.endpoint", "/callback/discord", "Server endpoint")
	f.String("server.redirect_url", "", "Server redirect URL")

	f.String("discord.token", "", "Discord bot token")
	f.String("discord.client_secret", "", "Discord client secret")
	f.Bool("discord.sync_commands", true, "Sync Discord commands (default: true)")

	f.String("reddit.client_id", "", "Reddit client ID")
	f.String("reddit.client_secret", "", "Reddit client secret")
	f.Int("reddit.requests_per_minute", 59, "Reddit requests per minute (default: 59)")
	f.Int("reddit.max_pages", 2, "Reddit max pages (default: 2)")

	f.String("database.type", string(DatabaseTypeSQLite), "Database type (sqlite, postgres)")

	f.String("database.sqlite.path", "./database.db", "SQLite path (default: ./database.db))")

	f.String("database.postgres.host", "localhost", "Postgres host (default: localhost)")
	f.Int("database.postgres.port", 5432, "Postgres port (default: 5432)")
	f.String("database.postgres.username", "postgres", "Postgres username (default: postgres)")
	f.String("database.postgres.password", "postgres", "Postgres password (default: postgres)")
	f.String("database.postgres.database", "reddit-bot", "Postgres database (default: reddit-bot)")
	f.String("database.postgres.ssl_mode", "disable", "Postgres SSL mode (default: disable)")

	f.String("otel.enabled", "false", "Otel enabled (default: false)")
	f.String("otel.instance_id", "01", "Otel instance ID (default: 01)")
	f.String("otel.metrics.listen_addr", ":8081", "Otel metrics listen address (default: :8081")
	f.String("otel.metrics.endpoint", "/metrics", "Otel metrics endpoint (default: /metrics)")

	if err := f.Parse(os.Args[1:]); err != nil {
		return Config{}, err
	}

	k := koanf.New(".")
	log.Info("Loading config from:", *path)
	if err := k.Load(file.Provider(*path), yaml.Parser()); err != nil {
		return Config{}, err
	}

	if err := k.Load(env.Provider("REDDIT_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "REDDIT_")), "_", ".", -1)
	}), nil); err != nil {
		return Config{}, err
	}

	// if err := k.Load(basicflag.Provider(f, "."), nil); err != nil {
	// 	return Config{}, err
	// }

	var config Config
	err := k.Unmarshal("", &config)
	return config, err
}

type Config struct {
	TestMode bool           `koanf:"test_mode"`
	Log      LogConfig      `koanf:"log"`
	Server   ServerConfig   `koanf:"server"`
	Discord  DiscordConfig  `koanf:"discord"`
	Reddit   RedditConfig   `koanf:"reddit"`
	Database DatabaseConfig `koanf:"database"`
	Metrics  MetricsConfig  `koanf:"metrics"`
}

func (c Config) String() string {
	return fmt.Sprintf("\nTestMode: %t\nLog: %s\nServer: %s\nDiscord: %s\nReddit: %v\nDatabase: %s\nMetrics: %s",
		c.TestMode,
		c.Log,
		c.Server,
		c.Discord,
		c.Reddit,
		c.Database,
		c.Metrics,
	)
}

func (c Config) Validate() error {
	if err := c.Log.Validate(); err != nil {
		return err
	}
	if err := c.Server.Validate(); err != nil {
		return err
	}
	if err := c.Discord.Validate(); err != nil {
		return err
	}
	if err := c.Reddit.Validate(); err != nil {
		return err
	}
	if err := c.Database.Validate(); err != nil {
		return err
	}
	return nil
}

type LogConfig struct {
	Level     log.Level `koanf:"level"`
	AddSource bool      `koanf:"add_source"`
}

func (c LogConfig) String() string {
	return fmt.Sprintf("\n  Level: %v\n  AddSource: %v",
		c.Level,
		c.AddSource,
	)
}

func (c LogConfig) Validate() error {
	if c.Level != log.LevelTrace && c.Level != log.LevelDebug && c.Level != log.LevelInfo && c.Level != log.LevelWarn && c.Level != log.LevelError && c.Level != log.LevelFatal && c.Level != log.LevelPanic {
		return fmt.Errorf("log.level must be one of: 0 (trace), 1 (debug), 2 (info), 3 (warn), 4 (error), 5 (fatal), 6 (panic)")
	}
	return nil
}

func (c LogConfig) Flags() int {
	flags := log.LstdFlags
	if c.AddSource {
		flags |= log.Lshortfile
	}
	return flags
}

type ServerConfig struct {
	Enabled     bool   `koanf:"enabled"`
	ListenAddr  string `koanf:"listen_addr"`
	Endpoint    string `koanf:"endpoint"`
	RedirectURL string `koanf:"redirect_url"`
}

func (c ServerConfig) String() string {
	return fmt.Sprintf("\n  Enabled: %v\n  ListenAddr: %v\n  Endpoint: %v\n  RedirectURL: %v",
		c.Enabled,
		c.ListenAddr,
		c.Endpoint,
		c.RedirectURL,
	)
}

func (c ServerConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("server.listen_addr must be set")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("server.endpoint must be set")
	}
	if c.RedirectURL == "" {
		return fmt.Errorf("server.redirect_url must be set")
	}
	return nil
}

type DiscordConfig struct {
	Token        string `koanf:"token"`
	ClientSecret string `koanf:"client_secret"`
	SyncCommands bool   `koanf:"sync_commands"`
}

func (c DiscordConfig) String() string {
	return fmt.Sprintf("\n  Token: %s\n  ClientSecret: %s\n  SyncCommands: %t",
		strings.Repeat("*", len(c.Token)),
		strings.Repeat("*", len(c.ClientSecret)),
		c.SyncCommands,
	)
}

func (c DiscordConfig) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("discord.token must be set")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("discord.client_secret must be set")
	}
	return nil
}

type RedditConfig struct {
	ClientID          string `koanf:"client_id"`
	ClientSecret      string `koanf:"client_secret"`
	RequestsPerMinute int    `koanf:"requests_per_minute"`
	MaxPages          int    `koanf:"max_pages"`
}

func (c RedditConfig) String() string {
	return fmt.Sprintf("\n  ClientID: %s\n  ClientSecret: %s\n  RequestsPerMinute: %d",
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
		c.RequestsPerMinute,
	)
}

func (c RedditConfig) Validate() error {
	if c.ClientID == "" {
		return fmt.Errorf("reddit.client_id must be set")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("reddit.client_secret must be set")
	}
	if c.RequestsPerMinute <= 0 {
		return fmt.Errorf("reddit.requests_per_minute must be greater than 0")
	}
	return nil
}

type DatabaseType string

const (
	DatabaseTypePostgres DatabaseType = "postgres"
	DatabaseTypeSQLite   DatabaseType = "sqlite"
)

type DatabaseConfig struct {
	Type           DatabaseType   `cfg:"type"`
	PostgresConfig PostgresConfig `koanf:"postgres"`
	SQLite         SQLiteConfig   `koanf:"sqlite"`
}

func (c DatabaseConfig) String() string {
	return fmt.Sprintf("\n  Type: %v\n  Postgres: %v\n  SQLite: %v",
		c.Type,
		c.PostgresConfig,
		c.SQLite,
	)
}

func (c DatabaseConfig) Validate() error {
	switch c.Type {
	case DatabaseTypePostgres:
		return c.PostgresConfig.Validate()
	case DatabaseTypeSQLite:
		return c.SQLite.Validate()
	default:
		return fmt.Errorf("unknown database type: %s", c.Type)
	}
}

type PostgresConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	Database string `koanf:"database"`
	SSLMode  string `koanf:"ssl_mode"`
}

func (c PostgresConfig) String() string {
	return fmt.Sprintf("\n   Host: %v\n   Port: %v\n   Username: %v\n   Password: %v\n   Database: %v\n   SSLMode: %v",
		c.Host,
		c.Port,
		c.Username,
		strings.Repeat("*", len(c.Password)),
		c.Database,
		c.SSLMode,
	)
}

func (c PostgresConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("database.postgres.host must be set")
	}
	if c.Port == 0 {
		return fmt.Errorf("database.postgres.port must be set")
	}
	if c.Username == "" {
		return fmt.Errorf("database.postgres.username must be set")
	}
	if c.Password == "" {
		return fmt.Errorf("database.postgres.password must be set")
	}
	if c.Database == "" {
		return fmt.Errorf("database.postgres.database must be set")
	}
	if c.SSLMode == "" {
		return fmt.Errorf("database.postgres.ssl_mode must be set")
	}
	return nil
}

func (c PostgresConfig) DataSourceName() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.Username,
		c.Password,
		c.Database,
		c.SSLMode,
	)
}

type SQLiteConfig struct {
	Path string `koanf:"path"`
}

func (c SQLiteConfig) String() string {
	return fmt.Sprintf("\n   Path: %v",
		c.Path,
	)
}

func (c SQLiteConfig) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("database.sqlite.path must be set")
	}
	return nil
}

func (c SQLiteConfig) DataSourceName() string {
	return c.Path
}

type MetricsConfig struct {
	Enabled    bool   `koanf:"enabled"`
	ListenAddr string `koanf:"listen_addr"`
	Endpoint   string `koanf:"endpoint"`
}

func (c MetricsConfig) String() string {
	return fmt.Sprintf("\n Enabled: %v\n ListenAddr: %v\n Endpoint: %v",
		c.Enabled,
		c.ListenAddr,
		c.Endpoint,
	)
}

func (c MetricsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("metrics.listen_addr must be set")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("metrics.endpoint must be set")
	}
	return nil
}
