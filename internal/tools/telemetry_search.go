// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchTracesInput struct {
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	SpanName    string `json:"span_name,omitempty" jsonschema:"Filter by span name (partial match)"`
	TraceID     string `json:"trace_id,omitempty" jsonschema:"Filter by trace ID (partial match)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of spans to return,100"`
}

type SearchTracesOutput struct {
	SpanCount int      `json:"span_count"`
	TraceIDs  []string `json:"trace_ids"`
	Spans     []string `json:"spans"`
}

// RegisterSearchTraces registers the search_traces tool
func RegisterSearchTraces(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[SearchTracesInput, SearchTracesOutput](server, &mcp.Tool{
		Name:        "search_traces",
		Description: "Search traces by criteria (service name, span name, trace ID). Returns matching spans with details.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchTracesInput) (*mcp.CallToolResult, SearchTracesOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		traces := ext.GetRecentTraces(1000, 0) // Get a large batch to search
		spans := []string{}
		traceIDMap := make(map[string]bool)
		spanCount := 0

		for _, td := range traces {
			if spanCount >= limit {
				break
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, SearchTracesOutput{}, ctx.Err()
			}

			for i := 0; i < td.ResourceSpans().Len(); i++ {
				if spanCount >= limit {
					break
				}

				rs := td.ResourceSpans().At(i)
				serviceName := "unknown"
				if sn, ok := rs.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				// Filter by service name if specified
				if input.ServiceName != "" && serviceName != input.ServiceName {
					continue
				}

				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					if spanCount >= limit {
						break
					}

					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						if spanCount >= limit {
							break
						}

						span := ss.Spans().At(k)
						spanName := span.Name()
						traceID := span.TraceID().String()

						// Filter by span name if specified (partial match)
						if input.SpanName != "" && !strings.Contains(strings.ToLower(spanName), strings.ToLower(input.SpanName)) {
							continue
						}

						// Filter by trace ID if specified (partial match)
						if input.TraceID != "" && !strings.Contains(strings.ToLower(traceID), strings.ToLower(input.TraceID)) {
							continue
						}

						spanCount++
						traceIDMap[traceID] = true
						spanSummary := fmt.Sprintf("trace_id=%s span_id=%s service=%s span=%s status=%s",
							traceID[:16]+"...",
							span.SpanID().String()[:8]+"...",
							serviceName,
							spanName,
							span.Status().Code().String())
						spans = append(spans, spanSummary)
					}
				}
			}
		}

		traceIDs := make([]string, 0, len(traceIDMap))
		for tid := range traceIDMap {
			traceIDs = append(traceIDs, tid)
		}

		return nil, SearchTracesOutput{
			SpanCount: spanCount,
			TraceIDs:  traceIDs,
			Spans:     spans,
		}, nil
	})
}

type SearchLogsInput struct {
	SeverityText string `json:"severity_text,omitempty" jsonschema:"Filter by severity (INFO, WARN, ERROR, etc.)"`
	Body         string `json:"body,omitempty" jsonschema:"Filter by log body (partial match)"`
	ServiceName  string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum number of logs to return,100"`
}

type SearchLogsOutput struct {
	LogCount int      `json:"log_count"`
	Logs     []string `json:"logs"`
}

// RegisterSearchLogs registers the search_logs tool
func RegisterSearchLogs(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[SearchLogsInput, SearchLogsOutput](server, &mcp.Tool{
		Name:        "search_logs",
		Description: "Search logs by criteria (severity, body text, service name). Returns matching log records.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchLogsInput) (*mcp.CallToolResult, SearchLogsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		logs := ext.GetRecentLogs(1000, 0) // Get a large batch to search
		logRecords := []string{}
		logCount := 0

		for _, ld := range logs {
			if logCount >= limit {
				break
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, SearchLogsOutput{}, ctx.Err()
			}

			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				if logCount >= limit {
					break
				}

				rl := ld.ResourceLogs().At(i)
				serviceName := "unknown"
				if sn, ok := rl.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				// Filter by service name if specified
				if input.ServiceName != "" && serviceName != input.ServiceName {
					continue
				}

				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					if logCount >= limit {
						break
					}

					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						if logCount >= limit {
							break
						}

						lr := sl.LogRecords().At(k)
						severityText := lr.SeverityText()
						body := lr.Body().AsString()

						// Filter by severity if specified
						if input.SeverityText != "" && !strings.EqualFold(severityText, input.SeverityText) {
							continue
						}

						// Filter by body text if specified (partial match)
						if input.Body != "" && !strings.Contains(strings.ToLower(body), strings.ToLower(input.Body)) {
							continue
						}

						logCount++
						logSummary := fmt.Sprintf("service=%s severity=%s body=%s",
							serviceName,
							severityText,
							truncateString(body, 80))
						logRecords = append(logRecords, logSummary)
					}
				}
			}
		}

		return nil, SearchLogsOutput{
			LogCount: logCount,
			Logs:     logRecords,
		}, nil
	})
}

