// Package logging builds Mini Bank's structured logger: leveled JSON output
// with every log line correlated to the OpenTelemetry trace/span that
// produced it, when one is present in the context.
package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// New builds a slog.Logger that writes JSON to w at the given level and
// tags every record with the supplied static attributes (service name,
// version, environment, ...).
func New(w io.Writer, level slog.Level, staticAttrs ...slog.Attr) *slog.Logger {
	base := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: level <= slog.LevelDebug,
	})
	return slog.New(&traceHandler{Handler: base}).With(attrsToArgs(staticAttrs)...)
}

// ParseLevel maps a case-insensitive level name (debug/info/warn/error) to
// its slog.Level, defaulting to Info for anything unrecognized.
func ParseLevel(name string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return slog.LevelInfo
	}
	return level
}

// traceHandler wraps a slog.Handler and, when the record's context carries
// a valid OpenTelemetry span, adds trace_id/span_id attributes to it — the
// standard way to pivot from a log line to the distributed trace it belongs
// to (and back, via span events) without threading IDs through every call
// manually.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return args
}
