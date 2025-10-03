// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestMCPHTTPEndpoint(t *testing.T) {
	ctx := context.Background()

	// Create extension with dynamic port
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}
	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))

	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(ctx)) })

	// Give server time to start
	runtime.Gosched()
	time.Sleep(100 * time.Millisecond)

	// Create MCP client
	transport := &mcp.StreamableClientTransport{
		Endpoint:   "http://" + cfg.Endpoint + "/mcp",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	defer session.Close()

	t.Run("list_tools_via_http", func(t *testing.T) {
		result, err := session.ListTools(ctx, nil)
		require.NoError(t, err)

		// Should have all registered tools
		assert.GreaterOrEqual(t, len(result.Tools), 11)
	})

	t.Run("call_tool_via_http", func(t *testing.T) {
		toolResult, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_telemetry_summary",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.False(t, toolResult.IsError)
	})

	t.Run("ping_via_http", func(t *testing.T) {
		err := session.Ping(ctx, nil)
		require.NoError(t, err)
	})
}

func TestMCPHTTPEndpointWithConfig(t *testing.T) {
	ctx := context.Background()

	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}
	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))

	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(ctx)) })

	// Notify config
	testConf := confmap.NewFromStringMap(map[string]any{
		"receivers": map[string]any{
			"otlp": map[string]any{},
		},
		"exporters": map[string]any{
			"debug": map[string]any{},
		},
	})
	require.NoError(t, ext.NotifyConfig(ctx, testConf))

	runtime.Gosched()
	time.Sleep(100 * time.Millisecond)

	// Create MCP client
	transport := &mcp.StreamableClientTransport{
		Endpoint:   "http://" + cfg.Endpoint + "/mcp",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	defer session.Close()

	t.Run("get_config_after_notify", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_config",
			Arguments: map[string]any{},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})

	t.Run("get_config_section_receivers", func(t *testing.T) {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "get_config",
			Arguments: map[string]any{
				"section": "receivers",
			},
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
	})
}

func TestMCPHTTPMultipleClients(t *testing.T) {
	ctx := context.Background()

	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}
	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))

	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(ctx)) })

	runtime.Gosched()
	time.Sleep(100 * time.Millisecond)

	// Create multiple clients
	for i := 0; i < 3; i++ {
		transport := &mcp.StreamableClientTransport{
			Endpoint:   "http://" + cfg.Endpoint + "/mcp",
			HTTPClient: &http.Client{Timeout: 5 * time.Second},
		}

		client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
		session, err := client.Connect(ctx, transport, nil)
		require.NoError(t, err)

		// Each client should be able to list tools
		result, err := session.ListTools(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Tools), 11)

		session.Close()
	}
}
