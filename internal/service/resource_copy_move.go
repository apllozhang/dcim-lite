package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
)

type CopyMoveInput struct {
	Code               string     `json:"code" binding:"required,max=50"`
	Name               string     `json:"name" binding:"required,max=150"`
	TargetDataCenterID *uuid.UUID `json:"targetDataCenterId"`
	TargetRoomID       *uuid.UUID `json:"targetRoomId"`
	// Version 为厂商基线风格：move 类接口的乐观锁版本可以放在请求体（套件与前端用 body，重建原先只认 query）。
	Version uint `json:"version"`
}

// checkCopyVersion 厂商对 copy 类接口校验请求体 version（差分用例 S07-DC-COPY-STALE-VERSION 等）。
// 未提供 version（0）时按旧行为放行，提供则必须与当前版本一致。
func checkCopyVersion(current, bodyVersion uint) error {
	if bodyVersion > 0 && bodyVersion != current {
		return apperr.New(409, "RESOURCE_VERSION_CONFLICT", "数据已被其他用户修改，请刷新后重试")
	}
	return nil
}

func (s *ResourceService) CopyDataCenter(id uuid.UUID, in CopyMoveInput) (*model.DataCenter, error) {
	src, err := s.store.GetDataCenter(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("数据中心")
		}
		return nil, err
	}
	if err := checkCopyVersion(src.Version, in.Version); err != nil {
		return nil, err
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	exists, err := s.store.CodeExistsDataCenter(in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}

	var created *model.DataCenter
	err = s.store.DB().Transaction(func(tx *gorm.DB) error {
		dst := *src
		dst.ID = uuid.New()
		dst.CreatedAt, dst.UpdatedAt = src.CreatedAt, src.UpdatedAt
		dst.Version = 1
		dst.Code, dst.Name = in.Code, in.Name
		dst.DeletedAt = gorm.DeletedAt{}
		dst.Rooms = nil
		if err := tx.Create(&dst).Error; err != nil {
			return err
		}

		var rooms []model.Room
		if err := tx.Unscoped().Where("data_center_id = ? AND deleted_at IS NULL", id).Find(&rooms).Error; err != nil {
			return err
		}
		for _, room := range rooms {
			newRoom := room
			newRoom.ID = uuid.New()
			newRoom.Version = 1
			newRoom.DataCenterID = dst.ID
			newRoom.DeletedAt = gorm.DeletedAt{}
			newRoom.Racks = nil
			if err := tx.Create(&newRoom).Error; err != nil {
				return err
			}
			var racks []model.Rack
			if err := tx.Unscoped().Where("room_id = ? AND deleted_at IS NULL", room.ID).Find(&racks).Error; err != nil {
				return err
			}
			for _, rack := range racks {
				newRack := rack
				newRack.ID = uuid.New()
				newRack.Version = 1
				newRack.RoomID = newRoom.ID
				newRack.DataCenterID = dst.ID
				newRack.DeletedAt = gorm.DeletedAt{}
				if err := tx.Create(&newRack).Error; err != nil {
					return err
				}
			}
		}
		created = &dst
		return nil
	})
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.GetDataCenter(created.ID)
}

func (s *ResourceService) CopyRoom(id uuid.UUID, in CopyMoveInput) (*model.Room, error) {
	src, err := s.store.GetRoom(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机房")
		}
		return nil, err
	}
	if err := checkCopyVersion(src.Version, in.Version); err != nil {
		return nil, err
	}
	targetDC := src.DataCenterID
	if in.TargetDataCenterID != nil {
		targetDC = *in.TargetDataCenterID
	}
	target, err := s.store.GetDataCenter(targetDC)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("目标数据中心")
		}
		return nil, err
	}
	if target.Status == model.StatusDisabled || target.Status == model.StatusArchived {
		return nil, apperr.ParentDisabled()
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	exists, err := s.store.CodeExistsRoom(targetDC, in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}

	var created *model.Room
	err = s.store.DB().Transaction(func(tx *gorm.DB) error {
		dst := *src
		dst.ID = uuid.New()
		dst.Version = 1
		dst.DataCenterID = targetDC
		dst.Code, dst.Name = in.Code, in.Name
		dst.DeletedAt = gorm.DeletedAt{}
		dst.Racks = nil
		if err := tx.Create(&dst).Error; err != nil {
			return err
		}
		var racks []model.Rack
		if err := tx.Unscoped().Where("room_id = ? AND deleted_at IS NULL", id).Find(&racks).Error; err != nil {
			return err
		}
		for _, rack := range racks {
			newRack := rack
			newRack.ID = uuid.New()
			newRack.Version = 1
			newRack.RoomID = dst.ID
			newRack.DataCenterID = targetDC
			newRack.DeletedAt = gorm.DeletedAt{}
			if err := tx.Create(&newRack).Error; err != nil {
				return err
			}
		}
		created = &dst
		return nil
	})
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.GetRoom(created.ID)
}

