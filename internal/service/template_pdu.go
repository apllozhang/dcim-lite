package service

import (
	"errors"
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
	if in.Name != "" {
		t.Name = in.Name
	}
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
		return apperr.New(409, "SYSTEM_TEMPLATE", "系统内置模板不可删除")
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
	store  *repository.PDUStore
	acks   *repository.ResourceStore
	devs   *repository.DeviceStore
	tmpl   *repository.TemplateStore
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
	if _, err := s.acks.GetRack(rackID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	if in.Status == "" {
		in.Status = model.PDUActive
	}
	exists, err := s.store.CodeExistsPDU(in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	item := &model.PDU{
		RackID: rackID, Code: strings.TrimSpace(in.Code), Name: strings.TrimSpace(in.Name),
		Manufacturer: in.Manufacturer, ModelNumber: in.ModelNumber, SerialNumber: in.SerialNumber,
		InputVoltage: in.InputVoltage, RatedPowerW: in.RatedPowerW, RatedCurrentA: in.RatedCurrentA,
		Status: in.Status, Remarks: in.Remarks,
	}
	if err := s.store.CreatePDU(item); err != nil {
		if repository.IsUniqueViolation(err) {
			// D1: 旧 500 → 409 DUPLICATE_CODE
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return item, nil
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
	if _, err := s.store.GetPDU(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("PDU")
		}
		return err
	}
	if err := s.store.SoftDeletePDU(id, version); err != nil {
		if repository.IsBizCode(err, "HAS_CHILDREN") {
			return apperr.New(409, "HAS_CHILDREN", "PDU 下存在插座，无法删除")
		}
		return mapStoreErr(err)
	}
	return nil
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

func (s *PDUService) CreateSocket(pduID uuid.UUID, in SocketInput) (*model.PDUSocket, error) {
	if _, err := s.store.GetPDU(pduID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("PDU")
		}
		return nil, err
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
			return nil, apperr.New(409, "DUPLICATE_CODE", "插座编号已存在")
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
	next := *existing
	next.SocketNo, next.Standard, next.AmperageA = in.SocketNo, in.Standard, in.AmperageA
	next.Label, next.Status = in.Label, in.Status
	if err := s.store.UpdateSocket(&next, existing.Version); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.New(409, "DUPLICATE_CODE", "插座编号已存在")
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
		if repository.IsBizCode(err, "SOCKET_CONNECTED") {
			// 契约：已连接阻止删除
			return apperr.New(409, "SOCKET_CONNECTED", "插座已连接设备，无法删除")
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
	sock, err := s.store.GetSocket(socketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("插座")
		}
		return nil, err
	}
	if sock.Status == model.SocketConnected {
		return nil, apperr.New(409, "SOCKET_CONNECTED", "插座已被占用")
	}
	pdu, err := s.store.GetPDU(sock.PDUID)
	if err != nil {
		return nil, err
	}
	dev, err := s.devs.GetDevice(in.DeviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备")
		}
		return nil, err
	}
	pos, err := s.devs.GetActivePosition(dev.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(409, "DEVICE_NOT_POSITIONED", "设备未上架，无法接电")
		}
		return nil, err
	}
	if pos.RackID != pdu.RackID {
		// D3 相关：PDU 与设备必须同机柜
		return nil, apperr.New(409, "PDU_DEVICE_RACK_MISMATCH", "PDU 与设备不在同一机柜")
	}
	role := model.RolePrimary
	if in.RedundancyRole != "" {
		if in.RedundancyRole != model.RolePrimary && in.RedundancyRole != model.RoleStandby {
			return nil, apperr.InvalidResource("redundancyRole 仅支持 PRIMARY/STANDBY")
		}
		role = in.RedundancyRole
	}
	has, err := s.store.DeviceHasRoleConnection(dev.ID, role)
	if err != nil {
		return nil, err
	}
	if has {
		// D3: 旧 500 → 409
		return nil, apperr.New(409, "SOCKET_CONNECTED", "该设备此供电角色已连接")
	}
	conn := &model.PDUConnection{
		SocketID: sock.ID, DeviceID: dev.ID, PowerW: in.PowerW,
		Circuit: in.Circuit, RedundancyRole: role,
		ConnectedAt: time.Now(), ConnectedBy: actor,
	}
	if err := s.store.CreateConnection(conn); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.New(409, "SOCKET_CONNECTED", "插座或供电角色已占用")
		}
		return nil, err
	}
	_ = s.store.UpdateSocketStatus(sock.ID, model.SocketConnected)
	return conn, nil
}

func (s *PDUService) Disconnect(id uuid.UUID, version uint) error {
	conn, err := s.store.GetConnection(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("连接")
		}
		return err
	}
	if err := s.store.SoftDeleteConnection(id, version); err != nil {
		return mapStoreErr(err)
	}
	_ = s.store.UpdateSocketStatus(conn.SocketID, model.SocketAvailable)
	return nil
}
