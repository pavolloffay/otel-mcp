// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/pavolloffay/otel-mcp/internal/tools"
)

func TestConcurrentMCPToolCalls(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock extension context
	mockCtx := newMockExtensionContext()

	// Create MCP server with tools
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	tools.RegisterGetConfig(server, mockCtx)
	tools.RegisterListConfiguredComponents(server, mockCtx)
	tools.RegisterGetTelemetrySummary(server, mockCtx)

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	// Create client
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	// Run concurrent tool calls
	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			// Each goroutine calls different tools
			tools := []string{"get_config", "list_configured_components", "get_telemetry_summary"}
			toolName := tools[routineID%len(tools)]

			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      toolName,
				Arguments: map[string]any{},
			})
			if err != nil {
				errors <- err
				return
			}

			if result.IsError {
				errors <- assert.AnError
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verify no errors occurred
	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentConfigUpdateAndToolCalls(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock extension context
	mockCtx := newMockExtensionContext()

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	tools.RegisterGetConfig(server, mockCtx)

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Goroutine 1: Update config repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			newConf := confmap.NewFromStringMap(map[string]any{
				"receivers": map[string]any{
					"otlp": map[string]any{
						"protocols": map[string]any{
							"grpc": map[string]any{
								"endpoint": "0.0.0.0:4317",
							},
						},
					},
				},
			})
			mockCtx.SetConf(newConf)
		}
	}()

	// Goroutines 2-11: Call get_config repeatedly
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				result, err := session.CallTool(ctx, &mcp.CallToolParams{
					Name:      "get_config",
					Arguments: map[string]any{},
				})
				if err != nil {
					errors <- err
					return
				}

				if result.IsError {
					errors <- assert.AnError
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Verify no errors occurred
	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentBufferOperations(t *testing.T) {
	ctx := context.Background()
	var ct, st mcp.Transport = mcp.NewInMemoryTransports()

	// Create mock extension context
	mockCtx := newMockExtensionContext()

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	tools.RegisterGetRecentTraces(server, mockCtx)
	tools.RegisterGetTelemetrySummary(server, mockCtx)

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer session.Close()

	var wg sync.WaitGroup

	// Goroutine 1: Add telemetry
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			td := ptrace.NewTraces()
			mockCtx.AddTrace(td)
		}
	}()

	// Goroutine 2-5: Read telemetry
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				_, _ = session.CallTool(ctx, &mcp.CallToolParams{
					Name: "get_recent_traces",
					Arguments: map[string]any{
						"limit": 10,
					},
				})
			}
		}()
	}

	// Goroutine 6: Get stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			_, _ = session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "get_telemetry_summary",
				Arguments: map[string]any{},
			})
		}
	}()

	wg.Wait()

	// No assertions needed - test passes if no data races occur
}

func TestConcurrentHTTPSessions(t *testing.T) {
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

	// Create multiple concurrent HTTP sessions
	const numSessions = 5
	var wg sync.WaitGroup
	errors := make(chan error, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			transport := &mcp.StreamableClientTransport{
				Endpoint:   "http://" + cfg.Endpoint + "/mcp",
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
			}

			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
			session, err := client.Connect(ctx, transport, nil)
			if err != nil {
				errors <- err
				return
			}
			defer session.Close()

			// Each session makes multiple requests
			for j := 0; j < 5; j++ {
				result, err := session.ListTools(ctx, nil)
				if err != nil {
					errors <- err
					return
				}

				if len(result.Tools) == 0 {
					errors <- assert.AnError
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verify no errors occurred
	for err := range errors {
		t.Error(err)
	}
}
