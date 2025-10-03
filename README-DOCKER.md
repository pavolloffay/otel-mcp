# OpenTelemetry Collector with MCP - Container Setup

This directory contains everything needed to run the OpenTelemetry Collector with MCP extension in a container.

## Quick Start

### Build and Run with Apple Container

```bash
# First, build the collector locally
builder --config manifest-dev.yaml

# Build the container image (using pre-built binary)
container build -f Dockerfile.prebuilt -t otelcol-mcp-dev:latest .

# Run the container
# Note: Volume mounting may have limitations with Apple container
container run -d \
  --name otelcol-mcp \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 8888:8888 \
  -p 8889:8889 \
  -p 13133:13133 \
  -p 55679:55679 \
  -p 1777:1777 \
  otelcol-mcp-dev:latest

# View logs
container logs -f otelcol-mcp

# List containers
container list

# Stop the container
container stop otelcol-mcp
container delete otelcol-mcp
```

### Alternative: Build and Run with Docker

```bash
# Build the image
docker build -t otelcol-mcp-dev .

# Run the container
docker run -d \
  --name otelcol-mcp \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 8888:8888 \
  -p 13133:13133 \
  -v $(pwd)/config.yaml:/etc/otelcol/config.yaml:ro \
  otelcol-mcp-dev
```

## Configuration

The collector is configured via `config.yaml`. Key features:

- **OTLP Receivers**: gRPC (4317) and HTTP (4318)
- **Host Metrics**: System monitoring enabled
- **MCP Extension**: AI-powered collector introspection with stdio transport
- **MCP Connector**: Buffers telemetry for MCP tools to query
- **Exporters**: Debug, Prometheus, File

### MCP Extension Configuration

The MCP extension provides an AI-accessible interface to the collector:

```yaml
extensions:
  mcp:
    traces_buffer_size: 100   # Number of trace batches to buffer
    metrics_buffer_size: 100  # Number of metric batches to buffer
    logs_buffer_size: 100     # Number of log batches to buffer
```

The extension runs an MCP server on stdio, allowing AI tools to:
- Query recent telemetry (traces, metrics, logs)
- Inspect collector configuration
- List configured components
- Get buffer statistics

### Ports

| Port  | Protocol | Service |
|-------|----------|---------|
| 9999  | HTTP     | MCP Server (Streaming HTTP) |
| 4317  | gRPC     | OTLP Receiver |
| 4318  | HTTP     | OTLP Receiver |
| 8888  | HTTP     | Collector Internal Metrics |
| 8889  | HTTP     | Prometheus Exporter |
| 13133 | HTTP     | Health Check |
| 55679 | HTTP     | Zpages |
| 1777  | HTTP     | pprof |
| 14250 | gRPC     | Jaeger |
| 14268 | HTTP     | Jaeger |
| 9411  | HTTP     | Zipkin |

## Development Features

### With Monitoring Stack

To start with Prometheus and Grafana using docker-compose (if available):

```bash
docker-compose --profile monitoring up -d
```

Or manually start individual containers:

```bash
# Start Prometheus
container run -d --name prometheus \
  -p 9090:9090 \
  -v $(pwd)/prometheus.yaml:/etc/prometheus/prometheus.yml:ro \
  prom/prometheus:latest

# Start Grafana
container run -d --name grafana \
  -p 3000:3000 \
  -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
  grafana/grafana:latest
```

Access:
- Collector: http://localhost:13133 (health check)
- Prometheus: http://localhost:9090 (if started)
- Grafana: http://localhost:3000 (admin/admin, if started)
- Zpages: http://localhost:55679/debug

### Health Check

```bash
curl http://localhost:13133/
```

### View Metrics

```bash
# Collector internal metrics
curl http://localhost:8888/metrics

# Exported metrics (Prometheus format)
curl http://localhost:8889/metrics
```

## Using MCP Extension

The MCP extension provides these tools for AI interaction:

### Configuration Tools
- `get_config` - Get current collector configuration
- `get_component_config` - Get config for specific component
- `list_configured_components` - List all configured components
- `get_pipeline_config` - Get pipeline configuration

### Telemetry Query Tools
- `get_recent_traces` - Get recent traces from buffer
- `get_recent_metrics` - Get recent metrics from buffer
- `get_recent_logs` - Get recent logs from buffer
- `get_telemetry_summary` - Get buffer statistics

### Runtime Status Tools
- `get_extensions` - List running extensions

## Sending Test Data

### Using curl (OTLP/HTTP)

```bash
# Send a test trace
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "test-service"}
        }]
      },
      "scopeSpans": [{
        "scope": {"name": "test"},
        "spans": [{
          "traceId": "5B8EFFF798038103D269B633813FC60C",
          "spanId": "EEE19B7EC3C1B174",
          "name": "test-span",
          "startTimeUnixNano": "1544712660000000000",
          "endTimeUnixNano": "1544712661000000000",
          "kind": 1
        }]
      }]
    }]
  }'
```

### Using OpenTelemetry SDKs

Configure your application to export to:
- OTLP/gRPC: `http://localhost:4317`
- OTLP/HTTP: `http://localhost:4318`

## Troubleshooting

### View Logs

```bash
# Using Apple container
container logs -f otelcol-mcp

# Or with docker-compose
docker-compose logs -f otelcol-mcp
```

### Check Container Status

```bash
# Using Apple container
container list
# or
container ls

# Or with docker-compose
docker-compose ps
```

### Inspect Configuration

```bash
# Using Apple container
container exec otelcol-mcp cat /etc/otelcol/config.yaml

# Or with Docker
docker exec otelcol-mcp cat /etc/otelcol/config.yaml
```

### Debug Mode

Edit `config.yaml` and change log level:

```yaml
service:
  telemetry:
    logs:
      level: debug
```

Then restart:

```bash
# Using Apple container
container stop otelcol-mcp
container start otelcol-mcp

# Or with docker-compose
docker-compose restart otelcol-mcp
```

## Building

### Dockerfile Options

Two Dockerfiles are provided:

1. **Dockerfile** - Multi-stage build that compiles from source
   - Builds the collector inside the container using OCB
   - Useful for CI/CD or when you don't have local Go environment
   - May have network issues with Alpine package repos in some environments

2. **Dockerfile.prebuilt** - Uses pre-built binary (recommended for Apple container)
   - Requires local build first: `builder --config manifest-dev.yaml`
   - Copies the binary from `./dist/otelcol-mcp-dev`
   - Faster builds and avoids network issues
   - Recommended for Apple container tool

### Build Arguments

The multi-stage Dockerfile uses:

1. **Builder Stage**: Compiles the collector using OCB
2. **Runtime Stage**: Minimal Alpine image with only the binary

### Custom Manifest

To use a different component set, edit `manifest-dev.yaml` and rebuild:

```bash
# Using Apple container
container build --no-cache -t otelcol-mcp-dev:latest .

# Or with docker-compose
docker-compose build --no-cache
```

## File Structure

```
.
├── Dockerfile              # Multi-stage build
├── docker-compose.yaml     # Compose orchestration
├── config.yaml            # Collector configuration
├── prometheus.yaml        # Prometheus scrape config
├── manifest-dev.yaml      # OCB manifest for components
└── .dockerignore          # Build context exclusions
```

## Security Notes

- Container runs as non-root user `otel`
- Config file mounted read-only
- No sensitive data in base image
- Health checks enabled
- Resource limits recommended for production

## License

Apache 2.0
