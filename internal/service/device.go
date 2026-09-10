package service

import (
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

type DeviceService struct {
	devices *repository.DeviceStore
	acks    *repository.ResourceStore
}

func NewDeviceService(devices *repository.DeviceStore, acks *repository.ResourceStore) *DeviceService {
	return &DeviceService{devices: devices, acks: acks}
}

// withTx 返回绑定同一事务的 service 副本，供跨聚合命令（审批、导入）复用。
func (s *DeviceService) withTx(tx *gorm.DB) *DeviceService {
	return &DeviceService{devices: s.devices.WithTx(tx), acks: s.acks.WithTx(tx)}
}

type DeviceTypeInput struct {
	Code               string   `json:"code" binding:"required,max=50"`
	Name               string   `json:"name" binding:"required,max=150"`
	Category           string   `json:"category" binding:"required"`
	Status             string   `json:"status"`
	DefaultHeightU     *int     `json:"defaultHeightU"`
	DefaultWeightKg    *float64 `json:"defaultWeightKg"`
	DefaultRatedPowerW *float64 `json:"defaultRatedPowerW"`
	DefaultPeakPowerW  *float64 `json:"defaultPeakPowerW"`
	DefaultDualPower   *bool    `json:"defaultDualPower"`
	Description        string   `json:"description"`
	SortOrder          int      `json:"sortOrder"`
}

type DeviceInput struct {
	TypeID             *uuid.UUID `json:"typeId"`
	Code               string     `json:"code"`
	Name               string     `json:"name" binding:"required,max=150"`
	AssetNumber        string     `json:"assetNumber"`
	SerialNumber       string     `json:"serialNumber"`
	Manufacturer       string     `json:"manufacturer"`
	ModelNumber        string     `json:"modelNumber"`
	Specification      string     `json:"specification"`
	FirmwareVersion    string     `json:"firmwareVersion"`
	PurchaseBatch      string     `json:"purchaseBatch"`
	Organization       string     `json:"organization"`
	Manager            string     `json:"manager"`
	Contact            string     `json:"contact"`
	BusinessSystem     string     `json:"businessSystem"`
	ApplicationName    string     `json:"applicationName"`
	LifecycleStatus    string     `json:"lifecycleStatus"`
	HeightU            *int       `json:"heightU"`
	WeightKg           *float64   `json:"weightKg"`
	RatedPowerW        *float64   `json:"ratedPowerW"`
	PeakPowerW         *float64   `json:"peakPowerW"`
	DualPowerRequired  *bool      `json:"dualPowerRequired"`
	ManagementIP       string     `json:"managementIp"`
	BusinessIP         string     `json:"businessIp"`
	MACAddress         string     `json:"macAddress"`
	ManagementProtocol string     `json:"managementProtocol"`
	MonitoringStatus   string     `json:"monitoringStatus"`
	Tags               string     `json:"tags"`
	Remarks            string     `json:"remarks"`
}

type PositionChangeInput struct {
	TargetRackID uuid.UUID `json:"targetRackId"`
	// RackID 为厂商基线字段名（rackId），与 targetRackId 等价；ALE 前端使用此字段。
	RackID      *uuid.UUID `json:"rackId"`
	StartU      int        `json:"startU" binding:"required"`
	Orientation string     `json:"orientation"`
	Reason      string     `json:"reason"`
}

// desiredRackID 兼容厂商的 rackId 字段命名（差分回放实测：套件与 ALE 均用 rackId）。
func (in PositionChangeInput) desiredRackID() (uuid.UUID, error) {
	if in.TargetRackID != uuid.Nil {
		return in.TargetRackID, nil
	}
	if in.RackID != nil && *in.RackID != uuid.Nil {
		return *in.RackID, nil
	}
	return uuid.Nil, apperr.InvalidResource("缺少机柜 ID（rackId 或 targetRackId）")
}

type DecommissionInput struct {
	Reason string `json:"reason"`
}

type ULayoutPosition struct {
	DeviceID    uuid.UUID     `json:"deviceId"`
	RackID      uuid.UUID     `json:"rackId"`
	StartU      int           `json:"startU"`
	EndU        int           `json:"endU"`
	HeightU     int           `json:"heightU"`
	Orientation string        `json:"orientation"`
	InstalledAt time.Time     `json:"installedAt"`
	Reason      string        `json:"reason,omitempty"`
	Device      *model.Device `json:"device,omitempty"`
}

// ULayout matches old frontend RoomScreenView: data.positions[].device
type ULayout struct {
	RackID    uuid.UUID         `json:"rackId"`
	UHeight   int               `json:"uHeight"`
	RackCode  string            `json:"rackCode"`
	RackName  string            `json:"rackName"`
	Status    string            `json:"status"`
	Positions []ULayoutPosition `json:"positions"`
}

func (s *DeviceService) ListDeviceTypes() ([]model.DeviceType, error) {
	return s.devices.ListDeviceTypes()
}

func (s *DeviceService) CreateDeviceType(in DeviceTypeInput) (*model.DeviceType, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Category == "" {
		return nil, apperr.InvalidResource("设备类型分类不能为空")
	}
	// 厂商基线对未知分类返回 400（S14-VAL-DTYPE-CAT 差分用例）
	if !validDeviceCategory(in.Category) {
		return nil, apperr.InvalidResource("设备分类无效")
	}
	if in.Status == "" {
		in.Status = "ACTIVE"
	}
	h := 1
	if in.DefaultHeightU != nil {
		h = *in.DefaultHeightU
	}
	dual := false
	if in.DefaultDualPower != nil {
		dual = *in.DefaultDualPower
	}
	exists, err := s.devices.CodeExistsDeviceType(in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	item := &model.DeviceType{
		Code: in.Code, Name: in.Name, Category: in.Category, Status: in.Status,
		DefaultHeightU: h, DefaultWeightKg: in.DefaultWeightKg,
		DefaultRatedPowerW: in.DefaultRatedPowerW, DefaultPeakPowerW: in.DefaultPeakPowerW,
		DefaultDualPower: dual, Description: in.Description, SortOrder: in.SortOrder,
	}
	if err := s.devices.CreateDeviceType(item); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return item, nil
}

func (s *DeviceService) UpdateDeviceType(id uuid.UUID, version uint, in DeviceTypeInput) (*model.DeviceType, error) {
	existing, err := s.devices.GetDeviceType(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备类型")
		}
		return nil, err
	}
	if in.Status == "" {
		in.Status = existing.Status
	}
	h := existing.DefaultHeightU
	if in.DefaultHeightU != nil {
		h = *in.DefaultHeightU
	}
	dual := existing.DefaultDualPower
	if in.DefaultDualPower != nil {
		dual = *in.DefaultDualPower
	}
	next := *existing
	next.Code, next.Name, next.Category = strings.TrimSpace(in.Code), strings.TrimSpace(in.Name), in.Category
	next.Status = in.Status
	next.DefaultHeightU = h
	next.DefaultWeightKg = in.DefaultWeightKg
	next.DefaultRatedPowerW = in.DefaultRatedPowerW
	next.DefaultPeakPowerW = in.DefaultPeakPowerW
	next.DefaultDualPower = dual
	next.Description, next.SortOrder = in.Description, in.SortOrder
	if err := s.devices.UpdateDeviceType(&next, version); err != nil {
		return nil, mapStoreErr(err)
	}
	return &next, nil
}

func (s *DeviceService) DeleteDeviceType(id uuid.UUID, version uint) error {
	if _, err := s.devices.GetDeviceType(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("设备类型")
		}
		return err
	}
	if err := s.devices.SoftDeleteDeviceType(id, version); err != nil {
		if repository.IsDeviceTypeInUse(err) {
			return apperr.New(409, "DEVICE_TYPE_IN_USE", "设备类型已被设备引用，无法删除")
		}
		return mapStoreErr(err)
	}
	return nil
}

