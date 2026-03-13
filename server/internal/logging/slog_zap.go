package logging

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type slogZapHandler struct {
	logger *zap.Logger
	attrs  []slog.Attr
	groups []string
}

func NewSlogZapHandler(logger *zap.Logger) slog.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &slogZapHandler{logger: logger}
}

func (h *slogZapHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.logger.Core().Enabled(toZapLevel(level))
}

func (h *slogZapHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]zap.Field, 0, len(h.attrs)+record.NumAttrs()+2)
	if !record.Time.IsZero() {
		fields = append(fields, zap.Time("record_time", record.Time))
	}
	fields = append(fields, attrsToFields(h.groups, h.attrs)...)
	record.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, attrToField(h.groups, attr))
		return true
	})

	h.logger.WithOptions(zap.AddCallerSkip(1)).Check(toZapLevel(record.Level), record.Message).Write(fields...)
	return nil
}

func (h *slogZapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	combined = append(combined, h.attrs...)
	combined = append(combined, attrs...)
	return &slogZapHandler{
		logger: h.logger,
		attrs:  combined,
		groups: append([]string(nil), h.groups...),
	}
}

func (h *slogZapHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groups := append(append([]string(nil), h.groups...), name)
	return &slogZapHandler{
		logger: h.logger,
		attrs:  append([]slog.Attr(nil), h.attrs...),
		groups: groups,
	}
}

func toZapLevel(level slog.Level) zapcore.Level {
	switch {
	case level >= slog.LevelError:
		return zapcore.ErrorLevel
	case level >= slog.LevelWarn:
		return zapcore.WarnLevel
	case level <= slog.LevelDebug:
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}

func attrsToFields(groups []string, attrs []slog.Attr) []zap.Field {
	fields := make([]zap.Field, 0, len(attrs))
	for _, attr := range attrs {
		fields = append(fields, attrToField(groups, attr))
	}
	return fields
}

func attrToField(groups []string, attr slog.Attr) zap.Field {
	attr.Value = attr.Value.Resolve()
	key := attr.Key
	if len(groups) > 0 {
		key = groups[0] + "." + key
		for _, group := range groups[1:] {
			key = group + "." + key
		}
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		return zap.String(key, attr.Value.String())
	case slog.KindInt64:
		return zap.Int64(key, attr.Value.Int64())
	case slog.KindUint64:
		return zap.Uint64(key, attr.Value.Uint64())
	case slog.KindFloat64:
		return zap.Float64(key, attr.Value.Float64())
	case slog.KindBool:
		return zap.Bool(key, attr.Value.Bool())
	case slog.KindDuration:
		return zap.Duration(key, attr.Value.Duration())
	case slog.KindTime:
		return zap.Time(key, attr.Value.Time())
	case slog.KindGroup:
		groupAttrs := attr.Value.Group()
		nestedFields := make([]zap.Field, 0, len(groupAttrs))
		nextGroups := append(append([]string(nil), groups...), attr.Key)
		for _, nested := range groupAttrs {
			nestedFields = append(nestedFields, attrToField(nextGroups, nested))
		}
		return zap.Any(key, nestedFields)
	case slog.KindAny:
		switch value := attr.Value.Any().(type) {
		case error:
			return zap.NamedError(key, value)
		case time.Time:
			return zap.Time(key, value)
		default:
			return zap.Any(key, value)
		}
	default:
		return zap.Any(key, attr.Value.Any())
	}
}
