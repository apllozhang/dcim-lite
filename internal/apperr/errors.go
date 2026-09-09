package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Code    string
	Message string
	Status  int
}

func (e *Error) Error() string { return e.Message }

func New(status int, code, message string) *Error {
	return &Error{Code: code, Message: message, Status: status}
}

func InvalidResource(format string, args ...any) *Error {
	return New(http.StatusBadRequest, "INVALID_RESOURCE", fmt.Sprintf("invalid resource: "+format, args...))
}

func Unauthorized() *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", "未登录或登录已过期")
}

func InvalidCredentials() *Error {
	return New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
}

func NotFound(resource string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", resource+"不存在")
}

func DuplicateCode() *Error {
	return New(http.StatusConflict, "DUPLICATE_CODE", "编码已存在")
}

func ResourceVersion() *Error {
	return New(http.StatusConflict, "RESOURCE_VERSION", "资源版本冲突，请刷新后重试")
}

func HasChildren() *Error {
	return New(http.StatusConflict, "HAS_CHILDREN", "存在下级资源，无法删除")
}

func ParentDisabled() *Error {
	return New(http.StatusConflict, "PARENT_DISABLED", "父级资源已停用")
}

func Forbidden() *Error {
	return New(http.StatusForbidden, "FORBIDDEN", "无权限")
}

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
