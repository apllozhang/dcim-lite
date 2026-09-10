package service

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

type ResourceService struct {
	store *repository.ResourceStore
}

func NewResourceService(store *repository.ResourceStore) *ResourceService {
	return &ResourceService{store: store}
}

type DataCenterInput struct {
	Code            string   `json:"code" binding:"required,max=50"`
	Name            string   `json:"name" binding:"required,max=150"`
	Address         string   `json:"address"`
	Longitude       *float64 `json:"longitude"`
	Latitude        *float64 `json:"latitude"`
	Status          string   `json:"status"`
	Manager         string   `json:"manager"`
	Contact         string   `json:"contact"`
	ServiceProvider string   `json:"serviceProvider"`
	Remarks         string   `json:"remarks"`
	SortOrder       int      `json:"sortOrder"`
}

type RoomInput struct {
	DataCenterID      *uuid.UUID `json:"dataCenterId"`
	Code              string     `json:"code" binding:"required,max=50"`
	Name              string     `json:"name" binding:"required,max=150"`
	Building          string     `json:"building"`
	Floor             string     `json:"floor"`
	RoomNumber        string     `json:"roomNumber"`
	AreaSquareMeters  *float64   `json:"areaSquareMeters"`
	ClearHeightMeters *float64   `json:"clearHeightMeters"`
	Purpose           string     `json:"purpose"`
	Status            string     `json:"status"`
	EnvironmentLevel  string     `json:"environmentLevel"`
	MaxLoadKg         *float64   `json:"maxLoadKg"`
	CoolingCapacityKw *float64   `json:"coolingCapacityKw"`
	DesignPowerKw     *float64   `json:"designPowerKw"`
	AvailablePowerKw  *float64   `json:"availablePowerKw"`
	UsedPowerKw       *float64   `json:"usedPowerKw"`
	RedundancyPolicy  string     `json:"redundancyPolicy"`
	Manager           string     `json:"manager"`
	Contact           string     `json:"contact"`
	OpenHours         string     `json:"openHours"`
	AccessNotes       string     `json:"accessNotes"`
	FloorPlanEnabled  bool       `json:"floorPlanEnabled"`
	FloorPlanFormat   string     `json:"floorPlanFormat"`
	RacksPerRow       *int       `json:"racksPerRow"`
	Remarks           string     `json:"remarks"`
	SortOrder         int        `json:"sortOrder"`
}

type RackInput struct {
	Code           string   `json:"code" binding:"required,max=50"`
	Name           string   `json:"name" binding:"required,max=150"`
	TemplateID     *uuid.UUID `json:"templateId"`
	Type           string   `json:"type"`
	Manufacturer   string   `json:"manufacturer"`
	ModelNumber    string   `json:"modelNumber"`
	SerialNumber   string   `json:"serialNumber"`
	AssetNumber    string   `json:"assetNumber"`
	UHeight        *int     `json:"uHeight"`
	WidthMm        *int     `json:"widthMm"`
	DepthMm        *int     `json:"depthMm"`
	HeightMm       *int     `json:"heightMm"`
	LoadCapacityKg *float64 `json:"loadCapacityKg"`
	Zone           string   `json:"zone"`
	RackRow        string   `json:"rackRow"`
	RackColumn     string   `json:"rackColumn"`
	Aisle          string   `json:"aisle"`
	XCoordinate    *float64 `json:"xCoordinate"`
	YCoordinate    *float64 `json:"yCoordinate"`
	Rotation       *int     `json:"rotation"`
	Status         string   `json:"status"`
	Manager        string   `json:"manager"`
	Department     string   `json:"department"`
	Purpose        string   `json:"purpose"`
	DualPower      *bool    `json:"dualPower"`
	InputCircuits  *int     `json:"inputCircuits"`
	RatedVoltage   *float64 `json:"ratedVoltage"`
	RatedCurrent   *float64 `json:"ratedCurrent"`
	RatedPowerKw   *float64 `json:"ratedPowerKw"`
	PeakPowerKw    *float64 `json:"peakPowerKw"`
	PDUCount       *int     `json:"pduCount"`
	Remarks        string   `json:"remarks"`
	SortOrder      int      `json:"sortOrder"`
}

