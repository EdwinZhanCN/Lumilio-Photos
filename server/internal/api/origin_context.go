package api

import (
	"errors"
	"net/http"

	"server/internal/httporigin"

	"github.com/gin-gonic/gin"
)

const requestOriginContextKey = "lumilio.request_origin"

func originPolicyMiddleware(policy *httporigin.Policy) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			resolved httporigin.RequestContext
			err      error
		)
		if c.Request.Method == http.MethodGet &&
			(c.Request.URL.Path == "/api/v1/health/live" ||
				c.Request.URL.Path == "/api/v1/health/ready") {
			resolved, err = policy.ResolveLoopbackHealth(c.Request)
		} else {
			resolved, err = policy.Resolve(c.Request)
		}
		if err != nil {
			writeOriginPolicyError(c, err)
			c.Abort()
			return
		}
		SetRequestOriginContext(c, resolved)
		c.Next()
	}
}

func SetRequestOriginContext(c *gin.Context, resolved httporigin.RequestContext) {
	c.Set(requestOriginContextKey, resolved)
}

func RequestOriginContext(c *gin.Context) (httporigin.RequestContext, bool) {
	if c == nil {
		return httporigin.RequestContext{}, false
	}
	value, exists := c.Get(requestOriginContextKey)
	if !exists {
		return httporigin.RequestContext{}, false
	}
	resolved, ok := value.(httporigin.RequestContext)
	return resolved, ok
}

func writeOriginPolicyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, httporigin.ErrInvalidPeerAddress),
		errors.Is(err, httporigin.ErrInvalidTargetOrigin),
		errors.Is(err, httporigin.ErrInvalidBrowserOrigin):
		WriteProblem(c, StatusProblem(http.StatusBadRequest, err))
	default:
		WriteProblem(c, StatusProblem(http.StatusBadRequest, err))
	}
}
