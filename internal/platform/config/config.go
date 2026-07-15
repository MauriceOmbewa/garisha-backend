package config

import "time"

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Google   GoogleConfig
	Redis    RedisConfig
}

type AppConfig struct {
	Name string
	Env  string
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// JWTConfig holds signing secret and token lifetimes.
// AccessTTL and RefreshTTL are parsed from environment variables as
// Go duration strings (e.g. "15m", "168h").
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type GoogleConfig struct {
	ClientID string
}

type RedisConfig struct {
	Host string
	Port string
}
