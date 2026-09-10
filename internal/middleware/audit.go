package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"dcim-lite/internal/model"
	"dcim-lite/internal/response"
)

// AuditWriter 由仓储层实现（结构化类型匹配），main 中注入。
type AuditWriter interface {
	WriteAudit(log *model.AuditLog) error
}

// Audit 对全部非 GET 请求（含登录）统一写审计日志：
// actor、动作、资源类型/ID、HTTP 结果与业务错误码、requestId。
// 写入失败打错误日志但不阻塞业务响应（深度同事务审计见 docs/HARDENING 报告的边界说明）。
func Audit(writer AuditWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if writer == nil || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		rec := &bodyRecorder{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = rec
		c.Next()

		entry := &model.AuditLog{
			Action:       auditAction(c),
			ResourceType: auditResourceType(c.FullPath()),
			RequestID:    requestIDFrom(c),
			Result:       "SUCCESS",
			Source:       "api",
		}
		if uid := UserID(c); uid != uuid.Nil {
			entry.UserID = &uid
		}
		if id, ok := pathResourceID(c); ok {
			entry.ResourceID = &id
		}
		if rec.status >= 400 {
			entry.Result = "FAILURE"
			entry.ErrorCode = errorCodeFromBody(rec.buf.Bytes())
		}
		if err := writer.WriteAudit(entry); err != nil {
			log.Printf("[AUDIT] write failed requestId=%s action=%s err=%v", entry.RequestID, entry.Action, err)
		}
	}
}

// bodyRecorder 包装 gin.Writer 以捕获响应体中的业务错误码。
type bodyRecorder struct {
	gin.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (w *bodyRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyRecorder) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func errorCodeFromBody(body []byte) string {
	var env struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(body, &env) == nil && env.Code != "" && env.Code != "SUCCESS" {
		return env.Code
	}
	return "HTTP_ERROR"
}

func requestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(response.RequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// auditResourceType 从路由模板推断资源类型：/api/v1/data-centers/:id/rooms -> data_center
func auditResourceType(fullPath string) string {
	if fullPath == "" {
		return "unknown"
	}
	segs := strings.Split(strings.TrimPrefix(fullPath, "/api/v1/"), "/")
	if len(segs) == 0 {
		return "unknown"
	}
	return strings.TrimSuffix(segs[0], "s")
}

// auditAction 动作 = HTTP 方法 + 语义子动作（copy/move/approve/reject 等）
func auditAction(c *gin.Context) string {
	action := c.Request.Method
	if c.FullPath() != "" {
		segs := strings.Split(strings.TrimPrefix(c.FullPath(), "/api/v1/"), "/")
		if len(segs) >= 3 && !strings.HasPrefix(segs[len(segs)-1], ":") {
			action += "_" + strings.ToUpper(segs[len(segs)-1])
		}
	}
	return action
}

func pathResourceID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Param("id")
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
