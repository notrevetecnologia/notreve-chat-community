package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	AdminPass   string
	RedisURL    string
	RabbitURL   string
}

func Load() Config {
	return Config{
		HTTPAddr:    getEnv("AGENTE_IA_ADDR", "127.0.0.1:8080"),
		DatabaseURL: getEnv("AGENTE_IA_DB", "postgres://agente_app@127.0.0.1:5433/agente_ia?sslmode=disable"),
		AdminPass:   getEnv("AGENTE_IA_PASS", ""),
		RedisURL:    getEnv("AGENTE_IA_REDIS", ""),
		RabbitURL:   getEnv("AGENTE_IA_RABBITMQ", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
