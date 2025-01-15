// Copyright 2025 Adevinta

// Package github provides a client to interact with GitHub Enterprise.
package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sync"

	gh "github.com/google/go-github/v67/github"

	"github.com/adevinta/ghe-reposec/internal/config"
)

var (
	// ErrTokenRequired is returned when a GitHub Enterprise token is not provided.
	ErrTokenRequired = fmt.Errorf("GitHub Enterprise token is required")
	// ErrAPIBaseURLRequired is returned when a GitHub Enterprise API base URL is not provided.
	ErrAPIBaseURLRequired = fmt.Errorf("GitHub Enterprise API base URL is required")
)

// Client is a GitHub client wrapper.
type Client struct {
	cfg    config.GHEConfig
	client *gh.Client
	logger *slog.Logger
	ctx    context.Context
}

// NewClient creates a new GitHub Enterprise client.
func NewClient(ctx context.Context, logger *slog.Logger, cfg config.GHEConfig) (*Client, error) {
	if cfg.Token == "" {
		return nil, ErrTokenRequired
	}
	if cfg.BaseURL == "" {
		return nil, ErrAPIBaseURLRequired
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	// NOTE: GitHub Enterprise API path is hardcoded.
	url, err := url.Parse(cfg.BaseURL + config.GHEAPIPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub Enteprise API URL: %w", err)
	}

	client := gh.NewClient(nil).WithAuthToken(cfg.Token)
	client.BaseURL = url

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with GitHub: %w", err)
	}

	logger.Debug("GitHub Enterprise token", "owner", user.GetLogin())

	return &Client{
		cfg:    cfg,
		logger: logger,
		client: client,
		ctx:    ctx,
	}, nil
}

// Organizations returns the list of all GitHub Enterprise organizations.
func (c *Client) Organizations() ([]string, error) {
	allOrgs := []string{}
	c.logger.Debug("listing organizations")
	orgsOpts := &gh.OrganizationsListOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		orgs, resp, err := c.client.Organizations.ListAll(
			context.WithValue(context.Background(), gh.SleepUntilPrimaryRateLimitResetWhenRateLimited, true),
			orgsOpts,
		)
		if err != nil {
			return []string{}, fmt.Errorf("failed to list organizations: %w", err)
		}
		for _, org := range orgs {
			allOrgs = append(allOrgs, org.GetLogin())
		}
		if resp.NextPage == 0 {
			break
		}
		orgsOpts.Since = *orgs[len(orgs)-1].ID
	}
	c.logger.Debug("listing organizations completed", "organizations", len(allOrgs))

	return allOrgs, nil
}

// Repositories returns the list of selected repositories from the targetOrg or
// all GitHub Enterprise organizations if targetOrg is not provided.
func (c *Client) Repositories(targetOrg string) ([]string, error) {
	var orgs []string
	var err error

	if targetOrg != "" {
		orgs = []string{targetOrg}
	} else {
		orgs, err = c.Organizations()
		if err != nil {
			return []string{}, fmt.Errorf("failed to list organizations: %w", err)
		}
	}

	c.logger.Debug("listing repositories")
	sem := make(chan struct{}, c.cfg.Concurrency)
	reposResultChan := make(chan []string)

	var wg sync.WaitGroup
	for _, org := range orgs {
		wg.Add(1)
		go orgRepositories(c, org, &wg, sem, reposResultChan)
	}
	go func() {
		wg.Wait()
		close(reposResultChan)
	}()

	selectedRepos := []string{}
	for repos := range reposResultChan {
		selectedRepos = append(selectedRepos, repos...)
	}
	c.logger.Debug("listing repositories completed", "repositories", len(selectedRepos))

	return selectedRepos, nil
}

func orgRepositories(c *Client, org string, wg *sync.WaitGroup, sem chan struct{}, resultChan chan<- []string) {
	defer wg.Done()

	sem <- struct{}{}
	defer func() { <-sem }()

	c.logger.Debug("obtaining repositories for organization", "organization", org)

	allRepos := []string{}
	listOpts := &gh.RepositoryListByOrgOptions{ListOptions: gh.ListOptions{PerPage: 100}}
	for {
		repos, resp, err := c.client.Repositories.ListByOrg(
			context.WithValue(c.ctx, gh.SleepUntilPrimaryRateLimitResetWhenRateLimited, true),
			org,
			listOpts,
		)
		if err != nil {
			c.logger.Error("failed to list repositories for organization", "organization", org, "error", err)
		}
		if err != nil && resp.NextPage == 0 {
			break
		}
		if err != nil && resp.NextPage != 0 {
			continue
		}
		for _, repo := range repos {
			// If repository is too big, skip it.
			if repo.Size != nil && *repo.Size > c.cfg.RepositorySizeLimit {
				c.logger.Warn("repository is too big, skipping", "size_kb", *repo.Size, "repository", repo.GetFullName())
				continue
			}
			// If repository is empty, skip it.
			if (repo.Size != nil && *repo.Size == 0) && !c.cfg.IncludeEmpty {
				c.logger.Warn("repository is empty, skipping", "repository", repo.GetFullName())
				continue
			}
			// If repository is archived, skip it.
			if (repo.Archived != nil && *repo.Archived) && !c.cfg.IncludeArchived {
				c.logger.Warn("repository is archived, skipping", "repository", repo.GetFullName())
				continue
			}
			// If repository is disabled, skip it.
			if (repo.Disabled != nil && *repo.Disabled) && !c.cfg.IncludeDisabled {
				c.logger.Warn("repository is disabled, skipping", "repository", repo.GetFullName())
				continue
			}
			// If repository is a fork, skip it.
			if (repo.Fork != nil && *repo.Fork) && !c.cfg.IncludeForks {
				c.logger.Warn("repository is a fork, skipping", "repository", repo.GetFullName())
				continue
			}
			// If repository is a template, skip it.
			if (repo.IsTemplate != nil && *repo.IsTemplate) && !c.cfg.IncludeTemplates {
				c.logger.Warn("repository is a template, skipping", "repository", repo.GetFullName())
				continue
			}
			allRepos = append(allRepos, *repo.CloneURL)
		}
		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}

	c.logger.Debug("organization repository listing completed", "organization", org, "repositories", len(allRepos))

	resultChan <- allRepos
}
