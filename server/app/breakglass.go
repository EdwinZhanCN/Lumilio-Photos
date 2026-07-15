package app

import (
	"context"

	"server/internal/service"

	"go.uber.org/zap"
)

// runBreakGlassIfRequested performs recovery from the explicit host control. It
// resets a locked-out admin to a random temporary password and clears all MFA
// factors, writing the password once to the isolated security log. Host control
// interpretation belongs to the standalone or desktop entry point, not this
// application package.
func runBreakGlassIfRequested(ctx context.Context, userService service.UserService, enabled bool, username string, logger *zap.Logger) {
	if !enabled {
		return
	}
	logger.Info("break-glass recovery requested",
		zap.String("operation", "auth.break_glass"),
		zap.String("outcome", "requested"),
		zap.String("requested_username", username),
	)
	result, target, err := userService.BreakGlassReset(ctx, username)
	if err != nil {
		logger.Error("break-glass reset failed",
			zap.String("operation", "auth.break_glass"),
			zap.String("outcome", "failed"),
			zap.String("requested_username", username),
			zap.Error(err),
		)
		return
	}

	logger.Warn("BREAK-GLASS: admin access reset — temporary password issued (shown once)",
		zap.String("operation", "auth.break_glass"),
		zap.String("outcome", "succeeded"),
		zap.String("username", target.Username),
		zap.String("temporary_password", result.TemporaryPassword),
		zap.Bool("cleared_totp", result.ClearedTOTP),
		zap.Bool("cleared_passkeys", result.ClearedPasskeys),
	)
	logger.Warn("BREAK-GLASS: sign in with the temporary password, then remove the recovery launch control and restart normally",
		zap.String("operation", "auth.break_glass"),
		zap.String("outcome", "operator_action_required"),
	)
}
