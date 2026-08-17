package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	RedisURL         string
	MinIOEndpoint        string
	MinIOPublicEndpoint  string
	MinIOAccessKey   string
	MinIOSecretKey   string
	MinIOBucket      string
	MinIOUseSSL      bool
	QuotaBytes       int64
	MaxUploadBytes   int64
	ProcessorToken   string
	DevAuth          bool
	OIDCIssuer       string
	OIDCAudience     string
	PresignTTL       int // seconds
}

func FromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr:       envOr("MEDIA_HTTP_ADDR", ":8090"),
		DatabaseURL:    strings.TrimSpace(os.Getenv("MEDIA_DATABASE_URL")),
		RedisURL:       envOr("MEDIA_REDIS_URL", "redis://redis:6379/0"),
		MinIOEndpoint:       envOr("MEDIA_MINIO_ENDPOINT", "minio:9000"),
		MinIOPublicEndpoint: strings.TrimSpace(os.Getenv("MEDIA_MINIO_PUBLIC_ENDPOINT")),
		MinIOAccessKey: envOr("MEDIA_MINIO_ACCESS_KEY", "minio"),
		MinIOSecretKey: envOr("MEDIA_MINIO_SECRET_KEY", "minio12345"),
		MinIOBucket:    envOr("MEDIA_MINIO_BUCKET", "media"),
		MinIOUseSSL:    envBool("MEDIA_MINIO_USE_SSL", false),
		ProcessorToken: strings.TrimSpace(os.Getenv("MEDIA_PROCESSOR_TOKEN")),
		DevAuth:        envBool("MEDIA_DEV_AUTH", true),
		OIDCIssuer:     strings.TrimRight(strings.TrimSpace(os.Getenv("OIDC_ISSUER")), "/"),
		OIDCAudience:   strings.TrimSpace(os.Getenv("OIDC_AUDIENCE")),
		PresignTTL:     envInt("MEDIA_PRESIGN_TTL_SEC", 900),
	}
	var err error
	cfg.QuotaBytes, err = envInt64("MEDIA_QUOTA_BYTES", 100*1024*1024)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxUploadBytes, err = envInt64("MEDIA_MAX_UPLOAD_BYTES", 20*1024*1024)
	if err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("MEDIA_DATABASE_URL is required")
	}
	if cfg.ProcessorToken == "" {
		return Config{}, fmt.Errorf("MEDIA_PROCESSOR_TOKEN is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) (int64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	return strconv.ParseInt(v, 10, 64)
}
