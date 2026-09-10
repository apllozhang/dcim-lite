package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

type TemplateService struct {
	store *repository.TemplateStore
	devs  *repository.DeviceStore
	acks  *repository.ResourceStore
}

func NewTemplateService(store *repository.TemplateStore, acks *repository.ResourceStore) *TemplateService {
	return &TemplateService{store: store, acks: acks}
}

type TemplateVersionInput struct {
	Type           string   `json:"type"`
	Manufacturer   string   `json:"manufacturer"`
	ModelNumber    string   `json:"modelNumber"`
	UHeight        *int     `json:"uHeight"`
	WidthMm        *int     `json:"widthMm"`
	DepthMm        *int     `json:"depthMm"`
	HeightMm       *int     `json:"heightMm"`
	LoadCapacityKg *float64 `json:"loadCapacityKg"`
	DualPower      *bool    `json:"dualPower"`
	InputCircuits  *int     `json:"inputCircuits"`
	RatedVoltage   *float64 `json:"ratedVoltage"`
	RatedCurrent   *float64 `json:"ratedCurrent"`
	RatedPowerKw   *float64 `json:"ratedPowerKw"`
	PeakPowerKw    *float64 `json:"peakPowerKw"`
	PDUCount       *int     `json:"pduCount"`
	ChangeNote     string   `json:"changeNote"`
}

type TemplateCreateInput struct {
	Code        string               `json:"code" binding:"required,max=50"`
	Name        string               `json:"name" binding:"required,max=150"`
	Description string               `json:"description"`
	Status      string               `json:"status"`
	Remarks     string               `json:"remarks"`
	Version     TemplateVersionInput `json:"version"`
}

type TemplateUpdateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Remarks     string `json:"remarks"`
}

func (s *TemplateService) List() ([]model.RackTemplate, error) {
	return s.store.List()
}

func (s *TemplateService) Create(in TemplateCreateInput, actor *uuid.UUID) (*model.RackTemplate, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Status == "" {
		in.Status = model.TemplateActive
	}
	v := applyVersionDefaults(in.Version, actor)
	t := &model.RackTemplate{
		Code: in.Code, Name: in.Name, Description: in.Description,
		Status: in.Status, CurrentRevision: 1, Remarks: in.Remarks,
	}
	if err := s.store.Create(t, &v); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return s.store.Get(t.ID)
}

func (s *TemplateService) Update(id uuid.UUID, in TemplateUpdateInput) (*model.RackTemplate, error) {
	t, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜模板")
		}
		return nil, err
	}
	// 厂商基线要求模板名称非空且 <=150（S16-TPL-UPD-EMPTY-NAME 差分用例）
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 150 {
		return nil, apperr.InvalidResource("模板名称不能为空且不能超过 150 个字符")
	}
	// 系统内置模板不可停用（厂商返回 409 SYSTEM_TEMPLATE_PROTECTED；名称校验先于本检查）
	if t.IsSystem && in.Status == model.TemplateDisabled {
		return nil, apperr.New(409, "SYSTEM_TEMPLATE_PROTECTED",
			"system template cannot be deleted or disabled: 系统内置模板不能停用")
	}
	t.Name = name
	t.Description = in.Description
	t.Remarks = in.Remarks
	if in.Status != "" {
		t.Status = in.Status
	}
	// version 从 body 或已加载对象；前端 PUT 不带 query version，用当前 version
	if err := s.store.UpdateMeta(t); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.Get(id)
}

func (s *TemplateService) Delete(id uuid.UUID, version uint) error {
	t, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("机柜模板")
		}
		return err
	}
	if t.IsSystem {
		return apperr.New(409, "SYSTEM_TEMPLATE_PROTECTED", "系统内置模板不可删除")
	}
	return mapStoreErr(s.store.SoftDelete(id, version))
}