func (s *ResourceService) CopyRack(id uuid.UUID, in CopyMoveInput) (*model.Rack, error) {
	src, err := s.store.GetRack(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	if err := checkCopyVersion(src.Version, in.Version); err != nil {
		return nil, err
	}
	targetRoomID := src.RoomID
	if in.TargetRoomID != nil {
		targetRoomID = *in.TargetRoomID
	}
	room, err := s.store.GetRoom(targetRoomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("目标机房")
		}
		return nil, err
	}
	if room.Status == model.StatusDisabled || room.Status == model.StatusArchived {
		return nil, apperr.ParentDisabled()
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	exists, err := s.store.CodeExistsRack(targetRoomID, in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}
	dst := *src
	dst.ID = uuid.New()
	dst.Version = 1
	dst.RoomID = targetRoomID
	dst.DataCenterID = room.DataCenterID
	dst.Code, dst.Name = in.Code, in.Name
	dst.DeletedAt = gorm.DeletedAt{}
	if err := s.store.CreateRack(&dst); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.GetRack(dst.ID)
}

func (s *ResourceService) MoveRoom(id uuid.UUID, version uint, in CopyMoveInput) (*model.Room, error) {
	if in.TargetDataCenterID == nil {
		return nil, apperr.InvalidResource("缺少 targetDataCenterId")
	}
	src, err := s.store.GetRoom(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机房")
		}
		return nil, err
	}
	if src.DataCenterID == *in.TargetDataCenterID {
		return nil, apperr.InvalidResource("目标数据中心与当前相同")
	}
	target, err := s.store.GetDataCenter(*in.TargetDataCenterID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("目标数据中心")
		}
		return nil, err
	}
	if target.Status == model.StatusDisabled || target.Status == model.StatusArchived {
		return nil, apperr.ParentDisabled()
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, apperr.InvalidResource("迁移需要提供新编码和名称")
	}
	exists, err := s.store.CodeExistsRoom(target.ID, in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}

	err = s.store.DB().Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Room{}).
			Where("id = ? AND version = ? AND deleted_at IS NULL", id, version).
			Updates(map[string]any{
				"data_center_id": target.ID,
				"code":           in.Code,
				"name":           in.Name,
				"version":        gorm.Expr("version + 1"),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("resource version conflict")
		}
		// 同步机柜上的 data_center_id 冗余字段
		if err := tx.Model(&model.Rack{}).
			Where("room_id = ? AND deleted_at IS NULL", id).
			Update("data_center_id", target.ID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if repository.IsVersionConflict(err) || err.Error() == "resource version conflict" {
			return nil, apperr.ResourceVersion()
		}
		return nil, mapStoreErr(err)
	}
	return s.store.GetRoom(id)
}

func (s *ResourceService) MoveRack(id uuid.UUID, version uint, in CopyMoveInput) (*model.Rack, error) {
	if in.TargetRoomID == nil {
		return nil, apperr.InvalidResource("缺少 targetRoomId")
	}
	src, err := s.store.GetRack(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("机柜")
		}
		return nil, err
	}
	if src.RoomID == *in.TargetRoomID {
		return nil, apperr.InvalidResource("目标机房与当前相同")
	}
	targetRoom, err := s.store.GetRoom(*in.TargetRoomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("目标机房")
		}
		return nil, err
	}
	if targetRoom.Status == model.StatusDisabled || targetRoom.Status == model.StatusArchived {
		return nil, apperr.ParentDisabled()
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if in.Code == "" || in.Name == "" {
		return nil, apperr.InvalidResource("迁移需要提供新编码和名称")
	}
	exists, err := s.store.CodeExistsRack(targetRoom.ID, in.Code, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.DuplicateCode()
	}

	res := s.store.DB().Model(&model.Rack{}).
		Where("id = ? AND version = ? AND deleted_at IS NULL", id, version).
		Updates(map[string]any{
			"room_id":        targetRoom.ID,
			"data_center_id": targetRoom.DataCenterID,
			"code":           in.Code,
			"name":           in.Name,
			"version":        gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return nil, mapStoreErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, apperr.ResourceVersion()
	}
	return s.store.GetRack(id)
}