type TreeRoom struct {
	model.Room
	Racks []model.Rack `json:"racks"`
}

type TreeDataCenter struct {
	model.DataCenter
	Rooms []TreeRoom `json:"rooms"`
}

func (s *ResourceService) Tree() ([]TreeDataCenter, error) {
	items, err := s.store.ListDataCenters()
	if err != nil {
		return nil, err
	}
	out := make([]TreeDataCenter, 0, len(items))
	for _, dc := range items {
		node := TreeDataCenter{DataCenter: dc, Rooms: make([]TreeRoom, 0, len(dc.Rooms))}
		for _, room := range dc.Rooms {
			racks := room.Racks
			if racks == nil {
				racks = []model.Rack{}
			}
			node.Rooms = append(node.Rooms, TreeRoom{Room: room, Racks: racks})
		}
		out = append(out, node)
	}
	return out, nil
}

// ListRacks 分页机柜列表（供台账页/导出；树接口仍保留全量结构）
func (s *ResourceService) ListRacks(q repository.RackQuery) ([]model.Rack, int64, error) {
	return s.store.ListRacks(q)
}

func (s *ResourceService) CreateDataCenter(in DataCenterInput) (*model.DataCenter, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, apperr.InvalidResource("数据中心编码和名称不能为空")
	}
	if in.Status == "" {
		in.Status = model.StatusOperating
	}
	exists, err := s.store.CodeExistsDataCenter(in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	item := &model.DataCenter{
		Code: in.Code, Name: in.Name, Address: in.Address,
		Longitude: in.Longitude, Latitude: in.Latitude, Status: in.Status,
		Manager: in.Manager, Contact: in.Contact, ServiceProvider: in.ServiceProvider,
		Remarks: in.Remarks, SortOrder: in.SortOrder,
	}
	if err := s.store.CreateDataCenter(item); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return item, nil
}

func (s *ResourceService) UpdateDataCenter(id uuid.UUID, version uint, in DataCenterInput) (*model.DataCenter, error) {
	existing, err := s.store.GetDataCenter(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("数据中心")
		}
		return nil, err
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Status == "" {
		in.Status = existing.Status
	}
	exists, err := s.store.CodeExistsDataCenter(in.Code, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	next := *existing
	next.Code, next.Name, next.Address = in.Code, in.Name, in.Address
	next.Longitude, next.Latitude = in.Longitude, in.Latitude
	next.Status, next.Manager, next.Contact = in.Status, in.Manager, in.Contact
	next.ServiceProvider, next.Remarks, next.SortOrder = in.ServiceProvider, in.Remarks, in.SortOrder
	if err := s.store.UpdateDataCenter(&next, version); err != nil {
		return nil, mapStoreErr(err)
	}
	return &next, nil
}

func (s *ResourceService) DeleteDataCenter(id uuid.UUID, version uint) error {
	if _, err := s.store.GetDataCenter(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("数据中心")
		}
		return err
	}
	n, err := s.store.CountRooms(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.HasChildren()
	}
	return mapStoreErr(s.store.SoftDeleteDataCenter(id, version))
}

