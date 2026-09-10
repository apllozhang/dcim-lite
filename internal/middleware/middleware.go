package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/response"
)

const (
	ContextUserIDKey = "userID"
	ContextRolesKey  = "roles"
)

type Claims struct {
	UserID string   `json:"userId"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

type UserFinder interface {
	FindByID(id uuid.UUID) (*model.User, error)
}

// TokenRevoker 记录已吊销的 token（登出/改密后立即失效），单实例内存实现。
type TokenRevoker interface {
	Revoke(jti string, expiresAt time.Time)
	IsRevoked(jti string) bool
}

// MemoryTokenRevoker 单实例内存黑名单：条目随 token 原有效期自动过期清理。
type MemoryTokenRevoker struct {
	mu    sync.Mutex
	items map[string]time.Time // jti -> expiresAt
}

func NewMemoryTokenRevoker() *MemoryTokenRevoker {
	r := &MemoryTokenRevoker{items: map[string]time.Time{}}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			r.gc()
		}
	}()
	return r
}

func (r *MemoryTokenRevoker) Revoke(jti string, expiresAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[jti] = expiresAt
}

func (r *MemoryTokenRevoker) IsRevoked(jti string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	exp, ok := r.items[jti]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(r.items, jti)
		return false
	}
	return true
}

func (r *MemoryTokenRevoker) gc() {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, exp := range r.items {
		if now.After(exp) {
			delete(r.items, k)
		}
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(response.HeaderRequestID)
		// 外部 requestId 仅接受有限长度与字符集，防止日志投毒
		if !validRequestID(rid) {
			rid = "req_" + uuid.NewString()
		}
		c.Set(response.RequestIDKey, rid)
		c.Writer.Header().Set(response.HeaderRequestID, rid)
		c.Next()
	}
}

func validRequestID(rid string) bool {
	if len(rid) < 8 || len(rid) > 64 {
		return false
	}
	for _, r := range rid {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				rid, _ := c.Get(response.RequestIDKey)
				requestID, _ := rid.(string)
				log.Printf("[PANIC] requestId=%s %v\n%s", requestID, rec, debug.Stack())
				response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
			}
		}()
		c.Next()
	}
}

func Auth(secret string, users UserFinder, revoker TokenRevoker) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, apperr.Unauthorized().Status, apperr.Unauthorized().Code, apperr.Unauthorized().Message)
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			response.Fail(c, apperr.Unauthorized().Status, apperr.Unauthorized().Code, apperr.Unauthorized().Message)
			return
		}
		if revoker != nil && claims.ID != "" && revoker.IsRevoked(claims.ID) {
			response.Fail(c, apperr.Unauthorized().Status, apperr.Unauthorized().Code, apperr.Unauthorized().Message)
			return
		}
		uid, err := uuid.Parse(claims.UserID)
		if err != nil {
			response.Fail(c, apperr.Unauthorized().Status, apperr.Unauthorized().Code, apperr.Unauthorized().Message)
			return
		}
		user, err := users.FindByID(uid)
		if err != nil || user == nil || !user.Enabled {
			response.Fail(c, apperr.Unauthorized().Status, apperr.Unauthorized().Code, apperr.Unauthorized().Message)
			return
		}
		c.Set(ContextUserIDKey, user.ID)
		c.Set(ContextRolesKey, user.RoleCodes())
		c.Set("currentUser", user)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, ok := c.Get(ContextRolesKey)
		if !ok {
			response.Fail(c, apperr.Forbidden().Status, apperr.Forbidden().Code, apperr.Forbidden().Message)
			return
		}
		list, _ := roles.([]string)
		for _, r := range list {
			if r == "system_admin" {
				c.Next()
				return
			}
		}
		response.Fail(c, apperr.Forbidden().Status, apperr.Forbidden().Code, apperr.Forbidden().Message)
	}
}

// RateLimit 单实例内存滑动窗口限流（按客户端 IP），用于登录等敏感端点。
type rateWindow struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

// NewRateLimit 每个窗口期内每 IP 最多 limit 次请求，超出返回 429。
func NewRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	rw := &rateWindow{hits: map[string][]time.Time{}, limit: limit, window: window}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rw.gc()
		}
	}()
	return func(c *gin.Context) {
		if !rw.allow(clientIP(c)) {
			response.Fail(c, http.StatusTooManyRequests, "RATE_LIMITED", "请求过于频繁，请稍后再试")
			return
		}
		c.Next()
	}
}

func (rw *rateWindow) allow(key string) bool {
	now := time.Now()
	rw.mu.Lock()
	defer rw.mu.Unlock()
	hits := rw.hits[key]
	kept := hits[:0]
	for _, t := range hits {
		if now.Sub(t) < rw.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rw.limit {
		rw.hits[key] = kept
		return false
	}
	rw.hits[key] = append(kept, now)
	return true
}

func (rw *rateWindow) gc() {
	now := time.Now()
	rw.mu.Lock()
	defer rw.mu.Unlock()
	for k, hits := range rw.hits {
		kept := hits[:0]
		for _, t := range hits {
			if now.Sub(t) < rw.window {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(rw.hits, k)
		} else {
			rw.hits[k] = kept
		}
	}
}

func clientIP(c *gin.Context) string {
	// 优先取反代透传的 X-Forwarded-For 首个地址
	if xf := c.GetHeader("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	return c.ClientIP()
}

func CurrentUser(c *gin.Context) *model.User {
	if v, ok := c.Get("currentUser"); ok {
		if u, ok := v.(*model.User); ok {
			return u
		}
	}
	return nil
}

func UserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(ContextUserIDKey); ok {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}
