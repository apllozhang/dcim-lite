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
	// 兼容厂商的 rackId 字段：直接用 in.TargetRackID 时，传 rackId 会写入零 UUID，
	// 触发 approval_records_target_rack_id_fkey 外键违约（500）。
	targetRackID, err := in.desiredRackID()
	if err != nil {
		return nil, err
	}
	rec := &model.ApprovalRecord{
		DeviceID: device.ID, Operation: op, Status: model.ApprovalPending,
		RequestedBy: actor, RequestedAt: time.Now(),
		RequestedDeviceVersion: device.Version,
		SourcePosition:         src,
		TargetRackID:           targetRackID,
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

func (s *ApprovalService) Reject(id uuid.UUID, version uint, comment string, actor *uuid.UUID) (*model.ApprovalRecord, error) {
	rec, err := s.store.GetApproval(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("审批单")
		}
		return nil, err
	}
	if rec.Status != model.ApprovalPending {
		return nil, apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已处理")
	}
	if actor == nil {
		return nil, apperr.Forbidden()
	}
	if err := s.store.Decide(id, version, model.ApprovalRejected, *actor, comment); err != nil {
		// 条件更新失败：并发已处理或 version 过期，统一按状态冲突拒绝
		if repository.IsVersionConflict(err) {
			return nil, apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已被并发处理或版本过期，请刷新")
		}
		return nil, mapStoreErr(err)
	}
	return s.store.GetApproval(id)
}

// Approve 在单个数据库事务内完成：条件更新审批单（乐观锁占单）→ 设备上架/移位 → 履历。
// 占单失败或执行失败均整体回滚，保证审批决定与副作用严格一次。
func (s *ApprovalService) Approve(id uuid.UUID, version uint, comment string, actor *uuid.UUID, requestID string) (*model.Device, error) {
	rec, err := s.store.GetApproval(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("审批单")
		}
		return nil, err
	}
	if rec.Status != model.ApprovalPending {
		return nil, apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已处理")
	}
	if actor == nil {
		return nil, apperr.Forbidden()
	}
	var placed *model.Device
	err = s.store.DB().Transaction(func(tx *gorm.DB) error {
		// 1. 条件更新审批单：仍为 PENDING（且 version 匹配时校验乐观锁），并发批准只有一个成功
		if err := s.store.WithTx(tx).Decide(id, version, model.ApprovalApproved, *actor, comment); err != nil {
			if repository.IsVersionConflict(err) {
				return apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已被并发处理或版本过期，请刷新")
			}
			return err
		}
		// 2. 锁定设备行并校验自申请以来未被修改：锁内校验、锁内执行，
		//    消除"版本检查通过后设备被并发编辑/移位"的竞态窗口
		dev, err := s.devices.WithTx(tx).GetDeviceLock(rec.DeviceID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("设备")
			}
			return err
		}
		if rec.RequestedDeviceVersion != 0 && dev.Version != rec.RequestedDeviceVersion {
			return apperr.New(409, "APPROVAL_STATE_CONFLICT", "设备已被修改，请重新申请")
		}
		// 3. 执行设备位置变更（与审批决定同事务，含履历）
		op, requireOff := model.OpAssign, true
		if rec.Operation == model.OpMove {
			op, requireOff = model.OpMove, false
		}
		placed, err = s.devSvc.PlaceTx(tx, rec.DeviceID, PositionChangeInput{
			TargetRackID: rec.TargetRackID,
			StartU:       rec.TargetStartU,
			Orientation:  rec.TargetOrientation,
			Reason:       rec.Reason,
		}, actor, requestID, op, requireOff)
		return err
	})
	if err != nil {
		return nil, err
	}
	return placed, nil
}
