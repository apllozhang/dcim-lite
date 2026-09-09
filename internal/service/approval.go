package service

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

type ApprovalService struct {
	store   *repository.ApprovalStore
	devices *repository.DeviceStore
	devSvc  *DeviceService
}

func NewApprovalService(store *repository.ApprovalStore, devices *repository.DeviceStore, devSvc *DeviceService) *ApprovalService {
	return &ApprovalService{store: store, devices: devices, devSvc: devSvc}
}

type PolicyInput struct {
	AssignApprovalEnabled *bool  `json:"assignApprovalEnabled"`
	MoveApprovalEnabled   *bool  `json:"moveApprovalEnabled"`
	ApprovalMode          string `json:"approvalMode"`
	DefaultApproverRole   string `json:"defaultApproverRole"`
}

type DecisionInput struct {
	Version uint   `json:"version"`
	Comment string `json:"comment"`
}

func (s *ApprovalService) GetPolicy() (*model.OperationPolicy, error) {
	return s.store.GetPolicy()
}

func (s *ApprovalService) UpdatePolicy(in PolicyInput) (*model.OperationPolicy, error) {
	p, err := s.store.GetPolicy()
	if err != nil {
		return nil, err
	}
	if in.AssignApprovalEnabled != nil {
		p.AssignApprovalEnabled = *in.AssignApprovalEnabled
	}
	if in.MoveApprovalEnabled != nil {
		p.MoveApprovalEnabled = *in.MoveApprovalEnabled
	}
	if in.ApprovalMode != "" {
		p.ApprovalMode = in.ApprovalMode
	}
	if in.DefaultApproverRole != "" {
		p.DefaultApproverRole = in.DefaultApproverRole
	}
	if err := s.store.SavePolicy(p); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.GetPolicy()
}

func (s *ApprovalService) List(status string) ([]model.ApprovalRecord, error) {
	return s.store.ListApprovals(status)
}

// RequiresApproval reports whether this op should go through approval.
func (s *ApprovalService) RequiresApproval(op string) (bool, error) {
	p, err := s.store.GetPolicy()
	if err != nil {
		return false, err
	}
	if op == model.OpAssign {
		return p.AssignApprovalEnabled, nil
	}
	if op == model.OpMove {
		return p.MoveApprovalEnabled, nil
	}
	return false, nil
}

func (s *ApprovalService) CreatePending(op string, device *model.Device, in PositionChangeInput, actor *uuid.UUID) (*model.ApprovalRecord, error) {
	orient := model.OrientNormal
	if in.Orientation != "" {
		orient = in.Orientation
	}
	var src *model.PositionSnapshot
	if pos, err := s.devices.GetActivePosition(device.ID); err == nil {
		src = &model.PositionSnapshot{
			RackID: pos.RackID, RoomID: pos.RoomID, DataCenterID: pos.DataCenterID,
			StartU: pos.StartU, HeightU: pos.HeightU, EndU: pos.EndU, Orientation: pos.Orientation,
		}
	}
	rec := &model.ApprovalRecord{
		DeviceID: device.ID, Operation: op, Status: model.ApprovalPending,
		RequestedBy: actor, RequestedAt: time.Now(),
		RequestedDeviceVersion: device.Version,
		SourcePosition:         src,
		TargetRackID:           in.TargetRackID,
		TargetStartU:           in.StartU,
		TargetHeightU:          device.HeightU,
		TargetOrientation:      orient,
		Reason:                 in.Reason,
	}
	if err := s.store.CreateApproval(rec); err != nil {
		return nil, err
	}
	rec.Device = device
	return rec, nil
}

func (s *ApprovalService) Reject(id uuid.UUID, comment string, actor *uuid.UUID) (*model.ApprovalRecord, error) {
	rec, err := s.store.GetApproval(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("审批单")
		}
		return nil, err
	}
	if rec.Status != model.ApprovalPending {
		return nil, apperr.New(409, "APPROVAL_STATE", "审批单已处理")
	}
	if actor == nil {
		return nil, apperr.Forbidden()
	}
	if err := s.store.Decide(id, model.ApprovalRejected, *actor, comment); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.GetApproval(id)
}

func (s *ApprovalService) Approve(id uuid.UUID, comment string, actor *uuid.UUID, requestID string) (*model.Device, error) {
	rec, err := s.store.GetApproval(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("审批单")
		}
		return nil, err
	}
	if rec.Status != model.ApprovalPending {
		return nil, apperr.New(409, "APPROVAL_STATE", "审批单已处理")
	}
	if actor == nil {
		return nil, apperr.Forbidden()
	}
	dev, err := s.devSvc.GetDevice(rec.DeviceID)
	if err != nil {
		return nil, err
	}
	if rec.RequestedDeviceVersion != 0 && dev.Version != rec.RequestedDeviceVersion {
		return nil, apperr.New(409, "APPROVAL_STATE", "设备已被修改，请重新申请")
	}
	op, requireOff := model.OpAssign, true
	if rec.Operation == model.OpMove {
		op, requireOff = model.OpMove, false
	}
	placed, err := s.devSvc.place(rec.DeviceID, PositionChangeInput{
		TargetRackID: rec.TargetRackID,
		StartU:       rec.TargetStartU,
		Orientation:  rec.TargetOrientation,
		Reason:       rec.Reason,
	}, actor, requestID, op, requireOff)
	if err != nil {
		return nil, err
	}
	if err := s.store.Decide(id, model.ApprovalApproved, *actor, comment); err != nil {
		return nil, mapStoreErr(err)
	}
	return placed, nil
}
