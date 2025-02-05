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
	"github.com/adevinta/ghe-reposec/internal/metrics"
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

	metrics, err := metrics.NewClient(ctx, &logger, cfg.MetricsCfg)
	if err != nil {
		logger.Error("failed to create metrics client", "error", err)
		os.Exit(1)
	}
	defer func() {
		metrics.Flush()
		metrics.Close()
	}()

	cli, err := github.NewClient(ctx, &logger, metrics, cfg.GHECfg)
	if err != nil {
		logger.Error("failed to create GitHub client", "error", err)
		metrics.ServiceCheck(2, err.Error(), []string{""})
		os.Exit(1)
	}

	lava, err := lava.NewClient(ctx, &logger, cfg.LavaCfg)
	if err != nil {
		logger.Error("failed to create Lava client", "error", err)
		metrics.ServiceCheck(2, err.Error(), []string{""})
		os.Exit(1)
	}

	repos, err := cli.Repositories(cfg.TargetOrg)
	if err != nil {
		logger.Error("failed to fetch repositories", "error", err)
		metrics.ServiceCheck(2, err.Error(), []string{""})
		os.Exit(1)
	}
	logger.Info("repositories selected", "count", len(repos), "duration", time.Since(st).Seconds())

	summary := lava.Scan(repos)
	pushSummaryMetrics(metrics, summary)

	err = output.Write(cfg.OutputFormat, cfg.OutputFilePath, summary)
	if err != nil {
		logger.Error("failed to write output", "error", err)
		metrics.ServiceCheck(2, err.Error(), []string{""})
		os.Exit(1)
	}
	logger.Info("output written", "file", cfg.OutputFilePath)

	metrics.Gauge("took", int(time.Since(st).Seconds()), []string{})
	metrics.ServiceCheck(0, "OK", []string{""})

	logger.Info("GitHub Enterprise reposec completed", "duration", time.Since(st).Seconds())
}

func pushSummaryMetrics(m *metrics.Client, s []lava.Summary) {
	sm := map[string]int{
		"with_controls":    0,
		"without_controls": 0,
		"error":            0,
	}
	cm := map[string]int{}
	for _, s := range s {
		if s.Error != "" {
			sm["error"]++
			continue
		}
		if s.ControlInPlace {
			sm["with_controls"]++
		} else {
			sm["without_controls"]++
		}
		for _, c := range s.Controls {
			cm[c]++
		}
	}
	for k, v := range sm {
		tags := []string{fmt.Sprintf("target:%s", k)}
		m.Gauge("summary.status", v, tags)
	}
	for k, v := range cm {
		tags := []string{fmt.Sprintf("control:%s", k)}
		m.Gauge("summary.controls", v, tags)
	}
}
