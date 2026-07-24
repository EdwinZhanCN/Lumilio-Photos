package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"server/config"

	"github.com/gin-gonic/gin"
)

func TestAuthRateLimiterReturnsRetryAfterAndRecoversAfterReset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, err := NewAuthRateLimiter(config.AuthRateLimitConfig{
		IPAttempts:      100,
		SubjectAttempts: 1,
		Window:          time.Minute,
		Lockout:         time.Minute,
		MaxEntries:      10,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := &AuthHandler{rateLimiter: limiter}

	firstContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	firstContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	if !handler.allowAuthUsername(firstContext, authRateScopeLogin, " ExampleUser ") {
		t.Fatal("first attempt was denied")
	}

	recorder := httptest.NewRecorder()
	blockedContext, _ := gin.CreateTestContext(recorder)
	blockedContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	if handler.allowAuthUsername(blockedContext, authRateScopeLogin, "exampleuser") {
		t.Fatal("second attempt was allowed")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q", got)
	}

	handler.resetAuthUsername(authRateScopeLogin, "EXAMPLEUSER")
	recoveredContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	recoveredContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	if !handler.allowAuthUsername(recoveredContext, authRateScopeLogin, "exampleuser") {
		t.Fatal("attempt after reset was denied")
	}
}

func TestRequestNetworkAddressesIncludesRemoteAndForwardedClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	context.Request.RemoteAddr = "127.0.0.1:43210"
	context.Request.Header.Set("X-Forwarded-For", "203.0.113.42")

	got := requestNetworkAddresses(context)
	if len(got) != 2 || got[0] != "127.0.0.1" || got[1] != "203.0.113.42" {
		t.Fatalf("network addresses = %#v", got)
	}
}

func TestAuthRateKeyDoesNotContainSensitiveValue(t *testing.T) {
	const secret = "sensitive-refresh-token"
	limiter, err := NewAuthRateLimiter(config.AuthRateLimitConfig{
		IPAttempts:      1,
		SubjectAttempts: 1,
		Window:          time.Minute,
		Lockout:         time.Minute,
		MaxEntries:      1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := limiter.authRateKey(authRateScopeRefresh, "credential", secret)
	if key == secret || len(key) != 64 {
		t.Fatalf("unexpected key = %q", key)
	}
}

func TestAuthRateKeysCannotBeCorrelatedAcrossProcesses(t *testing.T) {
	config := config.AuthRateLimitConfig{
		IPAttempts:      1,
		SubjectAttempts: 1,
		Window:          time.Minute,
		Lockout:         time.Minute,
		MaxEntries:      1,
	}
	first, err := NewAuthRateLimiter(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAuthRateLimiter(config, nil)
	if err != nil {
		t.Fatal(err)
	}

	firstKey := first.authRateKey(authRateScopeLogin, "username", "exampleuser")
	if secondKey := second.authRateKey(authRateScopeLogin, "username", "exampleuser"); firstKey == secondKey {
		t.Fatal("independent limiter instances produced a correlatable subject key")
	}
}
