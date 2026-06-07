package config

import (
	"github.com/joho/godotenv"
)

func fromEnv() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:        get("PORT", "8080"),
		GRPCPort:    get("GRPC_PORT", "9090"),
		SecretKey:   mustEnv("SECRET_KEY"),
		TempDir:     get("TEMP_DIR", "/tmp/gau-upload"),
		FileMaxSize: getEnvInt64("FILE_MAX_SIZE", 10*1024*1024),
		CDNURL:      get("CDN_URL", ""),
		Storage: StorageConfig{
			Endpoint:  mustEnv("STORAGE_ENDPOINT"),
			AccessKey: mustEnv("STORAGE_ACCESS_KEY"),
			SecretKey: mustEnv("STORAGE_SECRET_KEY"),
			Region:    get("STORAGE_REGION", "auto"),
			UseSSL:    get("STORAGE_USE_SSL", "false") == "true",
		},
		Queue: QueueConfig{
			Host:     get("QUEUE_HOST", "localhost"),
			Port:     get("QUEUE_PORT", "5672"),
			Username: get("QUEUE_USERNAME", "guest"),
			Password: get("QUEUE_PASSWORD", "guest"),
		},
	}

	validate(cfg)
	return cfg
}