func (s *DeviceService) ListDevices(q repository.DeviceQuery) ([]model.Device, int64, error) {
	return s.devices.ListDevices(q)
}

func (s *DeviceService) CreateDevice(in DeviceInput) (*model.Device, error) {
	if in.TypeID == nil {
		// 厂商基线对缺失设备类型返回 404（S14-VAL-DEV-NOTYPE 差分用例）
		return nil, apperr.NotFound("设备类型")
	}
	dt, err := s.devices.GetDeviceType(*in.TypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备类型")
		}
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	if in.Code == "" {
		in.Code = fmt.Sprintf("DEV-%s", uuid.NewString()[:8])
	}
	if in.LifecycleStatus == "" {
		in.LifecycleStatus = model.DeviceWaitingRack
	}
	h := dt.DefaultHeightU
	if in.HeightU != nil && *in.HeightU > 0 {
		h = *in.HeightU
	}
	dual := dt.DefaultDualPower
	if in.DualPowerRequired != nil {
		dual = *in.DualPowerRequired
	}
	weight := in.WeightKg
	if weight == nil {
		weight = dt.DefaultWeightKg
	}
	rated := in.RatedPowerW
	if rated == nil {
		rated = dt.DefaultRatedPowerW
	}
	peak := in.PeakPowerW
	if peak == nil {
		peak = dt.DefaultPeakPowerW
	}
	item := &model.Device{
		TypeID: dt.ID, Code: in.Code, Name: in.Name,
		AssetNumber: strings.TrimSpace(in.AssetNumber), SerialNumber: in.SerialNumber,
		Manufacturer: in.Manufacturer, ModelNumber: in.ModelNumber,
		Specification: in.Specification, FirmwareVersion: in.FirmwareVersion,
		PurchaseBatch: in.PurchaseBatch, Organization: in.Organization,
		Manager: in.Manager, Contact: in.Contact,
		BusinessSystem: in.BusinessSystem, ApplicationName: in.ApplicationName,
		LifecycleStatus: in.LifecycleStatus, HeightU: h,
		WeightKg: weight, RatedPowerW: rated, PeakPowerW: peak,
		DualPowerRequired: dual,
		ManagementIP:      in.ManagementIP, BusinessIP: in.BusinessIP, MACAddress: in.MACAddress,
		ManagementProtocol: in.ManagementProtocol, MonitoringStatus: in.MonitoringStatus,
		Tags: in.Tags, Remarks: in.Remarks,
	}
	if err := s.devices.CreateDevice(item); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return s.devices.GetDevice(item.ID)
}

