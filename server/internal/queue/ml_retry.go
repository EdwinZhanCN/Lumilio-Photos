package queue

import (
	"strings"
	"time"

	"github.com/riverqueue/river"
)

const mlInfraSnoozeDuration = 30 * time.Second

func maybeSnoozeMLInfraError(err error) error {
	if err == nil {
		return nil
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "failed to select node") ||
		strings.Contains(message, "no suitable nodes available") ||
		strings.Contains(message, "task not available") {
		return river.JobSnooze(mlInfraSnoozeDuration)
	}

	return err
}
