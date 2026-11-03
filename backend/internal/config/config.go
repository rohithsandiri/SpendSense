package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	LogstashHost string
	LogstashPort string
}

func Load() Config {
	return Config{
		Port:         env("PORT", "5000"),
		DatabaseURL:  env("DATABASE_URL", "postgres://spendsense:spendsense@127.0.0.1:5432/spendsense?sslmode=disable"),
		JWTSecret:    env("JWT_SECRET", "spendsense_secret_2026"),
		LogstashHost: os.Getenv("LOGSTASH_HOST"),
		LogstashPort: env("LOGSTASH_PORT", "5044"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