func (s *TemplateService) CreateVersion(id uuid.UUID, version uint, in TemplateVersionInput, actor *uuid.UUID) (*model.RackTemplate, error) {
	t, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜模板")
		}
		return nil, err
	}
	if t.Status != model.TemplateActive {
		return nil, apperr.New(409, "TEMPLATE_DISABLED", "模板已停用，不能新建版本")
	}
	rev := t.CurrentRevision + 1
	v := applyVersionDefaults(in, actor)
	v.Revision = rev
	// 使用请求中的 version（query）
	if err := s.store.CreateVersion(t, &v); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.Get(id)
}

func applyVersionDefaults(in TemplateVersionInput, actor *uuid.UUID) model.RackTemplateVersion {
	v := model.RackTemplateVersion{
		Type: "STANDARD", UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000,
		Manufacturer: in.Manufacturer, ModelNumber: in.ModelNumber,
		LoadCapacityKg: in.LoadCapacityKg, RatedVoltage: in.RatedVoltage,
		RatedCurrent: in.RatedCurrent, RatedPowerKw: in.RatedPowerKw,
		PeakPowerKw: in.PeakPowerKw, ChangeNote: in.ChangeNote, CreatedBy: actor,
	}
	if in.Type != "" {
		v.Type = in.Type
	}
	if in.UHeight != nil {
		v.UHeight = *in.UHeight
	}
	if in.WidthMm != nil {
		v.WidthMm = *in.WidthMm
	}
	if in.DepthMm != nil {
		v.DepthMm = *in.DepthMm
	}
	if in.HeightMm != nil {
		v.HeightMm = *in.HeightMm
	}
	if in.DualPower != nil {
		v.DualPower = *in.DualPower
	}
	if in.InputCircuits != nil {
		v.InputCircuits = *in.InputCircuits
	}
	if in.PDUCount != nil {
		v.PDUCount = *in.PDUCount
	}
	return v
}

// ApplyTemplateToRack fills rack fields + snapshot from a template version.
func ApplyTemplateToRack(t *model.RackTemplate, ver *model.RackTemplateVersion, rack *model.Rack) {
	if t == nil || ver == nil || rack == nil {
		return
	}
	rack.TemplateID = &t.ID
	vid := ver.ID
	rack.TemplateVersionID = &vid
	rack.TemplateCode = t.Code
	rack.TemplateName = t.Name
	rack.TemplateRevision = ver.Revision
	rack.TemplateSnapshot = &model.RackTemplateSnapshot{
		TemplateID: t.ID, VersionID: ver.ID,
		Code: t.Code, Name: t.Name, Revision: ver.Revision,
		Type: ver.Type, UHeight: ver.UHeight, WidthMm: ver.WidthMm,
		DepthMm: ver.DepthMm, HeightMm: ver.HeightMm, DualPower: ver.DualPower,
		PDUCount: ver.PDUCount,
	}
	rack.Type = ver.Type
	rack.Manufacturer = ver.Manufacturer
	rack.ModelNumber = ver.ModelNumber
	rack.UHeight = ver.UHeight
	rack.WidthMm = ver.WidthMm
	rack.DepthMm = ver.DepthMm
	rack.HeightMm = ver.HeightMm
	rack.LoadCapacityKg = ver.LoadCapacityKg
	rack.DualPower = ver.DualPower
	rack.InputCircuits = ver.InputCircuits
	rack.RatedVoltage = ver.RatedVoltage
	rack.RatedCurrent = ver.RatedCurrent
	rack.RatedPowerKw = ver.RatedPowerKw
	rack.PeakPowerKw = ver.PeakPowerKw
	rack.PDUCount = ver.PDUCount
}

type PDUService struct {
	store *repository.PDUStore
	acks  *repository.ResourceStore
	devs  *repository.DeviceStore
	tmpl  *repository.TemplateStore
}

func NewPDUService(store *repository.PDUStore, acks *repository.ResourceStore, devs *repository.DeviceStore) *PDUService {
	return &PDUService{store: store, acks: acks, devs: devs}
}

