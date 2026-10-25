package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTraceIDsInLogs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(traceHandler{slog.NewJSONHandler(&buf, nil)}).With("service", "x")

	ctx, span := sdktrace.NewTracerProvider().Tracer("t").Start(context.Background(), "op")
	log.InfoContext(ctx, "hello")
	span.End()
	log.InfoContext(context.Background(), "plain")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var traced, plain map[string]any
	if err := json.Unmarshal(lines[0], &traced); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[1], &plain); err != nil {
		t.Fatal(err)
	}
	if traced["trace_id"] != span.SpanContext().TraceID().String() || traced["span_id"] == nil || traced["service"] != "x" {
		t.Fatalf("traced line %v", traced)
	}
	if _, ok := plain["trace_id"]; ok {
		t.Fatalf("plain line has trace_id: %v", plain)
	}
}
