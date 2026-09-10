package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ApprovalPending  = "PENDING"
	ApprovalApproved = "APPROVED"
	ApprovalRejected = "REJECTED"

	OpAssign = "ASSIGN"
	OpMove   = "MOVE"
)

type OperationPolicy struct {
	BaseModel
	AssignApprovalEnabled bool   `gorm:"not null;default:false" json:"assignApprovalEnabled"`
	MoveApprovalEnabled   bool   `gorm:"not null;default:false" json:"moveApprovalEnabled"`
	ApprovalMode          string `gorm:"size:20;not null;default:SINGLE" json:"approvalMode"`
	DefaultApproverRole   string `gorm:"size:100;not null;default:system_admin" json:"defaultApproverRole"`
}

type ApprovalRecord struct {
	BaseModel
	DeviceID               uuid.UUID         `gorm:"type:uuid;not null;index" json:"deviceId"`
	Operation              string            `gorm:"size:30;not null;index" json:"operation"`
	Status                 string            `gorm:"size:20;not null;default:PENDING;index" json:"status"`
	RequestedBy            *uuid.UUID        `gorm:"type:uuid;index" json:"requestedBy,omitempty"`
	RequestedAt            time.Time         `gorm:"not null" json:"requestedAt"`
	RequestedDeviceVersion uint              `gorm:"not null" json:"requestedDeviceVersion"`
	SourcePosition         *PositionSnapshot `gorm:"type:jsonb;serializer:json" json:"sourcePosition,omitempty"`
	TargetRackID           uuid.UUID         `gorm:"type:uuid;not null;index" json:"targetRackId"`
	TargetStartU           int               `gorm:"not null" json:"targetStartU"`
	TargetHeightU          int               `gorm:"not null" json:"targetHeightU"`
	TargetOrientation      string            `gorm:"size:20;not null;default:NORMAL" json:"targetOrientation"`
	Reason                 string            `gorm:"size:500" json:"reason,omitempty"`
	DecidedBy              *uuid.UUID        `gorm:"type:uuid;index" json:"decidedBy,omitempty"`
	DecidedAt              *time.Time        `json:"decidedAt,omitempty"`
	DecisionComment        string            `gorm:"size:500" json:"decisionComment,omitempty"`
	Device                 *Device           `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}
