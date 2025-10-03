// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	"go.uber.org/zap"

	"github.com/austinparker/otel-mcp/internal/tools"
)

// mockExtensionContext implements tools.ExtensionContext for testing
type mockExtensionContext struct {
	mu               sync.RWMutex
	conf             *confmap.Conf
	moduleInfos      *service.ModuleInfos
	componentFactory hostcapabilities.ComponentFactory
	bufferStats      tools.BufferStats
	recentTraces     []ptrace.Traces
	recentMetrics    []pmetric.Metrics
	recentLogs       []plog.Logs
	logger           *zap.Logger
	host             component.Host
}

func (m *mockExtensionContext) GetCollectorConf() *confmap.Conf {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conf
}

func (m *mockExtensionContext) GetHost() component.Host {
	return m.host
}

func (m *mockExtensionContext) GetLogger() *zap.Logger {
	return m.logger
}

func (m *mockExtensionContext) GetBufferStats() tools.BufferStats {
	return m.bufferStats
}

func (m *mockExtensionContext) GetModuleInfos() *service.ModuleInfos {
	return m.moduleInfos
}

func (m *mockExtensionContext) GetComponentFactory() hostcapabilities.ComponentFactory {
	return m.componentFactory
}

func (m *mockExtensionContext) GetRecentTraces(limit, offset int) []ptrace.Traces {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if offset >= len(m.recentTraces) {
		return nil
	}
	end := offset + limit
	if end > len(m.recentTraces) {
		end = len(m.recentTraces)
	}
	return m.recentTraces[offset:end]
}

func (m *mockExtensionContext) GetRecentMetrics(limit, offset int) []pmetric.Metrics {
	if offset >= len(m.recentMetrics) {
		return nil
	}
	end := offset + limit
	if end > len(m.recentMetrics) {
		end = len(m.recentMetrics)
	}
	return m.recentMetrics[offset:end]
}

func (m *mockExtensionContext) GetRecentLogs(limit, offset int) []plog.Logs {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if offset >= len(m.recentLogs) {
		return nil
	}
	end := offset + limit
	if end > len(m.recentLogs) {
		end = len(m.recentLogs)
	}
	return m.recentLogs[offset:end]
}

// Helper methods for thread-safe writes in concurrent tests
func (m *mockExtensionContext) SetConf(conf *confmap.Conf) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conf = conf
}

func (m *mockExtensionContext) AddTrace(td ptrace.Traces) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recentTraces = append(m.recentTraces, td)
}

func newMockExtensionContext() *mockExtensionContext {
	logger, _ := zap.NewDevelopment()
	return &mockExtensionContext{
		conf: confmap.NewFromStringMap(map[string]any{
			"receivers": map[string]any{
				"otlp": map[string]any{
					"protocols": map[string]any{
						"grpc": map[string]any{
							"endpoint": "0.0.0.0:4317",
						},
					},
				},
			},
			"processors": map[string]any{
				"batch": map[string]any{},
			},
			"exporters": map[string]any{
				"debug": map[string]any{},
			},
			"service": map[string]any{
				"pipelines": map[string]any{
					"traces": map[string]any{
						"receivers":  []any{"otlp"},
						"processors": []any{"batch"},
						"exporters":  []any{"debug"},
					},
				},
			},
		}),
		moduleInfos: &service.ModuleInfos{
			Receiver: map[component.Type]service.ModuleInfo{
				component.MustNewType("otlp"): {BuilderRef: "go.opentelemetry.io/collector/receiver/otlpreceiver v0.136.0"},
			},
			Processor: map[component.Type]service.ModuleInfo{
				component.MustNewType("batch"): {BuilderRef: "go.opentelemetry.io/collector/processor/batchprocessor v0.136.0"},
			},
			Exporter: map[component.Type]service.ModuleInfo{
				component.MustNewType("debug"): {BuilderRef: "go.opentelemetry.io/collector/exporter/debugexporter v0.136.0"},
			},
		},
		bufferStats: tools.BufferStats{
			TracesCount:     5,
			TracesCapacity:  100,
			MetricsCount:    3,
			MetricsCapacity: 100,
			LogsCount:       10,
			LogsCapacity:    100,
		},
		logger: logger,
	}
}

