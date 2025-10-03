// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	typeStr           = "mcp"
	stability         = component.StabilityLevelDevelopment
	defaultBufferSize = 1000
	defaultEndpoint   = "localhost:9999"
)

// NewFactory creates a factory for the MCP extension
func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		createExtension,
		stability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:          defaultEndpoint,
		TracesBufferSize:  defaultBufferSize,
		MetricsBufferSize: defaultBufferSize,
		LogsBufferSize:    defaultBufferSize,
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	return newMCPExtension(config, set), nil
}
