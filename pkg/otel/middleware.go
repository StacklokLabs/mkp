package otel

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/StacklokLabs/mkp/pkg/otel"

// Middleware returns a tool handler middleware that adds OpenTelemetry tracing
func Middleware() server.ToolHandlerMiddleware {
	tracer := otel.Tracer(instrumentationName)
	meter := otel.Meter(instrumentationName)

	requestCounter, _ := meter.Int64Counter("mkp.tool.requests",
		metric.WithDescription("Number of tool requests"),
		metric.WithUnit("1"),
	)

	requestDuration, _ := meter.Float64Histogram("mkp.tool.duration",
		metric.WithDescription("Duration of tool requests"),
		metric.WithUnit("ms"),
	)

	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			toolName := request.Params.Name
			start := time.Now()

			ctx, span := tracer.Start(ctx, "tool."+toolName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("mcp.tool.name", toolName),
				),
			)
			defer span.End()

			result, err := next(ctx, request)

			duration := float64(time.Since(start).Milliseconds())
			attrs := []attribute.KeyValue{
				attribute.String("tool", toolName),
			}

			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
				attrs = append(attrs, attribute.Bool("error", true))
			} else if result != nil && result.IsError {
				span.SetStatus(codes.Error, "tool returned error")
				attrs = append(attrs, attribute.Bool("error", true))
			} else {
				span.SetStatus(codes.Ok, "")
				attrs = append(attrs, attribute.Bool("error", false))
			}

			requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
			requestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

			return result, err
		}
	}
}
