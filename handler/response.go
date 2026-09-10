package handler

import "github.com/gin-gonic/gin"

type envelope struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Meta    any        `json:"meta,omitempty"`
	Error   *errorBody `json:"error,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type paginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

func success(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{Success: true, Data: data})
}

func successWithMeta(c *gin.Context, status int, data any, meta any) {
	c.JSON(status, envelope{Success: true, Data: data, Meta: meta})
}

func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, envelope{Success: false, Error: &errorBody{Code: code, Message: message}})
}
