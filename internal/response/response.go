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

func Fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Envelope{
		Code:      code,
		Message:   message,
		RequestID: requestID(c),
	})
}
