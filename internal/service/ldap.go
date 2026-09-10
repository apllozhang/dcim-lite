package service

import (
	"errors"
	"net"
	"strings"
	"time"

	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

type LDAPService struct{ store *repository.LDAPStore }

func NewLDAPService(store *repository.LDAPStore) *LDAPService {
	return &LDAPService{store: store}
}

type LDAPInput struct {
	Enabled              *bool  `json:"enabled"`
	URL                  string `json:"url"`
	UseTLS               *bool  `json:"useTls"`
	SkipTLSVerify        *bool  `json:"skipTlsVerify"`
	BindDN               string `json:"bindDn"`
	BindPassword         string `json:"bindPassword"`
	BaseDN               string `json:"baseDn"`
	UserFilter           string `json:"userFilter"`
	UsernameAttribute    string `json:"usernameAttribute"`
	DisplayNameAttribute string `json:"displayNameAttribute"`
	EmailAttribute       string `json:"emailAttribute"`
	DefaultRole          string `json:"defaultRole"`
	AllowLocalFallback   *bool  `json:"allowLocalFallback"`
	GroupSearchBaseDN    string `json:"groupSearchBaseDn"`
	GroupFilter          string `json:"groupFilter"`
	GroupAttribute       string `json:"groupAttribute"`
	GroupRoleMappings    string `json:"groupRoleMappings"`
}

// PublicLDAP is API view: hasBindPassword instead of raw secret.
type PublicLDAP struct {
	ID                   string `json:"id"`
	Enabled              bool   `json:"enabled"`
	URL                  string `json:"url"`
	UseTLS               bool   `json:"useTls"`
	SkipTLSVerify        bool   `json:"skipTlsVerify"`
	BindDN               string `json:"bindDn"`
	BaseDN               string `json:"baseDn"`
	UserFilter           string `json:"userFilter"`
	UsernameAttribute    string `json:"usernameAttribute"`
	DisplayNameAttribute string `json:"displayNameAttribute"`
	EmailAttribute       string `json:"emailAttribute"`
	DefaultRole          string `json:"defaultRole"`
	AllowLocalFallback   bool   `json:"allowLocalFallback"`
	GroupSearchBaseDN    string `json:"groupSearchBaseDn"`
	GroupFilter          string `json:"groupFilter"`
	GroupAttribute       string `json:"groupAttribute"`
	GroupRoleMappings    string `json:"groupRoleMappings"`
	HasBindPassword      bool   `json:"hasBindPassword"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	Version              uint   `json:"version"`
}

func toPublic(c *model.LDAPConfig) *PublicLDAP {
	return &PublicLDAP{
		ID:                   c.ID.String(),
		Enabled:              c.Enabled,
		URL:                  c.URL,
		UseTLS:               c.UseTLS,
		SkipTLSVerify:        c.SkipTLSVerify,
		BindDN:               c.BindDN,
		BaseDN:               c.BaseDN,
		UserFilter:           c.UserFilter,
		UsernameAttribute:    c.UsernameAttribute,
		DisplayNameAttribute: c.DisplayNameAttribute,
		EmailAttribute:       c.EmailAttribute,
		DefaultRole:          c.DefaultRole,
		AllowLocalFallback:   c.AllowLocalFallback,
		GroupSearchBaseDN:    c.GroupSearchBaseDN,
		GroupFilter:          c.GroupFilter,
		GroupAttribute:       c.GroupAttribute,
		GroupRoleMappings:    c.GroupRoleMappings,
		HasBindPassword:      c.BindPassword != "",
		CreatedAt:            c.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:            c.UpdatedAt.Format(time.RFC3339Nano),
		Version:              c.Version,
	}
}

func (s *LDAPService) Get() (*PublicLDAP, error) {
	c, err := s.store.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return toPublic(model.DefaultLDAPConfig()), nil
		}
		return nil, err
	}
	return toPublic(c), nil
}

func (s *LDAPService) Update(in LDAPInput) (*PublicLDAP, error) {
	c, err := s.store.Get()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		c = model.DefaultLDAPConfig()
	}
	if in.Enabled != nil {
		c.Enabled = *in.Enabled
	}
	if in.URL != "" {
		c.URL = strings.TrimSpace(in.URL)
	}
	if in.UseTLS != nil {
		c.UseTLS = *in.UseTLS
	}
	if in.SkipTLSVerify != nil {
		c.SkipTLSVerify = *in.SkipTLSVerify
	}
	if in.BindDN != "" {
		c.BindDN = in.BindDN
	}
	if in.BindPassword != "" {
		c.BindPassword = in.BindPassword
	}
	if in.BaseDN != "" {
		c.BaseDN = in.BaseDN
	}
	if in.UserFilter != "" {
		c.UserFilter = in.UserFilter
	}
	if in.UsernameAttribute != "" {
		c.UsernameAttribute = in.UsernameAttribute
	}
	if in.DisplayNameAttribute != "" {
		c.DisplayNameAttribute = in.DisplayNameAttribute
	}
	if in.EmailAttribute != "" {
		c.EmailAttribute = in.EmailAttribute
	}
	if in.DefaultRole != "" {
		c.DefaultRole = in.DefaultRole
	}
	if in.AllowLocalFallback != nil {
		c.AllowLocalFallback = *in.AllowLocalFallback
	}
	if in.GroupSearchBaseDN != "" {
		c.GroupSearchBaseDN = in.GroupSearchBaseDN
	}
	if in.GroupFilter != "" {
		c.GroupFilter = in.GroupFilter
	}
	if in.GroupAttribute != "" {
		c.GroupAttribute = in.GroupAttribute
	}
	if in.GroupRoleMappings != "" {
		c.GroupRoleMappings = in.GroupRoleMappings
	}
	if err := s.store.Save(c); err != nil {
		return nil, mapStoreErr(err)
	}
	saved, err := s.store.Get()
	if err != nil {
		return nil, err
	}
	return toPublic(saved), nil
}

type LDAPTestResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// Test performs a lightweight TCP reachability check. Full bind/search needs a real directory (EXTERNAL_BLOCKED).
func (s *LDAPService) Test(in LDAPInput) (*LDAPTestResult, error) {
	url := strings.TrimSpace(in.URL)
	if url == "" {
		c, err := s.store.Get()
		if err == nil {
			url = c.URL
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if url == "" {
		// 厂商对空地址同样返回 400 LDAP_UNAVAILABLE（S11-LDAP-TEST 契约）
		return nil, apperr.New(400, "LDAP_UNAVAILABLE", "ldap unavailable: LDAP 地址为空")
	}
	addr := strings.TrimPrefix(strings.TrimPrefix(url, "ldap://"), "ldaps://")
	if i := strings.Index(addr, "/"); i >= 0 {
		addr = addr[:i]
	}
	if !strings.Contains(addr, ":") {
		if strings.HasPrefix(url, "ldaps://") {
			addr += ":636"
		} else {
			addr += ":389"
		}
	}
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		// 厂商行为：连通失败 → 400 LDAP_UNAVAILABLE（此前返回 200 造成「假成功」，
		// 差分用例 S11-LDAP-TEST 实测：厂商 400，重建 200）
		return nil, apperr.New(400, "LDAP_UNAVAILABLE",
			"ldap unavailable: 无法连接 LDAP 服务器")
	}
	_ = conn.Close()
	return &LDAPTestResult{
		OK:      true,
		Message: "TCP 连通成功（未做 bind 认证）",
		Details: []string{"正式环境需补充 bind、用户检索与证书校验"},
	}, nil
}

var _ = apperr.InvalidResource
