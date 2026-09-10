package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handlers struct {
	Auth     *AuthHandler
	Account  *AccountHandler
	Transfer *TransferHandler
}

func NewRouter(h Handlers, tokenVerifier TokenVerifier) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), RequestLogger())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
