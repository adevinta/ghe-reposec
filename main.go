// Copyright 2025 Adevinta

// ghe-reposec is a tool to scan GitHub Enterprise repositories in order to
// check if there are security controls in place.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/adevinta/ghe-reposec/internal/config"
	"github.com/adevinta/ghe-reposec/internal/github"
	"github.com/adevinta/ghe-reposec/internal/lava"
	"github.com/adevinta/ghe-reposec/internal/output"
)

func main() {
	st := time.Now()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logger := cfg.NewLogger()
	logger.Info("starting GitHub Enterprise reposec")
	logger.Info("configuration", "config", cfg.Redacted())

	ctx := context.Background()

	cli, err := github.NewClient(ctx, &logger, cfg.GHECfg)
	if err != nil {
		logger.Error("failed to create GitHub client", "error", err)
		os.Exit(1)
	}

	lava, err := lava.NewClient(ctx, &logger, cfg.LavaCfg)
	if err != nil {
		logger.Error("failed to create Lava client", "error", err)
		os.Exit(1)
	}

	repos, err := cli.Repositories(cfg.TargetOrg)
	if err != nil {
		logger.Error("failed to fetch repositories", "error", err)
		os.Exit(1)
	}
	logger.Info("repositories selected", "count", len(repos), "duration", time.Since(st).Seconds())

	summary := lava.Scan(repos)

	err = output.Write(cfg.OutputFormat, cfg.OutputFilePath, summary)
	if err != nil {
		logger.Error("failed to write output", "error", err)
		os.Exit(1)
	}
	logger.Info("output written", "file", cfg.OutputFilePath)

	logger.Info("GitHub Enterprise reposec completed", "duration", time.Since(st).Seconds())
}
