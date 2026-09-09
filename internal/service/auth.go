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

type AuthService struct {
	users  *repository.UserStore
	secret []byte
	ttl    time.Duration
}

func NewAuthService(users *repository.UserStore, secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: users, secret: []byte(secret), ttl: ttl}
}

type LoginResult struct {
	Token string     `json:"token"`
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
		return nil, apperr.InvalidCredentials()
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
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

func (s *AuthService) issueToken(user *model.User) (string, error) {
	now := time.Now()
	claims := &middleware.Claims{
		UserID: user.ID.String(),
		Roles:  user.RoleCodes(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}
