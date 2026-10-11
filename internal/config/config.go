// Package config загружает конфигурацию сервиса из флагов командной строки и переменных окружения.
package config

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"go.uber.org/zap/zapcore"
)

const defaultRunAddress = "localhost:8080"

var ErrInvalid = errors.New("invalid config")

type Config struct {
	RunAddress     string `env:"RUN_ADDRESS"`
	DatabaseURI    string `env:"DATABASE_URI"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`

	JWTSecret    string        `env:"JWT_SECRET"`
	TokenTTL     time.Duration `env:"TOKEN_TTL" envDefault:"24h"`
	SecureCookie bool          `env:"SECURE_COOKIE" envDefault:"false"`
	LogLevel     zapcore.Level `env:"LOG_LEVEL" envDefault:"info"`

	DB DBConfig `envPrefix:"DB_"`
}

type DBConfig struct {
	MaxConns        int32         `env:"MAX_CONNS" envDefault:"10"`
	MinConns        int32         `env:"MIN_CONNS" envDefault:"2"`
	MaxConnLifetime time.Duration `env:"MAX_CONN_LIFETIME" envDefault:"1h"`
	MaxConnIdleTime time.Duration `env:"MAX_CONN_IDLE_TIME" envDefault:"5m"`
}

func Load(args, environ []string) (Config, error) {
	var cfg Config

	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.StringVar(&cfg.RunAddress, "a", defaultRunAddress, "HTTP server address")
	fs.StringVar(&cfg.DatabaseURI, "d", "", "PostgreSQL connection URI")
	fs.StringVar(&cfg.AccrualAddress, "r", "", "accrual system address")
	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}

	if err := env.ParseWithOptions(&cfg, env.Options{Environment: env.ToMap(environ)}); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch {
	case c.RunAddress == "":
		return fmt.Errorf("%w: run address is empty (-a / RUN_ADDRESS)", ErrInvalid)
	case c.DatabaseURI == "":
		return fmt.Errorf("%w: database URI is empty (-d / DATABASE_URI)", ErrInvalid)
	case c.TokenTTL <= 0:
		return fmt.Errorf("%w: TOKEN_TTL must be positive, got %s", ErrInvalid, c.TokenTTL)
	case c.DB.MaxConns < 1:
		return fmt.Errorf("%w: DB_MAX_CONNS must be at least 1, got %d", ErrInvalid, c.DB.MaxConns)
	case c.DB.MinConns < 0 || c.DB.MinConns > c.DB.MaxConns:
		return fmt.Errorf("%w: DB_MIN_CONNS must be in 0..DB_MAX_CONNS (%d), got %d", ErrInvalid, c.DB.MaxConns, c.DB.MinConns)
	case c.DB.MaxConnLifetime <= 0:
		return fmt.Errorf("%w: DB_MAX_CONN_LIFETIME must be positive, got %s", ErrInvalid, c.DB.MaxConnLifetime)
	case c.DB.MaxConnIdleTime <= 0:
		return fmt.Errorf("%w: DB_MAX_CONN_IDLE_TIME must be positive, got %s", ErrInvalid, c.DB.MaxConnIdleTime)
	}
	return nil
}
