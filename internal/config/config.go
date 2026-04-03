package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	APIPort     string
}

func Load() Config {
	godotenv.Load(".env")

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		APIPort:     os.Getenv("API_PORT"),
	}

	return cfg
}