func TestMCPToolsWithInMemoryTransport(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock extension context
	mockCtx := newMockExtensionContext()

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)

	// Register all tools
	tools.RegisterGetConfig(server, mockCtx)
	tools.RegisterGetComponentConfig(server, mockCtx)
	tools.RegisterListConfiguredComponents(server, mockCtx)
	tools.RegisterGetPipelineConfig(server, mockCtx)
	tools.RegisterListAvailableComponents(server, mockCtx)
	tools.RegisterGetComponentSchema(server, mockCtx)
	tools.RegisterGetFactoryInfo(server, mockCtx)
	tools.RegisterGetRecentTraces(server, mockCtx)
	tools.RegisterGetRecentMetrics(server, mockCtx)
	tools.RegisterGetRecentLogs(server, mockCtx)
	tools.RegisterGetTelemetrySummary(server, mockCtx)

	// Connect server
	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	// Create client and connect
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	t.Run("list_tools", func(t *testing.T) {
		result, err := session.ListTools(ctx, nil)
		require.NoError(t, err)

		// Verify we have registered tools (at least the config inspection tools)
		assert.GreaterOrEqual(t, len(result.Tools), 11, "should have at least 11 tools registered")

		// Verify specific tools exist
		toolNames := make([]string, len(result.Tools))
		for i, tool := range result.Tools {
			toolNames[i] = tool.Name
		}
		assert.Contains(t, toolNames, "get_config")
		assert.Contains(t, toolNames, "get_component_config")
		assert.Contains(t, toolNames, "list_configured_components")
		assert.Contains(t, toolNames, "get_telemetry_summary")
	})

	t.Run("get_config_full", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_config",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.NotEmpty(t, result.Content)
	})

	t.Run("get_config_section", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_config",
			Arguments: map[string]any{"section": "receivers"},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_config_invalid_section", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_config",
			Arguments: map[string]any{"section": "invalid"},
		})

		// Should return error (either protocol error or IsError=true)
		if err == nil {
			assert.True(t, result.IsError, "should return error for invalid section")
		}
	})

	t.Run("list_configured_components", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "list_configured_components",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("list_configured_components_filtered", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "list_configured_components",
			Arguments: map[string]any{"kind": "receiver"},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_component_config", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_component_config",
			Arguments: map[string]any{
				"component_id": "otlp",
				"kind":         "receiver",
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_pipeline_config", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_pipeline_config",
			Arguments: map[string]any{
				"pipeline_id": "traces",
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("list_available_components", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "list_available_components",
			Arguments: map[string]any{
				"kind": "",
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_telemetry_summary", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_telemetry_summary",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})
}

func TestMCPToolsWithoutConfig(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock context without config
	mockCtx := newMockExtensionContext()
	mockCtx.conf = nil

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	tools.RegisterGetConfig(server, mockCtx)

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	t.Run("get_config_without_config", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_config",
			Arguments: map[string]any{},
		})

		// Should error or return IsError=true
		if err == nil {
			assert.True(t, result.IsError, "should error when config not available")
		}
	})
}

func TestMCPToolsWithoutHostCapabilities(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock context without host capabilities
	mockCtx := newMockExtensionContext()
	mockCtx.moduleInfos = nil
	mockCtx.componentFactory = nil

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	tools.RegisterListAvailableComponents(server, mockCtx)

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	t.Run("list_available_components_without_module_info", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "list_available_components",
			Arguments: map[string]any{
				"kind": "",
			},
		})

		// Should error when ModuleInfo capability is missing
		if err == nil {
			assert.True(t, result.IsError, "should error when ModuleInfo not available")
		}
	})
}

func TestMCPTelemetryTools(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock context with telemetry
	mockCtx := newMockExtensionContext()

	// Add some test telemetry
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	mockCtx.recentTraces = []ptrace.Traces{td}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	mockCtx.recentMetrics = []pmetric.Metrics{md}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	mockCtx.recentLogs = []plog.Logs{ld}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	tools.RegisterGetRecentTraces(server, mockCtx)
	tools.RegisterGetRecentMetrics(server, mockCtx)
	tools.RegisterGetRecentLogs(server, mockCtx)
	tools.RegisterGetTelemetrySummary(server, mockCtx)

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	t.Run("get_recent_traces", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_recent_traces",
			Arguments: map[string]any{
				"limit": 10,
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_recent_metrics", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_recent_metrics",
			Arguments: map[string]any{
				"limit": 10,
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_recent_logs", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_recent_logs",
			Arguments: map[string]any{
				"limit": 10,
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_telemetry_summary", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_telemetry_summary",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})
}
