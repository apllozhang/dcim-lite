package handler

import (
	"github.com/gin-gonic/gin"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type ImportHandler struct{ service *service.ImportService }

func NewImportHandler(s *service.ImportService) *ImportHandler {
	return &ImportHandler{service: s}
}

func (h *ImportHandler) Validate(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.ImportValidateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.Validate(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ImportHandler) Commit(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.ImportCommitInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	_ = actor
	item, err := h.service.Commit(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}
