// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// mockBuffer implements TelemetryBuffer for testing
type mockBuffer struct {
	traces  []ptrace.Traces
	metrics []pmetric.Metrics
	logs    []plog.Logs
}

func (m *mockBuffer) AddTraces(td ptrace.Traces) {
	m.traces = append(m.traces, td)
}

func (m *mockBuffer) AddMetrics(md pmetric.Metrics) {
	m.metrics = append(m.metrics, md)
}

func (m *mockBuffer) AddLogs(ld plog.Logs) {
	m.logs = append(m.logs, ld)
}

// mockExtension implements the TelemetryBuffer interface for testing
type mockExtension struct {
	component.Component
	buffer *mockBuffer
}

func (m *mockExtension) AddTraces(td ptrace.Traces) {
	m.buffer.AddTraces(td)
}

func (m *mockExtension) AddMetrics(md pmetric.Metrics) {
	m.buffer.AddMetrics(md)
}

func (m *mockExtension) AddLogs(ld plog.Logs) {
	m.buffer.AddLogs(ld)
}

// mockHost provides a mock host with MCP extension
type mockHost struct {
	component.Host
	extension *mockExtension
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("mcp"): m.extension,
	}
}

func TestMCPConnectorTraces(t *testing.T) {
	ctx := context.Background()
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))

	tracesSink := new(consumertest.TracesSink)
	conn := newConnector(set, tracesSink, nil, nil)
	require.NotNil(t, conn)

	buffer := &mockBuffer{}
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extension: &mockExtension{
			buffer: buffer,
		},
	}

	require.NoError(t, conn.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, conn.Shutdown(ctx)) })

	// Test consuming traces
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	require.NoError(t, conn.ConsumeTraces(ctx, td))

	// Verify traces were passed through
	assert.Len(t, tracesSink.AllTraces(), 1)

	// Verify traces were buffered
	assert.Len(t, buffer.traces, 1)
}

func TestMCPConnectorMetrics(t *testing.T) {
	ctx := context.Background()
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))

	metricsSink := new(consumertest.MetricsSink)
	conn := newConnector(set, nil, metricsSink, nil)
	require.NotNil(t, conn)

	buffer := &mockBuffer{}
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extension: &mockExtension{
			buffer: buffer,
		},
	}

	require.NoError(t, conn.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, conn.Shutdown(ctx)) })

	// Test consuming metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")

	require.NoError(t, conn.ConsumeMetrics(ctx, md))
	require.NoError(t, conn.ConsumeMetrics(ctx, md))

	// Verify metrics were passed through
	assert.Len(t, metricsSink.AllMetrics(), 2)

	// Verify metrics were buffered
	assert.Len(t, buffer.metrics, 2)
}

func TestMCPConnectorLogs(t *testing.T) {
	ctx := context.Background()
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))

	logsSink := new(consumertest.LogsSink)
	conn := newConnector(set, nil, nil, logsSink)
	require.NotNil(t, conn)

	buffer := &mockBuffer{}
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extension: &mockExtension{
			buffer: buffer,
		},
	}

	require.NoError(t, conn.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, conn.Shutdown(ctx)) })

	// Test consuming logs
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")

	require.NoError(t, conn.ConsumeLogs(ctx, ld))
	require.NoError(t, conn.ConsumeLogs(ctx, ld))
	require.NoError(t, conn.ConsumeLogs(ctx, ld))

	// Verify logs were passed through
	assert.Len(t, logsSink.AllLogs(), 3)

	// Verify logs were buffered
	assert.Len(t, buffer.logs, 3)
}

func TestMCPConnectorWithoutExtension(t *testing.T) {
	ctx := context.Background()
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))

	tracesSink := new(consumertest.TracesSink)
	conn := newConnector(set, tracesSink, nil, nil)
	require.NotNil(t, conn)

	// Start without MCP extension
	require.NoError(t, conn.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, conn.Shutdown(ctx)) })

	// Should still pass through data
	td := ptrace.NewTraces()
	require.NoError(t, conn.ConsumeTraces(ctx, td))

	assert.Len(t, tracesSink.AllTraces(), 1)
	// Buffer should be nil, so nothing buffered
	assert.Nil(t, conn.buffer)
}

func TestMCPConnectorCapabilities(t *testing.T) {
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))
	conn := newConnector(set, nil, nil, nil)

	caps := conn.Capabilities()
	assert.False(t, caps.MutatesData)
}

func TestMCPConnectorCloningOptimization(t *testing.T) {
	ctx := context.Background()
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))

	// Create a consumer that does NOT mutate data
	nonMutatingConsumer := &nonMutatingTracesConsumer{}
	conn := newConnector(set, nonMutatingConsumer, nil, nil)
	require.NotNil(t, conn)

	buffer := &mockBuffer{}
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extension: &mockExtension{
			buffer: buffer,
		},
	}

	require.NoError(t, conn.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, conn.Shutdown(ctx)) })

	// Verify cloning optimization flag
	assert.False(t, conn.nextTracesMutates, "should detect non-mutating consumer")

	// Test consuming traces
	td := ptrace.NewTraces()
	require.NoError(t, conn.ConsumeTraces(ctx, td))

	// Both should have same reference since no mutation
	assert.Len(t, buffer.traces, 1)
}

func TestMCPConnectorWithMutatingConsumer(t *testing.T) {
	ctx := context.Background()
	set := connectortest.NewNopSettings(component.MustNewType("mcp"))

	// Create a consumer that DOES mutate data
	mutatingConsumer := &mutatingTracesConsumer{}
	conn := newConnector(set, mutatingConsumer, nil, nil)
	require.NotNil(t, conn)

	buffer := &mockBuffer{}
	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extension: &mockExtension{
			buffer: buffer,
		},
	}

	require.NoError(t, conn.Start(ctx, host))
	t.Cleanup(func() { require.NoError(t, conn.Shutdown(ctx)) })

	// Verify cloning is enabled for mutating consumer
	assert.True(t, conn.nextTracesMutates, "should detect mutating consumer")

	// Test consuming traces
	td := ptrace.NewTraces()
	require.NoError(t, conn.ConsumeTraces(ctx, td))

	// Should have cloned data
	assert.Len(t, buffer.traces, 1)
}

// Test consumers
type nonMutatingTracesConsumer struct{}

func (*nonMutatingTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (*nonMutatingTracesConsumer) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}

type mutatingTracesConsumer struct{}

func (*mutatingTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (*mutatingTracesConsumer) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}
