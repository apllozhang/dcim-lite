package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"dcim-lite/internal/model"
)

func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

type UserStore struct{ db *gorm.DB }

func NewUserStore(db *gorm.DB) *UserStore { return &UserStore{db: db} }

// WithTx 返回绑定外部事务的存储副本；事务内的嵌套 Transaction 会退化为 savepoint。
func (s *UserStore) WithTx(tx *gorm.DB) *UserStore { return &UserStore{db: tx} }

func (s *UserStore) DB() *gorm.DB { return s.db }

func (s *UserStore) FindByID(id uuid.UUID) (*model.User, error) {
	var u model.User
	err := s.db.Preload("Roles").First(&u, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByIDLock 读取用户并加行锁，供“最后管理员”校验等事务内不变量使用。
func (s *UserStore) FindByIDLock(tx *gorm.DB, id uuid.UUID) (*model.User, error) {
	var u model.User
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Roles").First(&u, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) FindByUsername(username string) (*model.User, error) {
	var u model.User
	err := s.db.Preload("Roles").First(&u, "username = ?", username).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) UpdateLastLogin(id uuid.UUID) error {
	now := time.Now()
	return s.db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"last_login_at": now,
		"failed_logins": 0,
		"locked_until":  nil,
	}).Error
}

// RecordLoginFailure 累计失败次数；达到 threshold 后写入锁定截止时间并返回 true。
func (s *UserStore) RecordLoginFailure(id uuid.UUID, threshold int, lock time.Duration) (bool, error) {
	res := s.db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"failed_logins": gorm.Expr("failed_logins + 1"),
	})
	if res.Error != nil {
		return false, res.Error
	}
	var u model.User
	if err := s.db.Select("failed_logins").First(&u, "id = ?", id).Error; err != nil {
		return false, err
	}
	if int(u.FailedLogins) >= threshold {
		if err := s.db.Model(&model.User{}).Where("id = ?", id).
			Update("locked_until", time.Now().Add(lock)).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (s *UserStore) EnsureRolesList() ([]model.Role, error) {
	if _, _, err := s.EnsureRoles(); err != nil {
		return nil, err
	}
	var roles []model.Role
	err := s.db.Order("code asc").Find(&roles).Error
	return roles, err
}

func (s *UserStore) ListUsers(search, authSource string) ([]model.User, error) {
	q := s.db.Model(&model.User{})
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("username ILIKE ? OR display_name ILIKE ? OR email ILIKE ?", like, like, like)
	}
	if authSource != "" {
		q = q.Where("auth_source = ?", authSource)
	}
	var items []model.User
	err := q.Preload("Roles").Order("created_at asc").Find(&items).Error
	return items, err
}

func (s *UserStore) CreateUser(u *model.User) error {
	return s.db.Create(u).Error
}

func (s *UserStore) UpdateUser(id uuid.UUID, version uint, username, displayName, email, authSource string, enabled bool, roles []model.Role) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.User{}).
			Where("id = ? AND version = ?", id, version).
			Updates(map[string]any{
				"username":     username,
				"display_name": displayName,
				"email":        email,
				"auth_source":  authSource,
				"enabled":      enabled,
				"version":      gorm.Expr("version + 1"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errVersion()
		}
		var u model.User
		if err := tx.First(&u, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Model(&u).Association("Roles").Replace(roles); err != nil {
			return err
		}
		return nil
	})
}

