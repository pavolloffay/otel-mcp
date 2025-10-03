// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
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

	_, ok := cfg.(*Config)
	require.True(t, ok)

	// Verify config validation passes
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateTracesToTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	next := new(consumertest.TracesSink)
	conn, err := factory.CreateTracesToTraces(
		context.Background(),
		connectortest.NewNopSettings(component.MustNewType("mcp")),
		cfg,
		next,
	)

	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify it's our connector type
	mcpConn, ok := conn.(*mcpConnector)
	require.True(t, ok)
	assert.NotNil(t, mcpConn.nextTraces)
	assert.Nil(t, mcpConn.nextMetrics)
	assert.Nil(t, mcpConn.nextLogs)
}

func TestCreateMetricsToMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	next := new(consumertest.MetricsSink)
	conn, err := factory.CreateMetricsToMetrics(
		context.Background(),
		connectortest.NewNopSettings(component.MustNewType("mcp")),
		cfg,
		next,
	)

	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify it's our connector type
	mcpConn, ok := conn.(*mcpConnector)
	require.True(t, ok)
	assert.Nil(t, mcpConn.nextTraces)
	assert.NotNil(t, mcpConn.nextMetrics)
	assert.Nil(t, mcpConn.nextLogs)
}

func TestCreateLogsToLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	next := new(consumertest.LogsSink)
	conn, err := factory.CreateLogsToLogs(
		context.Background(),
		connectortest.NewNopSettings(component.MustNewType("mcp")),
		cfg,
		next,
	)

	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify it's our connector type
	mcpConn, ok := conn.(*mcpConnector)
	require.True(t, ok)
	assert.Nil(t, mcpConn.nextTraces)
	assert.Nil(t, mcpConn.nextMetrics)
	assert.NotNil(t, mcpConn.nextLogs)
}

func TestFactoryStability(t *testing.T) {
	factory := NewFactory()

	// Verify all factory methods return development stability
	assert.Equal(t, component.StabilityLevelDevelopment, factory.TracesToTracesStability())
	assert.Equal(t, component.StabilityLevelDevelopment, factory.MetricsToMetricsStability())
	assert.Equal(t, component.StabilityLevelDevelopment, factory.LogsToLogsStability())
}
