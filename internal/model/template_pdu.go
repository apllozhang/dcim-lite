package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	TemplateActive   = "ACTIVE"
	TemplateDisabled = "DISABLED"

	PDUActive = "ACTIVE"

	SocketAvailable = "AVAILABLE"
	SocketConnected = "CONNECTED"

	RolePrimary = "PRIMARY"
	RoleStandby = "STANDBY"
)

type RackTemplate struct {
	BaseModel
	Code            string                `gorm:"size:50;not null;uniqueIndex:idx_rack_templates_code_active,where:deleted_at IS NULL" json:"code"`
	Name            string                `gorm:"size:150;not null" json:"name"`
	Description     string                `gorm:"type:text" json:"description,omitempty"`
	Status          string                `gorm:"size:20;not null;default:ACTIVE" json:"status"`
	IsSystem        bool                  `gorm:"not null;default:false" json:"isSystem"`
	CurrentRevision int                   `gorm:"not null;default:1" json:"currentRevision"`
	Remarks         string                `gorm:"type:text" json:"remarks,omitempty"`
	Versions        []RackTemplateVersion `gorm:"foreignKey:TemplateID" json:"versions,omitempty"`
}

type RackTemplateVersion struct {
	BaseModel
	TemplateID      uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:idx_rack_template_revision_active,where:deleted_at IS NULL" json:"templateId"`
	Revision        int        `gorm:"not null;uniqueIndex:idx_rack_template_revision_active,where:deleted_at IS NULL" json:"revision"`
	Type            string     `gorm:"size:50;not null;default:STANDARD" json:"type"`
	Manufacturer    string     `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber     string     `gorm:"size:100" json:"modelNumber,omitempty"`
	UHeight         int        `gorm:"not null;default:42" json:"uHeight"`
	WidthMm         int        `gorm:"not null;default:600" json:"widthMm"`
	DepthMm         int        `gorm:"not null;default:1200" json:"depthMm"`
	HeightMm        int        `gorm:"not null;default:2000" json:"heightMm"`
	LoadCapacityKg  *float64   `json:"loadCapacityKg,omitempty"`
	DualPower       bool       `gorm:"not null;default:false" json:"dualPower"`
	InputCircuits   int        `gorm:"not null;default:0" json:"inputCircuits"`
	RatedVoltage    *float64   `json:"ratedVoltage,omitempty"`
	RatedCurrent    *float64   `json:"ratedCurrent,omitempty"`
	RatedPowerKw    *float64   `json:"ratedPowerKw,omitempty"`
	PeakPowerKw     *float64   `json:"peakPowerKw,omitempty"`
	PDUCount        int        `gorm:"not null;default:0" json:"pduCount"`
	ChangeNote      string     `gorm:"size:500" json:"changeNote,omitempty"`
	CreatedBy       *uuid.UUID `gorm:"type:uuid" json:"createdBy,omitempty"`
}

type PDU struct {
	BaseModel
	RackID        uuid.UUID `gorm:"type:uuid;not null;index" json:"rackId"`
	Code          string    `gorm:"size:80;not null;uniqueIndex:idx_pdus_code_active,where:deleted_at IS NULL" json:"code"`
	Name          string    `gorm:"size:150;not null" json:"name"`
	Manufacturer  string    `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber   string    `gorm:"size:100" json:"modelNumber,omitempty"`
	SerialNumber  string    `gorm:"size:120" json:"serialNumber,omitempty"`
	InputVoltage  *float64  `json:"inputVoltage,omitempty"`
	RatedPowerW   *float64  `json:"ratedPowerW,omitempty"`
	RatedCurrentA *float64  `json:"ratedCurrentA,omitempty"`
	Status        string    `gorm:"size:20;not null;default:ACTIVE" json:"status"`
	Remarks       string    `gorm:"type:text" json:"remarks,omitempty"`
}

type PDUSocket struct {
	BaseModel
	PDUID     uuid.UUID `gorm:"column:pdu_id;type:uuid;not null;index;uniqueIndex:idx_pdu_sockets_number_active,where:deleted_at IS NULL" json:"pduId"`
	SocketNo  int       `gorm:"not null;uniqueIndex:idx_pdu_sockets_number_active,where:deleted_at IS NULL" json:"socketNo"`
	Standard  string    `gorm:"size:10;not null" json:"standard"`
	AmperageA int       `gorm:"not null" json:"amperageA"`
	Label     string    `gorm:"size:100" json:"label,omitempty"`
	Status    string    `gorm:"size:20;not null;default:AVAILABLE" json:"status"`
}

type PDUConnection struct {
	BaseModel
	SocketID       uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:idx_pdu_connections_socket_active,where:deleted_at IS NULL" json:"socketId"`
	DeviceID       uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:idx_pdu_connections_device_role_active,where:deleted_at IS NULL" json:"deviceId"`
	PowerW         *float64   `json:"powerW,omitempty"`
	Circuit        string     `gorm:"size:100" json:"circuit,omitempty"`
	RedundancyRole string     `gorm:"size:20;not null;default:PRIMARY;uniqueIndex:idx_pdu_connections_device_role_active,where:deleted_at IS NULL" json:"redundancyRole"`
	ConnectedAt    time.Time  `gorm:"not null" json:"connectedAt"`
	ConnectedBy    *uuid.UUID `gorm:"type:uuid" json:"connectedBy,omitempty"`
}
