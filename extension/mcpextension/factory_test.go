// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	assert.Equal(t, component.MustNewType("mcp"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	mcpCfg, ok := cfg.(*Config)
	require.True(t, ok)

	assert.Equal(t, "localhost:9999", mcpCfg.Endpoint)
	assert.Equal(t, 1000, mcpCfg.TracesBufferSize)
	assert.Equal(t, 1000, mcpCfg.MetricsBufferSize)
	assert.Equal(t, 1000, mcpCfg.LogsBufferSize)

	// Verify config validation passes
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Update config to use available port
	mcpCfg := cfg.(*Config)
	mcpCfg.Endpoint = getAvailableLocalAddress(t)

	ext, err := createExtension(
		context.Background(),
		extensiontest.NewNopSettings(component.MustNewType("mcp")),
		cfg,
	)

	require.NoError(t, err)
	require.NotNil(t, ext)

	// Verify extension can start and stop
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestCreateExtensionWithCustomConfig(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  100,
		MetricsBufferSize: 200,
		LogsBufferSize:    300,
	}

	ext, err := createExtension(
		context.Background(),
		extensiontest.NewNopSettings(component.MustNewType("mcp")),
		cfg,
	)

	require.NoError(t, err)
	require.NotNil(t, ext)

	mcpExt, ok := ext.(*mcpExtension)
	require.True(t, ok)
	assert.Equal(t, cfg.Endpoint, mcpExt.config.Endpoint)

	// Verify buffer sizes by checking stats
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	stats := mcpExt.GetStats()
	assert.Equal(t, 100, stats.TracesCapacity)
	assert.Equal(t, 200, stats.MetricsCapacity)
	assert.Equal(t, 300, stats.LogsCapacity)
}
