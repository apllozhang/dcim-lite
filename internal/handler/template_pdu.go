package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type TemplateHandler struct{ service *service.TemplateService }

func NewTemplateHandler(s *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{service: s}
}

func (h *TemplateHandler) List(c *gin.Context) {
	items, err := h.service.List()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var in service.TemplateCreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	item, err := h.service.Create(in, &actor)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *TemplateHandler) Update(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.TemplateUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.Update(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	if err := h.service.Delete(id, version); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *TemplateHandler) CreateVersion(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.TemplateVersionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	item, err := h.service.CreateVersion(id, version, in, &actor)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

type PDUHandler struct{ service *service.PDUService }

func NewPDUHandler(s *service.PDUService) *PDUHandler {
	return &PDUHandler{service: s}
}

func (h *PDUHandler) ListByRack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	items, err := h.service.ListByRack(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *PDUHandler) Create(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.PDUInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.Create(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *PDUHandler) Update(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.PDUInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.Update(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *PDUHandler) Delete(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	if err := h.service.Delete(id, version); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *PDUHandler) ListSockets(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	items, err := h.service.ListSockets(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *PDUHandler) CreateSocket(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.SocketInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CreateSocket(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *PDUHandler) UpdateSocket(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.SocketInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdateSocket(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *PDUHandler) DeleteSocket(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteSocket(id, version); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *PDUHandler) ListConnections(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	items, err := h.service.ListConnections(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *PDUHandler) Connect(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.ConnectionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	item, err := h.service.Connect(id, in, &actor)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *PDUHandler) Disconnect(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	// 厂商前端 DELETE 把 version 放 body，套件同样只发 body；query 缺失时不能走
	// queryVersion（其会立即写错误响应，body 兜底成功后造成响应体双写）。
	version := uint(0)
	if raw := c.Query("version"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || n == 0 {
			writeAppError(c, apperr.InvalidResource("invalid resource: version 必须为正整数"))
			return
		}
		version = uint(n)
	} else {
		var body struct {
			Version uint `json:"version"`
		}
		if !bindOptionalJSON(c, &body) {
			return
		}
		version = body.Version
	}
	if version == 0 {
		// 厂商基线（S16-DISCONNECT-NO-VER）：缺 version 返回 400 INVALID_REQUEST
		writeAppError(c, apperr.New(400, "INVALID_REQUEST", "缺少 version 参数"))
		return
	}
	if err := h.service.Disconnect(id, version); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *PDUHandler) Impact(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Impact(id)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *PDUHandler) ForceArchive(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.ForceArchiveInput
	if !bindOptionalJSON(c, &in) {
		return
	}
	actor := middleware.UserID(c)
	rid, _ := c.Get(response.RequestIDKey)
	requestID, _ := rid.(string)
	item, err := h.service.ForceArchive(id, in, &actor, requestID)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

var _ = strconv.Itoa
