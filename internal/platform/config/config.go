package config

import "time"

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Google   GoogleConfig
	Redis    RedisConfig
	Mpesa    MpesaConfig
	Storage  StorageConfig
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

// MpesaConfig holds Safaricom Daraja API credentials.
type MpesaConfig struct {
	ConsumerKey     string
	ConsumerSecret  string
	Passkey         string // Lipa Na M-PESA online passkey
	ShortCode       string // Business short code (till / paybill)
	CallbackURL     string // Public HTTPS URL Safaricom will POST results to
	Environment     string // "sandbox" | "production"
}

// StorageConfig holds S3-compatible object storage credentials.
// Works with AWS S3, MinIO, Cloudflare R2, DigitalOcean Spaces.
type StorageConfig struct {
	Endpoint        string // e.g. "s3.amazonaws.com" or custom MinIO host
	Region          string // e.g. "us-east-1" or "auto" for R2
	Bucket          string // default bucket name
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool   // true in production
	PublicBaseURL   string // CDN/public URL prefix for generating download links
}
