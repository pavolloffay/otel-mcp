// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpextension

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	"go.uber.org/zap"

	"github.com/austinparker/otel-mcp/internal/buffer"
	"github.com/austinparker/otel-mcp/internal/tools"
)

var (
	_ extension.Extension                 = (*mcpExtension)(nil)
	_ extensioncapabilities.ConfigWatcher = (*mcpExtension)(nil)
	_ buffer.TelemetryBuffer              = (*mcpExtension)(nil)
)

type mcpExtension struct {
	config    *Config
	logger    *zap.Logger
	telemetry component.TelemetrySettings

	// MCP server
	server     *mcp.Server
	mu         sync.Mutex
	httpServer *http.Server
	cancelFunc context.CancelFunc

	// Configuration from collector - uses atomic.Value for lock-free reads
	collectorConf atomic.Value // stores *confmap.Conf

	// Telemetry buffer
	buffer buffer.TelemetryBuffer

	// Component host for introspection
	host component.Host

	// Host capabilities (optional)
	moduleInfos      atomic.Value // stores service.ModuleInfos
	componentFactory hostcapabilities.ComponentFactory
}

func newMCPExtension(cfg *Config, set extension.Settings) *mcpExtension {
	return &mcpExtension{
		config:    cfg,
		logger:    set.Logger,
		telemetry: set.TelemetrySettings,
		buffer:    buffer.New(cfg.TracesBufferSize, cfg.MetricsBufferSize, cfg.LogsBufferSize),
	}
}

func (e *mcpExtension) Start(_ context.Context, host component.Host) error {
	e.host = host
	e.logger.Info("Starting MCP extension")

	// Check for optional host capabilities
	if mi, ok := host.(hostcapabilities.ModuleInfo); ok {
		moduleInfos := mi.GetModuleInfos()
		e.moduleInfos.Store(moduleInfos)
		e.logger.Info("Host provides ModuleInfo capability",
			zap.Int("receivers", len(moduleInfos.Receiver)),
			zap.Int("processors", len(moduleInfos.Processor)),
			zap.Int("exporters", len(moduleInfos.Exporter)),
			zap.Int("extensions", len(moduleInfos.Extension)),
			zap.Int("connectors", len(moduleInfos.Connector)),
		)
	} else {
		e.logger.Warn("Host does not provide ModuleInfo capability - component discovery will be limited")
	}

	if cf, ok := host.(hostcapabilities.ComponentFactory); ok {
		e.componentFactory = cf
		e.logger.Info("Host provides ComponentFactory capability")
	} else {
		e.logger.Warn("Host does not provide ComponentFactory capability - factory inspection will be limited")
	}

	// Create MCP server
	serverInfo := &mcp.Implementation{
		Name:    "otel-collector-mcp",
		Version: "0.1.0",
	}

	server := mcp.NewServer(serverInfo, nil)
	e.server = server

	// Register all MCP tools
	if err := e.registerTools(); err != nil {
		return err
	}

	// Create StreamableHTTP handler for HTTP transport
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, nil)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	// Create listener to verify binding before returning from Start
	listener, err := net.Listen("tcp", e.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to bind MCP HTTP server to %s: %w", e.config.Endpoint, err)
	}

	// Protect httpServer and cancelFunc with mutex
	e.mu.Lock()
	e.httpServer = &http.Server{
		Addr:              e.config.Endpoint,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start HTTP server in background
	_, cancel := context.WithCancel(context.Background())
	e.cancelFunc = cancel
	httpServer := e.httpServer
	e.mu.Unlock()

	go func() {
		e.logger.Info("Starting MCP HTTP server", zap.String("endpoint", e.config.Endpoint))
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			e.logger.Error("MCP HTTP server error", zap.Error(err))
		}
	}()

	e.logger.Info("MCP extension started successfully", zap.String("endpoint", e.config.Endpoint))
	return nil
}

func (e *mcpExtension) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down MCP extension")

	// Get httpServer and cancelFunc under lock
	e.mu.Lock()
	httpServer := e.httpServer
	cancelFunc := e.cancelFunc
	e.mu.Unlock()

	// Stop HTTP server gracefully
	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			e.logger.Error("Error shutting down MCP HTTP server", zap.Error(err))
		}
	}

	if cancelFunc != nil {
		cancelFunc()
	}
	return nil
}

// NotifyConfig implements extensioncapabilities.ConfigWatcher
func (e *mcpExtension) NotifyConfig(_ context.Context, conf *confmap.Conf) error {
	e.collectorConf.Store(conf)
	e.logger.Info("Received collector configuration update")
	return nil
}

// TelemetryBuffer interface implementation - delegates to internal buffer
func (e *mcpExtension) AddTraces(td ptrace.Traces) {
	e.buffer.AddTraces(td)
}

func (e *mcpExtension) AddMetrics(md pmetric.Metrics) {
	e.buffer.AddMetrics(md)
}

func (e *mcpExtension) AddLogs(ld plog.Logs) {
	e.buffer.AddLogs(ld)
}

func (e *mcpExtension) GetRecentTraces(limit, offset int) []ptrace.Traces {
	return e.buffer.GetRecentTraces(limit, offset)
}

func (e *mcpExtension) GetRecentMetrics(limit, offset int) []pmetric.Metrics {
	return e.buffer.GetRecentMetrics(limit, offset)
}

func (e *mcpExtension) GetRecentLogs(limit, offset int) []plog.Logs {
	return e.buffer.GetRecentLogs(limit, offset)
}

func (e *mcpExtension) GetStats() buffer.BufferStats {
	return e.buffer.GetStats()
}

// ExtensionContext interface implementation for tools
func (e *mcpExtension) GetCollectorConf() *confmap.Conf {
	val := e.collectorConf.Load()
	if val == nil {
		return nil
	}
	return val.(*confmap.Conf)
}

func (e *mcpExtension) GetHost() component.Host {
	return e.host
}

func (e *mcpExtension) GetLogger() *zap.Logger {
	return e.logger
}

func (e *mcpExtension) GetBufferStats() tools.BufferStats {
	stats := e.buffer.GetStats()
	return tools.BufferStats{
		TracesCount:     stats.TracesCount,
		TracesCapacity:  stats.TracesCapacity,
		MetricsCount:    stats.MetricsCount,
		MetricsCapacity: stats.MetricsCapacity,
		LogsCount:       stats.LogsCount,
		LogsCapacity:    stats.LogsCapacity,
	}
}

func (e *mcpExtension) GetModuleInfos() *service.ModuleInfos {
	val := e.moduleInfos.Load()
	if val == nil {
		return nil
	}
	infos := val.(service.ModuleInfos)
	return &infos
}

func (e *mcpExtension) GetComponentFactory() hostcapabilities.ComponentFactory {
	return e.componentFactory
}
