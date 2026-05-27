package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/gauas/config-service/sdk"
	"github.com/joho/godotenv"
)

const localConfigPath = ".config/config.json"

func fromFile() (Config, bool) {
	data, err := os.ReadFile(localConfigPath)
	if err != nil {
		return Config{}, false
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Fatalf("config: invalid %s: %v", localConfigPath, err)
	}

	get := func(key string) string {
		v, _ := raw[key].(string)
		if v == "" {
			log.Fatalf("config: %s is required in %s", key, localConfigPath)
		}
		return v
	}
	opt := func(key, fallback string) string {
		if v, _ := raw[key].(string); v != "" {
			return v
		}
		return fallback
	}
	getInt64 := func(key string, fallback int64) int64 {
		if v, _ := raw[key].(float64); v > 0 {
			return int64(v)
		}
		return fallback
	}

	cfg := Config{
		Port:        opt("PORT", "8080"),
		SecretKey:   get("SECRET_KEY"),
		TempDir:     opt("TEMP_DIR", "/tmp/gau-upload"),
		FileMaxSize: getInt64("FILE_MAX_SIZE", 10*1024*1024),
		Storage: StorageConfig{
			Endpoint:  get("STORAGE_ENDPOINT"),
			AccessKey: get("STORAGE_ACCESS_KEY"),
			SecretKey: get("STORAGE_SECRET_KEY"),
			Region:    opt("STORAGE_REGION", "us-east-1"),
			UseSSL:    raw["STORAGE_USE_SSL"] == "true",
		},
		Queue: QueueConfig{
			Host:     opt("QUEUE_HOST", "localhost"),
			Port:     opt("QUEUE_PORT", "5672"),
			Username: opt("QUEUE_USERNAME", "guest"),
			Password: opt("QUEUE_PASSWORD", "guest"),
		},
	}

	validate(cfg)
	return cfg, true
}

func fromSDK() Config {
	_ = godotenv.Load()

	client := sdk.New(sdk.Options{
		BaseURL:   mustEnv("CONFIG_SERVICE_URL"),
		SecretKey: mustEnv("SECRET_KEY"),
	})

	remote, err := client.Get("upload-service", mustEnv("ENVIRONMENT"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	cfg := Config{
		Port:        remote.GetString("PORT", "8080"),
		SecretKey:   mustEnv("SECRET_KEY"),
		TempDir:     remote.GetString("TEMP_DIR", "/tmp/gau-upload"),
		FileMaxSize: parseInt64(remote.GetString("FILE_MAX_SIZE", ""), 10*1024*1024),
		Storage: StorageConfig{
			Endpoint:  remote.GetString("STORAGE_ENDPOINT", ""),
			AccessKey: remote.GetString("STORAGE_ACCESS_KEY", ""),
			SecretKey: remote.GetString("STORAGE_SECRET_KEY", ""),
			Region:    remote.GetString("STORAGE_REGION", "us-east-1"),
			UseSSL:    remote.GetString("STORAGE_USE_SSL", "") == "true",
		},
		Queue: QueueConfig{
			Host:     remote.GetString("QUEUE_HOST", "localhost"),
			Port:     remote.GetString("QUEUE_PORT", "5672"),
			Username: remote.GetString("QUEUE_USERNAME", "guest"),
			Password: remote.GetString("QUEUE_PASSWORD", "guest"),
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

func parseInt64(s string, fallback int64) int64 {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	return fallback
}
