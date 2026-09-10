package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/model"
)

type DeviceStore struct{ db *gorm.DB }

func NewDeviceStore(db *gorm.DB) *DeviceStore { return &DeviceStore{db: db} }

// WithTx 返回绑定外部事务的存储副本；在事务内调用其 Transaction 方法会退化为 savepoint。
func (s *DeviceStore) WithTx(tx *gorm.DB) *DeviceStore { return &DeviceStore{db: tx} }

func (s *DeviceStore) DB() *gorm.DB { return s.db }

func (s *DeviceStore) ListDeviceTypes() ([]model.DeviceType, error) {
	var items []model.DeviceType
	err := s.db.Order("sort_order asc, created_at asc").Find(&items).Error
	return items, err
}

func (s *DeviceStore) GetDeviceType(id uuid.UUID) (*model.DeviceType, error) {
	var item model.DeviceType
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *DeviceStore) CreateDeviceType(item *model.DeviceType) error {
	return s.db.Create(item).Error
}

func (s *DeviceStore) UpdateDeviceType(item *model.DeviceType, expected uint) error {
	res := s.db.Model(&model.DeviceType{}).
		Where("id = ? AND version = ?", item.ID, expected).
		Updates(map[string]any{
			"code": item.Code, "name": item.Name, "category": item.Category,
			"status": item.Status, "default_height_u": item.DefaultHeightU,
			"default_weight_kg":     item.DefaultWeightKg,
			"default_rated_power_w": item.DefaultRatedPowerW,
			"default_peak_power_w":  item.DefaultPeakPowerW,
			"default_dual_power":    item.DefaultDualPower,
			"description":           item.Description, "sort_order": item.SortOrder,
			"version": gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(item, "id = ?", item.ID).Error
}

func (s *DeviceStore) SoftDeleteDeviceType(id uuid.UUID, expected uint) error {
	var n int64
	if err := s.db.Model(&model.Device{}).Where("type_id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("device type in use")
	}
	res := s.db.Model(&model.DeviceType{}).
		Where("id = ? AND version = ?", id, expected).
		Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *DeviceStore) CountDevicesUsingType(typeID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Model(&model.Device{}).Where("type_id = ?", typeID).Count(&n).Error
	return n, err
}

func (s *DeviceStore) CodeExistsDeviceType(code string, exclude uuid.UUID) (bool, error) {
	var n int64
	q := s.db.Model(&model.DeviceType{}).Where("code = ?", code)
	if exclude != uuid.Nil {
		q = q.Where("id <> ?", exclude)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

type DeviceQuery struct {
	Page            int
	PageSize        int
	Search          string
	TypeID          *uuid.UUID
	LifecycleStatus string
	RackID          *uuid.UUID
	SortBy          string // code|name|createdAt|lifecycleStatus|heightU
	SortDir         string // asc|desc
}

func deviceOrderClause(sortBy, sortDir string) string {
	col := "created_at"
	switch sortBy {
	case "code":
		col = "code"
	case "name":
		col = "name"
	case "lifecycleStatus":
		col = "lifecycle_status"
	case "heightU":
		col = "height_u"
	case "createdAt", "":
		col = "created_at"
	default:
		col = "created_at"
	}
	dir := "ASC"
	if strings.EqualFold(sortDir, "desc") {
		dir = "DESC"
	}
	// id 作为稳定 tie-breaker：相同排序键时 offset 分页不重不漏
	return col + " " + dir + ", id ASC"
}

func (s *DeviceStore) ListDevices(q DeviceQuery) ([]model.Device, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 200 {
		q.PageSize = 10
	}
	base := s.db.Model(&model.Device{})
	if q.Search != "" {
		like := "%" + strings.TrimSpace(q.Search) + "%"
		base = base.Where("code ILIKE ? OR name ILIKE ? OR asset_number ILIKE ? OR serial_number ILIKE ? OR management_ip ILIKE ? OR business_ip ILIKE ?",
			like, like, like, like, like, like)
	}
	if q.TypeID != nil {
		base = base.Where("type_id = ?", *q.TypeID)
	}
	if q.LifecycleStatus != "" {
		base = base.Where("lifecycle_status = ?", q.LifecycleStatus)
	}
	if q.RackID != nil {
		base = base.Where(`EXISTS (
			SELECT 1 FROM rack_device_positions p
			WHERE p.device_id = devices.id AND p.rack_id = ? AND p.deleted_at IS NULL
		)`, *q.RackID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Device
	err := base.Preload("Type").
		Order(deviceOrderClause(q.SortBy, q.SortDir)).
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&items).Error
	return items, total, err
}

func (s *DeviceStore) GetDevice(id uuid.UUID) (*model.Device, error) {
	var item model.Device
	err := s.db.Preload("Type").First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *DeviceStore) CreateDevice(item *model.Device) error {
	return s.db.Create(item).Error
}

func (s *DeviceStore) UpdateDevice(item *model.Device, expected uint) error {
	res := s.db.Model(&model.Device{}).
		Where("id = ? AND version = ?", item.ID, expected).
		Updates(map[string]any{
			"type_id": item.TypeID, "code": item.Code, "name": item.Name,
			"asset_number": item.AssetNumber, "serial_number": item.SerialNumber,
			"manufacturer": item.Manufacturer, "model_number": item.ModelNumber,
			"specification": item.Specification, "firmware_version": item.FirmwareVersion,
			"purchase_batch": item.PurchaseBatch, "warranty_expires_at": item.WarrantyExpiresAt,
			"organization": item.Organization, "manager": item.Manager, "contact": item.Contact,
			"business_system": item.BusinessSystem, "application_name": item.ApplicationName,
			"height_u": item.HeightU, "width_mm": item.WidthMm, "depth_mm": item.DepthMm,
			"height_mm": item.HeightMm, "weight_kg": item.WeightKg,
			"rated_power_w": item.RatedPowerW, "peak_power_w": item.PeakPowerW,
			"input_voltage": item.InputVoltage, "dual_power_required": item.DualPowerRequired,
			"management_ip": item.ManagementIP, "business_ip": item.BusinessIP,
			"mac_address": item.MACAddress, "management_protocol": item.ManagementProtocol,
			"monitoring_status": item.MonitoringStatus,
			"external_qr_code":  item.ExternalQRCode, "external_qr_code_url": item.ExternalQRCodeURL,
			"tags": item.Tags, "remarks": item.Remarks,
			"version": gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.Preload("Type").First(item, "id = ?", item.ID).Error
}

func (s *DeviceStore) SoftDeleteDevice(id uuid.UUID, expected uint) error {
	res := s.db.Model(&model.Device{}).
		Where("id = ? AND version = ?", id, expected).
		Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}

func (s *DeviceStore) UpdateDeviceLifecycle(id uuid.UUID, status string) error {
	return s.db.Model(&model.Device{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"lifecycle_status": status,
			"version":          gorm.Expr("version + 1"),
		}).Error
}

func (s *DeviceStore) GetActivePosition(deviceID uuid.UUID) (*model.RackDevicePosition, error) {
	var p model.RackDevicePosition
	err := s.db.First(&p, "device_id = ? AND deleted_at IS NULL", deviceID).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *DeviceStore) ListOccupancies(rackID uuid.UUID) ([]model.RackUOccupancy, error) {
	var items []model.RackUOccupancy
	err := s.db.Where("rack_id = ? AND deleted_at IS NULL", rackID).Order("start_u asc").Find(&items).Error
	return items, err
}

func (s *DeviceStore) ListHistories(deviceID uuid.UUID) ([]model.DevicePositionHistory, error) {
	var items []model.DevicePositionHistory
	err := s.db.Where("device_id = ?", deviceID).Order("created_at desc").Find(&items).Error
	return items, err
}

// PlaceInRack creates position+occupancy atomically and soft-deletes any previous active position.
func (s *DeviceStore) PlaceInRack(pos *model.RackDevicePosition, occupancy *model.RackUOccupancy) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// release previous
		var old model.RackDevicePosition
		err := tx.Unscoped().First(&old, "device_id = ? AND deleted_at IS NULL", pos.DeviceID).Error
		if err == nil {
			if err := tx.Model(&model.RackDevicePosition{}).
				Where("id = ?", old.ID).
				Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.RackUOccupancy{}).
				Where("position_id = ?", old.ID).
				Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(pos).Error; err != nil {
			return err
		}
		occupancy.PositionID = pos.ID
		occupancy.DeviceID = pos.DeviceID
		occupancy.RackID = pos.RackID
		occupancy.StartU = pos.StartU
		occupancy.EndU = pos.EndU
		return tx.Create(occupancy).Error
	})
}

func (s *DeviceStore) RemoveFromRack(deviceID uuid.UUID) (*model.RackDevicePosition, error) {
	var old model.RackDevicePosition
	err := s.db.First(&old, "device_id = ? AND deleted_at IS NULL", deviceID).Error
	if err != nil {
		return nil, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RackDevicePosition{}).
			Where("id = ?", old.ID).
			Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		return tx.Model(&model.RackUOccupancy{}).
			Where("position_id = ?", old.ID).
			Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")}).Error
	})
	if err != nil {
		return nil, err
	}
	return &old, nil
}

func (s *DeviceStore) WriteHistory(h *model.DevicePositionHistory) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	return s.db.Create(h).Error
}

func (s *DeviceStore) SeedDeviceTypesIfEmpty() error {
	var n int64
	if err := s.db.Model(&model.DeviceType{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	seed := []model.DeviceType{
		{Code: "SERVER", Name: "服务器", Category: "SERVER", DefaultHeightU: 2, SortOrder: 10},
		{Code: "NETWORK", Name: "网络设备", Category: "NETWORK", DefaultHeightU: 1, SortOrder: 20},
		{Code: "STORAGE", Name: "存储设备", Category: "STORAGE", DefaultHeightU: 2, SortOrder: 30},
		{Code: "SECURITY", Name: "安全设备", Category: "SECURITY", DefaultHeightU: 1, SortOrder: 40},
		{Code: "POWER-ENV", Name: "配电与环境设备", Category: "POWER_ENVIRONMENT", DefaultHeightU: 1, SortOrder: 50},
		{Code: "ACCESSORY", Name: "机柜配件", Category: "ACCESSORY", DefaultHeightU: 1, SortOrder: 60},
	}
	for i := range seed {
		seed[i].Status = "ACTIVE"
		if err := s.db.Create(&seed[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func IsDeviceTypeInUse(err error) bool {
	return err != nil && err.Error() == "device type in use"
}

func IsExclusionViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "rack_u_occupancies_no_overlap") ||
		strings.Contains(msg, "23P01") ||
		strings.Contains(msg, "exclusion")
}