type PDUInput struct {
	Code          string   `json:"code" binding:"required,max=80"`
	Name          string   `json:"name" binding:"required,max=150"`
	Manufacturer  string   `json:"manufacturer"`
	ModelNumber   string   `json:"modelNumber"`
	SerialNumber  string   `json:"serialNumber"`
	InputVoltage  *float64 `json:"inputVoltage"`
	RatedPowerW   *float64 `json:"ratedPowerW"`
	RatedCurrentA *float64 `json:"ratedCurrentA"`
	Status        string   `json:"status"`
	Remarks       string   `json:"remarks"`
}

type SocketInput struct {
	SocketNo  int    `json:"socketNo" binding:"required"`
	Standard  string `json:"standard" binding:"required"`
	AmperageA int    `json:"amperageA" binding:"required"`
	Label     string `json:"label"`
	Status    string `json:"status"`
}

type ConnectionInput struct {
	DeviceID       uuid.UUID `json:"deviceId" binding:"required"`
	PowerW         *float64  `json:"powerW"`
	Circuit        string    `json:"circuit"`
	RedundancyRole string    `json:"redundancyRole"`
}

func (s *PDUService) ListByRack(rackID uuid.UUID) ([]model.PDU, error) {
	if _, err := s.acks.GetRack(rackID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	return s.store.ListByRack(rackID)
}

func (s *PDUService) Create(rackID uuid.UUID, in PDUInput) (*model.PDU, error) {
	var created *model.PDU
	err := s.store.DB().Transaction(func(tx *gorm.DB) error {
		store := s.store.WithTx(tx)
		acks := s.acks.WithTx(tx)
		// 锁 rack 行：与机柜删除保护共用串行化点，杜绝软删机柜下新增 PDU
		if _, err := acks.GetRackLock(rackID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("机柜")
			}
			return err
		}
		if in.Status == "" {
			in.Status = model.PDUActive
		}
		exists, err := store.CodeExistsPDU(in.Code, uuid.Nil)
		if err != nil {
			return err
		}
		if exists {
			return apperr.DuplicateCode()
		}
		item := &model.PDU{
			RackID: rackID, Code: strings.TrimSpace(in.Code), Name: strings.TrimSpace(in.Name),
			Manufacturer: in.Manufacturer, ModelNumber: in.ModelNumber, SerialNumber: in.SerialNumber,
			InputVoltage: in.InputVoltage, RatedPowerW: in.RatedPowerW, RatedCurrentA: in.RatedCurrentA,
			Status: in.Status, Remarks: in.Remarks,
		}
		if err := store.CreatePDU(item); err != nil {
			if repository.IsUniqueViolation(err) {
				// D1: 旧 500 → 409 DUPLICATE_CODE
				return apperr.DuplicateCode()
			}
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *PDUService) Update(id uuid.UUID, in PDUInput) (*model.PDU, error) {
	existing, err := s.store.GetPDU(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("PDU")
		}
		return nil, err
	}
	if in.Status == "" {
		in.Status = existing.Status
	}
	next := *existing
	next.Code, next.Name = strings.TrimSpace(in.Code), strings.TrimSpace(in.Name)
	next.Manufacturer, next.ModelNumber, next.SerialNumber = in.Manufacturer, in.ModelNumber, in.SerialNumber
	next.InputVoltage, next.RatedPowerW, next.RatedCurrentA = in.InputVoltage, in.RatedPowerW, in.RatedCurrentA
	next.Status, next.Remarks = in.Status, in.Remarks
	if err := s.store.UpdatePDU(&next, existing.Version); err != nil {
		return nil, mapStoreErr(err)
	}
	return &next, nil
}

func (s *PDUService) Delete(id uuid.UUID, version uint) error {
	return s.store.DB().Transaction(func(tx *gorm.DB) error {
		store := s.store.WithTx(tx)
		// 删除边界画在「连接」上（见 docs/COMPAT-DECISIONS.md 的 D5 决策）：
		// 插座是 PDU 的构成部分，随 PDU 一起下线；只要还有设备在取电就拒绝，
		// 避免台账出现「由已删除 PDU 供电」的记录。
		// 锁 PDU 行后再计数：与 Connect 串行化，杜绝"计数为零后并发接入新连接"。
		if _, err := store.GetPDULock(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("PDU")
			}
			return err
		}
		conns, err := store.CountActiveConnectionsByPDU(id)
		if err != nil {
			return err
		}
		if conns > 0 {
			return apperr.New(409, "PDU_IN_USE",
				fmt.Sprintf("该 PDU 正在为设备供电（%d 条连接），请先断开连接后再删除", conns))
		}
		return mapStoreErr(store.SoftDeletePDUWithSocketsTx(id, version))
	})
}

// PDUImpactDevice 影响清单里的设备条目。
type PDUImpactDevice struct {
	DeviceID       uuid.UUID `json:"deviceId"`
	DeviceCode     string    `json:"deviceCode"`
	DeviceName     string    `json:"deviceName"`
	SocketNo       int       `json:"socketNo"`
	RedundancyRole string    `json:"redundancyRole"`
}

// PDUImpactResult 强制归档的影响清单（先展示、后确认）。
type PDUImpactResult struct {
	PDUID       uuid.UUID         `json:"pduId"`
	PDUCode     string            `json:"pduCode"`
	PDUName     string            `json:"pduName"`
	Sockets     int64             `json:"sockets"`
	Connections int64             `json:"connections"`
	Devices     []PDUImpactDevice `json:"devices"`
	Warning     string            `json:"warning,omitempty"`
}

// Impact 返回强制归档的影响清单（不含写操作）。
func (s *PDUService) Impact(id uuid.UUID) (*PDUImpactResult, error) {
	pdu, err := s.store.GetPDU(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("PDU")
		}
		return nil, err
	}
	return s.impactOf(s.store, pdu)
}

