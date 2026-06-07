package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func fromEnv() Config {
	_ = godotenv.Load()

	cfg := Config{
		Port:       envOrDefault("PORT", "8080"),
		GRPCPort:   envOrDefault("GRPC_PORT", "9090"),
		SecretKey:  envOrDefault("SECRET_KEY", envOrDefault("PRIVATE_KEY", "")),
		PrivateKey: envOrDefault("PRIVATE_KEY", envOrDefault("SECRET_KEY", "")),
		CDNURL:     envOrDefault("CDN_URL", ""),
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
		Chunk: ChunkConfig{
			DefaultChunkSize: parseInt64(os.Getenv("DEFAULT_CHUNK_SIZE"), 10485760), // 10MB
			MaxChunkSize:     parseInt64(os.Getenv("MAX_CHUNK_SIZE"), 104857600),   // 100MB
			TempDir:          envOrDefault("TEMP_DIR", "/tmp/gau-upload"),
		},
		Limit: LimitConfig{
			ImageMaxSize: parseInt64(os.Getenv("IMAGE_MAX_SIZE"), 5242880),  // 5MB
			FileMaxSize:  parseInt64(os.Getenv("FILE_MAX_SIZE"), 10485760), // 10MB
		},
		Grafana: GrafanaConfig{
			OTLPEndpoint: parseOTLPEndpoint(envOrDefault("GRAFANA_OTLP_ENDPOINT", "https://grafana.gauas.online")),
			ServiceName:  envOrDefault("SERVICE_NAME", "gau-upload-service"),
		},
		Env: EnvironmentConfig{
			Mode:  envOrDefault("DEPLOY_ENV", "development"),
			Group: envOrDefault("GROUP_NAME", "local"),
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

func parseOTLPEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "http://") {
		return strings.TrimPrefix(endpoint, "http://")
	}
	return endpoint
}
