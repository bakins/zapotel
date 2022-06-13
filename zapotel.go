package zapotel

import (
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewEncoder() zapcore.Encoder {
	cfg := EncoderConfig()
	return zapcore.NewJSONEncoder(cfg)
}

func NewLogger(enab zapcore.LevelEnabler) *zap.Logger {
	c := NewCore(zapcore.AddSync(os.Stdout), enab)
	return zap.New(c, zap.ErrorOutput(zapcore.AddSync(os.Stderr)))
}

func NewCore(ws zapcore.WriteSyncer, enab zapcore.LevelEnabler) *Core {
	enc := NewEncoder()

	c := Core{
		next: zapcore.NewCore(enc, ws, enab),
	}

	return &c
}

func WrapCore(base zapcore.Core) zapcore.Core {
	c := Core{
		next: base,
	}

	return &c
}

// https://opentelemetry.io/docs/reference/specification/logs/data-model/
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/1db76fe08f076860dca5ee7cf846b315c6afb63a/pkg/stanza/entry/entry.go#L25

func EncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey:       zapcore.OmitKey,
		LevelKey:         "severity_text",
		TimeKey:          "timestamp",
		NameKey:          "scope_name",
		CallerKey:        zapcore.OmitKey,
		FunctionKey:      zapcore.OmitKey,
		StacktraceKey:    zapcore.OmitKey,
		SkipLineEnding:   false,
		EncodeLevel:      levelEncoder,
		EncodeTime:       zapcore.EpochNanosTimeEncoder,
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		ConsoleSeparator: "",
	}
}

type Core struct {
	next   zapcore.Core
	fields []zap.Field
}

func (c *Core) Enabled(l zapcore.Level) bool {
	return c.next.Enabled(l)
}

func (c *Core) Sync() error {
	return c.next.Sync()
}

func (c *Core) With(fields []zap.Field) zapcore.Core {
	newFields := make([]zap.Field, 0, len(c.fields)+len(fields))
	newFields = append(newFields, c.fields...)
	newFields = append(newFields, fields...)

	newCore := Core{
		next:   c.next,
		fields: newFields,
	}

	return &newCore
}

func (c *Core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}

	return ce
}

func (c *Core) Write(e zapcore.Entry, in []zapcore.Field) error {
	// https://opentelemetry.io/docs/reference/specification/logs/data-model/#log-and-event-record-definition

	// TODO: TraceFlags, InstrumentationScope,

	var (
		traceID  *zap.Field
		spanID   *zap.Field
		resource *zap.Field
	)

	// this tries to avoid allocations. may need additional tuning.

	length := len(in) + 3 + len(c.fields)
	out := make([]zap.Field, 0, length)
	out = append(out, zap.String("body", e.Message))
	out = append(out, zap.Int("severity", levelToSeverity(e.Level)))

	attributes := make([]zap.Field, 0, length)

	// this is a bit ugly. find our "special" fields
	for _, fields := range [][]zapcore.Field{c.fields, in} {
	FIELDS:
		for i := range fields {
			f := &fields[i]

			switch f.Type {
			case zapcore.InlineMarshalerType:
				switch f.Key {
				case spanIDKey:
					if _, ok := f.Interface.(spanIDWrapper); ok {
						spanID = f
						continue FIELDS
					}
				case traceIDKey:
					if _, ok := f.Interface.(traceIDWrapper); ok {
						traceID = f
						continue FIELDS
					}
				}

			case zapcore.ObjectMarshalerType:
				if f.Key == resourceKey {
					if _, ok := f.Interface.(resourceWrapper); ok {
						resource = f
						continue FIELDS
					}
				}
			}

			attributes = append(attributes, *f)

		}
	}

	if traceID != nil {
		out = append(out, *traceID)
	}

	if spanID != nil {
		out = append(out, *spanID)
	}

	if resource != nil {
		out = append(out, *resource)
	}

	out = append(out, zap.Namespace("attributes"))
	out = append(out, attributes...)

	return c.next.Write(e, out)
}

func levelToSeverity(l zapcore.Level) int {
	switch l {
	case zapcore.DebugLevel:
		return 5
	case zapcore.InfoLevel:
		return 9
	case zapcore.WarnLevel:
		return 13
	case zapcore.ErrorLevel:
		return 17
	case zapcore.PanicLevel, zapcore.FatalLevel:
		return 21
	default:
		return 9
	}
}

func SpanID(spanID trace.SpanID) zap.Field {
	if !spanID.IsValid() {
		return zap.Field{
			Key: zapcore.OmitKey,
		}
	}
	w := spanIDWrapper{
		spanID: spanID,
	}

	return zap.Inline(w)
}

type spanIDWrapper struct {
	spanID trace.SpanID
}

const (
	spanIDKey   = "span_id"
	traceIDKey  = "trace_id"
	resourceKey = "resource"
)

func (w spanIDWrapper) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddBinary(spanIDKey, w.spanID[:])

	return nil
}

func TraceID(traceID trace.TraceID) zap.Field {
	if !traceID.IsValid() {
		return zap.Field{
			Key: zapcore.OmitKey,
		}
	}

	w := traceIDWrapper{
		traceID: traceID,
	}

	return zap.Inline(w)
}

type traceIDWrapper struct {
	traceID trace.TraceID
}

func (w traceIDWrapper) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddBinary(traceIDKey, w.traceID[:])

	return nil
}

func Resource(resource *resource.Resource) zap.Field {
	if resource == nil {
		return zap.Field{
			Key: zapcore.OmitKey,
		}
	}

	w := resourceWrapper{
		resource: resource,
	}

	return zap.Object(resourceKey, w)
}

type resourceWrapper struct {
	resource *resource.Resource
}

func (r resourceWrapper) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	attributes := r.resource.Attributes()

	for _, a := range attributes {
		switch a.Value.Type() {
		case attribute.BOOL:
			enc.AddBool(string(a.Key), a.Value.AsBool())
		case attribute.INT64:
			enc.AddInt64(string(a.Key), a.Value.AsInt64())
		case attribute.FLOAT64:
			enc.AddFloat64(string(a.Key), a.Value.AsFloat64())
		case attribute.STRING:
			enc.AddString(string(a.Key), a.Value.AsString())
		case attribute.BOOLSLICE:
			zap.Bools(string(a.Key), a.Value.AsBoolSlice()).AddTo(enc)
		case attribute.INT64SLICE:
			zap.Int64s(string(a.Key), a.Value.AsInt64Slice()).AddTo(enc)
		case attribute.FLOAT64SLICE:
			zap.Float64s(string(a.Key), a.Value.AsFloat64Slice()).AddTo(enc)
		case attribute.STRINGSLICE:
			zap.Strings(string(a.Key), a.Value.AsStringSlice()).AddTo(enc)
		}
	}

	return nil
}

func levelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.DebugLevel:
		enc.AppendString("DEBUG")
	case zapcore.InfoLevel:
		enc.AppendString("INFO")
	case zapcore.WarnLevel:
		enc.AppendString("WARN")
	case zapcore.ErrorLevel:
		enc.AppendString("ERROR")
	case zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString("FATAL")
	}
}
