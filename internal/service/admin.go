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
	if in.Username == "" || in.DisplayName == "" {
		return nil, apperr.InvalidResource("用户名和显示名称不能为空")
	}
	if in.Password == "" {
		return nil, apperr.InvalidResource("密码不能为空")
	}
	if in.AuthSource == "" {
		in.AuthSource = "local"
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	roles, err := s.resolveRoles(in.RoleCodes)
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
	existing, err := s.users.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("用户")
		}
		return nil, err
	}
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.Username == "" || in.DisplayName == "" {
		return nil, apperr.InvalidResource("用户名和显示名称不能为空")
	}
	if in.AuthSource == "" {
		in.AuthSource = existing.AuthSource
	}
	enabled := existing.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	roles, err := s.resolveRoles(in.RoleCodes)
	if err != nil {
		return nil, err
	}
	// 最后管理员保护：禁止停用或去掉 system_admin
	if existing.HasRole("system_admin") && (!enabled || !hasRoleCode(roles, "system_admin")) {
		n, err := s.users.CountEnabledAdminsExcluding(id)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, apperr.New(409, "LAST_ADMIN", "不能停用或降级最后一个管理员")
		}
	}
	// 用户名冲突
	if in.Username != existing.Username {
		if other, err := s.users.FindByUsername(in.Username); err == nil && other.ID != id {
			return nil, apperr.New(409, "USER_CONFLICT", "用户名已存在")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if err := s.users.UpdateUser(id, version, in.Username, in.DisplayName, strings.TrimSpace(in.Email), in.AuthSource, enabled, roles); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.users.FindByID(id)
}

func (s *AdminService) DeleteUser(id uuid.UUID, version uint) error {
	existing, err := s.users.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("用户")
		}
		return err
	}
	if existing.HasRole("system_admin") {
		n, err := s.users.CountEnabledAdminsExcluding(id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.New(409, "LAST_ADMIN", "不能删除最后一个管理员")
		}
	}
	return mapStoreErr(s.users.SoftDeleteUser(id, version))
}

func (s *AdminService) ResetPassword(id uuid.UUID, version uint, password string) error {
	if len(password) < 6 {
		return apperr.InvalidResource("密码至少 6 位")
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
