package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rozy97/mini-bank/usecases"
)

type TransferHandler struct {
	transferUsecase *usecases.TransferUsecase
}

func NewTransferHandler(transferUsecase *usecases.TransferUsecase) *TransferHandler {
	return &TransferHandler{transferUsecase: transferUsecase}
}

type transferRequest struct {
	ToAccountID int64  `json:"to_account_id" binding:"required" example:"2"`
	Amount      int64  `json:"amount" binding:"required,gt=0" example:"100000"`
	Description string `json:"description" binding:"max=255" example:"Dinner split"`
}

type transferResponse struct {
	TransferID    int64     `json:"transfer_id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}

// Transfer godoc
// @Summary      Transfer funds
// @Description  Moves funds from the authenticated user's account to another account. Requires an Idempotency-Key header; retrying the same key with the same body is safe and returns the original result.
// @Tags         transfers
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header    string           true  "Client-generated key unique per transfer attempt"
// @Param        request          body      transferRequest  true  "Transfer details"
// @Success      201  {object}  envelope{data=transferResponse}
// @Failure      400  {object}  envelope
// @Failure      401  {object}  envelope
// @Failure      404  {object}  envelope
// @Failure      409  {object}  envelope
// @Failure      422  {object}  envelope
// @Router       /transfers [post]
func (h *TransferHandler) Transfer(c *gin.Context) {
	idempotencyKey := c.GetHeader("Idempotency-Key")

	raw, err := c.GetRawData()
	if err != nil {
		fail(c, http.StatusBadRequest, "INVALID_REQUEST", "unable to read request body")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var req transferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	hash := sha256.Sum256(raw)

	out, err := h.transferUsecase.Transfer(c.Request.Context(), usecases.TransferInput{
		FromUserID:     userIDFromContext(c),
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Description:    req.Description,
		IdempotencyKey: idempotencyKey,
		RequestHash:    hex.EncodeToString(hash[:]),
	})
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, http.StatusCreated, transferResponse{
		TransferID:    out.TransferID,
		FromAccountID: out.FromAccountID,
		ToAccountID:   out.ToAccountID,
		Amount:        out.Amount,
		Description:   out.Description,
		CreatedAt:     out.CreatedAt,
	})
}
