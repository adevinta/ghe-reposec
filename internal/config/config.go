// Copyright 2025 Adevinta

// Package config implements parsing of ghe-reposec configurations.
package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

const (
	// GHEAPIPath is the default GitHub Enterprise API path.
	GHEAPIPath = "/api/v3/"
)

// GHEConfig represents the GitHub Enterprise configuration.
type GHEConfig struct {
	Token       string `env:"GHE_TOKEN,required"`
	BaseURL     string `env:"GHE_BASE_URL,required"`
	Concurrency int    `env:"GHE_CONCURRENCY" envDefault:"15"`

	RepositorySizeLimit int  `env:"GHE_REPOSITORY_SIZE_LIMIT" envDefault:"3145728"` // 3GB
	IncludeArchived     bool `env:"GHE_INCLUDE_ARCHIVED" envDefault:"false"`
	IncludeEmpty        bool `env:"GHE_INCLUDE_EMPTY" envDefault:"false"`
	IncludeForks        bool `env:"GHE_INCLUDE_FORKS" envDefault:"false"`
	IncludeTemplates    bool `env:"GHE_INCLUDE_TEMPLATES" envDefault:"false"`
	IncludeDisabled     bool `env:"GHE_INCLUDE_DISABLED" envDefault:"false"`
	MinLastActivityDays int  `env:"GHE_MIN_LAST_ACTIVITY_DAYS" envDefault:"0"`
}

// LavaConfig represents the Lava configuration.
type LavaConfig struct {
	Token       string `env:"GHE_TOKEN,required"`
	BaseURL     string `env:"GHE_BASE_URL,required"`
	Concurrency int    `env:"LAVA_CONCURRENCY" envDefault:"10"`
	BinaryPath  string `env:"LAVA_BINARY_PATH" envDefault:"/usr/bin/lava"`
	// TODO: Build, publish and set a "production ready docker image" once the
	// check PR has been merged.
	CheckImage  string `env:"LAVA_CHECK_IMAGE" envDefault:"vulcansec/vulcan-repository-sctrl:a20516f-4aae88d"`
	ResultsPath string `env:"LAVA_RESULTS_PATH"`
}

// Config represents the ghe-reposec configuration.
type Config struct {
	LogLevel       string `env:"LOG_LEVEL" envDefault:"info"`
	LogOutput      string `env:"LOG_OUTPUT" envDefault:"stdout"`
	LogFormat      string `env:"LOG_OUTPUT_FORMAT" envDefault:"text"`
	TargetOrg      string `env:"TARGET_ORG"`
	OutputFilePath string `env:"OUTPUT_FILE" envDefault:"/tmp/reposec.csv"`
	OutputFormat   string `env:"OUTPUT_FORMAT" envDefault:"csv"`

	GHECfg  GHEConfig
	LavaCfg LavaConfig
}

// Redacted returns a secret redacted version of the configuration.
func (c Config) Redacted() Config {
	c.GHECfg.Token = "REDACTED"
	c.LavaCfg.Token = "REDACTED"
	return c
}

// Load parses the configuration from the environment.
func Load() (*Config, error) {
	var cfg Config
	err := env.ParseWithOptions(
		&cfg,
		env.Options{Prefix: "REPOSEC_"},
	)
	if err != nil {
		return nil, err
	}

	if cfg.LavaCfg.ResultsPath != "" && !strings.HasSuffix(cfg.LavaCfg.ResultsPath, "/") {
		cfg.LavaCfg.ResultsPath += "/"
	}

	return &cfg, nil
}

// NewLogger creates a new logger based on the configuration.
func (c *Config) NewLogger() slog.Logger {
	level := &slog.HandlerOptions{
		Level: parseLogLevel(c.LogLevel),
	}

	var output *os.File
	switch c.LogOutput {
	case "stderr":
		output = os.Stderr
	case "stdout":
		output = os.Stdout
	default:
		output = os.Stdout
	}

	var handler slog.Handler
	switch c.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(output, level)
	case "text":
		handler = slog.NewTextHandler(output, level)
	default:
		handler = slog.NewTextHandler(output, level)
	}

	logger := slog.New(handler)

	return *logger
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
