# Multi-stage build for OpenTelemetry Collector with MCP
# Stage 1: Build the collector
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

# Install OpenTelemetry Collector Builder
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.136.0

# Set working directory
WORKDIR /build

# Copy source code
COPY . .

# Build the collector using the manifest
RUN builder --config manifest-dev.yaml

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create a non-root user
RUN addgroup -S otel && adduser -S otel -G otel

# Copy the built collector binary
COPY --from=builder /build/dist/otelcol-mcp-dev /otelcol-mcp-dev

# Set ownership
RUN chown otel:otel /otelcol-mcp-dev

# Switch to non-root user
USER otel

# Expose common ports
# OTLP gRPC
EXPOSE 4317
# OTLP HTTP
EXPOSE 4318
# Prometheus metrics
EXPOSE 8888
# Health check
EXPOSE 13133
# Zpages
EXPOSE 55679
# Jaeger gRPC
EXPOSE 14250
# Jaeger thrift HTTP
EXPOSE 14268
# Zipkin
EXPOSE 9411

# Set entrypoint
ENTRYPOINT ["/otelcol-mcp-dev"]
CMD ["--config", "/etc/otelcol/config.yaml"]
