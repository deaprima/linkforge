package config

import (
	"reflect"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"DB_USER": "u", "DB_PASSWORD": "p", "DB_HOST": "h", "DB_PORT": "5432", "DB_NAME": "n",
		"AUTH_PORT": "50051", "JWT_SECRET": "s3cret",
		"JWT_ACCESS_DURATION": "15m", "JWT_REFRESH_DURATION": "168h", "GOOGLE_CLIENT_ID": "gid",
	} {
		t.Setenv(k, v)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	setRequired(t)
	for _, k := range []string{"FRONTEND_URL", "PASSWORD_RESET_TTL", "SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS", "SMTP_FROM"} {
		t.Setenv(k, "")
	}

	want := &Config{
		GRPCPort:       "50051",
		DatabaseDSN:    "postgres://u:p@h:5432/n?sslmode=disable&search_path=auth",
		JWTSecret:      "s3cret",
		JWTAccessDur:   15 * time.Minute,
		JWTRefreshDur:  168 * time.Hour,
		GoogleClientID: "gid",
		FrontendURL:    "http://localhost:5173",
		ResetTokenTTL:  time.Hour,
		SMTPPort:       "587",
		SMTPFrom:       "LinkForge <no-reply@linkforge.local>",
	}
	if got := LoadConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	setRequired(t)
	t.Setenv("FRONTEND_URL", "https://app.example.com")
	t.Setenv("PASSWORD_RESET_TTL", "30m")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USER", "mailer")
	t.Setenv("SMTP_PASS", "pw")
	t.Setenv("SMTP_FROM", "Me <me@example.com>")

	got := LoadConfig()
	if got.FrontendURL != "https://app.example.com" || got.ResetTokenTTL != 30*time.Minute {
		t.Errorf("frontend/ttl = %q / %v", got.FrontendURL, got.ResetTokenTTL)
	}
	if got.SMTPHost != "smtp.example.com" || got.SMTPPort != "465" || got.SMTPUser != "mailer" || got.SMTPPass != "pw" || got.SMTPFrom != "Me <me@example.com>" {
		t.Errorf("smtp = %+v", got)
	}
}

func TestLoadConfig_GoogleClientIDIsOptional(t *testing.T) {
	setRequired(t)
	t.Setenv("GOOGLE_CLIENT_ID", "")
	if got := LoadConfig().GoogleClientID; got != "" {
		t.Errorf("GoogleClientID = %q, want empty (Google sign-in disabled)", got)
	}
}
