package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/model"
)

// Rack diagram import kinds (frontend labels).
const (
	KindUnchanged       = "UNCHANGED"
	KindCreateNew       = "CREATE_NEW"
	KindUpdateExisting  = "UPDATE_EXISTING"
	KindMoveExisting    = "MOVE_EXISTING"
	KindRemoveMissing   = "REMOVE_MISSING"
	KindNeedsDecision   = "NEEDS_DECISION"
	KindError           = "ERROR"
)

// Commit actions for decision items.
const (
	ActionUpdateExisting = "UPDATE_EXISTING"
	ActionCreateNew      = "CREATE_NEW"
	ActionDecommission   = "DECOMMISSION"
	ActionIgnore         = "IGNORE"
)

type ImportDeviceRow struct {
	ClientID            string   `json:"clientId"`
	RackID              string   `json:"rackId"`
	RackCode            string   `json:"rackCode,omitempty"`
	StartU              int      `json:"startU"`
	EndU                int      `json:"endU"`
	Name                string   `json:"name"`
	SerialNumber        string   `json:"serialNumber,omitempty"`
	RatedPowerW         *float64 `json:"ratedPowerW,omitempty"`
	TypeID              string   `json:"typeId,omitempty"`
	TypeCategory        string   `json:"typeCategory,omitempty"`
	SourceDeviceID      string   `json:"sourceDeviceId,omitempty"`
	SourceDeviceVersion uint     `json:"sourceDeviceVersion,omitempty"`
	SourceDeviceCode    string   `json:"sourceDeviceCode,omitempty"`
	SourceRackID        string   `json:"sourceRackId,omitempty"`
	SourceStartU        int      `json:"sourceStartU,omitempty"`
	SourceEndU          int      `json:"sourceEndU,omitempty"`
}

type ImportValidateInput struct {
	FormatVersion   string             `json:"formatVersion"`
	DataCenterID    string             `json:"dataCenterId"`
	RoomID          string             `json:"roomId"`
	ExportedAt      string             `json:"exportedAt,omitempty"`
	CoveredRackIDs  []string           `json:"coveredRackIds"`
	Devices         []ImportDeviceRow  `json:"devices"`
	DefaultTypeID   string             `json:"defaultTypeId"`
}

type ImportItem struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	RequiresDecision bool     `json:"requiresDecision"`
	ClientID         string   `json:"clientId,omitempty"`
	RackID           string   `json:"rackId,omitempty"`
	RackCode         string   `json:"rackCode,omitempty"`
	StartU           int      `json:"startU,omitempty"`
	EndU             int      `json:"endU,omitempty"`
	Name             string   `json:"name,omitempty"`
	SourceDeviceID   string   `json:"sourceDeviceId,omitempty"`
	SourceDeviceCode string   `json:"sourceDeviceCode,omitempty"`
	Message          string   `json:"message,omitempty"`
	AllowedActions   []string `json:"allowedActions,omitempty"`
}

type ImportSummary struct {
	Errors     int `json:"errors"`
	Create     int `json:"create"`
	Update     int `json:"update"`
	Move       int `json:"move"`
	Removals   int `json:"removals"`
	Decisions  int `json:"decisions"`
	Unchanged  int `json:"unchanged"`
}

type ImportValidateResult struct {
	Token   string        `json:"token"`
	Items   []ImportItem  `json:"items"`
	Summary ImportSummary `json:"summary"`
}

type ImportDecision struct {
	ItemID string `json:"itemId"`
	Action string `json:"action"`
}

type ImportCommitInput struct {
	Token     string           `json:"token" binding:"required"`
	Decisions []ImportDecision `json:"decisions"`
}

type ImportCommitResult struct {
	Created       int `json:"created"`
	Updated       int `json:"updated"`
	Moved         int `json:"moved"`
	Decommissioned int `json:"decommissioned"`
	Ignored       int `json:"ignored"`
}

type importDraft struct {
	Token     string
	RoomID    uuid.UUID
	CreatedAt time.Time
	Items     []ImportItem
	Rows      map[string]ImportDeviceRow // itemID -> row
	DefaultType uuid.UUID
}

type ImportDraftStore struct {
	mu    sync.Mutex
	drafts map[string]*importDraft
	ttl   time.Duration
}

func NewImportDraftStore() *ImportDraftStore {
	return &ImportDraftStore{drafts: map[string]*importDraft{}, ttl: 30 * time.Minute}
}

func (s *ImportDraftStore) Put(d *importDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gc()
	s.drafts[d.Token] = d
}

