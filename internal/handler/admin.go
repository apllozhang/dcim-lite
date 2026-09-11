package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type AdminHandler struct{ service *service.AdminService }

func NewAdminHandler(s *service.AdminService) *AdminHandler {
	return &AdminHandler{service: s}
}

func (h *AdminHandler) ListRoles(c *gin.Context) {
	items, err := h.service.ListRoles()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	items, err := h.service.ListUsers(service.UserListQuery{
		Search:     c.Query("search"),
		AuthSource: c.Query("authSource"),
	})
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var in service.UserAdminInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	item, err := h.service.CreateUser(in, &actor)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.UserAdminInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	item, err := h.service.UpdateUser(id, version, in, &actor)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	actor := middleware.UserID(c)
	if err := h.service.DeleteUser(id, version, &actor); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.ResetPasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	actor := middleware.UserID(c)
	if err := h.service.ResetPassword(id, version, in.Password, &actor); err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"reset": true})
}

var _ = uuid.Nil
