package repository

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/model"
)

type LDAPStore struct{ db *gorm.DB }

func NewLDAPStore(db *gorm.DB) *LDAPStore { return &LDAPStore{db: db} }

func (s *LDAPStore) Get() (*model.LDAPConfig, error) {
	var c model.LDAPConfig
	err := s.db.Order("created_at asc").First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *LDAPStore) Save(c *model.LDAPConfig) error {
	if c.ID == uuid.Nil {
		existing, err := s.Get()
		if err == gorm.ErrRecordNotFound {
			if c.UserFilter == "" {
				c.UserFilter = "(|(uid={{username}})(sAMAccountName={{username}}))"
			}
			if c.UsernameAttribute == "" {
				c.UsernameAttribute = "uid"
			}
			if c.DisplayNameAttribute == "" {
				c.DisplayNameAttribute = "displayName"
			}
			if c.EmailAttribute == "" {
				c.EmailAttribute = "mail"
			}
			if c.DefaultRole == "" {
				c.DefaultRole = "user"
			}
			if c.GroupFilter == "" {
				c.GroupFilter = "(&(objectClass=groupOfNames)(member={{userDN}}))"
			}
			if c.GroupAttribute == "" {
				c.GroupAttribute = "cn"
			}
			if c.GroupRoleMappings == "" {
				c.GroupRoleMappings = "[]"
			}
			return s.db.Create(c).Error
		}
		if err != nil {
			return err
		}
		c.ID = existing.ID
		c.Version = existing.Version
		if c.BindPassword == "" {
			c.BindPassword = existing.BindPassword
		}
	}
	res := s.db.Model(&model.LDAPConfig{}).
		Where("id = ? AND version = ?", c.ID, c.Version).
		Updates(map[string]any{
			"enabled":                c.Enabled,
			"url":                    c.URL,
			"use_tls":                c.UseTLS,
			"skip_tls_verify":        c.SkipTLSVerify,
			"bind_dn":                c.BindDN,
			"bind_password":          c.BindPassword,
			"base_dn":                c.BaseDN,
			"user_filter":            c.UserFilter,
			"username_attribute":     c.UsernameAttribute,
			"display_name_attribute": c.DisplayNameAttribute,
			"email_attribute":        c.EmailAttribute,
			"default_role":           c.DefaultRole,
			"allow_local_fallback":   c.AllowLocalFallback,
			"group_search_base_dn":   c.GroupSearchBaseDN,
			"group_filter":           c.GroupFilter,
			"group_attribute":        c.GroupAttribute,
			"group_role_mappings":    c.GroupRoleMappings,
			"version":                gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(c, "id = ?", c.ID).Error
}
