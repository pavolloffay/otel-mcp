# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpenTelemetry Collector MCP Extension - An MCP (Model Context Protocol) extension and connector for the OpenTelemetry Collector that provides LLM-accessible tools for configuration management and telemetry inspection.

**Two main components:**
1. **MCP Extension** (`extension/mcpextension/`) - Runs an MCP server over HTTP and exposes 24 MCP tools for config/telemetry access
2. **MCP Connector** (`connector/mcpconnector/`) - Pass-through connector that captures telemetry and buffers it in the extension's circular buffer

## Development Commands

### Building the Collector
```bash
# Build using the OpenTelemetry Collector Builder
~/go/bin/builder --config manifest-dev.yaml

# The output binary will be at:
./dist/otelcol-mcp-dev
```

### Running the Collector
```bash
# Run with config.yaml
./dist/otelcol-mcp-dev --config config.yaml

# Note: The MCP extension runs on HTTP endpoint localhost:9999 (configurable in config.yaml)
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./connector/mcpconnector/
go test ./extension/mcpextension/
go test ./internal/buffer/

# Run with verbose output
go test -v ./...
```

### Linting
```bash
# Run linter (uses .golangci.yml config aligned with OTel Collector Contrib)
golangci-lint run

# Auto-fix issues where possible
golangci-lint run --fix
```

**IMPORTANT: Always keep lints and tests green**
- Before committing changes, run `golangci-lint run` and `go test ./...`
- All code must pass linting with zero warnings
- All tests must pass
- Use `//nolint:revive // reason` comments sparingly and only when genuinely needed

### Dependency Management
```bash
# Update dependencies
go mod tidy

# Verify dependencies
go mod verify
```

## Architecture Overview

### Extension Architecture
- Implements `extensioncapabilities.ConfigWatcher` to receive collector config updates via `NotifyConfig()`
- Implements `buffer.TelemetryBuffer` interface to store telemetry
- Uses `atomic.Value` for lock-free reads of collector configuration and module info
- Runs MCP server over HTTP using `StreamableHTTPHandler` (not stdio)
- Registers 24 MCP tools at startup via `registerTools()`
- Tools access extension capabilities through the `ExtensionContext` interface

### Connector Architecture
- Pass-through connector implementing `connector.Traces`, `connector.Metrics`, and `connector.Logs`
- Auto-discovers MCP extension via `host.GetExtensions()` by looking for extension with type "mcp"
- Buffers telemetry by calling extension's `TelemetryBuffer` interface methods
- Optimizes for zero-copy when downstream consumers don't mutate data (checks `Capabilities().MutatesData`)
- Clones telemetry only if downstream mutates to prevent buffer corruption

### Circular Buffer
- Thread-safe double-ended queue implementation in `internal/buffer/` using `github.com/earthboundkid/deque/v2`
- Separate buffers for traces, metrics, and logs
- Configurable capacity per signal type (default: 1000 batches each)
- Supports pagination via `limit` and `offset` parameters
- Tracks statistics: count and capacity

### Tool Organization (`internal/tools/`)
Tools are organized by category (24 total MCP tools):

**Config Inspection** (`config_inspection.go`) - 4 tools:
- `get_config` - Get current collector configuration
- `get_component_config` - Get specific component configuration
- `list_configured_components` - List all configured components
- `get_pipeline_config` - Get pipeline configuration

**Component Discovery** (`component_discovery.go`) - 3 tools:
- `list_available_components` - List available component types with versions
- `get_component_schema` - Get component configuration schema
- `get_factory_info` - Get factory metadata and stability level

**Config Modification** (`config_modification.go`) - 5 tools:
- `update_config` - Validate configuration changes (read-only)
- `add_component` - Validate adding components (read-only)
- `remove_component` - Validate removing components (read-only)
- `validate_config` - Validate complete configuration
- `update_pipeline` - Validate pipeline modifications (read-only)

**Telemetry Query** (`telemetry_query.go`) - 4 tools:
- `get_recent_traces` - Get recent traces as CSV
- `get_recent_metrics` - Get recent metrics with filtering
- `get_recent_logs` - Get recent logs as CSV
- `get_telemetry_summary` - Get buffer statistics

**Telemetry Search** (`telemetry_search.go`) - 5 tools:
- `search_traces` - Search traces by criteria (service, span name, trace ID)
- `search_logs` - Search logs by criteria (severity, body, service)
- `search_metrics` - Search metrics by criteria (name, service)
- `get_trace_by_id` - Get complete trace by ID
- `find_related_telemetry` - Find related logs/metrics for trace/span

**Runtime Status** (`runtime_status.go`) - 3 tools:
- `get_component_status` - Get runtime status of components
- `get_pipeline_metrics` - Get pipeline configuration metrics
- `get_extensions` - List running extensions

