package model

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	BaseModel
	Code string `gorm:"size:50;not null;uniqueIndex" json:"code"`
	Name string `gorm:"size:100;not null" json:"name"`
}

type User struct {
	BaseModel
	Username     string     `gorm:"size:100;not null;uniqueIndex" json:"username"`
	DisplayName  string     `gorm:"size:100;not null" json:"displayName"`
	Email        string     `gorm:"size:255" json:"email,omitempty"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	AuthSource   string     `gorm:"size:20;not null;default:local" json:"authSource"`
	Enabled      bool       `gorm:"not null;default:true" json:"enabled"`
	FailedLogins int        `gorm:"not null;default:0" json:"-"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	Roles        []Role     `gorm:"many2many:user_roles" json:"roles,omitempty"`
}

func (u *User) HasRole(code string) bool {
	for _, r := range u.Roles {
		if r.Code == code {
			return true
		}
	}
	return false
}

func (u *User) RoleCodes() []string {
	out := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		out = append(out, r.Code)
	}
	return out
}

func (u *User) RoleIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(u.Roles))
	for _, r := range u.Roles {
		out = append(out, r.ID)
	}
	return out
}

type AuditLog struct {
	BaseModel
	UserID       *uuid.UUID `gorm:"type:uuid;index" json:"userId,omitempty"`
	Action       string     `gorm:"size:100;not null;index" json:"action"`
	ResourceType string     `gorm:"size:100;not null;index" json:"resourceType"`
	ResourceID   *uuid.UUID `gorm:"type:uuid;index" json:"resourceId,omitempty"`
	RequestID    string     `gorm:"size:100;index" json:"requestId"`
	BeforeJSON   string     `gorm:"type:jsonb" json:"before,omitempty"`
	AfterJSON    string     `gorm:"type:jsonb" json:"after,omitempty"`
	Result       string     `gorm:"size:20;not null" json:"result"`
	ErrorCode    string     `gorm:"size:100" json:"errorCode,omitempty"`
	Source       string     `gorm:"size:50" json:"source"`
}
