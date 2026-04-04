package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	JWTSecret      string
	JWTExpiryHours int
	APIPort        string
}

func Load() Config {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Printf("Warning: could not load .env file: %v\n", err)
	}

	jwtExpiry := 24 // default: 24 hours
	if v := os.Getenv("JWT_EXPIRY_HOURS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			jwtExpiry = parsed
		}
	}

	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTExpiryHours: jwtExpiry,
		APIPort:        os.Getenv("API_PORT"),
	}

	return cfg
}
