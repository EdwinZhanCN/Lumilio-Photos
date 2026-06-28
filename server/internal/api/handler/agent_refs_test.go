package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/agent/ref"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func refTestContext(t *testing.T, userID int, url string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", url, nil)
	c.Set("current_user", &service.UserResponse{UserID: userID, Username: "tester"})
	return c, recorder
}

// INV-4: missing, cross-thread and cross-user refs are indistinguishable —
// all answer 404 without leaking existence.
func TestResolveRefScopeIsolation(t *testing.T) {
	store := ref.NewMemoryStore(0, 0)
	owner := ref.Scope{UserID: 1, ThreadID: "thread-a"}
	r := store.Create(owner, ref.Plan{Op: "filter_assets"}, "test", "", []uuid.UUID{uuid.New()}, false)

	h := NewAgentHandler(nil, store, nil, nil, nil)

	cases := map[string]string{
		"cross user":     "/api/v1/agent/refs/" + r.ID + "?thread_id=thread-a",
		"cross thread":   "/api/v1/agent/refs/" + r.ID + "?thread_id=thread-b",
		"missing ref":    "/api/v1/agent/refs/r99?thread_id=thread-a",
		"missing thread": "/api/v1/agent/refs/" + r.ID,
	}

	for name, url := range cases {
		userID := 1
		if name == "cross user" {
			userID = 2
		}
		c, recorder := refTestContext(t, userID, url)
		c.Params = gin.Params{{Key: "id", Value: refIDFromURL(url)}}

		_, ok := h.resolveRef(c)
		require.False(t, ok, "%s: resolveRef should fail", name)
		require.Equal(t, http.StatusNotFound, recorder.Code, "%s: must be 404", name)
	}
}

func TestResolveRefHappyPathRefreshesScope(t *testing.T) {
	store := ref.NewMemoryStore(0, 0)
	owner := ref.Scope{UserID: 1, ThreadID: "thread-a"}
	created := store.Create(owner, ref.Plan{Op: "filter_assets"}, "test", "", []uuid.UUID{uuid.New(), uuid.New()}, false)

	h := NewAgentHandler(nil, store, nil, nil, nil)
	c, _ := refTestContext(t, 1, "/api/v1/agent/refs/"+created.ID+"?thread_id=thread-a")
	c.Params = gin.Params{{Key: "id", Value: created.ID}}

	got, ok := h.resolveRef(c)
	require.True(t, ok)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, 2, got.Count())
}

func refIDFromURL(url string) string {
	// "/api/v1/agent/refs/<id>?..." → "<id>"
	rest := url[len("/api/v1/agent/refs/"):]
	for i, r := range rest {
		if r == '?' || r == '/' {
			return rest[:i]
		}
	}
	return rest
}