func (s *UserStore) SoftDeleteUser(id uuid.UUID, version uint) error {
	res := s.db.Model(&model.User{}).
		Where("id = ? AND version = ?", id, version).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"version":    gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *UserStore) UpdatePasswordHash(id uuid.UUID, version uint, hash string) error {
	res := s.db.Model(&model.User{}).
		Where("id = ? AND version = ?", id, version).
		Updates(map[string]any{
			"password_hash": hash,
			"version":       gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *UserStore) CountEnabledAdminsExcluding(id uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Model(&model.User{}).
		Where("enabled = ? AND deleted_at IS NULL AND id <> ?", true, id).
		Where(`EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = users.id AND r.code = 'system_admin' AND r.deleted_at IS NULL
		)`).
		Count(&n).Error
	return n, err
}

func (s *UserStore) EnsureRoles() (*model.Role, *model.Role, error) {
	adminRole := model.Role{Code: "system_admin", Name: "系统管理员"}
	userRole := model.Role{Code: "user", Name: "普通用户"}
	for _, r := range []*model.Role{&adminRole, &userRole} {
		var existing model.Role
		err := s.db.First(&existing, "code = ?", r.Code).Error
		if err == gorm.ErrRecordNotFound {
			if err := s.db.Create(r).Error; err != nil {
				return nil, nil, err
			}
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		*r = existing
	}
	return &adminRole, &userRole, nil
}

// EnsureAdmin 仅在用户不存在时创建种子管理员；已存在但无管理员角色的用户不会被静默提权。
func (s *UserStore) EnsureAdmin(username, password, displayName string) error {
	if username == "" || password == "" {
		return nil
	}
	adminRole, _, err := s.EnsureRoles()
	if err != nil {
		return err
	}
	var u model.User
	err = s.db.Preload("Roles").First(&u, "username = ?", username).Error
	if err == nil {
		if !u.HasRole("system_admin") {
			fmt.Printf("[WARN] bootstrap admin %q exists without system_admin role; skipping role grant (no silent privilege escalation)\n", username)
		}
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if displayName == "" {
		displayName = username
	}
	u = model.User{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		AuthSource:   "local",
		Enabled:      true,
		Roles:        []model.Role{*adminRole},
	}
	return s.db.Create(&u).Error
}

type ResourceStore struct{ db *gorm.DB }

func NewResourceStore(db *gorm.DB) *ResourceStore { return &ResourceStore{db: db} }

func (s *ResourceStore) WithTx(tx *gorm.DB) *ResourceStore { return &ResourceStore{db: tx} }

func (s *ResourceStore) DB() *gorm.DB { return s.db }

// CountActivePositions 统计机柜上的在位设备（未软删），删除保护用。
func (s *ResourceStore) CountActivePositions(rackID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Table("rack_device_positions").
		Where("rack_id = ? AND deleted_at IS NULL", rackID).Count(&n).Error
	return n, err
}

// CountPDUsByRack 统计机柜下未软删的 PDU，删除保护用。
func (s *ResourceStore) CountPDUsByRack(rackID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Table("pdus").
		Where("rack_id = ? AND deleted_at IS NULL", rackID).Count(&n).Error
	return n, err
}

func (s *ResourceStore) ListDataCenters() ([]model.DataCenter, error) {
	var items []model.DataCenter
	err := s.db.Preload("Rooms.Racks").Order("sort_order asc, created_at asc").Find(&items).Error
	return items, err
}

type RackQuery struct {
	Page       int
	PageSize   int
	Search     string
	Status     string
	RoomID     *uuid.UUID
	DataCenter *uuid.UUID
	SortBy     string // code|name|status|uHeight|createdAt
	SortDir    string
}

func (s *ResourceStore) ListRacks(q RackQuery) ([]model.Rack, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 200 {
		q.PageSize = 20
	}
	base := s.db.Model(&model.Rack{})
	if q.Search != "" {
		like := "%" + strings.TrimSpace(q.Search) + "%"
		base = base.Where("code ILIKE ? OR name ILIKE ? OR zone ILIKE ? OR manager ILIKE ? OR manufacturer ILIKE ?",
			like, like, like, like, like)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	if q.RoomID != nil {
		base = base.Where("room_id = ?", *q.RoomID)
	}
	if q.DataCenter != nil {
		base = base.Where("data_center_id = ?", *q.DataCenter)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	col := "created_at"
	switch q.SortBy {
	case "code":
		col = "code"
	case "name":
		col = "name"
	case "status":
		col = "status"
	case "uHeight":
		col = "u_height"
	}
	dir := "ASC"
	if strings.EqualFold(q.SortDir, "desc") {
		dir = "DESC"
	}
	var items []model.Rack
	// id 作为稳定 tie-breaker：相同排序键时 offset 分页不重不漏
	err := base.Order(col + " " + dir + ", id ASC").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&items).Error
	return items, total, err
}

func (s *ResourceStore) GetDataCenter(id uuid.UUID) (*model.DataCenter, error) {
	var item model.DataCenter
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ResourceStore) CreateDataCenter(item *model.DataCenter) error {
	return s.db.Create(item).Error
}

func (s *ResourceStore) UpdateDataCenter(item *model.DataCenter, expectedVersion uint) error {
	res := s.db.Model(&model.DataCenter{}).
		Where("id = ? AND version = ?", item.ID, expectedVersion).
		Updates(map[string]any{
			"code":             item.Code,
			"name":             item.Name,
			"address":          item.Address,
			"longitude":        item.Longitude,
			"latitude":         item.Latitude,
			"status":           item.Status,
			"manager":          item.Manager,
			"contact":          item.Contact,
			"service_provider": item.ServiceProvider,
			"remarks":          item.Remarks,
			"sort_order":       item.SortOrder,
			"version":          gorm.Expr("version + 1"),
			"updated_at":       time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(item, "id = ?", item.ID).Error
}

func (s *ResourceStore) SoftDeleteDataCenter(id uuid.UUID, expectedVersion uint) error {
	res := s.db.Model(&model.DataCenter{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"version":    gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *ResourceStore) CountRooms(dcID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Model(&model.Room{}).Where("data_center_id = ?", dcID).Count(&n).Error
	return n, err
}

func (s *ResourceStore) GetRoom(id uuid.UUID) (*model.Room, error) {
	var item model.Room
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ResourceStore) CreateRoom(item *model.Room) error {
	return s.db.Create(item).Error
}

func (s *ResourceStore) UpdateRoom(item *model.Room, expectedVersion uint) error {
	res := s.db.Model(&model.Room{}).
		Where("id = ? AND version = ?", item.ID, expectedVersion).
		Updates(map[string]any{
			"code":                item.Code,
			"name":                item.Name,
			"building":            item.Building,
			"floor":               item.Floor,
			"room_number":         item.RoomNumber,
			"area_square_meters":  item.AreaSquareMeters,
			"clear_height_meters": item.ClearHeightMeters,
			"purpose":             item.Purpose,
			"status":              item.Status,
			"environment_level":   item.EnvironmentLevel,
			"max_load_kg":         item.MaxLoadKg,
			"cooling_capacity_kw": item.CoolingCapacityKw,
			"design_power_kw":     item.DesignPowerKw,
			"available_power_kw":  item.AvailablePowerKw,
			"used_power_kw":       item.UsedPowerKw,
			"redundancy_policy":   item.RedundancyPolicy,
			"manager":             item.Manager,
			"contact":             item.Contact,
			"open_hours":          item.OpenHours,
			"access_notes":        item.AccessNotes,
			"floor_plan_enabled":  item.FloorPlanEnabled,
			"floor_plan_format":   item.FloorPlanFormat,
			"racks_per_row":       item.RacksPerRow,
			"remarks":             item.Remarks,
			"sort_order":          item.SortOrder,
			"version":             gorm.Expr("version + 1"),
			"updated_at":          time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(item, "id = ?", item.ID).Error
}

func (s *ResourceStore) SoftDeleteRoom(id uuid.UUID, expectedVersion uint) error {
	res := s.db.Model(&model.Room{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"version":    gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *ResourceStore) CountRacks(roomID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Model(&model.Rack{}).Where("room_id = ?", roomID).Count(&n).Error
	return n, err
}

func (s *ResourceStore) GetRack(id uuid.UUID) (*model.Rack, error) {
	var item model.Rack
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ResourceStore) CreateRack(item *model.Rack) error {
	return s.db.Create(item).Error
}

func (s *ResourceStore) UpdateRack(item *model.Rack, expectedVersion uint) error {
	res := s.db.Model(&model.Rack{}).
		Where("id = ? AND version = ?", item.ID, expectedVersion).
		Updates(map[string]any{
			"code":             item.Code,
			"name":             item.Name,
			"type":             item.Type,
			"manufacturer":     item.Manufacturer,
			"model_number":     item.ModelNumber,
			"serial_number":    item.SerialNumber,
			"asset_number":     item.AssetNumber,
			"u_height":         item.UHeight,
			"width_mm":         item.WidthMm,
			"depth_mm":         item.DepthMm,
			"height_mm":        item.HeightMm,
			"load_capacity_kg": item.LoadCapacityKg,
			"zone":             item.Zone,
			"rack_row":         item.RackRow,
			"rack_column":      item.RackColumn,
			"aisle":            item.Aisle,
			"x_coordinate":     item.XCoordinate,
			"y_coordinate":     item.YCoordinate,
			"rotation":         item.Rotation,
			"status":           item.Status,
			"manager":          item.Manager,
			"department":       item.Department,
			"purpose":          item.Purpose,
			"dual_power":       item.DualPower,
			"input_circuits":   item.InputCircuits,
			"rated_voltage":    item.RatedVoltage,
			"rated_current":    item.RatedCurrent,
			"rated_power_kw":   item.RatedPowerKw,
			"peak_power_kw":    item.PeakPowerKw,
			"pdu_count":        item.PDUCount,
			"remarks":          item.Remarks,
			"sort_order":       item.SortOrder,
			"version":          gorm.Expr("version + 1"),
			"updated_at":       time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(item, "id = ?", item.ID).Error
}

func (s *ResourceStore) SoftDeleteRack(id uuid.UUID, expectedVersion uint) error {
	res := s.db.Model(&model.Rack{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"version":    gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *ResourceStore) CodeExistsDataCenter(code string, excludeID uuid.UUID) (bool, error) {
	var n int64
	q := s.db.Model(&model.DataCenter{}).Where("code = ?", code)
	if excludeID != uuid.Nil {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (s *ResourceStore) CodeExistsRoom(dcID uuid.UUID, code string, excludeID uuid.UUID) (bool, error) {
	var n int64
	q := s.db.Model(&model.Room{}).Where("data_center_id = ? AND code = ?", dcID, code)
	if excludeID != uuid.Nil {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (s *ResourceStore) CodeExistsRack(roomID uuid.UUID, code string, excludeID uuid.UUID) (bool, error) {
	var n int64
	q := s.db.Model(&model.Rack{}).Where("room_id = ? AND code = ?", roomID, code)
	if excludeID != uuid.Nil {
		q = q.Where("id <> ?", excludeID)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (s *ResourceStore) WriteAudit(log *model.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.Result == "" {
		log.Result = "SUCCESS"
	}
	return s.db.Create(log).Error
}

type versionError struct{}

func (versionError) Error() string { return "resource version conflict" }

func errVersion() error { return versionError{} }

func IsVersionConflict(err error) bool {
	_, ok := err.(versionError)
	return ok
}

func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// 优先按 PostgreSQL SQLSTATE 判断；savepoint 回滚会把驱动错误包在文本里，保留字符串回退。
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return contains(err.Error(), "duplicate key") || contains(err.Error(), "SQLSTATE 23505")
}

func IsExclusionViolationError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
		return true
	}
	return contains(err.Error(), "SQLSTATE 23P01")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
