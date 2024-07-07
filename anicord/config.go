package anicord

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

var defaultConfig = Config{
	Log: LogConfig{
		Level: slog.LevelInfo,
	},
}

func ReadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open config: %w", err)
	}
	defer file.Close()

	cfg := defaultConfig
	if err = yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to decode config: %w", err)
	}
	return cfg, nil
}

type Config struct {
	Log     LogConfig     `yaml:"log"`
	Server  ServerConfig  `yaml:"server"`
	Discord DiscordConfig `yaml:"discord"`
	Anilist AnilistConfig `yaml:"anilist"`
	DB      DBConfig      `yaml:"db"`
}

type LogConfig struct {
	Level     slog.Level `cfg:"level"`
	Format    string     `cfg:"format"`
	AddSource bool       `cfg:"add_source"`
	NoColor   bool       `cfg:"no_color"`
}

type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	BaseURL    string `yaml:"base_url"`
}

type DiscordConfig struct {
	Token        string `yaml:"token"`
	ClientSecret string `yaml:"client_secret"`
	PublicKey    string `yaml:"public_key"`
}

type AnilistConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type DBConfig struct {
	Type string `yaml:"type"`

	// SQLite
	Path string `yaml:"path"`

	// PostgreSQL
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`
}

func (c DBConfig) PostgresDataSourceName() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Database,
		c.SSLMode,
	)
}
