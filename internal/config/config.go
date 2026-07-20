package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	JWTSecret         string
	JWTExpiryHours    int
	APIPort           string
	PipelineWorkers   int
	MaxFilesPerRepo   int
	MaxTokensPerBatch int
	GeminiAPIKey      string
	GeminiModel       string
	GeminiRPM         int
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

	pipelineWorkers := 2
	if v := os.Getenv("PIPELINE_WORKERS"); v != "" {
		if inputPipelineWorkers, err := strconv.Atoi(v); err == nil {
			pipelineWorkers = max(1, inputPipelineWorkers)
		}
	}

	maxFiles := 50
	if v := os.Getenv("MAX_FILES_PER_REPO"); v != "" {
		if inputMaxFiles, err := strconv.Atoi(v); err == nil {
			maxFiles = max(1, inputMaxFiles)
		}
	}

	maxTokensPerBatch := 50000
	if v := os.Getenv("PIPELINE_MAX_TOKENS_PER_BATCH"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			maxTokensPerBatch = max(1, parsed)
		}
	}

	GeminiAPIKey := os.Getenv("GEMINI_API_KEY")

	GeminiModel := "gemini-3.1-flash-lite"
	if v := os.Getenv("GEMINI_MODEL"); v != "" {
		GeminiModel = v
	}

	GeminiRPM := 10
	if v := os.Getenv("GEMINI_RPM_LIMIT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			GeminiRPM = max(1, parsed)
		}
	}

	cfg := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		JWTExpiryHours:    jwtExpiry,
		APIPort:           os.Getenv("API_PORT"),
		PipelineWorkers:   pipelineWorkers,
		MaxFilesPerRepo:   maxFiles,
		MaxTokensPerBatch: maxTokensPerBatch,
		GeminiAPIKey:      GeminiAPIKey,
		GeminiModel:       GeminiModel,
		GeminiRPM:         GeminiRPM,
	}

	return cfg
}