func (s *ResourceService) CreateRoom(dcID uuid.UUID, in RoomInput) (*model.Room, error) {
	dc, err := s.store.GetDataCenter(dcID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("数据中心")
		}
		return nil, err
	}
	if dc.Status == model.StatusDisabled || dc.Status == model.StatusArchived {
		return nil, apperr.ParentDisabled()
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, apperr.InvalidResource("机房编码和名称不能为空")
	}
	if in.Status == "" {
		in.Status = model.StatusOperating
	}
	racksPerRow := 8
	if in.RacksPerRow != nil {
		racksPerRow = *in.RacksPerRow
	}
	if in.FloorPlanFormat == "" {
		in.FloorPlanFormat = "SVG"
	}
	exists, err := s.store.CodeExistsRoom(dcID, in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	item := &model.Room{
		DataCenterID: dcID, Code: in.Code, Name: in.Name,
		Building: in.Building, Floor: in.Floor, RoomNumber: in.RoomNumber,
		AreaSquareMeters: in.AreaSquareMeters, ClearHeightMeters: in.ClearHeightMeters,
		Purpose: in.Purpose, Status: in.Status, EnvironmentLevel: in.EnvironmentLevel,
		MaxLoadKg: in.MaxLoadKg, CoolingCapacityKw: in.CoolingCapacityKw,
		DesignPowerKw: in.DesignPowerKw, AvailablePowerKw: in.AvailablePowerKw,
		UsedPowerKw: in.UsedPowerKw, RedundancyPolicy: in.RedundancyPolicy,
		Manager: in.Manager, Contact: in.Contact, OpenHours: in.OpenHours,
		AccessNotes: in.AccessNotes, FloorPlanEnabled: in.FloorPlanEnabled,
		FloorPlanFormat: in.FloorPlanFormat, RacksPerRow: racksPerRow,
		Remarks: in.Remarks, SortOrder: in.SortOrder,
	}
	if err := s.store.CreateRoom(item); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return item, nil
}

func (s *ResourceService) UpdateRoom(id uuid.UUID, version uint, in RoomInput) (*model.Room, error) {
	existing, err := s.store.GetRoom(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机房")
		}
		return nil, err
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Status == "" {
		in.Status = existing.Status
	}
	if in.FloorPlanFormat == "" {
		in.FloorPlanFormat = existing.FloorPlanFormat
	}
	racksPerRow := existing.RacksPerRow
	if in.RacksPerRow != nil {
		racksPerRow = *in.RacksPerRow
	}
	exists, err := s.store.CodeExistsRoom(existing.DataCenterID, in.Code, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	next := *existing
	next.Code, next.Name = in.Code, in.Name
	next.Building, next.Floor, next.RoomNumber = in.Building, in.Floor, in.RoomNumber
	next.AreaSquareMeters, next.ClearHeightMeters = in.AreaSquareMeters, in.ClearHeightMeters
	next.Purpose, next.Status, next.EnvironmentLevel = in.Purpose, in.Status, in.EnvironmentLevel
	next.MaxLoadKg, next.CoolingCapacityKw = in.MaxLoadKg, in.CoolingCapacityKw
	next.DesignPowerKw, next.AvailablePowerKw, next.UsedPowerKw = in.DesignPowerKw, in.AvailablePowerKw, in.UsedPowerKw
	next.RedundancyPolicy, next.Manager, next.Contact = in.RedundancyPolicy, in.Manager, in.Contact
	next.OpenHours, next.AccessNotes = in.OpenHours, in.AccessNotes
	next.FloorPlanEnabled, next.FloorPlanFormat, next.RacksPerRow = in.FloorPlanEnabled, in.FloorPlanFormat, racksPerRow
	next.Remarks, next.SortOrder = in.Remarks, in.SortOrder
	if err := s.store.UpdateRoom(&next, version); err != nil {
		return nil, mapStoreErr(err)
	}
	return &next, nil
}

func (s *ResourceService) DeleteRoom(id uuid.UUID, version uint) error {
	if _, err := s.store.GetRoom(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("机房")
		}
		return err
	}
	n, err := s.store.CountRacks(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return apperr.HasChildren()
	}
	return mapStoreErr(s.store.SoftDeleteRoom(id, version))
}

