package config

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

type JWTConfig struct {
	Secret string
}

type GoogleConfig struct {
	ClientID string
}

type RedisConfig struct {
	Host string
	Port string
}