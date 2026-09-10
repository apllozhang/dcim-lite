package service

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

type AdminService struct {
	users *repository.UserStore
}

func NewAdminService(users *repository.UserStore) *AdminService {
	return &AdminService{users: users}
}

type UserAdminInput struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	AuthSource  string   `json:"authSource"`
	Enabled     *bool    `json:"enabled"`
	RoleCodes   []string `json:"roleCodes"`
	// Roles 为厂商基线字段名（roles），与 roleCodes 等价；两者都传时以 roleCodes 为准。
	Roles []string `json:"roles"`
}

// desiredRoleCodes 兼容厂商的 roles 字段命名。
func (in UserAdminInput) desiredRoleCodes() []string {
	if len(in.RoleCodes) > 0 {
		return in.RoleCodes
	}
	return in.Roles
}

type ResetPasswordInput struct {
	Password string `json:"password" binding:"required"`
}

type UserListQuery struct {
	Search     string
	AuthSource string
}

func (s *AdminService) ListRoles() ([]model.Role, error) {
	return s.users.EnsureRolesList()
}

func (s *AdminService) ListUsers(q UserListQuery) ([]model.User, error) {
	return s.users.ListUsers(q.Search, q.AuthSource)
}

func (s *AdminService) CreateUser(in UserAdminInput) (*model.User, error) {
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	// 厂商对用户字段错误返回 INVALID_USER（差分用例 S14-VAL-USER-EMPTYNAME/SHORTPW、S15-USER-NO-PASSWORD）
	if in.Username == "" || in.DisplayName == "" {
		return nil, apperr.New(400, "INVALID_USER", "invalid user: 用户名和显示名称不能为空")
	}
	if len([]rune(in.Password)) < 8 {
		return nil, apperr.New(400, "INVALID_USER", "invalid user: 本地用户密码至少 8 位")
	}
	if in.AuthSource == "" {
		in.AuthSource = "local"
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	roles, err := s.resolveRoles(in.desiredRoleCodes())
	if err != nil {
		return nil, err
	}
	if _, err := s.users.FindByUsername(in.Username); err == nil {
		return nil, apperr.New(409, "USER_CONFLICT", "用户名已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Username: in.Username, DisplayName: in.DisplayName, Email: strings.TrimSpace(in.Email),
		PasswordHash: string(hash), AuthSource: in.AuthSource, Enabled: enabled,
		Roles: roles,
	}
	if err := s.users.CreateUser(u); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.New(409, "USER_CONFLICT", "用户名已存在")
		}
		return nil, err
	}
	return s.users.FindByID(u.ID)
}

func (s *AdminService) UpdateUser(id uuid.UUID, version uint, in UserAdminInput) (*model.User, error) {
	// 事务 + 目标用户行锁：并发修改/删除最后管理员时，不变量校验与写入串行化
	err := s.users.DB().Transaction(func(tx *gorm.DB) error {
		users := s.users.WithTx(tx)
		existing, err := users.FindByIDLock(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("用户")
			}
			return err
		}
		in.Username = strings.TrimSpace(in.Username)
		in.DisplayName = strings.TrimSpace(in.DisplayName)
		// 厂商基线允许部分更新：未传的字段保留原值（S11-USER-DISABLE 等差分用例）
		if in.Username == "" {
			in.Username = existing.Username
		}
		if in.DisplayName == "" {
			in.DisplayName = existing.DisplayName
		}
		if in.AuthSource == "" {
			in.AuthSource = existing.AuthSource
		}
		enabled := existing.Enabled
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		desired := in.desiredRoleCodes()
		var roles []model.Role
		if len(desired) == 0 {
			roles = existing.Roles // 未提供角色 → 保留现有
		} else {
			roles, err = s.resolveRoles(desired)
			if err != nil {
				return err
			}
		}
		// 最后管理员保护：禁止停用或去掉 system_admin
		if existing.HasRole("system_admin") && (!enabled || !hasRoleCode(roles, "system_admin")) {
			n, err := users.CountEnabledAdminsExcluding(id)
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.New(409, "LAST_ADMIN_PROTECTED", "不能停用或降级最后一个管理员")
			}
		}
		// 用户名冲突
		if in.Username != existing.Username {
			if other, err := users.FindByUsername(in.Username); err == nil && other.ID != id {
				return apperr.New(409, "USER_CONFLICT", "用户名已存在")
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return mapStoreErr(users.UpdateUser(id, version, in.Username, in.DisplayName, strings.TrimSpace(in.Email), in.AuthSource, enabled, roles))
	})
	if err != nil {
		return nil, err
	}
	return s.users.FindByID(id)
}

func (s *AdminService) DeleteUser(id uuid.UUID, version uint) error {
	return s.users.DB().Transaction(func(tx *gorm.DB) error {
		users := s.users.WithTx(tx)
		existing, err := users.FindByIDLock(tx, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("用户")
			}
			return err
		}
		if existing.HasRole("system_admin") {
			n, err := users.CountEnabledAdminsExcluding(id)
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.New(409, "LAST_ADMIN_PROTECTED", "不能删除最后一个管理员")
			}
		}
		return mapStoreErr(users.SoftDeleteUser(id, version))
	})
}

func (s *AdminService) ResetPassword(id uuid.UUID, version uint, password string) error {
	if len([]rune(password)) < 8 {
		return apperr.New(400, "INVALID_USER", "invalid user: 本地用户密码至少 8 位")
	}
	if _, err := s.users.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("用户")
		}
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return mapStoreErr(s.users.UpdatePasswordHash(id, version, string(hash)))
}

func (s *AdminService) resolveRoles(codes []string) ([]model.Role, error) {
	if len(codes) == 0 {
		return nil, apperr.InvalidResource("至少选择一个角色")
	}
	all, err := s.users.EnsureRolesList()
	if err != nil {
		return nil, err
	}
	byCode := map[string]model.Role{}
	for _, r := range all {
		byCode[r.Code] = r
	}
	out := make([]model.Role, 0, len(codes))
	for _, c := range codes {
		c = strings.TrimSpace(c)
		r, ok := byCode[c]
		if !ok {
			return nil, apperr.InvalidResource("未知角色 %s", c)
		}
		out = append(out, r)
	}
	return out, nil
}

func hasRoleCode(roles []model.Role, code string) bool {
	for _, r := range roles {
		if r.Code == code {
			return true
		}
	}
	return false
}
