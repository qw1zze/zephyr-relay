package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort string
	PingInterval int
	PongTimeout int
	PendingTTLDays int
	MaxEnvelopeSize int64
	ChallengesTTLSec int
}

func Load() *Config {
	return &Config{
		ServerPort:       getEnv("SERVER_PORT", "8081"),
		PingInterval:     getEnvInt("PING_INTERVAL_SEC", 30),
		PongTimeout:      getEnvInt("PONG_TIMEOUT_SEC", 60),
		PendingTTLDays:   getEnvInt("PENDING_TTL_DAYS", 30),
		MaxEnvelopeSize:  getEnvInt64("MAX_ENVELOPE_SIZE_KB", 64),
		ChallengesTTLSec: getEnvInt("CHALLENGE_TTL_SEC", 60),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}
