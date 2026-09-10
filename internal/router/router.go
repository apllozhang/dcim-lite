package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"dcim-lite/internal/handler"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/response"
)

type Deps struct {
	Secret          string
	LoginRatePerMin int
	Users           middleware.UserFinder
	Revoker         middleware.TokenRevoker
	Audit           middleware.AuditWriter
	Health          *handler.HealthHandler
	Auth            *handler.AuthHandler
	Res             *handler.ResourceHandler
	Device          *handler.DeviceHandler
	Admin           *handler.AdminHandler
	Template        *handler.TemplateHandler
	PDU             *handler.PDUHandler
	Approval        *handler.ApprovalHandler
	LDAP            *handler.LDAPHandler
	Import          *handler.ImportHandler
	ImportTpl       *handler.ImportTemplateHandler
	GinMode         string
}

func New(d Deps) *gin.Engine {
	if d.GinMode != "" {
		gin.SetMode(d.GinMode)
	}
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Recovery(), gin.Logger())

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, 404, "RESOURCE_NOT_FOUND", "接口不存在")
	})

	r.GET("/health/live", d.Health.Live)
	r.GET("/health/ready", d.Health.Ready)

	v1 := r.Group("/api/v1")
	// 全部非 GET 请求统一审计（含登录成败），落 audit_logs
	v1.Use(middleware.Audit(d.Audit))
	{
		// 登录端点：每 IP 限流（默认 10/分钟，LOGIN_RATE_LIMIT_PER_MIN 可调），防定向爆破
		limit := d.LoginRatePerMin
		if limit <= 0 {
			limit = 10
		}
		login := v1.Group("", middleware.NewRateLimit(limit, time.Minute))
		{
			login.POST("/auth/login", d.Auth.Login)
			login.GET("/auth/captcha", d.Auth.Captcha)
		}

		authed := v1.Group("")
		authed.Use(middleware.Auth(d.Secret, d.Users, d.Revoker))
		{
			// 只读查询：登录即可
			authed.GET("/auth/me", d.Auth.Me)
			authed.POST("/auth/logout", d.Auth.Logout)
			authed.GET("/resource-tree", d.Res.Tree)
			// 注意：不能注册 GET /racks，会与 /racks/:id/* 抢路由
			authed.GET("/racks-page", d.Res.ListRacks)
			authed.GET("/device-types", d.Device.ListDeviceTypes)
			authed.GET("/devices", d.Device.ListDevices)
			authed.GET("/devices/import-template", d.ImportTpl.Download)
			authed.GET("/devices/:id", d.Device.GetDevice)
			authed.GET("/devices/:id/history", d.Device.History)
			authed.GET("/racks/:id/u-layout", d.Device.ULayout)
			authed.GET("/rack-templates", d.Template.List)
			authed.GET("/racks/:id/pdus", d.PDU.ListByRack)
			authed.GET("/pdus/:id/sockets", d.PDU.ListSockets)
			authed.GET("/racks/:id/pdu-connections", d.PDU.ListConnections)

			// 业务写操作：要求 system_admin（授权边界收紧，见 docs/COMPAT-DECISIONS.md）
			writes := authed.Group("")
			writes.Use(middleware.RequireAdmin())
			{
				writes.POST("/data-centers", d.Res.CreateDataCenter)
				writes.PUT("/data-centers/:id", d.Res.UpdateDataCenter)
				writes.DELETE("/data-centers/:id", d.Res.DeleteDataCenter)
				writes.POST("/data-centers/:id/rooms", d.Res.CreateRoom)
				writes.POST("/data-centers/:id/copy", d.Res.CopyDataCenter)
				writes.PUT("/rooms/:id", d.Res.UpdateRoom)
				writes.DELETE("/rooms/:id", d.Res.DeleteRoom)
				writes.POST("/rooms/:id/racks", d.Res.CreateRack)
				writes.POST("/rooms/:id/copy", d.Res.CopyRoom)
				writes.POST("/rooms/:id/move", d.Res.MoveRoom)
				writes.POST("/rooms/:id/rack-diagram-import/validate", d.Import.Validate)
				writes.POST("/rooms/:id/rack-diagram-import/commit", d.Import.Commit)
				writes.PUT("/racks/:id", d.Res.UpdateRack)
				writes.DELETE("/racks/:id", d.Res.DeleteRack)
				writes.POST("/racks/:id/copy", d.Res.CopyRack)
				writes.POST("/racks/:id/move", d.Res.MoveRack)
				writes.POST("/device-types", d.Device.CreateDeviceType)
				writes.PUT("/device-types/:id", d.Device.UpdateDeviceType)
				writes.DELETE("/device-types/:id", d.Device.DeleteDeviceType)
				writes.POST("/devices", d.Device.CreateDevice)
				writes.PUT("/devices/:id", d.Device.UpdateDevice)
				writes.DELETE("/devices/:id", d.Device.DeleteDevice)
				writes.POST("/devices/:id/assign", d.Device.Assign)
				writes.POST("/devices/:id/move", d.Device.Move)
				writes.POST("/devices/:id/decommission", d.Device.Decommission)
				writes.POST("/rack-templates", d.Template.Create)
				writes.PUT("/rack-templates/:id", d.Template.Update)
				writes.DELETE("/rack-templates/:id", d.Template.Delete)
				writes.POST("/rack-templates/:id/versions", d.Template.CreateVersion)
				writes.POST("/racks/:id/pdus", d.PDU.Create)
				writes.PUT("/pdus/:id", d.PDU.Update)
				writes.DELETE("/pdus/:id", d.PDU.Delete)
				writes.POST("/pdus/:id/sockets", d.PDU.CreateSocket)
				writes.PUT("/pdu-sockets/:id", d.PDU.UpdateSocket)
				writes.DELETE("/pdu-sockets/:id", d.PDU.DeleteSocket)
				writes.POST("/pdu-sockets/:id/connection", d.PDU.Connect)
				writes.DELETE("/pdu-connections/:id", d.PDU.Disconnect)
			}

			admin := authed.Group("/admin")
			admin.Use(middleware.RequireAdmin())
			{
				admin.GET("/roles", d.Admin.ListRoles)
				admin.GET("/users", d.Admin.ListUsers)
				admin.POST("/users", d.Admin.CreateUser)
				admin.PUT("/users/:id", d.Admin.UpdateUser)
				admin.DELETE("/users/:id", d.Admin.DeleteUser)
				admin.POST("/users/:id/reset-password", d.Admin.ResetPassword)
				admin.GET("/approval-policy", d.Approval.GetPolicy)
				admin.PUT("/approval-policy", d.Approval.UpdatePolicy)
				admin.GET("/approvals", d.Approval.List)
				admin.POST("/approvals/:id/approve", d.Approval.Approve)
				admin.POST("/approvals/:id/reject", d.Approval.Reject)
				admin.GET("/ldap", d.LDAP.Get)
				admin.PUT("/ldap", d.LDAP.Update)
				admin.POST("/ldap/test", d.LDAP.Test)
			}
		}
	}
	return r
}
