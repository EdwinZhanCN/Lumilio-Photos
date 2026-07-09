package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"server/config"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetLoginOptions_InvalidUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")
	svc := service.NewAuthService(nil, nil, config.AuthConfig{SecretKeyPath: keyFile})
	h := NewAuthHandler(svc)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body, err := json.Marshal(map[string]string{"username": "ab"})
	require.NoError(t, err)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/options", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.GetLoginOptions(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.NotContains(t, payload, "totp")
	require.NotContains(t, payload, "passkey")
}

func TestGetLoginOptions_MissingUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")
	svc := service.NewAuthService(nil, nil, config.AuthConfig{SecretKeyPath: keyFile})
	h := NewAuthHandler(svc)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/options", bytes.NewReader([]byte(`{}`)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.GetLoginOptions(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
