// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr   = "mcp"
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for the MCP connector
func NewFactory() connector.Factory {
	return connector.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, stability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, stability),
		connector.WithLogsToLogs(createLogsToLogs, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesToTraces(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	next consumer.Traces,
) (connector.Traces, error) {
	return newConnector(set, next, nil, nil), nil
}

func createMetricsToMetrics(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	next consumer.Metrics,
) (connector.Metrics, error) {
	return newConnector(set, nil, next, nil), nil
}

func createLogsToLogs(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	next consumer.Logs,
) (connector.Logs, error) {
	return newConnector(set, nil, nil, next), nil
}
