package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/model"
)

type TemplateStore struct{ db *gorm.DB }

func NewTemplateStore(db *gorm.DB) *TemplateStore { return &TemplateStore{db: db} }

func (s *TemplateStore) DB() *gorm.DB { return s.db }

func (s *TemplateStore) List() ([]model.RackTemplate, error) {
	var items []model.RackTemplate
	err := s.db.Preload("Versions", func(db *gorm.DB) *gorm.DB {
		return db.Order("revision asc")
	}).Order("created_at asc").Find(&items).Error
	return items, err
}

func (s *TemplateStore) Get(id uuid.UUID) (*model.RackTemplate, error) {
	var item model.RackTemplate
	err := s.db.Preload("Versions", func(db *gorm.DB) *gorm.DB {
		return db.Order("revision asc")
	}).First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *TemplateStore) Create(t *model.RackTemplate, v *model.RackTemplateVersion) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		v.TemplateID = t.ID
		v.Revision = 1
		return tx.Create(v).Error
	})
}

func (s *TemplateStore) UpdateMeta(t *model.RackTemplate) error {
	res := s.db.Model(&model.RackTemplate{}).
		Where("id = ? AND version = ?", t.ID, t.Version).
		Updates(map[string]any{
			"name": t.Name, "description": t.Description, "status": t.Status,
			"remarks": t.Remarks, "version": gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(t, "id = ?", t.ID).Error
}

func (s *TemplateStore) SoftDelete(id uuid.UUID, expected uint) error {
	res := s.db.Model(&model.RackTemplate{}).
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

func (s *TemplateStore) CreateVersion(t *model.RackTemplate, v *model.RackTemplateVersion) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.RackTemplate{}).
			Where("id = ? AND version = ?", t.ID, t.Version).
			Updates(map[string]any{
				"current_revision": v.Revision,
				"version":          gorm.Expr("version + 1"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errVersion()
		}
		v.TemplateID = t.ID
		return tx.Create(v).Error
	})
}

func (s *TemplateStore) SeedSystemTemplateIfEmpty() error {
	var n int64
	if err := s.db.Model(&model.RackTemplate{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	t := model.RackTemplate{
		Code: "STANDARD-42U", Name: "标准 42U 机柜",
		Description: "系统内置的 42U 标准机柜模板",
		Status:      model.TemplateActive, IsSystem: true, CurrentRevision: 1,
	}
	v := model.RackTemplateVersion{
		Revision: 1, Type: "STANDARD", UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000,
		ChangeNote: "系统初始化版本",
	}
	return s.Create(&t, &v)
}

type PDUStore struct{ db *gorm.DB }

func NewPDUStore(db *gorm.DB) *PDUStore { return &PDUStore{db: db} }

func (s *PDUStore) WithTx(tx *gorm.DB) *PDUStore { return &PDUStore{db: tx} }

func (s *PDUStore) DB() *gorm.DB { return s.db }

func (s *PDUStore) ListByRack(rackID uuid.UUID) ([]model.PDU, error) {
	var items []model.PDU
	err := s.db.Where("rack_id = ?", rackID).Order("code asc").Find(&items).Error
	return items, err
}

func (s *PDUStore) GetPDU(id uuid.UUID) (*model.PDU, error) {
	var item model.PDU
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PDUStore) CreatePDU(item *model.PDU) error {
	return s.db.Create(item).Error
}

func (s *PDUStore) UpdatePDU(item *model.PDU, expected uint) error {
	res := s.db.Model(&model.PDU{}).
		Where("id = ? AND version = ?", item.ID, expected).
		Updates(map[string]any{
			"code": item.Code, "name": item.Name, "manufacturer": item.Manufacturer,
			"model_number": item.ModelNumber, "serial_number": item.SerialNumber,
			"input_voltage": item.InputVoltage, "rated_power_w": item.RatedPowerW,
			"rated_current_a": item.RatedCurrentA, "status": item.Status, "remarks": item.Remarks,
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

// CountActiveConnectionsByPDU 统计该 PDU 名下插座上的活动连接数。
// 这是删除保护的真实边界：插座是 PDU 的构成部分，连接才是跨实体的业务事实。
func (s *PDUStore) CountActiveConnectionsByPDU(pduID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Model(&model.PDUConnection{}).
		Where(`EXISTS (SELECT 1 FROM pdu_sockets so
			WHERE so.id = pdu_connections.socket_id AND so.pdu_id = ? AND so.deleted_at IS NULL)`, pduID).
		Count(&n).Error
	return n, err
}

// SoftDeletePDUWithSockets 级联软删 PDU 及其插座（插座随 PDU 一起下线）。
// 调用方必须已确认无活动连接，否则会产生「设备由已删除 PDU 供电」的孤儿记录。
func (s *PDUStore) SoftDeletePDUWithSockets(id uuid.UUID, expected uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.PDU{}).
			Where("id = ? AND version = ?", id, expected).
			Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errVersion()
		}
		return tx.Model(&model.PDUSocket{}).
			Where("pdu_id = ?", id).
			Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")}).
			Error
	})
}

func (s *PDUStore) CodeExistsPDU(code string, exclude uuid.UUID) (bool, error) {
	var n int64
	q := s.db.Model(&model.PDU{}).Where("code = ?", code)
	if exclude != uuid.Nil {
		q = q.Where("id <> ?", exclude)
	}
	err := q.Count(&n).Error
	return n > 0, err
}

func (s *PDUStore) ListSockets(pduID uuid.UUID) ([]model.PDUSocket, error) {
	var items []model.PDUSocket
	err := s.db.Where("pdu_id = ?", pduID).Order("socket_no asc").Find(&items).Error
	return items, err
}

func (s *PDUStore) GetSocket(id uuid.UUID) (*model.PDUSocket, error) {
	var item model.PDUSocket
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PDUStore) CreateSocket(item *model.PDUSocket) error {
	return s.db.Create(item).Error
}

func (s *PDUStore) UpdateSocket(item *model.PDUSocket, expected uint) error {
	res := s.db.Model(&model.PDUSocket{}).
		Where("id = ? AND version = ?", item.ID, expected).
		Updates(map[string]any{
			"socket_no": item.SocketNo, "standard": item.Standard,
			"amperage_a": item.AmperageA, "label": item.Label, "status": item.Status,
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

func (s *PDUStore) SoftDeleteSocket(id uuid.UUID, expected uint) error {
	var n int64
	if err := s.db.Model(&model.PDUConnection{}).Where("socket_id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return errSocketConnected
	}
	res := s.db.Model(&model.PDUSocket{}).
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

func (s *PDUStore) ListConnectionsByRack(rackID uuid.UUID) ([]model.PDUConnection, error) {
	var items []model.PDUConnection
	err := s.db.Where(`EXISTS (
		SELECT 1 FROM pdu_sockets so JOIN pdus p ON p.id = so.pdu_id
		WHERE so.id = pdu_connections.socket_id AND p.rack_id = ? AND so.deleted_at IS NULL AND p.deleted_at IS NULL
	)`, rackID).Order("connected_at asc").Find(&items).Error
	return items, err
}

func (s *PDUStore) GetConnection(id uuid.UUID) (*model.PDUConnection, error) {
	var item model.PDUConnection
	err := s.db.First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PDUStore) CreateConnection(item *model.PDUConnection) error {
	return s.db.Create(item).Error
}

func (s *PDUStore) SoftDeleteConnection(id uuid.UUID, expected uint) error {
	res := s.db.Model(&model.PDUConnection{}).
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

func (s *PDUStore) UpdateSocketStatus(id uuid.UUID, status string) error {
	return s.db.Model(&model.PDUSocket{}).Where("id = ?", id).Update("status", status).Error
}

func (s *PDUStore) DeviceHasRoleConnection(deviceID uuid.UUID, role string) (bool, error) {
	var n int64
	err := s.db.Model(&model.PDUConnection{}).
		Where("device_id = ? AND redundancy_role = ?", deviceID, role).Count(&n).Error
	return n > 0, err
}

var (
	errHasChildren     = &bizErr{"RESOURCE_HAS_CHILDREN"}
	errSocketConnected = &bizErr{"PDU_SOCKET_CONNECTED"}
	errPDURackMismatch = &bizErr{"PDU_DEVICE_RACK_MISMATCH"}
	errSocketBusy      = &bizErr{"SOCKET_UNAVAILABLE"}
)

type bizErr struct{ code string }

func (e *bizErr) Error() string { return e.code }

func IsBizCode(err error, code string) bool {
	if be, ok := err.(*bizErr); ok {
		return be.code == code
	}
	return false
}

func PDUDeviceRackMismatch() error { return errPDURackMismatch }
func SocketUnavailable() error     { return errSocketBusy }
