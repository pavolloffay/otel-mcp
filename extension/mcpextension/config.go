// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

var errInvalidBufferSize = errors.New("buffer size must be positive")

// Config defines configuration for the MCP extension
type Config struct {
	// Endpoint for the MCP HTTP server (e.g., "localhost:9999")
	Endpoint string `mapstructure:"endpoint"`

	// TracesBufferSize is the number of recent trace batches to keep in memory
	TracesBufferSize int `mapstructure:"traces_buffer_size"`

	// MetricsBufferSize is the number of recent metric batches to keep in memory
	MetricsBufferSize int `mapstructure:"metrics_buffer_size"`

	// LogsBufferSize is the number of recent log batches to keep in memory
	LogsBufferSize int `mapstructure:"logs_buffer_size"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.TracesBufferSize <= 0 {
		return errInvalidBufferSize
	}
	if cfg.MetricsBufferSize <= 0 {
		return errInvalidBufferSize
	}
	if cfg.LogsBufferSize <= 0 {
		return errInvalidBufferSize
	}
	return nil
}
