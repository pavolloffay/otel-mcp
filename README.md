# OpenTelemetry Collector MCP Extension

An MCP (Model Context Protocol) extension and connector for the OpenTelemetry Collector that provides LLM-accessible tools for configuration management and telemetry inspection.

## Overview

This project provides two components:

1. **MCP Extension** (`mcpextension`) - Runs an MCP server over HTTP (streaming HTTP transport) and exposes tools for config/telemetry access
2. **MCP Connector** (`mcpconnector`) - Captures telemetry flowing through pipelines and buffers it for querying

## Features

### 24 MCP Tools

#### Config Inspection (4 tools)
- `get_config` - Get current collector configuration (full or by section)
- `get_component_config` - Get config for a specific component
- `list_configured_components` - List all configured components
- `get_pipeline_config` - Get configuration for a pipeline

#### Component Discovery (3 tools)
- `list_available_components` - List available component types
- `get_component_schema` - Get config schema for a component type
- `get_factory_info` - Get factory metadata and stability levels

#### Config Modification (5 tools - stubs)
- `update_config` - Modify config and write to file
- `add_component` - Add new component to config
- `remove_component` - Remove component from config
- `validate_config` - Validate proposed config changes
- `update_pipeline` - Modify pipeline configuration

#### Telemetry Query (4 tools)
- `get_recent_traces` - Get recent traces from buffer
- `get_recent_metrics` - Get recent metrics from buffer
- `get_recent_logs` - Get recent logs from buffer
- `get_telemetry_summary` - Get buffer statistics

#### Telemetry Search (5 tools - stubs)
- `search_traces` - Search traces by criteria
- `search_logs` - Search logs by criteria
- `search_metrics` - Search metrics by criteria
- `get_trace_by_id` - Get specific trace by ID
- `find_related_telemetry` - Find related telemetry by trace context

#### Runtime/Status (3 tools)
- `get_component_status` - Get component runtime status
- `get_pipeline_metrics` - Get internal pipeline metrics
- `get_extensions` - List running extensions

## Architecture

### Extension
- Implements `extensioncapabilities.ConfigWatcher` to receive collector config updates
- Implements `TelemetryBuffer` interface to store telemetry
- Runs MCP server over stdio transport
- Registers all MCP tools on startup

### Connector
- Pass-through connector (Traces→Traces, Metrics→Metrics, Logs→Logs)
- Finds MCP extension via `component.Host.GetExtensions()`
- Clones telemetry and stores in extension's circular buffer
- Zero configuration needed

### Circular Buffer
- Thread-safe ring buffer for each signal type
- Configurable capacity per signal
- Stores recent batches for querying

## Configuration

### Extension Config

```yaml
extensions:
  mcp:
    traces_buffer_size: 1000   # Number of trace batches to buffer
    metrics_buffer_size: 1000  # Number of metric batches to buffer
    logs_buffer_size: 1000     # Number of log batches to buffer
```

### Connector Config

```yaml
connectors:
  mcp:
    # No configuration needed - auto-discovers extension
```

### Full Example

```yaml
extensions:
  mcp:
    traces_buffer_size: 1000
    metrics_buffer_size: 1000
    logs_buffer_size: 1000

connectors:
  mcp:

receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  debug:
    verbosity: detailed

service:
  extensions: [mcp]
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [mcp]
    traces/output:
      receivers: [mcp]
      exporters: [debug]
```

## Building

### As Part of a Collector Distribution

Use the [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder):

```yaml
# builder-config.yaml
dist:
  name: otelcol-mcp
  description: OpenTelemetry Collector with MCP support
  output_path: ./dist

extensions:
  - gomod: github.com/austinparker/otel-mcp/extension/mcpextension v0.1.0

connectors:
  - gomod: github.com/austinparker/otel-mcp/connector/mcpconnector v0.1.0
```

```bash
ocb --config builder-config.yaml
```

## Usage

### With Claude Desktop

Add to your Claude Desktop MCP config:

```json
{
  "mcpServers": {
    "otel-collector": {
      "command": "/path/to/otelcol-mcp",
      "args": ["--config", "/path/to/config.yaml"]
    }
  }
}
```

### Tool Examples

**Get current configuration:**
```
Use the get_config tool to show me the current receiver configuration
```

**Query recent telemetry:**
```
Use get_recent_traces with limit=5 to show me the last 5 trace batches
```

**List available components:**
```
What receivers are available in this collector? Use list_available_components
```

## Project Structure

```
otel-mcp/
├── extension/mcpextension/     # MCP extension implementation
│   ├── config.go
│   ├── factory.go
│   ├── extension.go
│   └── tools.go
├── connector/mcpconnector/     # Telemetry capture connector
│   ├── config.go
│   ├── factory.go
│   └── connector.go
├── internal/
│   ├── buffer/                 # Circular buffer implementation
│   │   └── buffer.go
│   └── tools/                  # MCP tool implementations
│       ├── context.go
│       ├── config_inspection.go
│       ├── component_discovery.go
│       ├── config_modification.go
│       ├── telemetry_query.go
│       ├── telemetry_search.go
│       └── runtime_status.go
├── go.mod
└── README.md
```

## Development Status

### ✅ Implemented
- Extension framework with MCP server
- Connector for telemetry capture
- Circular buffer for telemetry storage
- Config inspection tools (4/4)
- Component discovery tools (3/3)
- Telemetry query tools (4/4)
- Runtime status tool (1/3)

### 🚧 In Progress
- MCP SDK v1.0.0 API compatibility updates
- Config modification tools (0/5)
- Telemetry search tools (0/5)
- Runtime status tools (2/3)

### 📋 Planned
- Config file persistence for modifications
- Advanced search with filters
- Component status monitoring
- Performance optimizations

## Requirements

- Go 1.24+
- OpenTelemetry Collector v0.136.0+ (beta modules)
- OpenTelemetry Collector v1.42.0+ (stable modules)
- MCP Go SDK v1.0.0

## License

Apache 2.0 - See LICENSE file

## Contributing

This is an experimental project. Contributions welcome!

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Acknowledgments

- [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)
- [Model Context Protocol](https://modelcontextprotocol.io)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
