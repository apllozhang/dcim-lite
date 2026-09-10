package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
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
	KindUnchanged      = "UNCHANGED"
	KindCreateNew      = "CREATE_NEW"
	KindUpdateExisting = "UPDATE_EXISTING"
	KindMoveExisting   = "MOVE_EXISTING"
	KindRemoveMissing  = "REMOVE_MISSING"
	KindNeedsDecision  = "NEEDS_DECISION"
	KindError          = "ERROR"
)

// Commit actions for decision items.
const (
	ActionUpdateExisting = "UPDATE_EXISTING"
	ActionCreateNew      = "CREATE_NEW"
	ActionDecommission   = "DECOMMISSION"
	ActionIgnore         = "IGNORE"
	// ActionSkip 为调用方使用的等价忽略动作（厂商侧接受 SKIP；
	// 差分用例 S12-IMPORT-COMMIT 实测：套件在重建侧发送 SKIP，此前被判「删除项动作非法」→ 400）
	ActionSkip = "SKIP"
)

// ignored 判断是否为「忽略」类动作（IGNORE 与 SKIP 等价）。
func ignored(action string) bool {
	return action == ActionIgnore || action == ActionSkip
}

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
	FormatVersion  string            `json:"formatVersion"`
	DataCenterID   string            `json:"dataCenterId"`
	RoomID         string            `json:"roomId"`
	ExportedAt     string            `json:"exportedAt,omitempty"`
	CoveredRackIDs []string          `json:"coveredRackIds"`
	Devices        []ImportDeviceRow `json:"devices"`
	DefaultTypeID  string            `json:"defaultTypeId"`
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
	Errors    int `json:"errors"`
	Create    int `json:"create"`
	Update    int `json:"update"`
	Move      int `json:"move"`
	Removals  int `json:"removals"`
	Decisions int `json:"decisions"`
	Unchanged int `json:"unchanged"`
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
	Created        int `json:"created"`
	Updated        int `json:"updated"`
	Moved          int `json:"moved"`
	Decommissioned int `json:"decommissioned"`
	Ignored        int `json:"ignored"`
}

type importDraft struct {
	Token  string
	RoomID uuid.UUID
	// ActorID 为发起校验的用户；提交时必须同一用户（草稿不绑定发起人时，
	// 任何持有 token 的管理员都能以他人校验结果提交，破坏审计语义）。
	ActorID     uuid.UUID
	CreatedAt   time.Time
	Items       []ImportItem
	Rows        map[string]ImportDeviceRow // itemID -> row
	DefaultType uuid.UUID
	// Fingerprint 为校验时覆盖机柜内「在位设备-位置」的指纹；提交前重算，
	// 不一致即判定数据已变化（厂商 409 RACK_DIAGRAM_IMPORT_STALE）。
	// CoveredRacks 与 Fingerprint 必须成对使用，保证两侧机柜集合一致（否则会误报 STALE）。
	Fingerprint  string
	CoveredRacks []uuid.UUID
}

type ImportDraftStore struct {
	mu     sync.Mutex
	drafts map[string]*importDraft
	ttl    time.Duration
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

// rackPositionsFingerprint 计算覆盖机柜内在位设备位置的指纹（排序拼接后散列）。
func (s *ImportService) rackPositionsFingerprint(rackIDs []uuid.UUID) (string, error) {
	type posRow struct {
		RackID  uuid.UUID
		StartU  int
		EndU    int
		DevID   uuid.UUID
		HeightU int
	}
	var all []posRow
	for _, rid := range rackIDs {
		var positions []model.RackDevicePosition
		if err := s.devices.devices.DB().
			Where("rack_id = ? AND deleted_at IS NULL", rid).Find(&positions).Error; err != nil {
			return "", err
		}
		for _, p := range positions {
			all = append(all, posRow{p.RackID, p.StartU, p.EndU, p.DeviceID, p.HeightU})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].RackID != all[j].RackID {
			return all[i].RackID.String() < all[j].RackID.String()
		}
		if all[i].StartU != all[j].StartU {
			return all[i].StartU < all[j].StartU
		}
		return all[i].DevID.String() < all[j].DevID.String()
	})
	var sb strings.Builder
	for _, r := range all {
		fmt.Fprintf(&sb, "%s|%d|%d|%s|%d;", r.RackID, r.StartU, r.EndU, r.DevID, r.HeightU)
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:]), nil
}

// ImportService implements rack-diagram two-phase import.
type ImportService struct {
	drafts  *ImportDraftStore
	devices *DeviceService
}

func NewImportService(drafts *ImportDraftStore, devices *DeviceService) *ImportService {
	return &ImportService{drafts: drafts, devices: devices}
}