func (s *PDUService) impactOf(store *repository.PDUStore, pdu *model.PDU) (*PDUImpactResult, error) {
	sockets, rows, err := store.PDUImpact(pdu.ID)
	if err != nil {
		return nil, err
	}
	res := &PDUImpactResult{
		PDUID: pdu.ID, PDUCode: pdu.Code, PDUName: pdu.Name,
		Sockets: sockets, Connections: int64(len(rows)),
		Devices: make([]PDUImpactDevice, 0, len(rows)),
	}
	for _, row := range rows {
		res.Devices = append(res.Devices, PDUImpactDevice{
			DeviceID: row.DeviceID, DeviceCode: row.DeviceCode, DeviceName: row.DeviceName,
			SocketNo: row.SocketNo, RedundancyRole: row.RedundancyRole,
		})
	}
	if res.Connections > 0 {
		res.Warning = "该 PDU 正在为设备供电，强制归档会断开这些连接并可能造成配电记录失真，请确认已完成现场处置"
	}
	return res, nil
}

// ForceArchiveInput 强制归档入参：必须回报影响清单中的连接数并填写原因。
type ForceArchiveInput struct {
	Version            uint   `json:"version"`
	Reason             string `json:"reason"`
	ConfirmConnections int    `json:"confirmConnections"`
}

// ForceArchive 管理员强制归档 PDU：二次确认（须回报连接数）+ 必填原因 + 审计留痕。
// 用于「PDU 报废但设备仍在用」的现场处置场景（见 docs/COMPAT-DECISIONS.md D5）。
// 确认校验在 PDU 行锁内以重读的影响清单为准：客户端从 Impact 接口拿到的连接数
// 若已过期（期间并发接入了新连接），提交会被 409 拒绝，绝不归档未确认的连接。
func (s *PDUService) ForceArchive(id uuid.UUID, in ForceArchiveInput, actor *uuid.UUID, requestID string) (*PDUImpactResult, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return nil, apperr.InvalidResource("强制归档必须填写原因")
	}
	var result *PDUImpactResult
	err := s.store.DB().Transaction(func(tx *gorm.DB) error {
		store := s.store.WithTx(tx)
		pdu, err := store.GetPDULock(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("PDU")
			}
			return err
		}
		// 锁内重读影响清单：Connect 也锁同一行，从复检到归档执行之间不可能新增连接
		impact, err := s.impactOf(store, pdu)
		if err != nil {
			return err
		}
		if in.ConfirmConnections != int(impact.Connections) {
			return apperr.New(409, "IMPACT_CONFIRMATION_REQUIRED",
				fmt.Sprintf("影响清单已变化：该 PDU 现有 %d 条活动连接，请以 confirmConnections=%d 重新获取并提交",
					impact.Connections, impact.Connections))
		}
		if err := store.ForceArchivePDUTx(id, in.Version); err != nil {
			return err
		}
		// 详细审计与归档同事务：合规敏感动作不做 best-effort，审计写失败则归档一并回滚
		if err := s.writeArchiveAudit(tx, pdu, impact, in, actor, requestID); err != nil {
			return err
		}
		result = impact
		return nil
	})
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return result, nil
}

