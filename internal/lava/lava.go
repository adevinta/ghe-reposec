// Copyright 2025 Adevinta

// Package lava provides a client to interact with Lava.
package lava

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
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

	jobsChan := make(chan string, len(targets))
	jobResultsChan := make(chan []Summary, len(targets))
	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Concurrency; i++ {
		wg.Add(1)
		go c.worker(&wg, jobsChan, jobResultsChan)
	}

	for _, repo := range targets {
		jobsChan <- repo
	}
	close(jobsChan)

	wg.Wait()
	close(jobResultsChan)

	summary := []Summary{}
	for rs := range jobResultsChan {
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

func (c *Client) worker(wg *sync.WaitGroup, jobsChan <-chan string, jobResultsChan chan<- []Summary) {
	defer wg.Done()
	for repo := range jobsChan {
		summary := c.scanRepo(repo)
		jobResultsChan <- summary
	}
}

func (c *Client) scanRepo(repo string) []Summary {
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
	c.storeResults(repo, outBuf.Bytes(), errBuf.Bytes())

	if cmd.ProcessState.ExitCode() > 0 {
		c.logger.Error("failed to run Lava", "error", err, "repository", repo, "stderr", errBuf.String(), "stdout", outBuf.String(), "duration", time.Since(t).Seconds())
		summary = append(summary, Summary{Repository: repo, Error: fmt.Sprintf("error running Lava: %s", err.Error())})
		return summary
	}

	var lr []report.Vulnerability
	if err := json.Unmarshal(outBuf.Bytes(), &lr); err != nil {
		c.logger.Error("failed to unmarshal Lava report", "error", err, "repository", repo, "stderr", errBuf.String(), "stdout", outBuf.String(), "duration", time.Since(t).Seconds())
		summary = append(summary, Summary{Repository: repo, Error: fmt.Sprintf("error unmarsalling Lava report: %s", err.Error())})
		return summary
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

	return summary
}

func (c *Client) storeResults(target string, stdout, stderr []byte) {
	if c.cfg.ResultsPath == "" {
		return
	}

	org, repo, err := orgAndRepo(target)
	if err != nil {
		c.logger.Error("failed to parse target", "target", target, "error", err)
		return
	}

	resultsPath := fmt.Sprintf("%s%s/%s", c.cfg.ResultsPath, org, repo)
	err = os.MkdirAll(resultsPath, os.ModePerm)
	if err != nil {
		c.logger.Error("failed to create results directory", "path", resultsPath, "error", err)
		return
	}

	stdOutFile := fmt.Sprintf("%s/stdout.json", resultsPath)
	stdErrFile := fmt.Sprintf("%s/stderr.log", resultsPath)

	err = os.WriteFile(stdOutFile, stdout, 0644)
	if err != nil {
		c.logger.Error("failed to write stdout Lava scan results", "repository", target, "path", stdOutFile, "error", err)
		return
	}

	err = os.WriteFile(stdErrFile, stderr, 0644)
	if err != nil {
		c.logger.Error("failed to write stderr Lava scan results", "repository", target, "path", stdErrFile, "error", err)
		return
	}

	c.logger.Debug("Lava scan results stored", "repository", target, "stdout", stdOutFile, "stderr", stdErrFile)
}

func orgAndRepo(target string) (string, string, error) {
	parsedURL, err := url.Parse(target)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %v", err)
	}

	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub URL path: %s", parsedURL.Path)
	}

	return pathParts[0], pathParts[1], nil
}
