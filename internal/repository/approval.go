package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/model"
)

type ApprovalStore struct{ db *gorm.DB }

func NewApprovalStore(db *gorm.DB) *ApprovalStore { return &ApprovalStore{db: db} }

func (s *ApprovalStore) DB() *gorm.DB { return s.db }

func (s *ApprovalStore) GetPolicy() (*model.OperationPolicy, error) {
	var p model.OperationPolicy
	err := s.db.Order("created_at asc").First(&p).Error
	if err == gorm.ErrRecordNotFound {
		p = model.OperationPolicy{
			AssignApprovalEnabled: false,
			MoveApprovalEnabled:   false,
			ApprovalMode:          "SINGLE",
			DefaultApproverRole:   "system_admin",
		}
		if err := s.db.Create(&p).Error; err != nil {
			return nil, err
		}
		return &p, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ApprovalStore) SavePolicy(p *model.OperationPolicy) error {
	if p.ID == uuid.Nil {
		existing, err := s.GetPolicy()
		if err != nil {
			return err
		}
		p.ID = existing.ID
		p.Version = existing.Version
	}
	res := s.db.Model(&model.OperationPolicy{}).
		Where("id = ? AND version = ?", p.ID, p.Version).
		Updates(map[string]any{
			"assign_approval_enabled": p.AssignApprovalEnabled,
			"move_approval_enabled":   p.MoveApprovalEnabled,
			"approval_mode":           p.ApprovalMode,
			"default_approver_role":   p.DefaultApproverRole,
			"version":                 gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return s.db.First(p, "id = ?", p.ID).Error
}

func (s *ApprovalStore) ListApprovals(status string) ([]model.ApprovalRecord, error) {
	q := s.db.Preload("Device").Preload("Device.Type")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var items []model.ApprovalRecord
	err := q.Order("requested_at desc").Find(&items).Error
	return items, err
}

func (s *ApprovalStore) GetApproval(id uuid.UUID) (*model.ApprovalRecord, error) {
	var item model.ApprovalRecord
	err := s.db.Preload("Device").First(&item, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ApprovalStore) CreateApproval(item *model.ApprovalRecord) error {
	return s.db.Create(item).Error
}

func (s *ApprovalStore) Decide(id uuid.UUID, status string, actor uuid.UUID, comment string) error {
	now := time.Now()
	res := s.db.Model(&model.ApprovalRecord{}).
		Where("id = ? AND status = ?", id, model.ApprovalPending).
		Updates(map[string]any{
			"status":           status,
			"decided_by":       actor,
			"decided_at":       now,
			"decision_comment": comment,
			"version":          gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errVersion()
	}
	return nil
}
