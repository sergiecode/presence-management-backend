package config

import (
	"log"
	"os"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func LoadGoogleOAuthConfig() *GoogleOAuthConfig {
	cfg := &GoogleOAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("GOOGLE_REDIRECT_URI"),
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" {
		log.Fatal("Missing Google OAuth2 credentials in environment variables")
	}
	return cfg
}
