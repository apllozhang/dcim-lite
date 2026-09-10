package model

import (
	"github.com/google/uuid"
)

const (
	StatusOperating = "OPERATING"
	StatusPlanning  = "PLANNING"
	StatusDisabled  = "DISABLED"
	StatusArchived  = "ARCHIVED"

	RackStatusAvailable = "AVAILABLE"
	RackStatusPlanning  = "PLANNING"
	RackStatusPartial   = "PARTIAL"
	RackStatusFull      = "FULL"
	RackStatusMaint     = "MAINTENANCE"
	RackStatusDisabled  = "DISABLED"
)

type DataCenter struct {
	BaseModel
	Code            string   `gorm:"size:50;not null;uniqueIndex:idx_data_centers_code_active,where:deleted_at IS NULL" json:"code"`
	Name            string   `gorm:"size:150;not null;index" json:"name"`
	Address         string   `gorm:"size:500" json:"address,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Status          string   `gorm:"size:30;not null;default:OPERATING;index" json:"status"`
	Manager         string   `gorm:"size:100" json:"manager,omitempty"`
	Contact         string   `gorm:"size:100" json:"contact,omitempty"`
	ServiceProvider string   `gorm:"size:150" json:"serviceProvider,omitempty"`
	Remarks         string   `gorm:"type:text" json:"remarks,omitempty"`
	SortOrder       int      `gorm:"not null;default:0;index" json:"sortOrder"`
	Rooms           []Room   `gorm:"foreignKey:DataCenterID" json:"rooms,omitempty"`
}

type Room struct {
	BaseModel
	DataCenterID      uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_rooms_data_center_code,where:deleted_at IS NULL" json:"dataCenterId"`
	Code              string    `gorm:"size:50;not null;uniqueIndex:idx_rooms_data_center_code,where:deleted_at IS NULL" json:"code"`
	Name              string    `gorm:"size:150;not null;index" json:"name"`
	Building          string    `gorm:"size:100" json:"building,omitempty"`
	Floor             string    `gorm:"size:50" json:"floor,omitempty"`
	RoomNumber        string    `gorm:"size:50" json:"roomNumber,omitempty"`
	AreaSquareMeters  *float64  `json:"areaSquareMeters,omitempty"`
	ClearHeightMeters *float64  `json:"clearHeightMeters,omitempty"`
	Purpose           string    `gorm:"size:200" json:"purpose,omitempty"`
	Status            string    `gorm:"size:30;not null;default:OPERATING;index" json:"status"`
	EnvironmentLevel  string    `gorm:"size:50" json:"environmentLevel,omitempty"`
	MaxLoadKg         *float64  `json:"maxLoadKg,omitempty"`
	CoolingCapacityKw *float64  `json:"coolingCapacityKw,omitempty"`
	DesignPowerKw     *float64  `json:"designPowerKw,omitempty"`
	AvailablePowerKw  *float64  `json:"availablePowerKw,omitempty"`
	UsedPowerKw       *float64  `json:"usedPowerKw,omitempty"`
	RedundancyPolicy  string    `gorm:"size:100" json:"redundancyPolicy,omitempty"`
	Manager           string    `gorm:"size:100" json:"manager,omitempty"`
	Contact           string    `gorm:"size:100" json:"contact,omitempty"`
	OpenHours         string    `gorm:"size:100" json:"openHours,omitempty"`
	AccessNotes       string    `gorm:"type:text" json:"accessNotes,omitempty"`
	FloorPlanEnabled  bool      `gorm:"not null;default:false" json:"floorPlanEnabled"`
	FloorPlanFormat   string    `gorm:"size:20;not null;default:SVG" json:"floorPlanFormat"`
	RacksPerRow       int       `gorm:"not null;default:8" json:"racksPerRow"`
	Remarks           string    `gorm:"type:text" json:"remarks,omitempty"`
	SortOrder         int       `gorm:"not null;default:0;index" json:"sortOrder"`
	Racks             []Rack    `gorm:"foreignKey:RoomID" json:"racks,omitempty"`
}

