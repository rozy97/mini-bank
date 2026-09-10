package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenVerifier validates a bearer token and returns the user ID it
// authenticates.
type TokenVerifier interface {
	Verify(tokenString string) (int64, error)
}

const userIDContextKey = "user_id"

func AuthMiddleware(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, found := strings.Cut(c.GetHeader("Authorization"), " ")
		// The auth-scheme token is case-insensitive per RFC 7235 ss 2.1;
		// clients like curl or Swagger's UI commonly send "bearer" in
		// lowercase, so this must not do a case-sensitive prefix match.
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
			c.Abort()
			return
		}

		userID, err := verifier.Verify(token)
		if err != nil {
			fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

func userIDFromContext(c *gin.Context) int64 {
	v, _ := c.Get(userIDContextKey)
	id, _ := v.(int64)
	return id
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		slog.Info("http_request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
