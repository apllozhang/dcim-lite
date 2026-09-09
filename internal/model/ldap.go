package model

import "github.com/google/uuid"

// LDAPConfig is a single-row config. Bind password is never returned.
type LDAPConfig struct {
	BaseModel
	Enabled              bool   `gorm:"not null;default:false" json:"enabled"`
	URL                  string `gorm:"size:500;not null;default:''" json:"url"`
	UseTLS               bool   `gorm:"not null;default:false" json:"useTls"`
	SkipTLSVerify        bool   `gorm:"not null;default:false" json:"skipTlsVerify"`
	BindDN               string `gorm:"size:255;not null;default:''" json:"bindDn"`
	BindPassword         string `gorm:"size:500;not null;default:''" json:"-"`
	BaseDN               string `gorm:"size:255;not null;default:''" json:"baseDn"`
	UserFilter           string `gorm:"size:500;not null;default:'(|(uid={{username}})(sAMAccountName={{username}}))'" json:"userFilter"`
	UsernameAttribute    string `gorm:"size:100;not null;default:'uid'" json:"usernameAttribute"`
	DisplayNameAttribute string `gorm:"size:100;not null;default:'displayName'" json:"displayNameAttribute"`
	EmailAttribute       string `gorm:"size:100;not null;default:'mail'" json:"emailAttribute"`
	DefaultRole          string `gorm:"size:100;not null;default:'user'" json:"defaultRole"`
	AllowLocalFallback   bool   `gorm:"not null;default:true" json:"allowLocalFallback"`
	GroupSearchBaseDN    string `gorm:"size:255;not null;default:''" json:"groupSearchBaseDn"`
	GroupFilter          string `gorm:"size:500;not null;default:'(&(objectClass=groupOfNames)(member={{userDN}}))'" json:"groupFilter"`
	GroupAttribute       string `gorm:"size:100;not null;default:'cn'" json:"groupAttribute"`
	GroupRoleMappings    string `gorm:"type:jsonb" json:"groupRoleMappings"`
}

func DefaultLDAPConfig() *LDAPConfig {
	return &LDAPConfig{
		Enabled:              false,
		UserFilter:           "(|(uid={{username}})(sAMAccountName={{username}}))",
		UsernameAttribute:    "uid",
		DisplayNameAttribute: "displayName",
		EmailAttribute:       "mail",
		DefaultRole:          "user",
		AllowLocalFallback:   true,
		GroupFilter:          "(&(objectClass=groupOfNames)(member={{userDN}}))",
		GroupAttribute:       "cn",
		GroupRoleMappings:    "[]",
	}
}

var _ = uuid.Nil