func (s *DeviceService) UpdateDevice(id uuid.UUID, version uint, in DeviceInput) (*model.Device, error) {
	existing, err := s.devices.GetDevice(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备")
		}
		return nil, err
	}
	typeID := existing.TypeID
	if in.TypeID != nil {
		typeID = *in.TypeID
	}
	if _, err := s.devices.GetDeviceType(typeID); err != nil {
		return nil, apperr.NotFound("设备类型")
	}
	h := existing.HeightU
	if in.HeightU != nil && *in.HeightU > 0 {
		// 在位设备禁止改高度（契约）
		if pos, err := s.devices.GetActivePosition(id); err == nil && pos != nil && *in.HeightU != existing.HeightU {
			return nil, apperr.New(409, "DEVICE_POSITIONED", "设备已在位，高度不可修改")
		}
		h = *in.HeightU
	}
	next := *existing
	next.TypeID = typeID
	next.Code = strings.TrimSpace(in.Code)
	if next.Code == "" {
		next.Code = existing.Code
	}
	next.Name = strings.TrimSpace(in.Name)
	next.AssetNumber = strings.TrimSpace(in.AssetNumber)
	next.SerialNumber = in.SerialNumber
	next.Manufacturer = in.Manufacturer
	next.ModelNumber = in.ModelNumber
	next.Specification = in.Specification
	next.FirmwareVersion = in.FirmwareVersion
	next.PurchaseBatch = in.PurchaseBatch
	next.Organization = in.Organization
	next.Manager = in.Manager
	next.Contact = in.Contact
	next.BusinessSystem = in.BusinessSystem
	next.ApplicationName = in.ApplicationName
	next.HeightU = h
	if in.WeightKg != nil {
		next.WeightKg = in.WeightKg
	}
	if in.RatedPowerW != nil {
		next.RatedPowerW = in.RatedPowerW
	}
	if in.PeakPowerW != nil {
		next.PeakPowerW = in.PeakPowerW
	}
	if in.DualPowerRequired != nil {
		next.DualPowerRequired = *in.DualPowerRequired
	}
	next.ManagementIP = in.ManagementIP
	next.BusinessIP = in.BusinessIP
	next.MACAddress = in.MACAddress
	next.ManagementProtocol = in.ManagementProtocol
	next.MonitoringStatus = in.MonitoringStatus
	next.Tags = in.Tags
	next.Remarks = in.Remarks
	// 生命周期状态不在普通更新中随意改，上架/下架走专用接口
	next.LifecycleStatus = existing.LifecycleStatus
	if err := s.devices.UpdateDevice(&next, version); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.devices.GetDevice(id)
}

