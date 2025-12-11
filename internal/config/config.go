package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort  string
	DatabaseURL string

	Skinport struct {
		APIURL   string
		ClientID string
		APIKey   string
	}
}

func Load() (*Config, error) {
	// Load .env file if it exists (useful for local dev)
	_ = godotenv.Load()

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be set")
	}

	skinportAPIURL := os.Getenv("SKINPORT_API_URL")
	if skinportAPIURL == "" {
		return nil, fmt.Errorf("SKINPORT_API_URL must be set")
	}

	skinportClientID := os.Getenv("SKINPORT_CLIENT_ID")
	if skinportClientID == "" {
		return nil, fmt.Errorf("SKINPORT_CLIENT_ID must be set")
	}

	skinportAPIKey := os.Getenv("SKINPORT_API_KEY")
	if skinportAPIKey == "" {
		return nil, fmt.Errorf("SKINPORT_API_KEY must be set")
	}

	return &Config{
		ServerPort:  serverPort,
		DatabaseURL: databaseURL,
		Skinport: struct {
			APIURL   string
			ClientID string
			APIKey   string
		}{
			APIURL:   skinportAPIURL,
			ClientID: skinportClientID,
			APIKey:   skinportAPIKey,
		},
	}, nil
}
