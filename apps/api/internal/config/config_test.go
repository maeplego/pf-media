package config

import "testing"

func TestFromEnvStagingRejectsDevAuth(t *testing.T) {
	t.Setenv("MEDIA_ENV", "staging")
	t.Setenv("MEDIA_DEV_AUTH", "true")
	t.Setenv("MEDIA_DATABASE_URL", "postgres://media:media@localhost/media")
	t.Setenv("MEDIA_PROCESSOR_TOKEN", "tok")
	t.Setenv("OIDC_ISSUER", "http://idp")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error when staging enables MEDIA_DEV_AUTH")
	}
}

func TestFromEnvStagingRequiresOIDC(t *testing.T) {
	t.Setenv("MEDIA_ENV", "staging")
	t.Setenv("MEDIA_DEV_AUTH", "false")
	t.Setenv("MEDIA_DATABASE_URL", "postgres://media:media@localhost/media")
	t.Setenv("MEDIA_PROCESSOR_TOKEN", "tok")
	t.Setenv("OIDC_ISSUER", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("staging must require OIDC_ISSUER")
	}
}

func TestFromEnvStagingOK(t *testing.T) {
	t.Setenv("MEDIA_ENV", "staging")
	t.Setenv("MEDIA_DEV_AUTH", "false")
	t.Setenv("MEDIA_DATABASE_URL", "postgres://media:media@localhost/media")
	t.Setenv("MEDIA_PROCESSOR_TOKEN", "tok")
	t.Setenv("OIDC_ISSUER", "http://idp")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != EnvStaging || cfg.DevAuth {
		t.Fatalf("unexpected %+v", cfg)
	}
}
