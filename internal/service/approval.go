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
		// 厂商语义（S08-APPROVE-STALE-REJECTED）：version 校验优先于状态——
		// 携带过期 version 的重复决定返回 RESOURCE_VERSION_CONFLICT，而非状态冲突
		if version != 0 && version != rec.Version {
			return nil, apperr.ResourceVersion()
		}
		return nil, apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已处理")
	}
	if actor == nil {
		return nil, apperr.Forbidden()
	}
	// 审批决定与审计同事务（合规敏感动作不做 best-effort）
	if err := s.store.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.store.WithTx(tx).Decide(id, version, model.ApprovalRejected, *actor, comment); err != nil {
			// 条件更新失败：区分 version 过期与已被并发处理（同 Approve 的厂商语义）
			if repository.IsVersionConflict(err) {
				return s.classifyDecideConflict(s.store.DB(), id)
			}
			return err
		}
		rid := rec.ID
		return tx.Create(&model.AuditLog{
			UserID: actor, Action: "APPROVAL_REJECT", ResourceType: "approval", ResourceID: &rid,
			AfterJSON: auditJSON(map[string]any{"deviceId": rec.DeviceID, "operation": rec.Operation, "comment": comment}),
			Result:    "SUCCESS", Source: "api",
		}).Error
	}); err != nil {
		return nil, err
	}
	return s.store.GetApproval(id)
}

// classifyDecideConflict 区分条件占单失败的两种语义（厂商基线，S08-APPROVE-STALE-REJECTED）：
// 审批单仍是 PENDING → 请求携带的 version 已过期（RESOURCE_VERSION_CONFLICT）；
// 状态已变 → 被并发处理（APPROVAL_STATE_CONFLICT）。必须在同一事务内读当前状态。
func (s *ApprovalService) classifyDecideConflict(tx *gorm.DB, id uuid.UUID) error {
	cur, err := s.store.WithTx(tx).GetApproval(id)
	if err == nil && cur.Status == model.ApprovalPending {
		return apperr.ResourceVersion()
	}
	return apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已被并发处理，请刷新")
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
		// 同 Reject：version 校验优先于状态检查（厂商基线语义）
		if version != 0 && version != rec.Version {
			return nil, apperr.ResourceVersion()
		}
		return nil, apperr.New(409, "APPROVAL_STATE_CONFLICT", "审批单已处理")
	}
	if actor == nil {
		return nil, apperr.Forbidden()
	}
	var placed *model.Device
	err = s.store.DB().Transaction(func(tx *gorm.DB) error {
		// 1. 条件更新审批单：仍为 PENDING（且 version 匹配时校验乐观锁），并发批准只有一个成功。
		//    失败时区分 version 过期与被并发处理（厂商语义不同）
		if err := s.store.WithTx(tx).Decide(id, version, model.ApprovalApproved, *actor, comment); err != nil {
			if repository.IsVersionConflict(err) {
				return s.classifyDecideConflict(tx, id)
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
			// 厂商基线（S08-APPROVE-STALE-REJECTED）：设备在申请后被修改 → RESOURCE_VERSION_CONFLICT
			return apperr.ResourceVersion()
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
		if err != nil {
			return err
		}
		// 审批决定审计与业务同事务（合规敏感动作不做 best-effort）
		rid := rec.ID
		return tx.Create(&model.AuditLog{
			UserID: actor, Action: "APPROVAL_APPROVE", ResourceType: "approval", ResourceID: &rid,
			AfterJSON: auditJSON(map[string]any{"deviceId": rec.DeviceID, "operation": rec.Operation, "comment": comment}),
			Result:    "SUCCESS", Source: "api",
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return placed, nil
}
