package service

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

// 登录失败锁定策略：连续失败达到阈值后临时锁定
const (
	loginFailureThreshold = 5
	loginLockDuration     = 15 * time.Minute
)

type AuthService struct {
	users   *repository.UserStore
	secret  []byte
	ttl     time.Duration
	revoker middleware.TokenRevoker
}

func NewAuthService(users *repository.UserStore, secret string, ttl time.Duration, revoker middleware.TokenRevoker) *AuthService {
	return &AuthService{users: users, secret: []byte(secret), ttl: ttl, revoker: revoker}
}

type LoginResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *AuthService) Login(username, password string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, apperr.InvalidCredentials()
	}
	user, err := s.users.FindByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.InvalidCredentials()
		}
		return nil, err
	}
	if !user.Enabled {
		// 厂商基线对停用账户返回独立错误码（差分用例 S11-USER-DISABLED-LOGIN）
		return nil, apperr.New(401, "USER_DISABLED", "账户已停用")
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remain := time.Until(*user.LockedUntil).Round(time.Minute)
		return nil, apperr.New(401, "USER_LOCKED", "失败次数过多，账户已临时锁定，请约 "+remain.String()+" 后重试")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		// 失败计数：达到阈值写入 locked_until（登录成功时清零）
		_, _ = s.users.RecordLoginFailure(user.ID, loginFailureThreshold, loginLockDuration)
		return nil, apperr.InvalidCredentials()
	}
	token, err := s.issueToken(user)
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdateLastLogin(user.ID); err != nil {
		return nil, err
	}
	user, err = s.users.FindByID(user.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}

func (s *AuthService) Me(userID uuid.UUID) (*model.User, error) {
	user, err := s.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.Unauthorized()
		}
		return nil, err
	}
	return user, nil
}

// Logout 吊销当前 token（jti 进入黑名单），使登出后旧 token 立即失效。
func (s *AuthService) Logout(tokenString string) error {
	if s.revoker == nil || tokenString == "" {
		return nil
	}
	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		// 已过期的 token 无需吊销；格式非法按未登录处理
		return nil
	}
	if claims.ID != "" {
		exp := time.Now().Add(s.ttl)
		if claims.ExpiresAt != nil {
			exp = claims.ExpiresAt.Time
		}
		s.revoker.Revoke(claims.ID, exp)
	}
	return nil
}

func (s *AuthService) issueToken(user *model.User) (string, error) {
	now := time.Now()
	claims := &middleware.Claims{
		UserID: user.ID.String(),
		Roles:  user.RoleCodes(),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(), // jti：登出/改密吊销的锚点
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}
