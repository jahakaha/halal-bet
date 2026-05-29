package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL        string
	TelegramToken      string
	FootballDataKey    string
	SofascoreKey       string
	Port               string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		TelegramToken:   os.Getenv("TELEGRAM_TOKEN"),
		FootballDataKey: os.Getenv("FOOTBALL_DATA_ORG_KEY"),
		SofascoreKey:    os.Getenv("SOFASCORE_RAPIDAPI_KEY"),
		Port:            os.Getenv("PORT"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}