type SearchMetricsInput struct {
	MetricName  string `json:"metric_name,omitempty" jsonschema:"Filter by metric name (partial match)"`
	ServiceName string `json:"service_name,omitempty" jsonschema:"Filter by service name"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of metrics to return,100"`
}

type SearchMetricsOutput struct {
	MetricCount int      `json:"metric_count"`
	Metrics     []string `json:"metrics"`
}

// RegisterSearchMetrics registers the search_metrics tool
func RegisterSearchMetrics(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[SearchMetricsInput, SearchMetricsOutput](server, &mcp.Tool{
		Name:        "search_metrics",
		Description: "Search metrics by criteria (metric name, service name). Returns matching metrics with details.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchMetricsInput) (*mcp.CallToolResult, SearchMetricsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 100
		}

		metricsData := ext.GetRecentMetrics(1000, 0) // Get a large batch to search
		metrics := []string{}
		metricCount := 0

		for _, md := range metricsData {
			if metricCount >= limit {
				break
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, SearchMetricsOutput{}, ctx.Err()
			}

			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				if metricCount >= limit {
					break
				}

				rm := md.ResourceMetrics().At(i)
				serviceName := "unknown"
				if sn, ok := rm.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				// Filter by service name if specified
				if input.ServiceName != "" && serviceName != input.ServiceName {
					continue
				}

				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					if metricCount >= limit {
						break
					}

					sm := rm.ScopeMetrics().At(j)
					for k := 0; k < sm.Metrics().Len(); k++ {
						if metricCount >= limit {
							break
						}

						metric := sm.Metrics().At(k)
						metricName := metric.Name()

						// Filter by metric name if specified (partial match)
						if input.MetricName != "" && !strings.Contains(strings.ToLower(metricName), strings.ToLower(input.MetricName)) {
							continue
						}

						metricCount++
						metricSummary := fmt.Sprintf("service=%s name=%s type=%s unit=%s",
							serviceName,
							metricName,
							metric.Type().String(),
							metric.Unit())
						metrics = append(metrics, metricSummary)
					}
				}
			}
		}

		return nil, SearchMetricsOutput{
			MetricCount: metricCount,
			Metrics:     metrics,
		}, nil
	})
}

type GetTraceByIDInput struct {
	TraceID string `json:"trace_id" jsonschema:"Full trace ID to retrieve,required"`
}

type GetTraceByIDOutput struct {
	TraceID   string   `json:"trace_id"`
	SpanCount int      `json:"span_count"`
	Spans     []string `json:"spans"`
	Found     bool     `json:"found"`
}

// RegisterGetTraceByID registers the get_trace_by_id tool
func RegisterGetTraceByID(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[GetTraceByIDInput, GetTraceByIDOutput](server, &mcp.Tool{
		Name:        "get_trace_by_id",
		Description: "Get a specific trace by trace ID. Returns all spans for the trace.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetTraceByIDInput) (*mcp.CallToolResult, GetTraceByIDOutput, error) {
		if input.TraceID == "" {
			return nil, GetTraceByIDOutput{}, errors.New("trace_id is required")
		}

		traces := ext.GetRecentTraces(1000, 0) // Get all recent traces
		spans := []string{}
		found := false

		for _, td := range traces {
			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, GetTraceByIDOutput{}, ctx.Err()
			}

			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rs := td.ResourceSpans().At(i)
				serviceName := "unknown"
				if sn, ok := rs.Resource().Attributes().Get("service.name"); ok {
					serviceName = sn.AsString()
				}

				for j := 0; j < rs.ScopeSpans().Len(); j++ {
					ss := rs.ScopeSpans().At(j)
					for k := 0; k < ss.Spans().Len(); k++ {
						span := ss.Spans().At(k)
						traceID := span.TraceID().String()

						// Match exact trace ID
						if traceID == input.TraceID {
							found = true
							spanDetails := fmt.Sprintf("span_id=%s parent=%s service=%s name=%s kind=%s status=%s",
								span.SpanID().String(),
								span.ParentSpanID().String(),
								serviceName,
								span.Name(),
								span.Kind().String(),
								span.Status().Code().String())
							spans = append(spans, spanDetails)
						}
					}
				}
			}
		}

		return nil, GetTraceByIDOutput{
			TraceID:   input.TraceID,
			SpanCount: len(spans),
			Spans:     spans,
			Found:     found,
		}, nil
	})
}

