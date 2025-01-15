// Copyright 2025 Adevinta

// Package lava provides a client to interact with Lava.
package lava

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	report "github.com/adevinta/vulcan-report"

	"github.com/adevinta/ghe-reposec/internal/config"
)

var (
	// ErrTokenRequired is returned when a GitHub Enterprise token is not
	// provided.
	ErrTokenRequired = fmt.Errorf("GitHub Enterprise token is required")
	// ErrAPIBaseURLRequired is returned when a GitHub Enterprise API base URL
	// is not provided.
	ErrAPIBaseURLRequired = fmt.Errorf("GitHub Enterprise API base URL is required")
	// ErrLavaBinaryNotFound is returned when the Lava binary is not found in
	// the execution path.
	ErrLavaBinaryNotFound = fmt.Errorf("failed to find Lava binary")
	// ErrLavaCheckImageRequired is returned when a Lava check image is not provided.
	ErrLavaCheckImageRequired = fmt.Errorf("lava check image is required")
)

// Summary represents a Lava scan summary.
type Summary struct {
	Repository       string
	Controls         []string
	ControlInPlace   bool
	NumberOfControls int
	Error            string
}

// Client is a Lava client wrapper.
type Client struct {
	cfg    config.LavaConfig
	logger *slog.Logger
	ctx    context.Context
}

// NewClient creates a new Lava client.
func NewClient(ctx context.Context, logger *slog.Logger, cfg config.LavaConfig) (*Client, error) {
	if cfg.Token == "" {
		return nil, ErrTokenRequired
	}
	if cfg.BaseURL == "" {
		return nil, ErrAPIBaseURLRequired
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if _, err := exec.LookPath(cfg.BinaryPath); err != nil {
		return nil, ErrLavaBinaryNotFound
	}
	if cfg.CheckImage == "" {
		return nil, ErrLavaCheckImageRequired
	}

	return &Client{
		cfg:    cfg,
		logger: logger,
		ctx:    ctx,
	}, nil
}

// Scan runs a Lava scan against the provided repositories.
func (c *Client) Scan(targets []string) []Summary {
	c.logger.Debug("start scanning repositories")

	scanResultChan := make(chan []Summary)
	sem := make(chan struct{}, c.cfg.Concurrency)
	var wg sync.WaitGroup
	for _, repo := range targets {
		wg.Add(1)
		go scanRepo(c, repo, &wg, sem, scanResultChan)
	}
	go func() {
		wg.Wait()
		close(scanResultChan)
	}()

	summary := []Summary{}
	for rs := range scanResultChan {
		for _, s := range rs {
			summary = append(summary, s)
			c.logger.Info(
				"live repository summary",
				"repository", s.Repository,
				"control_in_place", s.ControlInPlace,
				"controls", strings.Join(s.Controls, "#"),
				"number_of_controls", s.NumberOfControls,
				"error", s.Error,
			)
		}
	}
	c.logger.Debug("scanning repositories completed")

	return summary
}

func scanRepo(c *Client, repo string, wg *sync.WaitGroup, sem chan struct{}, resultChan chan<- []Summary) {
	defer wg.Done()
	sem <- struct{}{}
	defer func() { <-sem }()
	summary := []Summary{}

	t := time.Now()
	c.logger.Debug("repository scan started", "repository", repo)

	lavaCmdArgs := []string{
		"run",
		"-var", fmt.Sprintf("GITHUB_ENTERPRISE_ENDPOINT=%s", c.cfg.BaseURL),
		"-var", fmt.Sprintf("GITHUB_ENTERPRISE_TOKEN=%s", c.cfg.Token),
		"-type=GitRepository",
		"-show", "info",
		"-fmt=json",
		c.cfg.CheckImage,
		repo,
	}

	c.logger.Debug("scan repository command", "repository", repo, "args", strings.Replace(strings.Join(lavaCmdArgs, " "), c.cfg.Token, "REDACTED", -1))

	cmd := exec.CommandContext(c.ctx, c.cfg.BinaryPath, lavaCmdArgs...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if cmd.ProcessState.ExitCode() > 0 {
		c.logger.Error("failed to run Lava", "error", err, "repository", repo, "stderr", errBuf.String(), "stdout", outBuf.String(), "duration", time.Since(t).Seconds())
		summary = append(summary, Summary{Repository: repo, Error: fmt.Sprintf("error running Lava: %s", err.Error())})
		resultChan <- summary
		return
	}

	var lr []report.Vulnerability
	if err := json.Unmarshal(outBuf.Bytes(), &lr); err != nil {
		c.logger.Error("failed to unmarshal Lava report", "error", err, "repository", repo, "stderr", errBuf.String(), "stdout", outBuf.String(), "duration", time.Since(t).Seconds())
		summary = append(summary, Summary{Repository: repo, Error: fmt.Sprintf("error unmarsalling Lava report: %s", err.Error())})
		resultChan <- summary
		return
	}

	for _, r := range lr {
		s := Summary{
			Repository:     r.AffectedResource,
			ControlInPlace: r.Score == 0,
			Controls:       []string{},
		}
		for _, resource := range r.Resources {
			for _, control := range resource.Rows {
				if val, ok := control["Control"]; ok {
					s.NumberOfControls++
					s.Controls = append(s.Controls, val)
				}
			}
		}
		summary = append(summary, s)
	}

	c.logger.Info("repository scan completed successfully", "repository", repo, "duration", time.Since(t).Seconds())

	resultChan <- summary
}
