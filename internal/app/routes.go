package app

// RouteSpec 描述一条路由及其权限级别，是路由清单契约与权限矩阵测试的单一事实源：
// 路由增删改必须同步更新这里（以及 docs/openapi.yaml）。
type RouteSpec struct {
	Method string
	Path   string
	// Auth: "public" 匿名可访问；"user" 登录即可；"admin" 需要 system_admin
	Auth string
}

// RouteInventory 全量路由清单（与 internal/router/router.go 的注册一一对应）。
func RouteInventory() []RouteSpec {
	return []RouteSpec{
		// 公共：健康、指标与登录
		{Method: "GET", Path: "/health/live", Auth: "public"},
		{Method: "GET", Path: "/health/ready", Auth: "public"},
		{Method: "GET", Path: "/metrics", Auth: "public"},
		{Method: "POST", Path: "/api/v1/auth/login", Auth: "public"},
		{Method: "GET", Path: "/api/v1/auth/captcha", Auth: "public"},
		{Method: "POST", Path: "/api/v1/telemetry/frontend-errors", Auth: "public"},
		// 登录即可（只读 + 登出）
		{Method: "GET", Path: "/api/v1/auth/me", Auth: "user"},
		{Method: "POST", Path: "/api/v1/auth/logout", Auth: "user"},
		{Method: "GET", Path: "/api/v1/resource-tree", Auth: "user"},
		{Method: "GET", Path: "/api/v1/racks-page", Auth: "user"},
		{Method: "GET", Path: "/api/v1/device-types", Auth: "user"},
		{Method: "GET", Path: "/api/v1/devices", Auth: "user"},
		{Method: "GET", Path: "/api/v1/devices/import-template", Auth: "user"},
		{Method: "GET", Path: "/api/v1/devices/:id", Auth: "user"},
		{Method: "GET", Path: "/api/v1/devices/:id/history", Auth: "user"},
		{Method: "GET", Path: "/api/v1/racks/:id/u-layout", Auth: "user"},
		{Method: "GET", Path: "/api/v1/rack-templates", Auth: "user"},
		{Method: "GET", Path: "/api/v1/racks/:id/pdus", Auth: "user"},
		{Method: "GET", Path: "/api/v1/pdus/:id/sockets", Auth: "user"},
		{Method: "GET", Path: "/api/v1/racks/:id/pdu-connections", Auth: "user"},
		// system_admin：资源写
		{Method: "POST", Path: "/api/v1/data-centers", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/data-centers/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/data-centers/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/data-centers/:id/rooms", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/data-centers/:id/copy", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/rooms/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/rooms/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/rooms/:id/racks", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/rooms/:id/copy", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/rooms/:id/move", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/rooms/:id/rack-diagram-import/validate", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/rooms/:id/rack-diagram-import/commit", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/racks/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/racks/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/racks/:id/copy", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/racks/:id/move", Auth: "admin"},
		// system_admin：设备类型与设备写
		{Method: "POST", Path: "/api/v1/device-types", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/device-types/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/device-types/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/devices", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/devices/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/devices/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/devices/:id/assign", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/devices/:id/move", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/devices/:id/decommission", Auth: "admin"},
		// system_admin：模板与 PDU 写
		{Method: "POST", Path: "/api/v1/rack-templates", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/rack-templates/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/rack-templates/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/rack-templates/:id/versions", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/racks/:id/pdus", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/pdus/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/pdus/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/pdus/:id/sockets", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/pdu-sockets/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/pdu-sockets/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/pdu-sockets/:id/connection", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/pdu-connections/:id", Auth: "admin"},
		{Method: "GET", Path: "/api/v1/pdus/:id/archive-impact", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/pdus/:id/force-archive", Auth: "admin"},
		// system_admin：后台
		{Method: "GET", Path: "/api/v1/admin/roles", Auth: "admin"},
		{Method: "GET", Path: "/api/v1/admin/users", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/admin/users", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/admin/users/:id", Auth: "admin"},
		{Method: "DELETE", Path: "/api/v1/admin/users/:id", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/admin/users/:id/reset-password", Auth: "admin"},
		{Method: "GET", Path: "/api/v1/admin/approval-policy", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/admin/approval-policy", Auth: "admin"},
		{Method: "GET", Path: "/api/v1/admin/approvals", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/admin/approvals/:id/approve", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/admin/approvals/:id/reject", Auth: "admin"},
		{Method: "GET", Path: "/api/v1/admin/ldap", Auth: "admin"},
		{Method: "PUT", Path: "/api/v1/admin/ldap", Auth: "admin"},
		{Method: "POST", Path: "/api/v1/admin/ldap/test", Auth: "admin"},
		{Method: "GET", Path: "/api/v1/admin/telemetry/frontend-errors", Auth: "admin"},
	}
}
