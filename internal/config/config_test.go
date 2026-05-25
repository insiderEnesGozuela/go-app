package config

import (
	"errors"
	"testing"
	"time"
)

func TestConfig_validate(t *testing.T) {
	// base returns a config that passes validation. Each test mutates one
	// field to assert it surfaces the specific error we expect.
	base := func() Config {
		return Config{
			App:  AppConfig{Name: "go-wallet", Env: "development"},
			HTTP: HTTPConfig{Port: "8080", ShutdownTimeout: 15 * time.Second},
			Database: DatabaseConfig{
				Host: "localhost", Port: "5432", User: "u", Password: "p", Name: "n",
				SSLMode: "disable", MaxOpenConns: 25, MaxIdleConns: 5,
				ConnMaxLifetime: 5 * time.Minute,
			},
			Log: LogConfig{Level: "info"},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
		errSub  string
	}{
		{name: "valid baseline", mutate: func(*Config) {}, wantErr: false},
		{
			name:    "empty http port",
			mutate:  func(c *Config) { c.HTTP.Port = "" },
			wantErr: true, errSub: "http.port",
		},
		{
			name:    "zero shutdown timeout",
			mutate:  func(c *Config) { c.HTTP.ShutdownTimeout = 0 },
			wantErr: true, errSub: "shutdown_timeout",
		},
		{
			name:    "zero max open conns",
			mutate:  func(c *Config) { c.Database.MaxOpenConns = 0 },
			wantErr: true, errSub: "max_open_conns",
		},
		{
			name:    "idle exceeds open",
			mutate:  func(c *Config) { c.Database.MaxIdleConns = 100; c.Database.MaxOpenConns = 10 },
			wantErr: true, errSub: "max_idle_conns",
		},
		{
			name:    "unknown env",
			mutate:  func(c *Config) { c.App.Env = "qa" },
			wantErr: true, errSub: "app.env",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			err := cfg.validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr && tc.errSub != "" && !contains(err.Error(), tc.errSub) {
				t.Fatalf("expected error containing %q, got %q", tc.errSub, err.Error())
			}
		})
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	d := DatabaseConfig{
		Host: "localhost", Port: "5432", User: "u",
		Password: "p", Name: "n", SSLMode: "disable",
	}
	want := "host=localhost port=5432 user=u password=p dbname=n sslmode=disable"
	if got := d.DSN(); got != want {
		t.Errorf("DSN()\n got: %q\nwant: %q", got, want)
	}
}

func TestLoad_invalidConfigWrapping(t *testing.T) {
	t.Setenv("DB_PASSWORD", "x")
	t.Setenv("APP_ENV", "qa") // invalid

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid APP_ENV")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected error to wrap ErrInvalidConfig, got %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
