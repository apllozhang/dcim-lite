package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/model"
	"dcim-lite/internal/repository"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type DeviceHandler struct {
	service   *service.DeviceService
	approvals *service.ApprovalService
}

func NewDeviceHandler(s *service.DeviceService, a *service.ApprovalService) *DeviceHandler {
	return &DeviceHandler{service: s, approvals: a}
}

func (h *DeviceHandler) ListDeviceTypes(c *gin.Context) {
	items, err := h.service.ListDeviceTypes()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *DeviceHandler) CreateDeviceType(c *gin.Context) {
	var in service.DeviceTypeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CreateDeviceType(in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *DeviceHandler) UpdateDeviceType(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.DeviceTypeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdateDeviceType(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *DeviceHandler) DeleteDeviceType(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteDeviceType(id, version); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *DeviceHandler) ListDevices(c *gin.Context) {
	q := repository.DeviceQuery{
		Page:            atoiDefault(c.Query("page"), 1),
		PageSize:        atoiDefault(c.Query("pageSize"), 10),
		Search:          c.Query("search"),
		LifecycleStatus: c.Query("lifecycleStatus"),
		SortBy:          c.Query("sortBy"),
		SortDir:         c.Query("sortDir"),
	}
	if v := c.Query("typeId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeAppError(c, apperr.InvalidResource("typeId 非法"))
			return
		}
		q.TypeID = &id
	}
	if v := c.Query("rackId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeAppError(c, apperr.InvalidResource("rackId 非法"))
			return
		}
		q.RackID = &id
	}
	items, total, err := h.service.ListDevices(q)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (h *DeviceHandler) GetDevice(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.GetDevice(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *DeviceHandler) CreateDevice(c *gin.Context) {
	var in service.DeviceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CreateDevice(in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *DeviceHandler) UpdateDevice(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.DeviceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdateDevice(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *DeviceHandler) DeleteDevice(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteDevice(id, version); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *DeviceHandler) Assign(c *gin.Context) {
	h.positionChange(c, true)
}

func (h *DeviceHandler) Move(c *gin.Context) {
	h.positionChange(c, false)
}

func (h *DeviceHandler) positionChange(c *gin.Context, assign bool) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.PositionChangeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	rid, _ := c.Get(response.RequestIDKey)
	requestID, _ := rid.(string)
	op := "ASSIGN"
	if !assign {
		op = "MOVE"
	}
	if h.approvals != nil {
		need, err := h.approvals.RequiresApproval(op)
		if err != nil {
			writeAppError(c, err)
			return
		}
		if need {
			dev, err := h.service.GetDevice(id)
			if err != nil {
				writeAppError(c, err)
				return
			}
			rec, err := h.approvals.CreatePending(op, dev, in, &actor)
			if err != nil {
				writeAppError(c, err)
				return
			}
			// 厂商基线形状：{executed:false, approval:{...}}
			response.OK(c, gin.H{"executed": false, "approval": rec})
			return
		}
	}
	var dev *model.Device
	var err error
	if assign {
		dev, err = h.service.Assign(id, in, &actor, requestID)
	} else {
		dev, err = h.service.Move(id, in, &actor, requestID)
	}
	if err != nil {
		writeAppError(c, err)
		return
	}
	// 厂商基线形状：{executed:true, device:{...}, position:{...}}
	pos, err := h.service.ActivePosition(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"executed": true, "device": dev, "position": pos})
}

func (h *DeviceHandler) Decommission(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.DecommissionInput
	if !bindOptionalJSON(c, &in) {
		return
	}
	actor := middleware.UserID(c)
	rid, _ := c.Get(response.RequestIDKey)
	requestID, _ := rid.(string)
	dev, err := h.service.Decommission(id, in, &actor, requestID)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, dev)
}

func (h *DeviceHandler) History(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	items, err := h.service.ListHistory(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *DeviceHandler) ULayout(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	layout, err := h.service.ULayout(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, layout)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

var _ = http.StatusOK