func (s *ImportDraftStore) Take(token string) (*importDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gc()
	d, ok := s.drafts[token]
	if ok {
		delete(s.drafts, token)
	}
	return d, ok
}

func (s *ImportDraftStore) gc() {
	now := time.Now()
	for k, v := range s.drafts {
		if now.Sub(v.CreatedAt) > s.ttl {
			delete(s.drafts, k)
		}
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "imp_" + hex.EncodeToString(b)
}

// ImportService implements rack-diagram two-phase import.
type ImportService struct {
	drafts  *ImportDraftStore
	devices *DeviceService
}

func NewImportService(drafts *ImportDraftStore, devices *DeviceService) *ImportService {
	return &ImportService{drafts: drafts, devices: devices}
}

func (s *ImportService) Validate(roomID uuid.UUID, in ImportValidateInput) (*ImportValidateResult, error) {
	room, err := s.devices.acks.GetRoom(roomID)
	if err != nil {
		return nil, apperr.NotFound("机房")
	}
	defaultType := uuid.Nil
	if in.DefaultTypeID != "" {
		defaultType, err = uuid.Parse(in.DefaultTypeID)
		if err != nil {
			return nil, apperr.InvalidResource("defaultTypeId 非法")
		}
		if _, err := s.devices.devices.GetDeviceType(defaultType); err != nil {
			return nil, apperr.NotFound("设备类型")
		}
	}
	if len(in.Devices) == 0 {
		return nil, apperr.InvalidResource("导入内容为空")
	}

	// index existing devices on covered racks
	covered := make([]uuid.UUID, 0, len(in.CoveredRackIDs))
	for _, id := range in.CoveredRackIDs {
		rid, err := uuid.Parse(id)
		if err != nil {
			return nil, apperr.InvalidResource("coveredRackIds 含非法 ID")
		}
		if _, err := s.devices.acks.GetRack(rid); err != nil {
			return nil, apperr.NotFound("机柜")
		}
		covered = append(covered, rid)
	}
	existingByRack := map[uuid.UUID][]*model.RackDevicePosition{}
	seenDevice := map[uuid.UUID]bool{}
	for _, rid := range covered {
		var positions []model.RackDevicePosition
		if err := s.devices.devices.DB().Where("rack_id = ? AND deleted_at IS NULL", rid).Find(&positions).Error; err != nil {
			return nil, err
		}
		for i := range positions {
			p := positions[i]
			existingByRack[rid] = append(existingByRack[rid], &p)
			seenDevice[p.DeviceID] = false
		}
	}

	items := make([]ImportItem, 0, len(in.Devices))
	rows := map[string]ImportDeviceRow{}
	summary := ImportSummary{}

	for i, row := range in.Devices {
		itemID := fmt.Sprintf("item-%d", i+1)
		item := ImportItem{
			ID: itemID, ClientID: row.ClientID,
			RackID: row.RackID, RackCode: row.RackCode,
			StartU: row.StartU, EndU: row.EndU, Name: row.Name,
			SourceDeviceID: row.SourceDeviceID, SourceDeviceCode: row.SourceDeviceCode,
		}
		if strings.TrimSpace(row.Name) == "" {
			item.Kind = KindError
			item.Message = "设备名称为空"
			summary.Errors++
			items = append(items, item)
			continue
		}
		rackID, err := uuid.Parse(row.RackID)
		if err != nil {
			item.Kind = KindError
			item.Message = "rackId 非法"
			summary.Errors++
			items = append(items, item)
			continue
		}
		if row.StartU < 1 || row.EndU < row.StartU {
			item.Kind = KindError
			item.Message = "U 位区间非法"
			summary.Errors++
			items = append(items, item)
			continue
		}
		height := row.EndU - row.StartU + 1

		// match source device
		var src *model.Device
		if row.SourceDeviceID != "" {
			sid, err := uuid.Parse(row.SourceDeviceID)
			if err == nil {
				if d, err := s.devices.GetDevice(sid); err == nil {
					src = d
				}
			}
		}
		if src == nil && row.SourceDeviceCode != "" {
			// fallback by code among covered rack devices
			for _, poss := range existingByRack {
				for _, p := range poss {
					d, err := s.devices.GetDevice(p.DeviceID)
					if err == nil && d.Code == row.SourceDeviceCode {
						src = d
						break
					}
				}
				if src != nil {
					break
				}
			}
		}

		if src == nil {
			item.Kind = KindCreateNew
			summary.Create++
		} else {
			seenDevice[src.ID] = true
			pos, _ := s.devices.devices.GetActivePosition(src.ID)
			samePos := pos != nil && pos.RackID == rackID && pos.StartU == row.StartU && pos.EndU == row.EndU
			sameMeta := src.Name == row.Name && (row.SerialNumber == "" || src.SerialNumber == row.SerialNumber)
			switch {
			case samePos && sameMeta:
				item.Kind = KindUnchanged
				summary.Unchanged++
			case samePos && !sameMeta:
				item.Kind = KindUpdateExisting
				summary.Update++
			case !samePos:
				item.Kind = KindMoveExisting
				summary.Move++
			default:
				item.Kind = KindNeedsDecision
				item.RequiresDecision = true
				item.AllowedActions = []string{ActionUpdateExisting, ActionCreateNew, ActionIgnore}
				summary.Decisions++
			}
			item.SourceDeviceCode = src.Code
		}
		rows[itemID] = row
		_ = height
		_ = room
		items = append(items, item)
	}

	// remove missing
	for rid, poss := range existingByRack {
		for _, p := range poss {
			if seenDevice[p.DeviceID] {
				continue
			}
			d, err := s.devices.GetDevice(p.DeviceID)
			if err != nil {
				continue
			}
			itemID := fmt.Sprintf("missing-%s", d.ID.String()[:8])
			items = append(items, ImportItem{
				ID: itemID, Kind: KindRemoveMissing, RequiresDecision: true,
				RackID: rid.String(), StartU: p.StartU, EndU: p.EndU,
				Name: d.Name, SourceDeviceID: d.ID.String(), SourceDeviceCode: d.Code,
				Message: "Excel 中缺失，需确认是否下架",
				AllowedActions: []string{ActionDecommission, ActionIgnore},
			})
			summary.Removals++
			summary.Decisions++
			rows[itemID] = ImportDeviceRow{
				SourceDeviceID: d.ID.String(), RackID: rid.String(),
				StartU: p.StartU, EndU: p.EndU, Name: d.Name,
			}
		}
	}

	draft := &importDraft{
		Token: newToken(), RoomID: roomID, CreatedAt: time.Now(),
		Items: items, Rows: rows, DefaultType: defaultType,
	}
	s.drafts.Put(draft)
	return &ImportValidateResult{Token: draft.Token, Items: items, Summary: summary}, nil
}

// Commit 两阶段导入的提交阶段：全部行操作在单个数据库事务中执行，
// 任意一行失败整体回滚（零残留）；失败时草稿回填，修复决策后可重试。
func (s *ImportService) Commit(roomID uuid.UUID, in ImportCommitInput) (*ImportCommitResult, error) {
	draft, ok := s.drafts.Take(in.Token)
	if !ok {
		return nil, apperr.New(400, "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED", "导入草稿不存在或已过期，请重新校验")
	}
	if draft.RoomID != roomID {
		s.drafts.Put(draft)
		return nil, apperr.InvalidResource("草稿与机房不匹配")
	}
	actionByItem := map[string]string{}
	for _, d := range in.Decisions {
		actionByItem[d.ItemID] = d.Action
	}

	res := &ImportCommitResult{}
	now := time.Now()
	err := s.devices.devices.DB().Transaction(func(tx *gorm.DB) error {
		svc := &DeviceService{devices: s.devices.devices.WithTx(tx), acks: s.devices.acks.WithTx(tx)}
		for _, item := range draft.Items {
			row := draft.Rows[item.ID]
			switch item.Kind {
			case KindError:
				return apperr.InvalidResource("存在阻断错误，不能导入: %s", item.Message)
			case KindUnchanged:
				continue
			case KindCreateNew:
				if item.RequiresDecision && actionByItem[item.ID] == ActionIgnore {
					res.Ignored++
					continue
				}
				if err := s.commitCreate(svc, draft, row, now); err != nil {
					return err
				}
				res.Created++
			case KindUpdateExisting:
				if err := s.commitUpdate(svc, item, row); err != nil {
					return err
				}
				res.Updated++
			case KindMoveExisting:
				if err := s.commitMove(svc, item, row, now); err != nil {
					return err
				}
				res.Moved++
			case KindRemoveMissing:
				action := actionByItem[item.ID]
				if action == ActionIgnore || action == "" {
					res.Ignored++
					continue
				}
				if action != ActionDecommission {
					return apperr.InvalidResource("删除项动作非法")
				}
				if err := s.commitDecommission(svc, item); err != nil {
					return err
				}
				res.Decommissioned++
			case KindNeedsDecision:
				action := actionByItem[item.ID]
				switch action {
				case ActionIgnore:
					res.Ignored++
				case ActionUpdateExisting:
					if err := s.commitUpdate(svc, item, row); err != nil {
						return err
					}
					res.Updated++
				case ActionCreateNew, "":
					if action == "" {
						return apperr.InvalidResource("请完成全部人工确认")
					}
					if err := s.commitCreate(svc, draft, row, now); err != nil {
						return err
					}
					res.Created++
				default:
					return apperr.InvalidResource("未知决策动作")
				}
			}
		}
		return nil
	})
	if err != nil {
		// 事务回滚：把草稿放回（保留原 CreatedAt，TTL 语义不变），客户端修复后可重试
		s.drafts.Put(draft)
		return nil, err
	}
	return res, nil
}

func (s *ImportService) commitCreate(svc *DeviceService, draft *importDraft, row ImportDeviceRow, now time.Time) error {
	typeID := draft.DefaultType
	if row.TypeID != "" {
		if id, err := uuid.Parse(row.TypeID); err == nil {
			typeID = id
		}
	}
	if typeID == uuid.Nil {
		return apperr.InvalidResource("缺少设备类型")
	}
	rackID, err := uuid.Parse(row.RackID)
	if err != nil {
		return err
	}
	code := row.SourceDeviceCode
	if code == "" {
		code = fmt.Sprintf("IMP-%s", uuid.NewString()[:8])
	}
	height := row.EndU - row.StartU + 1
	dev := &model.Device{
		TypeID: typeID, Code: code, Name: row.Name,
		SerialNumber: row.SerialNumber, LifecycleStatus: model.DeviceWaitingRack,
		HeightU: height, RatedPowerW: row.RatedPowerW,
	}
	if err := svc.devices.CreateDevice(dev); err != nil {
		return err
	}
	return s.placeDevice(svc, dev.ID, rackID, row.StartU, height, now, "导入上架")
}

func (s *ImportService) commitUpdate(svc *DeviceService, item ImportItem, row ImportDeviceRow) error {
	sid, err := uuid.Parse(item.SourceDeviceID)
	if err != nil {
		return err
	}
	dev, err := svc.devices.GetDevice(sid)
	if err != nil {
		return err
	}
	next := *dev
	if row.Name != "" {
		next.Name = row.Name
	}
	if row.SerialNumber != "" {
		next.SerialNumber = row.SerialNumber
	}
	if row.RatedPowerW != nil {
		next.RatedPowerW = row.RatedPowerW
	}
	return svc.devices.UpdateDevice(&next, dev.Version)
}

func (s *ImportService) commitMove(svc *DeviceService, item ImportItem, row ImportDeviceRow, now time.Time) error {
	sid, err := uuid.Parse(item.SourceDeviceID)
	if err != nil {
		return err
	}
	rackID, err := uuid.Parse(row.RackID)
	if err != nil {
		return err
	}
	height := row.EndU - row.StartU + 1
	return s.placeDevice(svc, sid, rackID, row.StartU, height, now, "导入迁移")
}

func (s *ImportService) commitDecommission(svc *DeviceService, item ImportItem) error {
	sid, err := uuid.Parse(item.SourceDeviceID)
	if err != nil {
		return err
	}
	if _, err := svc.devices.RemoveFromRack(sid); err != nil {
		return err
	}
	return svc.devices.UpdateDeviceLifecycle(sid, model.DeviceOffRack)
}

func (s *ImportService) placeDevice(svc *DeviceService, deviceID, rackID uuid.UUID, startU, height int, now time.Time, reason string) error {
	rack, err := svc.acks.GetRack(rackID)
	if err != nil {
		return apperr.NotFound("机柜")
	}
	endU := startU + height - 1
	if endU > rack.UHeight {
		return apperr.InvalidResource("超出机柜 U 高度")
	}
	pos := &model.RackDevicePosition{
		DeviceID: deviceID, RackID: rack.ID, RoomID: rack.RoomID, DataCenterID: rack.DataCenterID,
		StartU: startU, HeightU: height, EndU: endU, Orientation: model.OrientNormal,
		InstalledAt: now, Reason: reason,
	}
	if err := svc.devices.PlaceInRack(pos, &model.RackUOccupancy{}); err != nil {
		return err
	}
	return svc.devices.UpdateDeviceLifecycle(deviceID, model.DeviceRunning)
}
