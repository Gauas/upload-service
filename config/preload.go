package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func fromEnv() Config {
	_ = godotenv.Load()

	cfg := Config{
		Port:        envOrDefault("PORT", "8080"),
		SecretKey:   mustEnv("SECRET_KEY"),
		TempDir:     envOrDefault("TEMP_DIR", "/tmp/gau-upload"),
		FileMaxSize: parseInt64(os.Getenv("FILE_MAX_SIZE"), 10*1024*1024),
		CDNURL:      envOrDefault("CDN_URL", ""),
		Storage: StorageConfig{
			Endpoint:  mustEnv("STORAGE_ENDPOINT"),
			AccessKey: mustEnv("STORAGE_ACCESS_KEY"),
			SecretKey: mustEnv("STORAGE_SECRET_KEY"),
			Region:    envOrDefault("STORAGE_REGION", "us-east-1"),
			UseSSL:    envOrDefault("STORAGE_USE_SSL", "false") == "true",
		},
		Queue: QueueConfig{
			Host:     envOrDefault("QUEUE_HOST", "localhost"),
			Port:     envOrDefault("QUEUE_PORT", "5672"),
			Username: envOrDefault("QUEUE_USERNAME", "guest"),
			Password: envOrDefault("QUEUE_PASSWORD", "guest"),
		},
	}

	validate(cfg)
	return cfg
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("config: %s is required", key)
	}
	return v
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func parseInt64(s string, fallback int64) int64 {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	return fallback
}
