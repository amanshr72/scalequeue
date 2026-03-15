package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisURL      string
	MySQLURL      string
	WorkerCount   int
	ConsumerGroup string
	StreamName    string
	WorkerID      string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found, using environment variables")
	}

	workerCount, err := strconv.Atoi(getEnv("WORKER_COUNT", "3"))
	if err != nil {
		workerCount = 3
	}

	cfg := &Config{
		RedisURL:      mustGetEnv("REDIS_URL"),
		MySQLURL:      mustGetEnv("MYSQL_URL"),
		WorkerCount:   workerCount,
		ConsumerGroup: getEnv("CONSUMER_GROUP", "scalequeue-workers"),
		StreamName:    getEnv("STREAM_NAME", "jobs:queue"),
		WorkerID:      getEnv("WORKER_ID", "worker-1"),
	}

	return cfg
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("[Config] Required environment variable %s is not set", key)
	}
	return val
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