func (s *DeviceService) DeleteDevice(id uuid.UUID, version uint) error {
	dev, err := s.devices.GetDevice(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("设备")
		}
		return err
	}
	if _, err := s.devices.GetActivePosition(id); err == nil {
		return apperr.New(409, "DEVICE_POSITIONED", "设备在位，请先下架")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	_ = dev
	return mapStoreErr(s.devices.SoftDeleteDevice(id, version))
}

func (s *DeviceService) GetDevice(id uuid.UUID) (*model.Device, error) {
	dev, err := s.devices.GetDevice(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备")
		}
		return nil, err
	}
	return dev, nil
}

func (s *DeviceService) Assign(id uuid.UUID, in PositionChangeInput, actor *uuid.UUID, requestID string) (*model.Device, error) {
	return s.place(id, in, actor, requestID, "ASSIGN", true)
}

func (s *DeviceService) Move(id uuid.UUID, in PositionChangeInput, actor *uuid.UUID, requestID string) (*model.Device, error) {
	return s.place(id, in, actor, requestID, "MOVE", false)
}

// PlaceTx 在外部事务内执行上架/移位（位置+占用+生命周期+履历原子提交）。
func (s *DeviceService) PlaceTx(tx *gorm.DB, id uuid.UUID, in PositionChangeInput, actor *uuid.UUID, requestID, op string, requireOff bool) (*model.Device, error) {
	return s.withTx(tx).place(id, in, actor, requestID, op, requireOff)
}