// writeArchiveAudit 强制归档留痕：影响清单 + 原因写入 audit_logs（在归档事务内执行）。
func (s *PDUService) writeArchiveAudit(tx *gorm.DB, pdu *model.PDU, impact *PDUImpactResult,
	in ForceArchiveInput, actor *uuid.UUID, requestID string) error {
	payload, _ := json.Marshal(map[string]any{
		"pduCode":     pdu.Code,
		"reason":      in.Reason,
		"sockets":     impact.Sockets,
		"connections": impact.Connections,
		"devices":     impact.Devices,
	})
	after := string(payload)
	pid := pdu.ID
	return s.acks.WithTx(tx).WriteAudit(&model.AuditLog{
		UserID: actor, Action: "FORCE_ARCHIVE", ResourceType: "pdu", ResourceID: &pid,
		RequestID: requestID, AfterJSON: &after, Result: "SUCCESS", Source: "api",
	})
}

func (s *PDUService) ListSockets(pduID uuid.UUID) ([]model.PDUSocket, error) {
	if _, err := s.store.GetPDU(pduID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("PDU")
		}
		return nil, err
	}
	return s.store.ListSockets(pduID)
}

// validSocketStandard 插座制式：厂商仅接受 CN/EU（差分用例 S10-SOCKET-BAD-STD、S15-SOCKET-PUT-EMPTY-STD）。
func validSocketStandard(v string) bool {
	v = strings.TrimSpace(v)
	return v == "CN" || v == "EU"
}

func (s *PDUService) CreateSocket(pduID uuid.UUID, in SocketInput) (*model.PDUSocket, error) {
	if _, err := s.store.GetPDU(pduID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("PDU")
		}
		return nil, err
	}
	if !validSocketStandard(in.Standard) {
		return nil, apperr.InvalidResource("插座制式必须为 CN 或 EU")
	}
	if in.Status == "" {
		in.Status = model.SocketAvailable
	}
	item := &model.PDUSocket{
		PDUID: pduID, SocketNo: in.SocketNo, Standard: in.Standard,
		AmperageA: in.AmperageA, Label: in.Label, Status: in.Status,
	}
	if err := s.store.CreateSocket(item); err != nil {
		if repository.IsUniqueViolation(err) {
			// D2: 旧 500 → 409
			return nil, apperr.New(409, "RESOURCE_CODE_DUPLICATE", "插座编号已存在")
		}
		return nil, err
	}
	return item, nil
}

func (s *PDUService) UpdateSocket(id uuid.UUID, in SocketInput) (*model.PDUSocket, error) {
	existing, err := s.store.GetSocket(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("插座")
		}
		return nil, err
	}
	if in.Status == "" {
		in.Status = existing.Status
	}
	if !validSocketStandard(in.Standard) {
		return nil, apperr.InvalidResource("插座制式必须为 CN 或 EU")
	}
	next := *existing
	next.SocketNo, next.Standard, next.AmperageA = in.SocketNo, in.Standard, in.AmperageA
	next.Label, next.Status = in.Label, in.Status
	if err := s.store.UpdateSocket(&next, existing.Version); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.New(409, "RESOURCE_CODE_DUPLICATE", "插座编号已存在")
		}
		return nil, mapStoreErr(err)
	}
	return &next, nil
}

