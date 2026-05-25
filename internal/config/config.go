package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// ErrInvalidConfig wraps any validation failure surfaced while loading config.
// Defined here (not /internal/domain) because it is infra-layer concern.
var ErrInvalidConfig = errors.New("invalid config")

type Config struct {
	App      AppConfig      `yaml:"app"`
	HTTP     HTTPConfig     `yaml:"http"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
}

type AppConfig struct {
	Name string `yaml:"name" env:"APP_NAME" env-default:"go-wallet"`
	Env  string `yaml:"env"  env:"APP_ENV"  env-default:"development"`
}

type HTTPConfig struct {
	Port            string        `yaml:"port"             env:"HTTP_PORT"             env-default:"8080"`
	ReadTimeout     time.Duration `yaml:"read_timeout"     env:"HTTP_READ_TIMEOUT"     env-default:"5s"`
	WriteTimeout    time.Duration `yaml:"write_timeout"    env:"HTTP_WRITE_TIMEOUT"    env-default:"10s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"15s"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"              env:"DB_HOST"              env-default:"localhost"`
	Port            string        `yaml:"port"              env:"DB_PORT"              env-default:"5432"`
	User            string        `yaml:"user"              env:"DB_USER"              env-default:"wallet"`
	Password        string        `yaml:"password"          env:"DB_PASSWORD"          env-required:"true"`
	Name            string        `yaml:"name"              env:"DB_NAME"              env-default:"wallet"`
	SSLMode         string        `yaml:"sslmode"           env:"DB_SSLMODE"           env-default:"disable"`
	MaxOpenConns    int32         `yaml:"max_open_conns"    env:"DB_MAX_OPEN_CONNS"    env-default:"25"`
	MaxIdleConns    int32         `yaml:"max_idle_conns"    env:"DB_MAX_IDLE_CONNS"    env-default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" env-default:"5m"`
}

type LogConfig struct {
	Level  string `yaml:"level"  env:"LOG_LEVEL"  env-default:"info"`
	Pretty bool   `yaml:"pretty" env:"LOG_PRETTY" env-default:"false"`
}

// DSN builds a libpq-style connection string consumable by pgx.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// Load reads config from optional YAML file (path via CONFIG_PATH) and env vars.
// Env vars always override file values. .env files are auto-loaded by cleanenv.
func Load() (*Config, error) {
	var cfg Config

	if path := os.Getenv("CONFIG_PATH"); path != "" {
		if err := cleanenv.ReadConfig(path, &cfg); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", path, err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("read env: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.HTTP.Port == "" {
		return errors.New("http.port must not be empty")
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		return errors.New("http.shutdown_timeout must be > 0")
	}
	if c.Database.MaxOpenConns <= 0 {
		return errors.New("database.max_open_conns must be > 0")
	}
	if c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return errors.New("database.max_idle_conns must be between 0 and max_open_conns")
	}
	switch c.App.Env {
	case "development", "staging", "production":
	default:
		return fmt.Errorf("app.env must be development|staging|production, got %q", c.App.Env)
	}
	return nil
}