func (s *ImportService) Validate(roomID uuid.UUID, in ImportValidateInput, actor *uuid.UUID) (*ImportValidateResult, error) {
	room, err := s.devices.acks.GetRoom(roomID)
	if err != nil {
		return nil, apperr.NotFound("机房")
	}
	// 路径机房是唯一事实源：body 中重复的 roomId/dataCenterId 若与路径或真实父级不一致，
	// 属于请求构造错误，直接拒绝（防止两套标识漂移导致的越界导入）
	if in.RoomID != "" {
		if bodyRoom, err := uuid.Parse(in.RoomID); err != nil || bodyRoom != roomID {
			return nil, apperr.InvalidResource("roomId 与 URL 中的机房不一致")
		}
	}
	if in.DataCenterID != "" {
		if bodyDC, err := uuid.Parse(in.DataCenterID); err != nil || bodyDC != room.DataCenterID {
			return nil, apperr.InvalidResource("dataCenterId 与机房的所属数据中心不一致")
		}
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
	// P0-02（复评）：covered rack 必须属于 URL 指定的机房——否则以机房 A 的路径
	// 可以引用机房 B 的机柜，破坏路径资源边界与导入审计语义
	covered := make([]uuid.UUID, 0, len(in.CoveredRackIDs))
	coveredSet := make(map[uuid.UUID]bool, len(in.CoveredRackIDs))
	for _, id := range in.CoveredRackIDs {
		rid, err := uuid.Parse(id)
		if err != nil {
			return nil, apperr.InvalidResource("coveredRackIds 含非法 ID")
		}
		rack, err := s.devices.acks.GetRack(rid)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperr.NotFound("机柜")
			}
			return nil, err
		}
		if rack.RoomID != roomID {
			return nil, apperr.InvalidResource(
				fmt.Sprintf("机柜 %s 不属于当前机房，coveredRackIds 只能包含 URL 指定机房内的机柜", id))
		}
		covered = append(covered, rid)
		coveredSet[rid] = true
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
		// P0-02（复评）：每行的 rackId 必须是覆盖机柜集合成员，行级阻断
		if !coveredSet[rackID] {
			item.Kind = KindError
			item.Message = "rackId 不在 coveredRackIds 覆盖集合内"
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
				Message:        "Excel 中缺失，需确认是否下架",
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

	fingerprint, err := s.rackPositionsFingerprint(covered)
	if err != nil {
		return nil, err
	}
	if actor == nil || *actor == uuid.Nil {
		return nil, apperr.Forbidden()
	}
	draft := &importDraft{
		Token: newToken(), RoomID: roomID, ActorID: *actor, CreatedAt: time.Now(),
		Items: items, Rows: rows, DefaultType: defaultType,
		Fingerprint: fingerprint, CoveredRacks: covered,
	}
	s.drafts.Put(draft)
	return &ImportValidateResult{Token: draft.Token, Items: items, Summary: summary}, nil
}

// Commit 两阶段导入的提交阶段：全部行操作在单个数据库事务中执行，
// 任意一行失败整体回滚（零残留）；失败时草稿回填，修复决策后可重试。
func (s *ImportService) Commit(roomID uuid.UUID, in ImportCommitInput, actor *uuid.UUID) (*ImportCommitResult, error) {
	draft, ok := s.drafts.Take(in.Token)
	if !ok {
		// 厂商行为：草稿过期返回 410 Gone（S12-IMPORT-RECOMMIT-EXPIRED 实测）
		return nil, apperr.New(410, "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED", "导入校验已过期，请重新选择文件校验")
	}
	// 草稿与发起人绑定：其他管理员取得 token 也不能以他人校验结果提交。
	// 校验失败回填草稿，原发起人仍可重试。
	if actor == nil || *actor == uuid.Nil || draft.ActorID != *actor {
		s.drafts.Put(draft)
		return nil, apperr.New(403, "IMPORT_DRAFT_OWNER_MISMATCH",
			"导入草稿属于发起校验的用户，请由该用户提交或重新校验")
	}
	if draft.RoomID != roomID {
		s.drafts.Put(draft)
		return nil, apperr.InvalidResource("草稿与机房不匹配")
	}
	actionByItem := map[string]string{}
	for _, d := range in.Decisions {
		actionByItem[d.ItemID] = d.Action
	}

	// 校验后数据是否被改动（厂商 409 RACK_DIAGRAM_IMPORT_STALE）
	if draft.Fingerprint != "" {
		current, err := s.rackPositionsFingerprint(draft.CoveredRacks)
		if err != nil {
			return nil, err
		}
		if current != draft.Fingerprint {
			return nil, apperr.New(409, "RACK_DIAGRAM_IMPORT_STALE",
				"校验后机柜或设备数据发生变化，请重新校验")
		}
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
				if item.RequiresDecision && ignored(actionByItem[item.ID]) {
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
				if ignored(action) || action == "" {
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
				case ActionIgnore, ActionSkip:
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