func (s *PDUService) DeleteSocket(id uuid.UUID, version uint) error {
	if _, err := s.store.GetSocket(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("插座")
		}
		return err
	}
	if err := s.store.SoftDeleteSocket(id, version); err != nil {
		if repository.IsBizCode(err, "PDU_SOCKET_CONNECTED") {
			// 契约：已连接阻止删除
			return apperr.New(409, "PDU_SOCKET_CONNECTED", "插座已连接设备，无法删除")
		}
		return mapStoreErr(err)
	}
	return nil
}

func (s *PDUService) ListConnections(rackID uuid.UUID) ([]model.PDUConnection, error) {
	if _, err := s.acks.GetRack(rackID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	return s.store.ListConnectionsByRack(rackID)
}

func (s *PDUService) Connect(socketID uuid.UUID, in ConnectionInput, actor *uuid.UUID) (*model.PDUConnection, error) {
	var result *model.PDUConnection
	err := s.store.DB().Transaction(func(tx *gorm.DB) error {
		store := s.store.WithTx(tx)
		sock, err := store.GetSocket(socketID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("插座")
			}
			return err
		}
		if sock.Status == model.SocketConnected {
			return apperr.New(409, "PDU_SOCKET_CONNECTED", "插座已被占用")
		}
		// 锁 PDU 行：与 Delete/ForceArchive 串行化——归档/删除的确认与执行
		// 都发生在锁内，这里的新增连接不可能"逃过"它们的事务内复检
		pdu, err := store.GetPDULock(sock.PDUID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("PDU")
			}
			return err
		}
		dev, err := s.devs.GetDevice(in.DeviceID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("设备")
			}
			return err
		}
		pos, err := s.devs.GetActivePosition(dev.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.New(409, "DEVICE_NOT_POSITIONED", "设备未上架，无法接电")
			}
			return err
		}
		if pos.RackID != pdu.RackID {
			// D3 相关：PDU 与设备必须同机柜
			return apperr.New(409, "PDU_DEVICE_RACK_MISMATCH", "PDU 与设备不在同一机柜")
		}
		role := model.RolePrimary
		if in.RedundancyRole != "" {
			if in.RedundancyRole != model.RolePrimary && in.RedundancyRole != model.RoleStandby {
				return apperr.InvalidResource("redundancyRole 仅支持 PRIMARY/STAND_BY")
			}
			role = in.RedundancyRole
		}
		has, err := store.DeviceHasRoleConnection(dev.ID, role)
		if err != nil {
			return err
		}
		if has {
			// D3: 旧 500 → 409
			return apperr.New(409, "PDU_SOCKET_CONNECTED", "该设备此供电角色已连接")
		}
		conn := &model.PDUConnection{
			SocketID: sock.ID, DeviceID: dev.ID, PowerW: in.PowerW,
			Circuit: in.Circuit, RedundancyRole: role,
			ConnectedAt: time.Now(), ConnectedBy: actor,
		}
		if err := store.CreateConnection(conn); err != nil {
			if repository.IsUniqueViolation(err) {
				return apperr.New(409, "PDU_SOCKET_CONNECTED", "插座或供电角色已占用")
			}
			return err
		}
		// 连接与插座状态同事务：状态更新失败整体回滚，杜绝漂移
		if err := store.UpdateSocketStatus(sock.ID, model.SocketConnected); err != nil {
			return err
		}
		result = conn
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PDUService) Disconnect(id uuid.UUID, version uint) error {
	conn, err := s.store.GetConnection(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("连接")
		}
		return err
	}
	return s.store.DB().Transaction(func(tx *gorm.DB) error {
		store := s.store.WithTx(tx)
		if err := store.SoftDeleteConnection(id, version); err != nil {
			return mapStoreErr(err)
		}
		// 连接删除与插座释放同事务
		return store.UpdateSocketStatus(conn.SocketID, model.SocketAvailable)
	})
}
