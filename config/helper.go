package config

import (
	"log"
	"os"
	"strconv"
)

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	log.Fatalf("config: %s is required", key)
	return ""
}

func getEnvInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}

	return n
}
