package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env                string
	HTTPAddr           string
	DatabaseURL        string
	RedisURL           string
	S3Endpoint         string
	S3PublicEndpoint   string
	S3Region           string
	S3AccessKey        string
	S3SecretKey        string
	S3Bucket           string
	S3UseSSL           bool
	QuotaBytes         int64
	MaxUploadBytes     int64
	ProcessorToken     string
	DevAuth            bool
	OIDCIssuer         string
	OIDCInternalBase   string
	OIDCAudience       string
	PresignTTL         int // seconds
}

const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

func FromEnv() (Config, error) {
	cfg := Config{
		Env:              normalizeEnv(os.Getenv("MEDIA_ENV")),
		HTTPAddr:         envOr("MEDIA_HTTP_ADDR", ":8090"),
		DatabaseURL:      strings.TrimSpace(os.Getenv("MEDIA_DATABASE_URL")),
		RedisURL:         envOr("MEDIA_REDIS_URL", "redis://redis:6379/0"),
		S3Endpoint:       envOrAlt("MEDIA_S3_ENDPOINT", "MEDIA_MINIO_ENDPOINT", "garage:3900"),
		S3PublicEndpoint: strings.TrimSpace(firstNonEmpty(os.Getenv("MEDIA_S3_PUBLIC_ENDPOINT"), os.Getenv("MEDIA_MINIO_PUBLIC_ENDPOINT"))),
		S3Region:         envOrAlt("MEDIA_S3_REGION", "MEDIA_MINIO_REGION", "garage"),
		S3AccessKey:      envOrAlt("MEDIA_S3_ACCESS_KEY", "MEDIA_MINIO_ACCESS_KEY", "GKdevmedia00000001"),
		S3SecretKey:      envOrAlt("MEDIA_S3_SECRET_KEY", "MEDIA_MINIO_SECRET_KEY", "dev-media-secret-key-for-local-compose-demo"),
		S3Bucket:         envOrAlt("MEDIA_S3_BUCKET", "MEDIA_MINIO_BUCKET", "media"),
		S3UseSSL:         envBoolAlt("MEDIA_S3_USE_SSL", "MEDIA_MINIO_USE_SSL", false),
		ProcessorToken:   strings.TrimSpace(os.Getenv("MEDIA_PROCESSOR_TOKEN")),
		DevAuth:          envBool("MEDIA_DEV_AUTH", true),
		OIDCIssuer:       strings.TrimRight(strings.TrimSpace(os.Getenv("OIDC_ISSUER")), "/"),
		OIDCInternalBase: strings.TrimRight(strings.TrimSpace(os.Getenv("OIDC_INTERNAL_BASE")), "/"),
		OIDCAudience:     strings.TrimSpace(os.Getenv("OIDC_AUDIENCE")),
		PresignTTL:       envInt("MEDIA_PRESIGN_TTL_SEC", 900),
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
	if (cfg.Env == EnvStaging || cfg.Env == EnvProduction) && cfg.DevAuth {
		return Config{}, fmt.Errorf("MEDIA_DEV_AUTH must be false when MEDIA_ENV=%s", cfg.Env)
	}
	if (cfg.Env == EnvStaging || cfg.Env == EnvProduction) && cfg.OIDCIssuer == "" {
		return Config{}, fmt.Errorf("OIDC_ISSUER is required when MEDIA_ENV=%s", cfg.Env)
	}
	if cfg.Env != EnvDevelopment && cfg.Env != EnvStaging && cfg.Env != EnvProduction {
		return Config{}, fmt.Errorf("unsupported MEDIA_ENV %q (use development, staging, or production)", cfg.Env)
	}
	return cfg, nil
}

func normalizeEnv(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "dev", "development", "local", "demo":
		return EnvDevelopment
	case "staging", "stage":
		return EnvStaging
	case "production", "prod":
		return EnvProduction
	default:
		return strings.ToLower(strings.TrimSpace(v))
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envOrAlt(primary, legacy, fallback string) string {
	if v := envOr(primary, ""); v != "" {
		return v
	}
	return envOr(legacy, fallback)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
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

func envBoolAlt(primary, legacy string, fallback bool) bool {
	if strings.TrimSpace(os.Getenv(primary)) != "" {
		return envBool(primary, fallback)
	}
	if strings.TrimSpace(os.Getenv(legacy)) != "" {
		return envBool(legacy, fallback)
	}
	return fallback
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
