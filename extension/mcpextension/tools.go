// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"github.com/austinparker/otel-mcp/internal/tools"
)

// registerTools registers all MCP tools with the server
func (e *mcpExtension) registerTools() error {
	// Config inspection tools
	tools.RegisterGetConfig(e.server, e)
	tools.RegisterGetComponentConfig(e.server, e)
	tools.RegisterListConfiguredComponents(e.server, e)
	tools.RegisterGetPipelineConfig(e.server, e)

	// Component discovery tools
	tools.RegisterListAvailableComponents(e.server, e)
	tools.RegisterGetComponentSchema(e.server, e)
	tools.RegisterGetFactoryInfo(e.server, e)

	// Config validation tools
	tools.RegisterValidateConfigSection(e.server, e)
	tools.RegisterAddComponent(e.server, e)
	tools.RegisterRemoveComponent(e.server, e)
	tools.RegisterValidateConfig(e.server, e)
	tools.RegisterUpdatePipeline(e.server, e)

	// Telemetry query tools (consolidated from search + recent)
	tools.RegisterQueryTraces(e.server, e)
	tools.RegisterQueryLogs(e.server, e)
	tools.RegisterQueryMetrics(e.server, e)
	tools.RegisterGetTelemetrySummary(e.server, e)

	// Specialized telemetry tools
	tools.RegisterGetTraceByID(e.server, e)
	tools.RegisterFindRelatedTelemetry(e.server, e)

	// Runtime/status tools
	tools.RegisterGetComponentStatus(e.server, e)
	tools.RegisterGetPipelineMetrics(e.server, e)
	tools.RegisterGetExtensions(e.server, e)

	return nil
}