// place 位置写入、生命周期更新与履历记录在同一事务中提交；
// 任何一步失败整体回滚，不存在“位置已写入但生命周期/履历缺失”的中间态。
func (s *DeviceService) place(id uuid.UUID, in PositionChangeInput, actor *uuid.UUID, requestID, op string, requireOff bool) (*model.Device, error) {
	var result *model.Device
	err := s.devices.DB().Transaction(func(tx *gorm.DB) error {
		svc := s.withTx(tx)
		dev, err := svc.placeInTx(id, in, actor, requestID, op, requireOff)
		if err != nil {
			return err
		}
		result = dev
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DeviceService) placeInTx(id uuid.UUID, in PositionChangeInput, actor *uuid.UUID, requestID, op string, requireOff bool) (*model.Device, error) {
	dev, err := s.devices.GetDevice(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备")
		}
		return nil, err
	}
	if requireOff {
		if dev.LifecycleStatus != model.DeviceWaitingRack && dev.LifecycleStatus != model.DeviceOffRack {
			return nil, apperr.New(409, "INVALID_RESOURCE", "当前状态不可上架")
		}
	} else {
		if dev.LifecycleStatus != model.DeviceRunning && dev.LifecycleStatus != model.DeviceMaintenance {
			return nil, apperr.New(409, "INVALID_RESOURCE", "当前状态不可移位")
		}
	}
	rackID, err := in.desiredRackID()
	if err != nil {
		return nil, err
	}
	rack, err := s.acks.GetRack(rackID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	if rack.Status == model.RackStatusDisabled || rack.Status == model.RackStatusPlanning {
		return nil, apperr.New(409, "RACK_UNAVAILABLE", "目标机柜不可用")
	}
	if in.StartU < 1 {
		return nil, apperr.InvalidResource("startU 必须 >= 1")
	}
	if dev.HeightU < 1 {
		return nil, apperr.InvalidResource("设备高度无效")
	}
	endU := in.StartU + dev.HeightU - 1
	if endU > rack.UHeight {
		return nil, apperr.InvalidResource("超出机柜 U 高度")
	}
	orient := model.OrientNormal
	if in.Orientation != "" {
		if in.Orientation != model.OrientNormal && in.Orientation != model.OrientReverse {
			return nil, apperr.InvalidResource("orientation 仅支持 NORMAL/REVERSE")
		}
		orient = in.Orientation
	}

	// 服务层预检：区间重叠（数据库排他约束仍是最终防线）
	occs, err := s.devices.ListOccupancies(rack.ID)
	if err != nil {
		return nil, err
	}
	for _, o := range occs {
		if o.DeviceID == id {
			continue
		}
		if rangesOverlap(in.StartU, endU, o.StartU, o.EndU) {
			return nil, apperr.New(409, "RACK_U_CONFLICT", "U 位区间与在位设备重叠")
		}
	}

	var fromSnap *model.PositionSnapshot
	if old, err := s.devices.GetActivePosition(id); err == nil {
		fromSnap = &model.PositionSnapshot{
			RackID: old.RackID, RoomID: old.RoomID, DataCenterID: old.DataCenterID,
			StartU: old.StartU, HeightU: old.HeightU, EndU: old.EndU, Orientation: old.Orientation,
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	pos := &model.RackDevicePosition{
		DeviceID: id, RackID: rack.ID, RoomID: rack.RoomID, DataCenterID: rack.DataCenterID,
		StartU: in.StartU, HeightU: dev.HeightU, EndU: endU, Orientation: orient,
		InstalledAt: time.Now(), InstalledBy: actor, Reason: strings.TrimSpace(in.Reason),
	}
	if err := s.devices.PlaceInRack(pos, &model.RackUOccupancy{}); err != nil {
		if repository.IsExclusionViolation(err) || repository.IsUniqueViolation(err) {
			return nil, apperr.New(409, "RACK_U_CONFLICT", "U 位区间与在位设备重叠")
		}
		return nil, err
	}
	if err := s.devices.UpdateDeviceLifecycle(id, model.DeviceRunning); err != nil {
		return nil, err
	}
	toSnap := model.PositionSnapshot{
		RackID: rack.ID, RoomID: rack.RoomID, DataCenterID: rack.DataCenterID,
		StartU: in.StartU, HeightU: dev.HeightU, EndU: endU, Orientation: orient,
	}
	if err := s.writeHistory(id, op, fromSnap, &toSnap, in.Reason, actor, requestID); err != nil {
		return nil, err
	}
	return s.devices.GetDevice(id)
}

// Decommission 下架位置移除、生命周期更新与履历记录在同一事务中提交。
func (s *DeviceService) Decommission(id uuid.UUID, in DecommissionInput, actor *uuid.UUID, requestID string) (*model.Device, error) {
	var result *model.Device
	err := s.devices.DB().Transaction(func(tx *gorm.DB) error {
		svc := s.withTx(tx)
		dev, err := svc.decommissionInTx(id, in, actor, requestID)
		if err != nil {
			return err
		}
		result = dev
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *DeviceService) decommissionInTx(id uuid.UUID, in DecommissionInput, actor *uuid.UUID, requestID string) (*model.Device, error) {
	dev, err := s.devices.GetDevice(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备")
		}
		return nil, err
	}
	if dev.LifecycleStatus == model.DeviceScrapped || dev.LifecycleStatus == model.DeviceOffRack {
		return nil, apperr.New(409, "INVALID_RESOURCE", "设备已下架或已报废")
	}
	var fromSnap *model.PositionSnapshot
	if old, err := s.devices.RemoveFromRack(id); err == nil {
		fromSnap = &model.PositionSnapshot{
			RackID: old.RackID, RoomID: old.RoomID, DataCenterID: old.DataCenterID,
			StartU: old.StartU, HeightU: old.HeightU, EndU: old.EndU, Orientation: old.Orientation,
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := s.devices.UpdateDeviceLifecycle(id, model.DeviceOffRack); err != nil {
		return nil, err
	}
	if err := s.writeHistory(id, "DECOMMISSION", fromSnap, nil, in.Reason, actor, requestID); err != nil {
		return nil, err
	}
	return s.devices.GetDevice(id)
}

func (s *DeviceService) ListHistory(id uuid.UUID) ([]model.DevicePositionHistory, error) {
	if _, err := s.devices.GetDevice(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("设备")
		}
		return nil, err
	}
	return s.devices.ListHistories(id)
}

func (s *DeviceService) ULayout(rackID uuid.UUID) (*ULayout, error) {
	rack, err := s.acks.GetRack(rackID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	// 直接读在位记录，保证 orientation/installedAt 与 positions 一致
	var positions []model.RackDevicePosition
	if err := s.devices.DB().
		Where("rack_id = ? AND deleted_at IS NULL", rackID).
		Order("start_u asc").Find(&positions).Error; err != nil {
		return nil, err
	}
	// 一次批量取回全部设备，避免逐个查询（N+1）
	deviceIDs := make([]uuid.UUID, 0, len(positions))
	for _, p := range positions {
		deviceIDs = append(deviceIDs, p.DeviceID)
	}
	devicesByID := map[uuid.UUID]*model.Device{}
	if len(deviceIDs) > 0 {
		var devs []model.Device
		if err := s.devices.DB().Preload("Type").
			Where("id IN ? AND deleted_at IS NULL", deviceIDs).
			Find(&devs).Error; err != nil {
			return nil, err
		}
		for i := range devs {
			devicesByID[devs[i].ID] = &devs[i]
		}
	}
	out := &ULayout{
		RackID: rack.ID, UHeight: rack.UHeight,
		RackCode: rack.Code, RackName: rack.Name, Status: rack.Status,
		Positions: make([]ULayoutPosition, 0, len(positions)),
	}
	for _, p := range positions {
		dev, ok := devicesByID[p.DeviceID]
		if !ok {
			continue
		}
		out.Positions = append(out.Positions, ULayoutPosition{
			DeviceID: p.DeviceID, RackID: p.RackID,
			StartU: p.StartU, EndU: p.EndU, HeightU: p.HeightU,
			Orientation: p.Orientation, InstalledAt: p.InstalledAt, Reason: p.Reason,
			Device: dev,
		})
	}
	return out, nil
}

// writeHistory 与业务写入同事务调用：履历是业务可信链的一部分，失败即回滚。
func (s *DeviceService) writeHistory(deviceID uuid.UUID, op string, from, to *model.PositionSnapshot, reason string, actor *uuid.UUID, requestID string) error {
	h := &model.DevicePositionHistory{
		DeviceID: deviceID, Operation: op, Reason: reason,
		ActorID: actor, RequestID: requestID,
		FromPosition: from, ToPosition: to,
	}
	return s.devices.WriteHistory(h)
}

func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart <= bEnd && bStart <= aEnd
}

// validDeviceCategory 设备分类枚举（与种子数据一致；厂商基线拒绝未知分类）。
func validDeviceCategory(c string) bool {
	switch c {
	case "SERVER", "NETWORK", "STORAGE", "SECURITY", "POWER_ENVIRONMENT", "ACCESSORY":
		return true
	}
	return false
}
