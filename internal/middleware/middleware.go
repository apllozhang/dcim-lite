package middleware

import (
	"strings"

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

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(response.HeaderRequestID)
		if rid == "" {
			rid = "req_" + uuid.NewString()
		}
		c.Set(response.RequestIDKey, rid)
		c.Writer.Header().Set(response.HeaderRequestID, rid)
		c.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				response.Fail(c, 500, "INTERNAL_ERROR", "服务器内部错误")
			}
		}()
		c.Next()
	}
}

func Auth(secret string, users UserFinder) gin.HandlerFunc {
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
