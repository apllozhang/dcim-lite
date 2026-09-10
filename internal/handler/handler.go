package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/apperr"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/repository"
	"dcim-lite/internal/response"
	"dcim-lite/internal/service"
)

type HealthHandler struct{ db *gorm.DB }

func NewHealthHandler(db *gorm.DB) *HealthHandler { return &HealthHandler{db: db} }

func (h *HealthHandler) Live(c *gin.Context) {
	response.OK(c, gin.H{"status": "live"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "NOT_READY", "数据库不可用")
		return
	}
	if err := sqlDB.Ping(); err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "NOT_READY", "数据库不可用")
		return
	}
	response.OK(c, gin.H{"status": "ready"})
}

type AuthHandler struct {
	service *service.AuthService
	captcha *service.CaptchaService
}

func NewAuthHandler(s *service.AuthService, cap *service.CaptchaService) *AuthHandler {
	return &AuthHandler{service: s, captcha: cap}
}

type loginInput struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	CaptchaID string `json:"captchaId"`
	Captcha   string `json:"captcha"`
}

func (h *AuthHandler) Captcha(c *gin.Context) {
	ch, err := h.captcha.Issue()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, ch)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var in loginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("用户名和密码不能为空"))
		return
	}
	if h.captcha != nil {
		if err := h.captcha.Verify(in.CaptchaID, strings.TrimSpace(in.Captcha)); err != nil {
			writeAppError(c, err)
			return
		}
	}
	res, err := h.service.Login(in.Username, in.Password)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, res)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, err := h.service.Me(middleware.UserID(c))
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// 吊销当前 token（jti 黑名单），登出后旧 token 立即失效
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		_ = h.service.Logout(strings.TrimPrefix(header, "Bearer "))
	}
	response.OK(c, gin.H{"loggedOut": true})
}

type ResourceHandler struct{ service *service.ResourceService }

func NewResourceHandler(s *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{service: s}
}

func (h *ResourceHandler) Tree(c *gin.Context) {
	items, err := h.service.Tree()
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *ResourceHandler) ListRacks(c *gin.Context) {
	q := repository.RackQuery{
		Page:     atoiDefault(c.Query("page"), 1),
		PageSize: atoiDefault(c.Query("pageSize"), 20),
		Search:   c.Query("search"),
		Status:   c.Query("status"),
		SortBy:   c.Query("sortBy"),
		SortDir:  c.Query("sortDir"),
	}
	if v := c.Query("roomId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeAppError(c, apperr.InvalidResource("roomId 非法"))
			return
		}
		q.RoomID = &id
	}
	if v := c.Query("dataCenterId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeAppError(c, apperr.InvalidResource("dataCenterId 非法"))
			return
		}
		q.DataCenter = &id
	}
	items, total, err := h.service.ListRacks(q)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (h *ResourceHandler) CreateDataCenter(c *gin.Context) {
	var in service.DataCenterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CreateDataCenter(in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) UpdateDataCenter(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.DataCenterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdateDataCenter(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) DeleteDataCenter(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	err := h.service.DeleteDataCenter(id, version)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *ResourceHandler) CreateRoom(c *gin.Context) {
	dcID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.RoomInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CreateRoom(dcID, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) UpdateRoom(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.RoomInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdateRoom(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) DeleteRoom(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	err := h.service.DeleteRoom(id, version)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *ResourceHandler) CreateRack(c *gin.Context) {
	roomID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.RackInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CreateRack(roomID, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) UpdateRack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.RackInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.UpdateRack(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) DeleteRack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	err := h.service.DeleteRack(id, version)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *ResourceHandler) CopyDataCenter(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.CopyMoveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CopyDataCenter(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) CopyRoom(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.CopyMoveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CopyRoom(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) MoveRoom(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.CopyMoveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.MoveRoom(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) CopyRack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var in service.CopyMoveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.CopyRack(id, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func (h *ResourceHandler) MoveRack(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	version, ok := queryVersion(c)
	if !ok {
		return
	}
	var in service.CopyMoveInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return
	}
	item, err := h.service.MoveRack(id, version, in)
	if err != nil {
		writeAppError(c, err)
		return
	}
	response.OK(c, item)
}

func writeAppError(c *gin.Context, err error) {
	if e, ok := apperr.As(err); ok {
		response.Fail(c, e.Status, e.Code, e.Message)
		return
	}
	response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}

func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_RESOURCE", "invalid resource: 非法的资源 ID")
		return uuid.Nil, false
	}
	return id, true
}

func queryVersion(c *gin.Context) (uint, bool) {
	raw := c.Query("version")
	if raw == "" {
		response.Fail(c, http.StatusBadRequest, "INVALID_RESOURCE", "invalid resource: 缺少 version 参数")
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		response.Fail(c, http.StatusBadRequest, "INVALID_RESOURCE", "invalid resource: version 必须为正整数")
		return 0, false
	}
	return uint(n), true
}

func bindMessage(err error) string {
	if err == nil {
		return "参数校验失败"
	}
	return "请求体格式错误或缺少必填字段"
}

// bindOptionalJSON 绑定可选请求体：空 body（EOF）合法，格式错误返回 false 并已写响应。
func bindOptionalJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil && !errors.Is(err, io.EOF) {
		writeAppError(c, apperr.InvalidResource("%s", bindMessage(err)))
		return false
	}
	return true
}
