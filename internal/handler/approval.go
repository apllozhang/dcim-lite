package handler

import (
	"github.com/gin-gonic/gin"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type ApprovalHandler struct{ service *service.ApprovalService }

func NewApprovalHandler(s *service.ApprovalService) *ApprovalHandler {
	return &ApprovalHandler{service: s}
}

func (h *ApprovalHandler) GetPolicy(c *gin.Context) {
	item, err := h.service.GetPolicy()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ApprovalHandler) UpdatePolicy(c *gin.Context) {
	var in service.PolicyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdatePolicy(in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ApprovalHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Query("status"))
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *ApprovalHandler) Approve(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.DecisionInput
	_ = c.ShouldBindJSON(&in)
	actor := middleware.UserID(c)
	rid, _ := c.Get(response.RequestIDKey)
	requestID, _ := rid.(string)
	dev, err := h.service.Approve(id, in.Comment, &actor, requestID)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, dev)
}

func (h *ApprovalHandler) Reject(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.DecisionInput
	_ = c.ShouldBindJSON(&in)
	actor := middleware.UserID(c)
	rec, err := h.service.Reject(id, in.Comment, &actor)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, rec)
}
