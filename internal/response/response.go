package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "requestId"
const HeaderRequestID = "X-Request-Id"

type Envelope struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"requestId"`
}

func requestID(c *gin.Context) string {
	if v, ok := c.Get(RequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{
		Code:      "SUCCESS",
		Message:   "操作成功",
		Data:      data,
		RequestID: requestID(c),
	})
}

// Created 资源创建成功：与厂商基线一致返回 201（绑定/校验失败仍为 4xx）。
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{
		Code:      "SUCCESS",
		Message:   "操作成功",
		Data:      data,
		RequestID: requestID(c),
	})
}

func Fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Envelope{
		Code:      code,
		Message:   message,
		RequestID: requestID(c),
	})
}