type FindRelatedTelemetryInput struct {
	TraceID string `json:"trace_id,omitempty" jsonschema:"Trace ID to find related telemetry"`
	SpanID  string `json:"span_id,omitempty" jsonschema:"Span ID to find related telemetry"`
}

type FindRelatedTelemetryOutput struct {
	TraceID     string   `json:"trace_id,omitempty"`
	SpanCount   int      `json:"span_count"`
	LogCount    int      `json:"log_count"`
	MetricCount int      `json:"metric_count"`
	Spans       []string `json:"spans,omitempty"`
	Logs        []string `json:"logs,omitempty"`
	Metrics     []string `json:"metrics,omitempty"`
}

// RegisterFindRelatedTelemetry registers the find_related_telemetry tool
func RegisterFindRelatedTelemetry(server *mcp.Server, ext ExtensionContext) {
	mcp.AddTool[FindRelatedTelemetryInput, FindRelatedTelemetryOutput](server, &mcp.Tool{
		Name:        "find_related_telemetry",
		Description: "Find related telemetry (logs, metrics) based on trace context. Correlates logs and metrics with trace/span IDs.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
			OpenWorldHint:  boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input FindRelatedTelemetryInput) (*mcp.CallToolResult, FindRelatedTelemetryOutput, error) {
		if input.TraceID == "" && input.SpanID == "" {
			return nil, FindRelatedTelemetryOutput{}, errors.New("either trace_id or span_id is required")
		}

		output := FindRelatedTelemetryOutput{
			TraceID: input.TraceID,
		}

		// Find related spans if trace ID is provided
		if input.TraceID != "" {
			traces := ext.GetRecentTraces(1000, 0)
			for _, td := range traces {
				// Check for context cancellation
				if ctx.Err() != nil {
					return nil, FindRelatedTelemetryOutput{}, ctx.Err()
				}

				for i := 0; i < td.ResourceSpans().Len(); i++ {
					rs := td.ResourceSpans().At(i)
					for j := 0; j < rs.ScopeSpans().Len(); j++ {
						ss := rs.ScopeSpans().At(j)
						for k := 0; k < ss.Spans().Len(); k++ {
							span := ss.Spans().At(k)
							if span.TraceID().String() == input.TraceID {
								output.SpanCount++
								if output.Spans == nil {
									output.Spans = []string{}
								}
								output.Spans = append(output.Spans, fmt.Sprintf("span_id=%s name=%s",
									span.SpanID().String(), span.Name()))
							}
						}
					}
				}
			}
		}

		// Find related logs
		logs := ext.GetRecentLogs(1000, 0)
		for _, ld := range logs {
			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, FindRelatedTelemetryOutput{}, ctx.Err()
			}

			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				rl := ld.ResourceLogs().At(i)
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					for k := 0; k < sl.LogRecords().Len(); k++ {
						lr := sl.LogRecords().At(k)

						// Check if log has matching trace/span ID
						logTraceID := lr.TraceID().String()
						logSpanID := lr.SpanID().String()

						matched := false
						if input.TraceID != "" && logTraceID == input.TraceID {
							matched = true
						}
						if input.SpanID != "" && logSpanID == input.SpanID {
							matched = true
						}

						if matched {
							output.LogCount++
							if output.Logs == nil {
								output.Logs = []string{}
							}
							output.Logs = append(output.Logs, fmt.Sprintf("severity=%s body=%s",
								lr.SeverityText(), truncateString(lr.Body().AsString(), 60)))
						}
					}
				}
			}
		}

		// Note: Metrics typically don't have trace/span context in OTLP,
		// so we can't easily correlate them without exemplars
		// Leaving metric count at 0 for now

		return nil, output, nil
	})
}

// Helper function to truncate strings in a UTF-8 safe manner
func truncateString(s string, maxLen int) string {
	// Convert to runes to handle multi-byte UTF-8 characters correctly
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
