package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	DeviceWaitingRack    = "WAITING_RACK"
	DeviceRunning        = "RUNNING"
	DeviceMaintenance    = "MAINTENANCE"
	DevicePendingRemoval = "PENDING_REMOVAL"
	DeviceOffRack        = "OFF_RACK"
	DeviceScrapped       = "SCRAPPED"

	OrientNormal  = "NORMAL"
	OrientReverse = "REVERSE"
)

type DeviceType struct {
	BaseModel
	Code               string   `gorm:"size:50;not null;uniqueIndex:idx_device_types_code_active,where:deleted_at IS NULL" json:"code"`
	Name               string   `gorm:"size:150;not null" json:"name"`
	Category           string   `gorm:"size:40;not null" json:"category"`
	Status             string   `gorm:"size:20;not null;default:ACTIVE" json:"status"`
	DefaultHeightU     int      `gorm:"not null;default:1" json:"defaultHeightU"`
	DefaultWeightKg    *float64 `json:"defaultWeightKg,omitempty"`
	DefaultRatedPowerW *float64 `json:"defaultRatedPowerW,omitempty"`
	DefaultPeakPowerW  *float64 `json:"defaultPeakPowerW,omitempty"`
	DefaultDualPower   bool     `gorm:"not null;default:false" json:"defaultDualPower"`
	Description        string   `gorm:"type:text" json:"description,omitempty"`
	SortOrder          int      `gorm:"not null;default:0" json:"sortOrder"`
}

type Device struct {
	BaseModel
	TypeID             uuid.UUID   `gorm:"type:uuid;not null;index" json:"typeId"`
	Type               *DeviceType `gorm:"foreignKey:TypeID" json:"type,omitempty"`
	Code               string      `gorm:"size:80;not null;uniqueIndex:idx_devices_code_active,where:deleted_at IS NULL" json:"code"`
	Name               string      `gorm:"size:150;not null;index" json:"name"`
	AssetNumber        string      `gorm:"size:100;uniqueIndex:idx_devices_asset_active,where:deleted_at IS NULL AND asset_number <> ''" json:"assetNumber,omitempty"`
	SerialNumber       string      `gorm:"size:120;index" json:"serialNumber,omitempty"`
	Manufacturer       string      `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber        string      `gorm:"size:100" json:"modelNumber,omitempty"`
	Specification      string      `gorm:"size:500" json:"specification,omitempty"`
	FirmwareVersion    string      `gorm:"size:100" json:"firmwareVersion,omitempty"`
	PurchaseBatch      string      `gorm:"size:100" json:"purchaseBatch,omitempty"`
	WarrantyExpiresAt  *time.Time  `json:"warrantyExpiresAt,omitempty"`
	Organization       string      `gorm:"size:150" json:"organization,omitempty"`
	Manager            string      `gorm:"size:100" json:"manager,omitempty"`
	Contact            string      `gorm:"size:100" json:"contact,omitempty"`
	BusinessSystem     string      `gorm:"size:150" json:"businessSystem,omitempty"`
	ApplicationName    string      `gorm:"size:150" json:"applicationName,omitempty"`
	LifecycleStatus    string      `gorm:"size:30;not null;default:WAITING_RACK;index" json:"lifecycleStatus"`
	HeightU            int         `gorm:"not null;default:1" json:"heightU"`
	WidthMm            *int64      `json:"widthMm,omitempty"`
	DepthMm            *int64      `json:"depthMm,omitempty"`
	HeightMm           *int64      `json:"heightMm,omitempty"`
	WeightKg           *float64    `json:"weightKg,omitempty"`
	RatedPowerW        *float64    `json:"ratedPowerW,omitempty"`
	PeakPowerW         *float64    `json:"peakPowerW,omitempty"`
	InputVoltage       *float64    `json:"inputVoltage,omitempty"`
	DualPowerRequired  bool        `gorm:"not null;default:false" json:"dualPowerRequired"`
	ManagementIP       string      `gorm:"size:64" json:"managementIp,omitempty"`
	BusinessIP         string      `gorm:"size:64" json:"businessIp,omitempty"`
	MACAddress         string      `gorm:"size:64" json:"macAddress,omitempty"`
	ManagementProtocol string      `gorm:"size:100" json:"managementProtocol,omitempty"`
	MonitoringStatus   string      `gorm:"size:50" json:"monitoringStatus,omitempty"`
	ExternalQRCode     string      `gorm:"size:255" json:"externalQrCode,omitempty"`
	ExternalQRCodeURL  string      `gorm:"size:500" json:"externalQrCodeUrl,omitempty"`
	Tags               string      `gorm:"size:500" json:"tags,omitempty"`
	Remarks            string      `gorm:"type:text" json:"remarks,omitempty"`
}

type RackDevicePosition struct {
	BaseModel
	DeviceID     uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:idx_rack_device_positions_device_active,where:deleted_at IS NULL" json:"deviceId"`
	RackID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"rackId"`
	RoomID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"roomId"`
	DataCenterID uuid.UUID  `gorm:"type:uuid;not null;index" json:"dataCenterId"`
	StartU       int        `gorm:"not null" json:"startU"`
	HeightU      int        `gorm:"not null" json:"heightU"`
	EndU         int        `gorm:"not null" json:"endU"`
	Orientation  string     `gorm:"size:20;not null;default:NORMAL" json:"orientation"`
	InstalledAt  time.Time  `gorm:"not null" json:"installedAt"`
	InstalledBy  *uuid.UUID `gorm:"type:uuid;index" json:"installedBy,omitempty"`
	Reason       string     `gorm:"size:500" json:"reason,omitempty"`
}

type RackUOccupancy struct {
	BaseModel
	PositionID uuid.UUID `gorm:"type:uuid;not null;index" json:"positionId"`
	DeviceID   uuid.UUID `gorm:"type:uuid;not null;index" json:"deviceId"`
	RackID     uuid.UUID `gorm:"type:uuid;not null;index" json:"rackId"`
	StartU     int       `gorm:"not null" json:"startU"`
	EndU       int       `gorm:"not null" json:"endU"`
}

type DevicePositionHistory struct {
	BaseModel
	DeviceID     uuid.UUID         `gorm:"type:uuid;not null;index" json:"deviceId"`
	Operation    string            `gorm:"size:30;not null" json:"operation"`
	FromPosition *PositionSnapshot `gorm:"type:jsonb;serializer:json" json:"fromPosition,omitempty"`
	ToPosition   *PositionSnapshot `gorm:"type:jsonb;serializer:json" json:"toPosition,omitempty"`
	Reason       string            `gorm:"size:500" json:"reason,omitempty"`
	ActorID      *uuid.UUID        `gorm:"type:uuid" json:"actorId,omitempty"`
	RequestID    string            `gorm:"size:100" json:"requestId,omitempty"`
}

type PositionSnapshot struct {
	RackID       uuid.UUID `json:"rackId,omitempty"`
	RoomID       uuid.UUID `json:"roomId,omitempty"`
	DataCenterID uuid.UUID `json:"dataCenterId,omitempty"`
	StartU       int       `json:"startU,omitempty"`
	HeightU      int       `json:"heightU,omitempty"`
	EndU         int       `json:"endU,omitempty"`
	Orientation  string    `json:"orientation,omitempty"`
}