All tools receive an `ExtensionContext` interface that provides access to:
- Collector configuration (`confmap.Conf`)
- Component host for introspection
- Module info for component discovery (requires `hostcapabilities.ModuleInfo`)
- Component factory for schema inspection (requires `hostcapabilities.ComponentFactory`)
- Telemetry buffer for querying

## Key Dependencies

- **Go 1.24+** (required)
- **OpenTelemetry Collector v0.136.0** (beta modules)
- **OpenTelemetry Collector v1.42.0** (stable modules)
- **MCP Go SDK v1.0.0** (`github.com/modelcontextprotocol/go-sdk`)

The project uses both stable and beta collector modules:
- Stable modules (v1.x): `component`, `confmap`, `consumer`, `extension`, `pdata`, etc.
- Beta modules (v0.x): `connector`, testing packages, capabilities packages

## Configuration

### Extension Configuration
```yaml
extensions:
  mcp:
    endpoint: localhost:9999        # HTTP endpoint for MCP server
    traces_buffer_size: 1000        # Number of trace batches to buffer
    metrics_buffer_size: 1000       # Number of metric batches to buffer
    logs_buffer_size: 1000          # Number of log batches to buffer
```

### Connector Configuration
The connector requires no configuration - it auto-discovers the MCP extension at startup.

### Pipeline Structure
The connector must be placed between data sources and the final output:
```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [mcp]  # Buffer telemetry in connector
    traces/buffer:
      receivers: [mcp]  # Connector passes through buffered data
      exporters: [debug]
```

## Important Implementation Details

### Thread Safety
- Extension uses `atomic.Value` for config and module info (lock-free reads)
- Buffer uses `sync.RWMutex` for concurrent access to the deque
- Connector clones data when downstream mutates to prevent races

### Zero-Copy Optimization
The connector checks if downstream consumers mutate data:
```go
if c.nextTracesMutates {
    tdClone := ptrace.NewTraces()
    td.CopyTo(tdClone)
    c.buffer.AddTraces(tdClone)
} else {
    c.buffer.AddTraces(td)  // Safe to share
}
```

### Host Capabilities
The extension optionally uses host capabilities:
- `hostcapabilities.ModuleInfo` - For component discovery (listing available components)
- `hostcapabilities.ComponentFactory` - For factory metadata and schema inspection
- Falls back gracefully if host doesn't provide these capabilities

### Config Updates
The extension implements `ConfigWatcher` to receive config updates:
- Config updates arrive via `NotifyConfig()` callback
- Stored in `atomic.Value` for concurrent access
- Tools can query current config at any time via `GetCollectorConf()`

## Development Status

**Fully Implemented (24/24 tools):**
- ✅ Extension framework with HTTP-based MCP server
- ✅ Connector for telemetry capture with zero-copy optimization
- ✅ Fixed-capacity deque buffer with thread-safe operations
- ✅ Config inspection tools (4/4) - full read access to collector config
- ✅ Component discovery tools (3/3) - list components, schemas, factory info
- ✅ Config modification tools (5/5) - validation only, no persistence
- ✅ Telemetry query tools (4/4) - CSV output, filtering, pagination
- ✅ Telemetry search tools (5/5) - search by criteria with context cancellation
- ✅ Runtime status tools (3/3) - component status, pipeline metrics, extensions

**Read-Only Limitations:**
- Config modification tools validate changes but cannot persist to disk or reload collector
- Would require file system write access and collector reload capability
- Currently provides validation and inspection only

## Code Quality Standards

### Linting Configuration
This project uses golangci-lint with configuration aligned to OpenTelemetry Collector Contrib standards:
- **23 enabled linters** including: revive, gocritic, gosec, govet, staticcheck, errcheck, etc.
- **2 enabled formatters**: gci (import ordering), gofumpt (strict formatting)
- Configuration in `.golangci.yml`
- Must maintain zero lint warnings at all times

### Common Lint Issues and Solutions
1. **Context usage**: Use `ctx.Err()` checks in loops that iterate over large datasets (search operations)
2. **Unused receivers**: Use `*Type` instead of named receiver when receiver is not used in method body
3. **Error creation**: Use `errors.New()` for static strings, `fmt.Errorf()` only when formatting
4. **Modern Go**: Use `any` instead of `interface{}` (Go 1.18+)
5. **Nolint comments**: Only use `//nolint:revive // reason` when context is kept for interface compatibility

### Testing Standards
- Tests are located next to implementation files (`*_test.go`)
- Use `componenttest`, `connectortest`, and `extensiontest` helpers from OTel Collector
- Mock `TelemetryBuffer` interface in connector tests for isolation
- All tests must pass before committing
- **Current test coverage**: 65 tests across 3 packages (connector, extension, buffer)
- No test files for `internal/tools` (MCP tool handlers tested via extension tests)
