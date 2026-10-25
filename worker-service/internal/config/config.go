package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisURL, MySQLURL, StreamName, ConsumerGroup, HTTPAddr            string
	WorkerCount                                                        int
	Lease, RecoveryInterval, RecoveryIdle, ShutdownTimeout, JobTimeout time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	c := &Config{RedisURL: os.Getenv("REDIS_URL"), MySQLURL: os.Getenv("MYSQL_URL"), StreamName: env("STREAM_NAME", "jobs:queue"), ConsumerGroup: env("CONSUMER_GROUP", "scalequeue-workers"), HTTPAddr: env("HTTP_ADDR", ":9090")}
	var err error
	c.WorkerCount, err = strconv.Atoi(env("WORKER_COUNT", "3"))
	if err != nil || c.WorkerCount < 1 || c.WorkerCount > 100 {
		return nil, fmt.Errorf("WORKER_COUNT must be 1..100")
	}
	for k, p := range map[string]*time.Duration{"LEASE_DURATION": &c.Lease, "RECOVERY_INTERVAL": &c.RecoveryInterval, "RECOVERY_IDLE": &c.RecoveryIdle, "SHUTDOWN_TIMEOUT": &c.ShutdownTimeout, "JOB_TIMEOUT": &c.JobTimeout} {
		defaults := map[string]string{"LEASE_DURATION": "30s", "RECOVERY_INTERVAL": "5s", "RECOVERY_IDLE": "30s", "SHUTDOWN_TIMEOUT": "25s", "JOB_TIMEOUT": "2m"}
		*p, err = time.ParseDuration(env(k, defaults[k]))
		if err != nil || *p < time.Millisecond {
			return nil, fmt.Errorf("invalid %s", k)
		}
	}
	if c.Lease < 30*time.Millisecond {
		return nil, fmt.Errorf("lease must be at least 30ms")
	}
	if c.RedisURL == "" || c.MySQLURL == "" {
		return nil, fmt.Errorf("REDIS_URL and MYSQL_URL are required")
	}
	return c, nil
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
