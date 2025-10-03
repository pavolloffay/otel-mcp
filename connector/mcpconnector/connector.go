// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// TelemetryBuffer is the interface the connector uses to store telemetry
// This must match the interface exposed by the MCP extension
type TelemetryBuffer interface {
	AddTraces(td ptrace.Traces)
	AddMetrics(md pmetric.Metrics)
	AddLogs(ld plog.Logs)
}

// mcpConnector implements a pass-through connector that also buffers telemetry
type mcpConnector struct {
	logger *zap.Logger
	set    connector.Settings

	// Next consumers in the pipeline
	nextTraces  consumer.Traces
	nextMetrics consumer.Metrics
	nextLogs    consumer.Logs

	// Reference to MCP extension's buffer
	buffer TelemetryBuffer

	// Track if downstream consumers mutate data
	nextTracesMutates  bool
	nextMetricsMutates bool
	nextLogsMutates    bool
}

var (
	_ connector.Traces  = (*mcpConnector)(nil)
	_ connector.Metrics = (*mcpConnector)(nil)
	_ connector.Logs    = (*mcpConnector)(nil)
)

func newConnector(
	set connector.Settings,
	nextTraces consumer.Traces,
	nextMetrics consumer.Metrics,
	nextLogs consumer.Logs,
) *mcpConnector {
	c := &mcpConnector{
		logger:      set.Logger,
		set:         set,
		nextTraces:  nextTraces,
		nextMetrics: nextMetrics,
		nextLogs:    nextLogs,
	}

	// Check if downstream consumers mutate data
	if nextTraces != nil {
		c.nextTracesMutates = nextTraces.Capabilities().MutatesData
	}
	if nextMetrics != nil {
		c.nextMetricsMutates = nextMetrics.Capabilities().MutatesData
	}
	if nextLogs != nil {
		c.nextLogsMutates = nextLogs.Capabilities().MutatesData
	}

	return c
}

//nolint:revive // ctx unused but kept for interface compatibility
func (c *mcpConnector) Start(ctx context.Context, host component.Host) error {
	c.logger.Info("Starting MCP connector, searching for MCP extension")

	// Find the MCP extension
	extensions := host.GetExtensions()
	for id, ext := range extensions {
		if id.Type().String() == "mcp" {
			if buffer, ok := ext.(TelemetryBuffer); ok {
				c.buffer = buffer
				c.logger.Info("Found MCP extension, telemetry buffering enabled")
				return nil
			}
		}
	}

	c.logger.Warn("MCP extension not found, telemetry buffering disabled")
	return nil
}

//nolint:revive // ctx unused but kept for interface compatibility
func (c *mcpConnector) Shutdown(ctx context.Context) error {
	return nil
}

func (*mcpConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces buffers traces and passes them through
func (c *mcpConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Buffer telemetry - only clone if downstream mutates
	if c.buffer != nil {
		if c.nextTracesMutates {
			// Clone to prevent downstream mutations from affecting buffer
			tdClone := ptrace.NewTraces()
			td.CopyTo(tdClone)
			c.buffer.AddTraces(tdClone)
		} else {
			// Safe to share since downstream doesn't mutate
			c.buffer.AddTraces(td)
		}
	}

	// Pass through to next consumer
	if c.nextTraces != nil {
		return c.nextTraces.ConsumeTraces(ctx, td)
	}
	return nil
}

// ConsumeMetrics buffers metrics and passes them through
func (c *mcpConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Buffer telemetry - only clone if downstream mutates
	if c.buffer != nil {
		if c.nextMetricsMutates {
			// Clone to prevent downstream mutations from affecting buffer
			mdClone := pmetric.NewMetrics()
			md.CopyTo(mdClone)
			c.buffer.AddMetrics(mdClone)
		} else {
			// Safe to share since downstream doesn't mutate
			c.buffer.AddMetrics(md)
		}
	}

	// Pass through to next consumer
	if c.nextMetrics != nil {
		return c.nextMetrics.ConsumeMetrics(ctx, md)
	}
	return nil
}

// ConsumeLogs buffers logs and passes them through
func (c *mcpConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Buffer telemetry - only clone if downstream mutates
	if c.buffer != nil {
		if c.nextLogsMutates {
			// Clone to prevent downstream mutations from affecting buffer
			ldClone := plog.NewLogs()
			ld.CopyTo(ldClone)
			c.buffer.AddLogs(ldClone)
		} else {
			// Safe to share since downstream doesn't mutate
			c.buffer.AddLogs(ld)
		}
	}

	// Pass through to next consumer
	if c.nextLogs != nil {
		return c.nextLogs.ConsumeLogs(ctx, ld)
	}
	return nil
}
