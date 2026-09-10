package handler

import (
	"github.com/gin-gonic/gin"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type LDAPHandler struct{ service *service.LDAPService }

func NewLDAPHandler(s *service.LDAPService) *LDAPHandler {
	return &LDAPHandler{service: s}
}

func (h *LDAPHandler) Get(c *gin.Context) {
	item, err := h.service.Get()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *LDAPHandler) Update(c *gin.Context) {
	var in service.LDAPInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.Update(in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *LDAPHandler) Test(c *gin.Context) {
	var in service.LDAPInput
	if !bindOptionalJSON(c, &in) {
		return
	}
	item, err := h.service.Test(in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}
