package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Handlers struct {
	Auth     *AuthHandler
	Account  *AccountHandler
	Transfer *TransferHandler
}

func NewRouter(h Handlers, tokenVerifier TokenVerifier, serviceName string) *gin.Engine {
	r := gin.New()

	// Trust X-Forwarded-For / X-Real-IP only from private-network peers (the
	// nginx container in front of this service), not from the public
	// internet — otherwise c.ClientIP() would just echo back whatever a
	// client claims. Gin refuses to start without an explicit choice here.
	if err := r.SetTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}); err != nil {
		panic(fmt.Errorf("set trusted proxies: %w", err))
	}

	// otelgin starts the request span (and puts it in context) before
	// Recovery and RequestLogger run, so both can read the trace ID and
	// Recovery can record a panic onto the span.
	r.Use(
		otelgin.Middleware(serviceName, otelgin.WithFilter(func(req *http.Request) bool {
			return req.URL.Path != healthCheckPath
		})),
		Recovery(),
		RequestLogger(),
	)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /swagger/*any serves the raw OpenAPI spec (doc.json) and swaggo's
	// default UI. /docs is a custom UI that auto-authorizes from /auth/login
	// responses; see docs_page.go.
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/docs", DocsPage)

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	auth.POST("/register", h.Auth.Register)
	auth.POST("/login", h.Auth.Login)

	protected := v1.Group("")
	protected.Use(AuthMiddleware(tokenVerifier))

	accounts := protected.Group("/accounts")
	accounts.GET("/me/balance", h.Account.GetBalance)
	accounts.GET("/me/history", h.Account.GetHistory)

	transfers := protected.Group("/transfers")
	transfers.POST("", h.Transfer.Transfer)

	return r
}
