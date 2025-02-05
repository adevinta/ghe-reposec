// Copyright 2025 Adevinta

// Package metrics provides a wrapper to interact with StatsD.
package metrics

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/DataDog/datadog-go/statsd"

	"github.com/adevinta/ghe-reposec/internal/config"
)

var (
	// ClientNotInitializedMsg is logged when the metrics client is not
	// initialized and metrics are enabled.
	ClientNotInitializedMsg = "metrics client not initialized"
)

const (
	// DefaultMetricsClientAddr is the default metrics client address.
	DefaultMetricsClientAddr = "localhost:8125"
)

// Client represents a metrics service client.
type Client struct {
	cfg    config.MetricsConfig
	client *statsd.Client
	logger *slog.Logger
	ctx    context.Context
}

// NewClient creates a new metrics client based on environment variables config.
func NewClient(ctx context.Context, logger *slog.Logger, cfg config.MetricsConfig) (*Client, error) {
	if !cfg.Enabled {
		logger.Info("metrics reporting disabled")
		return &Client{}, nil
	}
	address := cfg.Address
	if address == "" {
		logger.Warn("metrics address not provided, using default", "address", DefaultMetricsClientAddr)
		address = DefaultMetricsClientAddr
	}

	statsd, err := statsd.New(address)
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:    cfg,
		client: statsd,
		logger: logger,
		ctx:    ctx,
	}, nil
}

// Gauge sends a gauge metric to the metrics service.
func (c *Client) Gauge(name string, value int, tags []string) {
	if !c.cfg.Enabled {
		return
	}
	if c.client == nil {
		c.logger.Warn(ClientNotInitializedMsg)
		return
	}
	tags = append(tags, c.cfg.Tags...)
	name = fmt.Sprintf("%s.%s", c.cfg.Namespace, name)
	err := c.client.Gauge(name, float64(value), tags, 1)
	if err != nil {
		c.logger.Error("gauge metric push error", "error", err)
		return
	}
	c.logger.Debug("gauge metric pushed", "name", name, "value", value, "tags", tags)
}

// ServiceCheck sends a service satus signal to the metrics service.
func (c *Client) ServiceCheck(status byte, message string, tags []string) {
	if !c.cfg.Enabled {
		return
	}
	if c.client == nil {
		c.logger.Warn(ClientNotInitializedMsg)
		return
	}
	tags = append(tags, c.cfg.Tags...)
	name := fmt.Sprintf("%s.service_check", c.cfg.Namespace)
	err := c.client.ServiceCheck(&statsd.ServiceCheck{
		Name:    name,
		Status:  statsd.ServiceCheckStatus(status),
		Tags:    tags,
		Message: message,
	})
	if err != nil {
		c.logger.Error("service check push error", "error", err)
		return
	}
	c.logger.Debug("service check pushed", "status", status, "message", message)
}

// Close closes the metrics client.
func (c *Client) Close() {
	if !c.cfg.Enabled {
		return
	}
	if c.client == nil {
		c.logger.Warn(ClientNotInitializedMsg)
		return
	}
	err := c.client.Close()
	if err != nil {
		c.logger.Error("metrics client close error", "error", err)
		return
	}
	c.logger.Debug("metrics client closed")
}

// Flush flushes the metrics client.
func (c *Client) Flush() {
	if !c.cfg.Enabled {
		return
	}
	if c.client == nil {
		c.logger.Warn(ClientNotInitializedMsg)
		return
	}
	err := c.client.Flush()
	if err != nil {
		c.logger.Error("metrics client flush error", "error", err)
		return
	}
	c.logger.Debug("metrics client flushed")
}
