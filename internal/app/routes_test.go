package app

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"dcim-lite/internal/config"
)

// 路由清单契约：任何路由增删改都会让本测试失败，必须同步更新预期清单
// （以及 docs/openapi.yaml），防止路由漂移与契约腐化。
func TestRouteInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AppEnv: "development", JWTSecret: "test-secret", JWTExpiresIn: 7200e9}
	a := Build(&gorm.DB{}, cfg) // 装配不触库，路由注册无需真实连接

	got := map[string]bool{}
	for _, r := range a.Engine.Routes() {
		got[r.Method+" "+r.Path] = true
	}

	expected := []string{
		// 公共：健康与登录
		"GET /health/live", "GET /health/ready",
		"POST /api/v1/auth/login", "GET /api/v1/auth/captcha",
		// 登录即可（只读 + 登出）
		"GET /api/v1/auth/me", "POST /api/v1/auth/logout",
		"GET /api/v1/resource-tree", "GET /api/v1/racks-page",
		"GET /api/v1/device-types", "GET /api/v1/devices", "GET /api/v1/devices/import-template",
		"GET /api/v1/devices/:id", "GET /api/v1/devices/:id/history", "GET /api/v1/racks/:id/u-layout",
		"GET /api/v1/rack-templates",
		"GET /api/v1/racks/:id/pdus", "GET /api/v1/pdus/:id/sockets", "GET /api/v1/racks/:id/pdu-connections",
		// system_admin：资源写
		"POST /api/v1/data-centers", "PUT /api/v1/data-centers/:id", "DELETE /api/v1/data-centers/:id",
		"POST /api/v1/data-centers/:id/rooms", "POST /api/v1/data-centers/:id/copy",
		"PUT /api/v1/rooms/:id", "DELETE /api/v1/rooms/:id",
		"POST /api/v1/rooms/:id/racks", "POST /api/v1/rooms/:id/copy", "POST /api/v1/rooms/:id/move",
		"POST /api/v1/rooms/:id/rack-diagram-import/validate", "POST /api/v1/rooms/:id/rack-diagram-import/commit",
		"PUT /api/v1/racks/:id", "DELETE /api/v1/racks/:id", "POST /api/v1/racks/:id/copy", "POST /api/v1/racks/:id/move",
		// system_admin：设备类型与设备写
		"POST /api/v1/device-types", "PUT /api/v1/device-types/:id", "DELETE /api/v1/device-types/:id",
		"POST /api/v1/devices", "PUT /api/v1/devices/:id", "DELETE /api/v1/devices/:id",
		"POST /api/v1/devices/:id/assign", "POST /api/v1/devices/:id/move", "POST /api/v1/devices/:id/decommission",
		// system_admin：模板与 PDU 写
		"POST /api/v1/rack-templates", "PUT /api/v1/rack-templates/:id", "DELETE /api/v1/rack-templates/:id",
		"POST /api/v1/rack-templates/:id/versions",
		"POST /api/v1/racks/:id/pdus", "PUT /api/v1/pdus/:id", "DELETE /api/v1/pdus/:id",
		"POST /api/v1/pdus/:id/sockets", "PUT /api/v1/pdu-sockets/:id", "DELETE /api/v1/pdu-sockets/:id",
		"POST /api/v1/pdu-sockets/:id/connection", "DELETE /api/v1/pdu-connections/:id",
		"GET /api/v1/pdus/:id/archive-impact", "POST /api/v1/pdus/:id/force-archive",
		// system_admin：后台
		"GET /api/v1/admin/roles", "GET /api/v1/admin/users", "POST /api/v1/admin/users",
		"PUT /api/v1/admin/users/:id", "DELETE /api/v1/admin/users/:id", "POST /api/v1/admin/users/:id/reset-password",
		"GET /api/v1/admin/approval-policy", "PUT /api/v1/admin/approval-policy",
		"GET /api/v1/admin/approvals", "POST /api/v1/admin/approvals/:id/approve", "POST /api/v1/admin/approvals/:id/reject",
		"GET /api/v1/admin/ldap", "PUT /api/v1/admin/ldap", "POST /api/v1/admin/ldap/test",
	}

	for _, e := range expected {
		if !got[e] {
			t.Errorf("missing route: %s", e)
		}
	}
	for g := range got {
		found := false
		for _, e := range expected {
			if e == g {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected route: %s (update expected inventory + docs/openapi.yaml)", g)
		}
	}
	if len(got) != len(expected) {
		t.Errorf("route count = %d, expected %d", len(got), len(expected))
	}
	fmt.Printf("inventory: %d routes verified\n", len(expected))
}
