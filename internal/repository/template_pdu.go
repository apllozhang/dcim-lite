package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	if err := s.Create(&t, &v); err != nil {
		// 多副本同时冷启动：对方已写入系统模板则唯一索引冲突，视为已存在
		if IsUniqueViolation(err) {
			return nil
		}
		return err
	}
	return nil
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

// GetPDULock 读取 PDU 并加行锁（FOR UPDATE）。Connect/Delete/ForceArchive 共用
// 该串行化点：连接操作不更新 PDU version，单靠乐观锁挡不住"确认影响清单后并发
// 新增连接"，归档会把用户未确认的连接一并断开。
func (s *PDUStore) GetPDULock(id uuid.UUID) (*model.PDU, error) {
	var item model.PDU
	err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error
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
		return s.WithTx(tx).SoftDeletePDUWithSocketsTx(id, expected)
	})
}

// SoftDeletePDUWithSocketsTx 在调用方事务内执行级联软删（不开新事务），
// 供服务层把行锁、连接复核与删除放进同一事务。
func (s *PDUStore) SoftDeletePDUWithSocketsTx(id uuid.UUID, expected uint) error {
	res := s.db.Model(&model.PDU{}).
		Where("id = ? AND version = ?", id, expected).
		Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.Model(&model.PDUSocket{}).
		Where("pdu_id = ?", id).
		Updates(map[string]any{"deleted_at": time.Now(), "version": gorm.Expr("version + 1")}).
		Error
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

// PDUImpactRow 描述一条受影响的供电连接（供强制归档的影响清单）。
type PDUImpactRow struct {
	ConnectionID   uuid.UUID `json:"connectionId"`
	SocketNo       int       `json:"socketNo"`
	DeviceID       uuid.UUID `json:"deviceId"`
	DeviceCode     string    `json:"deviceCode"`
	DeviceName     string    `json:"deviceName"`
	RedundancyRole string    `json:"redundancyRole"`
}

// PDUImpact 统计 PDU 的影响面：插座数、活动连接数及其明细。
func (s *PDUStore) PDUImpact(pduID uuid.UUID) (int64, []PDUImpactRow, error) {
	var sockets int64
	if err := s.db.Model(&model.PDUSocket{}).Where("pdu_id = ?", pduID).Count(&sockets).Error; err != nil {
		return 0, nil, err
	}
	var rows []PDUImpactRow
	err := s.db.Table("pdu_connections c").
		Select("c.id AS connection_id, so.socket_no, c.device_id, d.code AS device_code, "+
			"d.name AS device_name, c.redundancy_role").
		Joins("JOIN pdu_sockets so ON so.id = c.socket_id AND so.deleted_at IS NULL").
		Joins("JOIN devices d ON d.id = c.device_id").
		Where("so.pdu_id = ? AND c.deleted_at IS NULL", pduID).
		Order("so.socket_no asc").
		Scan(&rows).Error
	return sockets, rows, err
}

// ForceArchivePDU 管理员强制归档：断开全部连接、下线插座、归档 PDU（单事务）。
// 仅在调用方已确认影响清单后使用（服务层强制二次确认）。
func (s *PDUStore) ForceArchivePDU(id uuid.UUID, expected uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return s.WithTx(tx).ForceArchivePDUTx(id, expected)
	})
}

// ForceArchivePDUTx 在调用方事务内执行强制归档（不开新事务），
// 供服务层把行锁、影响清单复检、归档与审计放进同一事务。
func (s *PDUStore) ForceArchivePDUTx(id uuid.UUID, expected uint) error {
	now := time.Now()
	// 子查询不过滤已删插座：归档要连"挂在已软删插座上的残留连接"一并清掉
	if err := s.db.Model(&model.PDUConnection{}).
		Where("socket_id IN (SELECT id FROM pdu_sockets WHERE pdu_id = ?)", id).
		Updates(map[string]any{"deleted_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.PDUSocket{}).Where("pdu_id = ?", id).
		Updates(map[string]any{"deleted_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
		return err
	}
	res := s.db.Model(&model.PDU{}).
		Where("id = ? AND version = ?", id, expected).
		Updates(map[string]any{"deleted_at": now, "version": gorm.Expr("version + 1")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
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

// GetSocketLock 读取插座并加行锁（FOR UPDATE）。PDU 聚合统一锁序为
// PDU 行 → socket 行（见 service 层 CreateSocket/UpdateSocket/DeleteSocket/
// Connect/Disconnect），全部 socket 级写路径都必须先持有其 PDU 行锁。
func (s *PDUStore) GetSocketLock(id uuid.UUID) (*model.PDUSocket, error) {
	var item model.PDUSocket
	err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error
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

// SoftDeleteSocket 乐观锁软删插座。已连接检查不在本方法内：调用方必须在
// PDU 行锁内经 CountActiveConnectionsBySocket 确认无活动连接后再调用，
// 否则与 Connect 之间存在"计数为零后并发接入"的竞态窗口。
func (s *PDUStore) SoftDeleteSocket(id uuid.UUID, expected uint) error {
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

// CountActiveConnectionsBySocket 统计插座上的活动连接数（供 DeleteSocket 在锁内复核）。
func (s *PDUStore) CountActiveConnectionsBySocket(socketID uuid.UUID) (int64, error) {
	var n int64
	err := s.db.Model(&model.PDUConnection{}).
		Where("socket_id = ?", socketID).Count(&n).Error
	return n, err
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

// UpdateSocketStatus 由 Connect/Disconnect 在 PDU→socket 行锁内调用，维护
// socket 缓存状态。零行更新（插座已被并发删除/归档）必须报错回滚，防止
// 事务在"连接指向已删插座"的不一致态下提交。
func (s *PDUStore) UpdateSocketStatus(id uuid.UUID, status string) error {
	res := s.db.Model(&model.PDUSocket{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *PDUStore) DeviceHasRoleConnection(deviceID uuid.UUID, role string) (bool, error) {
	var n int64
	err := s.db.Model(&model.PDUConnection{}).
		Where("device_id = ? AND redundancy_role = ?", deviceID, role).Count(&n).Error
	return n > 0, err
}

var (
	errHasChildren     = &bizErr{"RESOURCE_HAS_CHILDREN"}
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
