package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rozy97/mini-bank/usecases"
)

type AuthHandler struct {
	authUsecase *usecases.AuthUsecase
}

func NewAuthHandler(authUsecase *usecases.AuthUsecase) *AuthHandler {
	return &AuthHandler{authUsecase: authUsecase}
}

type registerRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100" example:"Jane Doe"`
	Email    string `json:"email" binding:"required,email" example:"jane@example.com"`
	Password string `json:"password" binding:"required,min=8" example:"strongpassword123"`
}

type registerResponse struct {
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AccountID int64  `json:"account_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a user and an account prepopulated with an initial balance of 10,000,000 IDR.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      registerRequest  true  "Registration details"
// @Success      201      {object}  envelope{data=registerResponse}
// @Failure      400      {object}  envelope
// @Failure      409      {object}  envelope
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	out, err := h.authUsecase.Register(c.Request.Context(), usecases.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, http.StatusCreated, registerResponse{
		UserID:    out.UserID,
		Name:      out.Name,
		Email:     out.Email,
		AccountID: out.AccountID,
		Balance:   out.Balance,
		Currency:  out.Currency,
	})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"jane@example.com"`
	Password string `json:"password" binding:"required" example:"strongpassword123"`
}

type loginResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Login godoc
// @Summary      Log in
// @Description  Authenticates a user and returns a bearer token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      loginRequest  true  "Credentials"
// @Success      200      {object}  envelope{data=loginResponse}
// @Failure      400      {object}  envelope
// @Failure      401      {object}  envelope
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	out, err := h.authUsecase.Login(c.Request.Context(), usecases.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, http.StatusOK, loginResponse{
		AccessToken: out.Token,
		TokenType:   "Bearer",
		ExpiresAt:   out.ExpiresAt,
	})
}
