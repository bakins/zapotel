package zapotel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/require"

	"github.com/bakins/zapotel"
)

func TestCore(t *testing.T) {
	var buf bytes.Buffer

	c := zapotel.NewCore(zapcore.AddSync(&buf), zapcore.DebugLevel)
	logger := zap.New(c)

	r, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.Bool("bool", true),
			attribute.StringSlice("string_slice", []string{"a", "b"}),
			attribute.Float64("float", 64),
		),
	)
	require.NoError(t, err)

	logger = logger.Named("testing")

	logger = logger.With(zapotel.Resource(r))

	logger = logger.With(zap.Duration("seconds", time.Millisecond*1450))

	logger.Info(
		"testing 1234",
		zap.String("field_one", "one"), zap.Int8("field_two", 2),
	)

	require.NoError(t, logger.Sync())

	want := Entry{
		ScopeName:    "testing",
		Body:         "testing 1234",
		SeverityText: "INFO",
		Severity:     9,
		Resource: map[string]interface{}{
			"bool":         true,
			"string_slice": []interface{}{"a", "b"},
			"float":        64.0,
		},
		Attributes: map[string]interface{}{
			"field_one": "one",
			"field_two": float64(2),
			"seconds":   float64(1.45),
		},
	}

	var got Entry
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	got.Timestamp = 0

	require.Equal(t, want, got)
}

// based on
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/f531a708efce0e43cc4e999fd66f5ee7411e9c0e/pkg/stanza/entry/entry.go
type Entry struct {
	Body              interface{}            `json:"body"                    yaml:"body"`
	Attributes        map[string]interface{} `json:"attributes,omitempty"    yaml:"attributes,omitempty"`
	Resource          map[string]interface{} `json:"resource,omitempty"      yaml:"resource,omitempty"`
	SeverityText      string                 `json:"severity_text,omitempty" yaml:"severity_text,omitempty"`
	ScopeName         string                 `json:"scope_name"              yaml:"scope_name"`
	SpanID            []byte                 `json:"span_id,omitempty"       yaml:"span_id,omitempty"`
	TraceID           []byte                 `json:"trace_id,omitempty"      yaml:"trace_id,omitempty"`
	TraceFlags        []byte                 `json:"trace_flags,omitempty"   yaml:"trace_flags,omitempty"`
	ObservedTimestamp int64                  `json:"observed_timestamp,omitempty"      yaml:"observed_timestamp"`
	Timestamp         int64                  `json:"timestamp"               yaml:"timestamp"`
	Severity          int                    `json:"severity"                yaml:"severity"`
}

func BenchmarkCore(b *testing.B) {
	var buf bytes.Buffer
	buf.Grow(8192)

	c := zapotel.NewCore(zapcore.AddSync(&buf), zapcore.DebugLevel)
	logger := zap.New(c)

	r, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.Bool("bool", true),
			attribute.StringSlice("string_slice", []string{"a", "b"}),
			attribute.Float64("float", 64),
		),
	)
	require.NoError(b, err)

	logger = logger.Named("testing")
	logger = logger.With(zapotel.Resource(r))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		logger.Info(
			"testing 1234",
			zap.String("field_one", "one"), zap.Int8("field_two", 2),
			zap.Duration("seconds", time.Millisecond*1450),
		)

		buf.Reset()
	}
}
