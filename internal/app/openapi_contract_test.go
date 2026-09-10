package app

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"dcim-lite/internal/config"
)

// OpenAPI 契约测试：docs/openapi.yaml 的操作集合必须与真实路由一一对应。
// 路由或契约任一方漂移都会失败，强制两边同步演进。
func TestOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	ops := map[string]bool{}
	for path, item := range doc.Paths {
		for verb := range item {
			switch verb {
			case "get", "post", "put", "delete":
				ops[strings.ToUpper(verb)+" "+path] = true
			}
		}
	}

	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AppEnv: "development", JWTSecret: "test-secret", JWTExpiresIn: 7200e9}
	a := Build(&gorm.DB{}, cfg)
	reParam := regexp.MustCompile(`:(\w+)`)
	for _, r := range a.Engine.Routes() {
		// gin 路由参数 :id 转换为 OpenAPI 风格 {id}
		key := r.Method + " " + reParam.ReplaceAllString(r.Path, "{$1}")
		if !ops[key] {
			t.Errorf("route not declared in docs/openapi.yaml: %s", key)
		}
	}
	for key := range ops {
		found := false
		for _, r := range a.Engine.Routes() {
			if key == r.Method+" "+reParam.ReplaceAllString(r.Path, "{$1}") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("openapi operation has no real route: %s", key)
		}
	}
	fmt.Printf("openapi contract: %d operations matched\n", len(ops))
}
