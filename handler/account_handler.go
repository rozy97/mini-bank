package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rozy97/mini-bank/usecases"
)

type AccountHandler struct {
	accountUsecase *usecases.AccountUsecase
}

func NewAccountHandler(accountUsecase *usecases.AccountUsecase) *AccountHandler {
	return &AccountHandler{accountUsecase: accountUsecase}
}

type balanceResponse struct {
	AccountID int64  `json:"account_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
}

// GetBalance godoc
// @Summary      Get current balance
// @Description  Returns the authenticated user's account balance.
// @Tags         accounts
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  envelope{data=balanceResponse}
// @Failure      401  {object}  envelope
// @Failure      404  {object}  envelope
// @Router       /accounts/me/balance [get]
func (h *AccountHandler) GetBalance(c *gin.Context) {
	out, err := h.accountUsecase.GetBalance(c.Request.Context(), userIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}

	success(c, http.StatusOK, balanceResponse{
		AccountID: out.AccountID,
		Balance:   out.Balance,
		Currency:  out.Currency,
	})
}

type historyEntryResponse struct {
	ID           int64     `json:"id"`
	TransferID   *int64    `json:"transfer_id"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

// GetHistory godoc
// @Summary      Get paginated balance history
// @Description  Returns the authenticated user's ledger entries, newest first.
// @Tags         accounts
// @Produce      json
// @Security     BearerAuth
// @Param        page       query     int  false  "Page number (default 1)"
// @Param        page_size  query     int  false  "Items per page, max 100 (default 20)"
// @Success      200  {object}  envelope{data=[]historyEntryResponse,meta=paginationMeta}
// @Failure      401  {object}  envelope
// @Failure      404  {object}  envelope
// @Router       /accounts/me/history [get]
func (h *AccountHandler) GetHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	out, err := h.accountUsecase.GetHistory(c.Request.Context(), usecases.HistoryInput{
		UserID:   userIDFromContext(c),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	entries := make([]historyEntryResponse, len(out.Entries))
	for i, e := range out.Entries {
		entries[i] = historyEntryResponse{
			ID:           e.ID,
			TransferID:   e.TransferID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			CreatedAt:    e.CreatedAt,
		}
	}

	successWithMeta(c, http.StatusOK, entries, paginationMeta{
		Page:       out.Page,
		PageSize:   out.PageSize,
		TotalItems: out.TotalItems,
		TotalPages: out.TotalPages,
	})
}
