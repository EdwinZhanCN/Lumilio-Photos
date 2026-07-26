package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server/config"
	"server/internal/api"
	"server/internal/api/ratelimit"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	authRateScopeLoginOptions   = "login_options"
	authRateScopeLogin          = "login"
	authRateScopePasskeyOptions = "passkey_options"
	authRateScopePasskeyVerify  = "passkey_verify"
	authRateScopeMFAVerify      = "mfa_verify"
	authRateScopeRefresh        = "refresh"
)

// AuthRateLimiter applies independent network and subject policies at the HTTP
// authentication boundary. Keys are hashed before entering the bounded stores.
type AuthRateLimiter struct {
	network *ratelimit.Limiter
	subject *ratelimit.Limiter
	logger  *zap.Logger
	key     [32]byte
}

// NewAuthRateLimiter constructs the authentication limiter from immutable
// runtime configuration.
func NewAuthRateLimiter(cfg config.AuthRateLimitConfig, logger *zap.Logger) (*AuthRateLimiter, error) {
	network, err := ratelimit.New(ratelimit.Policy{
		Attempts:   cfg.IPAttempts,
		Window:     cfg.Window,
		Lockout:    cfg.Lockout,
		MaxEntries: cfg.MaxEntries,
	})
	if err != nil {
		return nil, err
	}
	subject, err := ratelimit.New(ratelimit.Policy{
		Attempts:   cfg.SubjectAttempts,
		Window:     cfg.Window,
		Lockout:    cfg.Lockout,
		MaxEntries: cfg.MaxEntries,
	})
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	limiter := &AuthRateLimiter{network: network, subject: subject, logger: logger}
	if _, err := rand.Read(limiter.key[:]); err != nil {
		return nil, fmt.Errorf("generate authentication rate-limit key: %w", err)
	}
	return limiter, nil
}

func (h *AuthHandler) allowAuthNetwork(c *gin.Context, scope string) bool {
	if h.rateLimiter == nil {
		return true
	}

	for _, address := range requestNetworkAddresses(c) {
		key := h.rateLimiter.authRateKey(scope, "network", address)
		if decision := h.rateLimiter.network.Allow(key); !decision.Allowed {
			h.writeAuthRateLimit(c, scope, "network", key, decision.RetryAfter)
			return false
		}
	}
	return true
}

func (h *AuthHandler) allowAuthUsername(c *gin.Context, scope, username string) bool {
	return h.allowAuthSubject(c, scope, "username", strings.ToLower(strings.TrimSpace(username)))
}

func (h *AuthHandler) allowAuthOpaqueSubject(c *gin.Context, scope, value string) bool {
	return h.allowAuthSubject(c, scope, "credential", strings.TrimSpace(value))
}

func (h *AuthHandler) allowAuthSubject(c *gin.Context, scope, dimension, value string) bool {
	if h.rateLimiter == nil {
		return true
	}
	key := h.rateLimiter.authRateKey(scope, dimension, value)
	if decision := h.rateLimiter.subject.Allow(key); !decision.Allowed {
		h.writeAuthRateLimit(c, scope, dimension, key, decision.RetryAfter)
		return false
	}
	return true
}

func (h *AuthHandler) resetAuthUsername(scope, username string) {
	h.resetAuthSubject(scope, "username", strings.ToLower(strings.TrimSpace(username)))
}

func (h *AuthHandler) resetAuthOpaqueSubject(scope, value string) {
	h.resetAuthSubject(scope, "credential", strings.TrimSpace(value))
}

func (h *AuthHandler) resetAuthSubject(scope, dimension, value string) {
	if h.rateLimiter == nil {
		return
	}
	h.rateLimiter.subject.Reset(h.rateLimiter.authRateKey(scope, dimension, value))
}

func (h *AuthHandler) writeAuthRateLimit(c *gin.Context, scope, dimension, digest string, retryAfter time.Duration) {
	seconds := int64((retryAfter + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.FormatInt(seconds, 10))
	h.rateLimiter.logger.Warn("authentication request rate limited",
		zap.String("operation", "auth.rate_limit"),
		zap.String("scope", scope),
		zap.String("dimension", dimension),
		zap.String("key_digest", digest),
		zap.Int64("retry_after_seconds", seconds),
	)
	api.GinError(
		c,
		http.StatusTooManyRequests,
		errors.New("authentication rate limit exceeded"),
		http.StatusTooManyRequests,
		"Too many authentication attempts. Try again later.",
	)
	c.Abort()
}

func requestNetworkAddresses(c *gin.Context) []string {
	resolved, ok := api.RequestOriginContext(c)
	if !ok || !resolved.ClientIP.IsValid() {
		return []string{"unknown"}
	}
	return []string{resolved.ClientIP.String()}
}

func (l *AuthRateLimiter) authRateKey(scope, dimension, value string) string {
	digest := hmac.New(sha256.New, l.key[:])
	_, _ = digest.Write([]byte(scope + "\x00" + dimension + "\x00" + value))
	return hex.EncodeToString(digest.Sum(nil))
}
