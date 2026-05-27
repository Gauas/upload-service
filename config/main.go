package config

import "log"

type StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
	UseSSL    bool
}

type QueueConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

type Config struct {
	Port        string
	SecretKey   string
	TempDir     string
	FileMaxSize int64
	Storage     StorageConfig
	Queue       QueueConfig
}

func New() Config {
	if cfg, ok := fromFile(); ok {
		return cfg
	}
	return fromSDK()
}

func validate(cfg Config) {
	if cfg.Port == "" {
		log.Fatal("config: PORT is required")
	}
	if cfg.SecretKey == "" {
		log.Fatal("config: SECRET_KEY is required")
	}
	if cfg.Storage.Endpoint == "" {
		log.Fatal("config: STORAGE_ENDPOINT is required")
	}
	if cfg.Storage.AccessKey == "" {
		log.Fatal("config: STORAGE_ACCESS_KEY is required")
	}
}
