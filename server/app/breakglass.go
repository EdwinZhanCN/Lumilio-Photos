package app

import (
	"context"
	"os"
	"strings"

	"server/internal/service"

	"go.uber.org/zap"
)

// runBreakGlassIfRequested performs an operator-triggered admin recovery when the
// LUMILIO_BREAK_GLASS env flag is set. It resets a locked-out admin to a random
// temporary password and clears all MFA factors, printing the password to the
// logs once. The operator should unset the flag and restart afterwards.
func runBreakGlassIfRequested(ctx context.Context, userService service.UserService, logger *zap.Logger) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LUMILIO_BREAK_GLASS"))) {
	case "true", "1", "yes", "on":
	default:
		return
	}

	username := strings.TrimSpace(os.Getenv("LUMILIO_BREAK_GLASS_USERNAME"))
	result, target, err := userService.BreakGlassReset(ctx, username)
	if err != nil {
		logger.Error("break-glass reset failed",
			zap.String("operation", "auth.break_glass"),
			zap.String("requested_username", username),
			zap.Error(err),
		)
		return
	}

	logger.Warn("BREAK-GLASS: admin access reset — temporary password issued (shown once)",
		zap.String("operation", "auth.break_glass"),
		zap.String("username", target.Username),
		zap.String("temporary_password", result.TemporaryPassword),
		zap.Bool("cleared_totp", result.ClearedTOTP),
		zap.Bool("cleared_passkeys", result.ClearedPasskeys),
	)
	logger.Warn("BREAK-GLASS: sign in with the temporary password, then unset LUMILIO_BREAK_GLASS and restart",
		zap.String("operation", "auth.break_glass"),
	)
}
