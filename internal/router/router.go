package router

import (
	"github.com/gin-gonic/gin"

	"dcim-lite/internal/handler"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/response"
)

type Deps struct {
	Secret    string
	Users     middleware.UserFinder
	Health    *handler.HealthHandler
	Auth      *handler.AuthHandler
	Res       *handler.ResourceHandler
	Device    *handler.DeviceHandler
	Admin     *handler.AdminHandler
	Template  *handler.TemplateHandler
	PDU       *handler.PDUHandler
	Approval  *handler.ApprovalHandler
	LDAP      *handler.LDAPHandler
	Import    *handler.ImportHandler
	ImportTpl *handler.ImportTemplateHandler
	GinMode   string
}

func New(d Deps) *gin.Engine {
	if d.GinMode != "" {
		gin.SetMode(d.GinMode)
	}
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Recovery(), gin.Logger())

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, 404, "NOT_FOUND", "接口不存在")
	})

	r.GET("/health/live", d.Health.Live)
	r.GET("/health/ready", d.Health.Ready)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", d.Auth.Login)
		v1.GET("/auth/captcha", d.Auth.Captcha)

		authed := v1.Group("")
		authed.Use(middleware.Auth(d.Secret, d.Users))
		{
			authed.GET("/auth/me", d.Auth.Me)
			authed.POST("/auth/logout", d.Auth.Logout)
			authed.GET("/resource-tree", d.Res.Tree)
			// 注意：不能注册 GET /racks，会与 /racks/:id/* 抢路由
			authed.GET("/racks-page", d.Res.ListRacks)
			authed.POST("/data-centers", d.Res.CreateDataCenter)
			authed.PUT("/data-centers/:id", d.Res.UpdateDataCenter)
			authed.DELETE("/data-centers/:id", d.Res.DeleteDataCenter)
			authed.POST("/data-centers/:id/rooms", d.Res.CreateRoom)
			authed.POST("/data-centers/:id/copy", d.Res.CopyDataCenter)
			authed.PUT("/rooms/:id", d.Res.UpdateRoom)
			authed.DELETE("/rooms/:id", d.Res.DeleteRoom)
			authed.POST("/rooms/:id/racks", d.Res.CreateRack)
			authed.POST("/rooms/:id/copy", d.Res.CopyRoom)
			authed.POST("/rooms/:id/move", d.Res.MoveRoom)
			authed.POST("/rooms/:id/rack-diagram-import/validate", d.Import.Validate)
			authed.POST("/rooms/:id/rack-diagram-import/commit", d.Import.Commit)
			authed.PUT("/racks/:id", d.Res.UpdateRack)
			authed.DELETE("/racks/:id", d.Res.DeleteRack)
			authed.POST("/racks/:id/copy", d.Res.CopyRack)
			authed.POST("/racks/:id/move", d.Res.MoveRack)
			authed.GET("/device-types", d.Device.ListDeviceTypes)
			authed.POST("/device-types", d.Device.CreateDeviceType)
			authed.PUT("/device-types/:id", d.Device.UpdateDeviceType)
			authed.DELETE("/device-types/:id", d.Device.DeleteDeviceType)
			authed.GET("/devices", d.Device.ListDevices)
			authed.GET("/devices/import-template", d.ImportTpl.Download)
			authed.GET("/devices/:id", d.Device.GetDevice)
			authed.POST("/devices", d.Device.CreateDevice)
			authed.PUT("/devices/:id", d.Device.UpdateDevice)
			authed.DELETE("/devices/:id", d.Device.DeleteDevice)
			authed.POST("/devices/:id/assign", d.Device.Assign)
			authed.POST("/devices/:id/move", d.Device.Move)
			authed.POST("/devices/:id/decommission", d.Device.Decommission)
			authed.GET("/devices/:id/history", d.Device.History)
			authed.GET("/racks/:id/u-layout", d.Device.ULayout)

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

			authed.GET("/rack-templates", d.Template.List)
			authed.POST("/rack-templates", d.Template.Create)
			authed.PUT("/rack-templates/:id", d.Template.Update)
			authed.DELETE("/rack-templates/:id", d.Template.Delete)
			authed.POST("/rack-templates/:id/versions", d.Template.CreateVersion)

			authed.GET("/racks/:id/pdus", d.PDU.ListByRack)
			authed.POST("/racks/:id/pdus", d.PDU.Create)
			authed.PUT("/pdus/:id", d.PDU.Update)
			authed.DELETE("/pdus/:id", d.PDU.Delete)
			authed.GET("/pdus/:id/sockets", d.PDU.ListSockets)
			authed.POST("/pdus/:id/sockets", d.PDU.CreateSocket)
			authed.PUT("/pdu-sockets/:id", d.PDU.UpdateSocket)
			authed.DELETE("/pdu-sockets/:id", d.PDU.DeleteSocket)
			authed.POST("/pdu-sockets/:id/connection", d.PDU.Connect)
			authed.GET("/racks/:id/pdu-connections", d.PDU.ListConnections)
			authed.DELETE("/pdu-connections/:id", d.PDU.Disconnect)
		}
	}
	return r
}
