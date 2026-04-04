package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	APIPort     string
}

func Load() Config {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Printf("Warning: could not load .env file: %v\n", err)
	}

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		APIPort:     os.Getenv("API_PORT"),
	}

	return cfg
}
