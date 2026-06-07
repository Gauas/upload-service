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

type ChunkConfig struct {
	DefaultChunkSize int64
	MaxChunkSize     int64
	TempDir          string
}

type LimitConfig struct {
	ImageMaxSize int64
	FileMaxSize  int64
}

type GrafanaConfig struct {
	OTLPEndpoint string
	ServiceName  string
}

type EnvironmentConfig struct {
	Mode  string
	Group string
}

type Config struct {
	Port       string
	GRPCPort   string
	SecretKey  string
	PrivateKey string
	CDNURL     string
	Storage    StorageConfig
	Queue      QueueConfig
	Chunk      ChunkConfig
	Limit      LimitConfig
	Grafana    GrafanaConfig
	Env        EnvironmentConfig
}

func New() Config {
	return fromEnv()
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
