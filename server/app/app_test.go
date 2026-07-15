package app

import (
	"context"
	"strings"
	"testing"

	"server/config"
)

func TestRunRejectsStructLiteralConfig(t *testing.T) {
	err := Run(context.Background(), config.AppConfig{}, OperatorControls{})
	if err == nil || !strings.Contains(err.Error(), "strict manifest loader") {
		t.Fatalf("expected unvalidated config rejection, got %v", err)
	}
}
