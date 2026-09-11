package handler

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/response"
)

// 前端错误遥测(P1-R6):浏览器 ErrorReporter 的生产落点。
// 字段白名单(release/route/message/apiStatus/apiCode/requestId/role),
// message 截断,内存环形缓冲(非持久化——可观测性最小闭环,检索由 admin 接口提供)。

const (
	telemetryRingSize   = 500
	telemetryMaxMessage = 500
)

// FrontendError 一条白名单化后的前端错误事件。
type FrontendError struct {
	ReceivedAt time.Time `json:"receivedAt"`
	Release    string    `json:"release,omitempty"`
	Route      string    `json:"route,omitempty"`
	Message    string    `json:"message"`
	APIStatus  int       `json:"apiStatus,omitempty"`
	APICode    string    `json:"apiCode,omitempty"`
	RequestID  string    `json:"requestId,omitempty"`
	Role       string    `json:"role,omitempty"`
}

// TelemetryStore 并发安全的环形缓冲。
type TelemetryStore struct {
	mu      sync.Mutex
	ring    []FrontendError
	next    int
	wrapped bool
}

func NewTelemetryStore() *TelemetryStore {
	return &TelemetryStore{ring: make([]FrontendError, telemetryRingSize)}
}

func (s *TelemetryStore) add(e FrontendError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next == telemetryRingSize {
		s.next = 0
		s.wrapped = true
	}
	s.ring[s.next] = e
	s.next++
}

// Recent 返回最新事件(新→旧)。
func (s *TelemetryStore) Recent(n int) []FrontendError {
	s.mu.Lock()
	defer s.mu.Unlock()
	filled := s.next
	if s.wrapped {
		filled = telemetryRingSize
	}
	if n > filled {
		n = filled
	}
	out := make([]FrontendError, 0, n)
	for i := 0; i < n; i++ {
		idx := s.next - 1 - i
		if idx < 0 {
			idx += telemetryRingSize
		}
		out = append(out, s.ring[idx])
	}
	return out
}

type TelemetryHandler struct {
	store *TelemetryStore
}

func NewTelemetryHandler(store *TelemetryStore) *TelemetryHandler {
	return &TelemetryHandler{store: store}
}

type frontendErrorInput struct {
	Release   string `json:"release"`
	Route     string `json:"route"`
	Message   string `json:"message"`
	APIStatus int    `json:"apiStatus"`
	APICode   string `json:"apiCode"`
	RequestID string `json:"requestId"`
	Role      string `json:"role"`
}

var telemetryRoles = map[string]bool{"anonymous": true, "user": true, "system_admin": true}

// ReportFrontendError POST /api/v1/telemetry/frontend-errors
// 匿名可上报(登录页错误也需要可见),由路由层的每 IP 限流约束滥用。
func (h *TelemetryHandler) ReportFrontendError(c *gin.Context) {
	var in frontendErrorInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	msg := strings.TrimSpace(in.Message)
	if msg == "" {
		writeAppError(c, apperr.InvalidResource("message 不能为空"))
		return
	}
	if len(msg) > telemetryMaxMessage {
		msg = msg[:telemetryMaxMessage]
	}
	role := in.Role
	if !telemetryRoles[role] {
		role = ""
	}
	e := FrontendError{
		ReceivedAt: time.Now().UTC(),
		Release:    sanitizeTelemetryField(in.Release, 40),
		Route:      sanitizeTelemetryField(in.Route, 200),
		Message:    msg,
		APIStatus:  in.APIStatus,
		APICode:    sanitizeTelemetryField(in.APICode, 60),
		RequestID:  sanitizeTelemetryField(in.RequestID, 80),
		Role:       role,
	}
	h.store.add(e)
	response.OK(c, gin.H{"received": true})
}

// ListFrontendErrors GET /api/v1/admin/telemetry/frontend-errors?limit=
func (h *TelemetryHandler) ListFrontendErrors(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit <= 0 || limit > telemetryRingSize {
		limit = 100
	}
	response.OK(c, gin.H{"items": h.store.Recent(limit)})
}

// sanitizeTelemetryField 白名单外字段一律丢弃;白名单内做长度与可打印性收敛。
func sanitizeTelemetryField(v string, max int) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	if len(v) > max {
		v = v[:max]
	}
	return v
}
