package app

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"dcim-lite/internal/config"
)

// 路由清单契约：任何路由增删改都会让本测试失败，必须同步更新 RouteInventory
// （以及 docs/openapi.yaml），防止路由漂移与契约腐化。
func TestRouteInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AppEnv: "development", JWTSecret: "test-secret", JWTExpiresIn: 7200e9}
	a := Build(&gorm.DB{}, cfg) // 装配不触库，路由注册无需真实连接

	got := map[string]bool{}
	for _, r := range a.Engine.Routes() {
		got[r.Method+" "+r.Path] = true
	}

	expected := RouteInventory()
	for _, spec := range expected {
		key := spec.Method + " " + spec.Path
		if !got[key] {
			t.Errorf("missing route: %s", key)
		}
		delete(got, key)
	}
	for key := range got {
		t.Errorf("unexpected route: %s (update RouteInventory + docs/openapi.yaml)", key)
	}
	if len(a.Engine.Routes()) != len(expected) {
		t.Errorf("route count = %d, expected %d", len(a.Engine.Routes()), len(expected))
	}
	fmt.Printf("inventory: %d routes verified\n", len(expected))
}
