package logging

import (
	"log"

	"go.uber.org/zap"
)

func RedirectStandardLog(logger *zap.Logger) func() {
	if logger == nil {
		logger = zap.NewNop()
	}
	log.SetFlags(0)
	return zap.RedirectStdLog(logger.WithOptions(zap.AddCallerSkip(1)).With(zap.String("component", "stdlib")))
}