func (s *ResourceService) CreateRack(roomID uuid.UUID, in RackInput) (*model.Rack, error) {
	room, err := s.store.GetRoom(roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机房")
		}
		return nil, err
	}
	if room.Status == model.StatusDisabled || room.Status == model.StatusArchived {
		return nil, apperr.ParentDisabled()
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, apperr.InvalidResource("机柜编码和名称不能为空")
	}
	item := applyRackDefaults(room, in)
	if in.TemplateID != nil {
		if err := s.applyTemplate(item, *in.TemplateID); err != nil {
			return nil, err
		}
		// 显式 override 字段优先于模板
		applyRackOverrides(item, in)
	}
	exists, err := s.store.CodeExistsRack(roomID, in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	if err := s.store.CreateRack(item); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil, apperr.DuplicateCode()
		}
		return nil, err
	}
	return item, nil
}

func (s *ResourceService) applyTemplate(rack *model.Rack, templateID uuid.UUID) error {
	var t model.RackTemplate
	if err := s.store.DB().First(&t, "id = ? AND deleted_at IS NULL", templateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("机柜模板")
		}
		return err
	}
	if t.Status != model.TemplateActive {
		return apperr.New(409, "TEMPLATE_DISABLED", "模板已停用")
	}
	var ver model.RackTemplateVersion
	if err := s.store.DB().First(&ver, "template_id = ? AND revision = ? AND deleted_at IS NULL", t.ID, t.CurrentRevision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("模板版本")
		}
		return err
	}
	ApplyTemplateToRack(&t, &ver, rack)
	return nil
}

func applyRackOverrides(rack *model.Rack, in RackInput) {
	if in.Type != "" {
		rack.Type = in.Type
	}
	if in.UHeight != nil {
		rack.UHeight = *in.UHeight
	}
	if in.WidthMm != nil {
		rack.WidthMm = *in.WidthMm
	}
	if in.DepthMm != nil {
		rack.DepthMm = *in.DepthMm
	}
	if in.HeightMm != nil {
		rack.HeightMm = *in.HeightMm
	}
	if in.LoadCapacityKg != nil {
		rack.LoadCapacityKg = in.LoadCapacityKg
	}
	if in.DualPower != nil {
		rack.DualPower = *in.DualPower
	}
	if in.InputCircuits != nil {
		rack.InputCircuits = *in.InputCircuits
	}
	if in.RatedVoltage != nil {
		rack.RatedVoltage = in.RatedVoltage
	}
	if in.RatedCurrent != nil {
		rack.RatedCurrent = in.RatedCurrent
	}
	if in.RatedPowerKw != nil {
		rack.RatedPowerKw = in.RatedPowerKw
	}
	if in.PeakPowerKw != nil {
		rack.PeakPowerKw = in.PeakPowerKw
	}
	if in.PDUCount != nil {
		rack.PDUCount = *in.PDUCount
	}
	if in.Manufacturer != "" {
		rack.Manufacturer = in.Manufacturer
	}
	if in.ModelNumber != "" {
		rack.ModelNumber = in.ModelNumber
	}
}