type RackTemplateSnapshot struct {
	TemplateID uuid.UUID `json:"templateId"`
	VersionID  uuid.UUID `json:"versionId"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Revision   int       `json:"revision"`
	Type       string    `json:"type"`
	UHeight    int       `json:"uHeight"`
	WidthMm    int       `json:"widthMm"`
	DepthMm    int       `json:"depthMm"`
	HeightMm   int       `json:"heightMm"`
	DualPower  bool      `json:"dualPower"`
	PDUCount   int       `json:"pduCount"`
}

type Rack struct {
	BaseModel
	TemplateID        *uuid.UUID            `gorm:"type:uuid;index" json:"templateId,omitempty"`
	TemplateVersionID *uuid.UUID            `gorm:"type:uuid;index" json:"templateVersionId,omitempty"`
	TemplateCode      string                `gorm:"size:50" json:"templateCode,omitempty"`
	TemplateName      string                `gorm:"size:150" json:"templateName,omitempty"`
	TemplateRevision  int                   `gorm:"not null;default:0" json:"templateRevision,omitempty"`
	TemplateSnapshot  *RackTemplateSnapshot `gorm:"type:jsonb;serializer:json" json:"templateSnapshot,omitempty"`
	DataCenterID      uuid.UUID             `gorm:"type:uuid;not null;index" json:"dataCenterId"`
	RoomID            uuid.UUID             `gorm:"type:uuid;not null;index;uniqueIndex:idx_racks_room_code,where:deleted_at IS NULL" json:"roomId"`
	Code              string                `gorm:"size:50;not null;uniqueIndex:idx_racks_room_code,where:deleted_at IS NULL" json:"code"`
	Name              string                `gorm:"size:150;not null;index" json:"name"`
	Type              string                `gorm:"size:50;not null;default:STANDARD" json:"type"`
	Manufacturer      string                `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber       string                `gorm:"size:100" json:"modelNumber,omitempty"`
	SerialNumber      string                `gorm:"size:100" json:"serialNumber,omitempty"`
	AssetNumber       string                `gorm:"size:100" json:"assetNumber,omitempty"`
	UHeight           int                   `gorm:"not null;default:42" json:"uHeight"`
	WidthMm           int                   `gorm:"not null;default:600" json:"widthMm"`
	DepthMm           int                   `gorm:"not null;default:1200" json:"depthMm"`
	HeightMm          int                   `gorm:"not null;default:2000" json:"heightMm"`
	LoadCapacityKg    *float64              `json:"loadCapacityKg,omitempty"`
	Zone              string                `gorm:"size:100" json:"zone,omitempty"`
	RackRow           string                `gorm:"size:50" json:"rackRow,omitempty"`
	RackColumn        string                `gorm:"size:50" json:"rackColumn,omitempty"`
	Aisle             string                `gorm:"size:100" json:"aisle,omitempty"`
	XCoordinate       *float64              `json:"xCoordinate,omitempty"`
	YCoordinate       *float64              `json:"yCoordinate,omitempty"`
	Rotation          int                   `gorm:"not null;default:0" json:"rotation"`
	Status            string                `gorm:"size:30;not null;default:AVAILABLE;index" json:"status"`
	Manager           string                `gorm:"size:100" json:"manager,omitempty"`
	Department        string                `gorm:"size:100" json:"department,omitempty"`
	Purpose           string                `gorm:"size:200" json:"purpose,omitempty"`
	DualPower         bool                  `gorm:"not null;default:false" json:"dualPower"`
	InputCircuits     int                   `gorm:"not null;default:0" json:"inputCircuits"`
	RatedVoltage      *float64              `json:"ratedVoltage,omitempty"`
	RatedCurrent      *float64              `json:"ratedCurrent,omitempty"`
	RatedPowerKw      *float64              `json:"ratedPowerKw,omitempty"`
	PeakPowerKw       *float64              `json:"peakPowerKw,omitempty"`
	PDUCount          int                   `gorm:"not null;default:0" json:"pduCount"`
	Remarks           string                `gorm:"type:text" json:"remarks,omitempty"`
	SortOrder         int                   `gorm:"not null;default:0;index" json:"sortOrder"`
}
