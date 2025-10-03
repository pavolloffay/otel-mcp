// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestMCPExtensionUsage(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	// Give a chance for the server goroutine to run
	runtime.Gosched()

	// Verify HTTP server is running by checking /mcp endpoint exists
	_, port, err := net.SplitHostPort(cfg.Endpoint)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:" + port + "/mcp")
	require.NoError(t, err)
	defer resp.Body.Close()

	// MCP endpoint exists (even if it returns error without proper headers)
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)
}

func TestMCPExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := getAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	cfg := &Config{
		Endpoint:          endpoint,
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	// Start should now fail immediately with port conflict error
	err = ext.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to bind MCP HTTP server")
}

func TestMCPExtensionMultipleStarts(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	// Second Start should fail with port already in use error
	err := ext.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to bind MCP HTTP server")
}

func TestMCPExtensionMultipleShutdowns(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(context.Background()))
	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestMCPExtensionShutdownWithoutStart(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestMCPExtensionConfigWatcher(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  10,
		MetricsBufferSize: 10,
		LogsBufferSize:    10,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	// Test config notification
	testConf := confmap.NewFromStringMap(map[string]any{
		"receivers": map[string]any{
			"otlp": map[string]any{},
		},
	})

	require.NoError(t, ext.NotifyConfig(context.Background(), testConf))

	// Verify config was stored
	storedConf := ext.GetCollectorConf()
	require.NotNil(t, storedConf)
	assert.Equal(t, testConf.ToStringMap(), storedConf.ToStringMap())
}

func TestMCPExtensionBufferOperations(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  5,
		MetricsBufferSize: 5,
		LogsBufferSize:    5,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	// Test adding traces
	td := ptrace.NewTraces()
	ext.AddTraces(td)

	traces := ext.GetRecentTraces(10, 0)
	assert.Len(t, traces, 1)

	// Test adding metrics
	md := pmetric.NewMetrics()
	ext.AddMetrics(md)

	metrics := ext.GetRecentMetrics(10, 0)
	assert.Len(t, metrics, 1)

	// Test adding logs
	ld := plog.NewLogs()
	ext.AddLogs(ld)

	logs := ext.GetRecentLogs(10, 0)
	assert.Len(t, logs, 1)

	// Test buffer stats
	stats := ext.GetStats()
	assert.Equal(t, 1, stats.TracesCount)
	assert.Equal(t, 1, stats.MetricsCount)
	assert.Equal(t, 1, stats.LogsCount)
	assert.Equal(t, 5, stats.TracesCapacity)
	assert.Equal(t, 5, stats.MetricsCapacity)
	assert.Equal(t, 5, stats.LogsCapacity)
}

func TestMCPExtensionBufferCapacity(t *testing.T) {
	cfg := &Config{
		Endpoint:          getAvailableLocalAddress(t),
		TracesBufferSize:  3,
		MetricsBufferSize: 3,
		LogsBufferSize:    3,
	}

	ext := newMCPExtension(cfg, extensiontest.NewNopSettings(component.MustNewType("mcp")))
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.Background())) })

	// Add more items than capacity
	for i := 0; i < 5; i++ {
		ext.AddTraces(ptrace.NewTraces())
	}

	// Should only have capacity amount
	traces := ext.GetRecentTraces(10, 0)
	assert.Len(t, traces, 3)
}

// Helper to get available local address
func getAvailableLocalAddress(t *testing.T) string {
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().String()
}
