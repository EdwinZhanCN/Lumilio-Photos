package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestSlogZapHandlerMapsLevelsAndAttrs(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	handler := NewSlogZapHandler(zap.New(core))
	logger := slog.New(handler)

	logger.ErrorContext(context.Background(), "bridge error", slog.String("job_kind", "process_clip"), slog.Int64("job_id", 12))

	entries := observed.All()
	if assert.Len(t, entries, 1) {
		assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
		assert.Equal(t, "bridge error", entries[0].Message)
		assert.Equal(t, "process_clip", entries[0].ContextMap()["job_kind"])
		assert.EqualValues(t, 12, entries[0].ContextMap()["job_id"])
	}
}