func applyRackDefaults(room *model.Room, in RackInput) *model.Rack {
	item := &model.Rack{
		DataCenterID: room.DataCenterID, RoomID: room.ID,
		Code: in.Code, Name: in.Name,
		Type: "STANDARD", Status: model.RackStatusAvailable,
		UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000,
		Manufacturer: in.Manufacturer, ModelNumber: in.ModelNumber,
		SerialNumber: in.SerialNumber, AssetNumber: in.AssetNumber,
		LoadCapacityKg: in.LoadCapacityKg, Zone: in.Zone, RackRow: in.RackRow,
		RackColumn: in.RackColumn, Aisle: in.Aisle,
		XCoordinate: in.XCoordinate, YCoordinate: in.YCoordinate,
		Manager: in.Manager, Department: in.Department, Purpose: in.Purpose,
		Remarks: in.Remarks, SortOrder: in.SortOrder,
	}
	if in.Type != "" {
		item.Type = in.Type
	}
	if in.Status != "" {
		item.Status = in.Status
	}
	if in.UHeight != nil {
		item.UHeight = *in.UHeight
	}
	if in.WidthMm != nil {
		item.WidthMm = *in.WidthMm
	}
	if in.DepthMm != nil {
		item.DepthMm = *in.DepthMm
	}
	if in.HeightMm != nil {
		item.HeightMm = *in.HeightMm
	}
	if in.Rotation != nil {
		item.Rotation = *in.Rotation
	}
	if in.DualPower != nil {
		item.DualPower = *in.DualPower
	}
	if in.InputCircuits != nil {
		item.InputCircuits = *in.InputCircuits
	}
	if in.RatedVoltage != nil {
		item.RatedVoltage = in.RatedVoltage
	}
	if in.RatedCurrent != nil {
		item.RatedCurrent = in.RatedCurrent
	}
	if in.RatedPowerKw != nil {
		item.RatedPowerKw = in.RatedPowerKw
	}
	if in.PeakPowerKw != nil {
		item.PeakPowerKw = in.PeakPowerKw
	}
	if in.PDUCount != nil {
		item.PDUCount = *in.PDUCount
	}
	return item
}

func (s *ResourceService) UpdateRack(id uuid.UUID, version uint, in RackInput) (*model.Rack, error) {
	existing, err := s.store.GetRack(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	room, err := s.store.GetRoom(existing.RoomID)
	if err != nil {
		return nil, err
	}
	next := applyRackDefaults(room, in)
	next.ID = existing.ID
	next.CreatedAt = existing.CreatedAt
	next.Version = existing.Version
	next.DataCenterID = existing.DataCenterID
	next.RoomID = existing.RoomID
	next.TemplateID = existing.TemplateID
	next.TemplateVersionID = existing.TemplateVersionID
	next.TemplateCode = existing.TemplateCode
	next.TemplateName = existing.TemplateName
	next.TemplateRevision = existing.TemplateRevision
	next.TemplateSnapshot = existing.TemplateSnapshot
	if next.Status == "" {
		next.Status = existing.Status
	}
	exists, err := s.store.CodeExistsRack(existing.RoomID, next.Code, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	if err := s.store.UpdateRack(next, version); err != nil {
		return nil, mapStoreErr(err)
	}
	return next, nil
}

func (s *ResourceService) DeleteRack(id uuid.UUID, version uint) error {
	if _, err := s.store.GetRack(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("机柜")
		}
		return err
	}
	// R3：机柜下存在在位设备或 PDU 时禁止删除，避免软删后留下不可见的在位数据
	if n, err := s.store.CountActivePositions(id); err != nil {
		return err
	} else if n > 0 {
		return apperr.New(409, "HAS_CHILDREN", "机柜内存在在位设备，无法删除")
	}
	if n, err := s.store.CountPDUsByRack(id); err != nil {
		return err
	} else if n > 0 {
		return apperr.New(409, "HAS_CHILDREN", "机柜下存在 PDU，无法删除")
	}
	return mapStoreErr(s.store.SoftDeleteRack(id, version))
}

func mapStoreErr(err error) error {
	if err == nil {
		return nil
	}
	if repository.IsVersionConflict(err) {
		return apperr.ResourceVersion()
	}
	if repository.IsUniqueViolation(err) {
		return apperr.DuplicateCode()
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("资源")
	}
	return err
}

func (s *ResourceService) Audit(actor *uuid.UUID, requestID, action, resourceType string, resourceID *uuid.UUID, result, errCode string) {
	log := &model.AuditLog{
		UserID: actor, Action: action, ResourceType: resourceType,
		ResourceID: resourceID, RequestID: requestID, Result: result, ErrorCode: errCode,
		Source: "api",
	}
	_ = s.store.WriteAudit(log)
}
