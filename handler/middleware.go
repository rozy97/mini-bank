package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

// healthCheckPath is excluded from request logging (and tracing, see
// router.go) to keep container-orchestrator liveness probes from drowning
// out real traffic in the logs.
const healthCheckPath = "/healthz"

// RequestLogger logs one structured line per request, correlated to its
// OpenTelemetry trace via traceHandler (see pkg/logging), and echoes that
// trace ID back to the client as X-Request-Id so it can be quoted in a bug
// report and looked up directly in logs or a trace backend.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
			c.Writer.Header().Set("X-Request-Id", sc.TraceID().String())
		}

		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		if path == healthCheckPath {
			return
		}

		level := slog.LevelInfo
		if status := c.Writer.Status(); status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		slog.Log(c.Request.Context(), level, "http_request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// Recovery replaces gin.Recovery(): a panic in a handler is caught, logged
// with its stack trace, recorded on the active span, and turned into the
// same JSON error envelope every other failure returns — a bare panic must
// never reach the client as a raw connection reset or an empty 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			err := fmt.Errorf("panic: %v", r)
			slog.ErrorContext(c.Request.Context(), "panic recovered",
				"error", err.Error(),
				"path", c.Request.URL.Path,
				"stack", string(debug.Stack()),
			)

			if span := trace.SpanFromContext(c.Request.Context()); span.IsRecording() {
				span.RecordError(err)
				span.SetStatus(codes.Error, "panic recovered")
			}

			fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
			c.Abort()
		}()
		c.Next()
	}
}
