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

// TestOpenAPISchemaContract 在路由对应关系之上，进一步断言契约文件的自洽与完整：
//   - 所有 $ref 均可解析（无悬空引用）；
//   - 写操作（POST/PUT/DELETE，除 version 走 query 的 DELETE 与无 body 的 logout）
//     均声明 requestBody；
//   - 每个操作都声明 responses；
//   - components.schemas 覆盖关键实体。
func TestOpenAPISchemaContract(t *testing.T) {
	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	dangling := 0
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			for k, child := range v {
				if k == "$ref" {
					ref, _ := child.(string)
					cur := any(doc)
					for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
						m, isMap := cur.(map[string]any)
						if !isMap {
							cur = nil
							break
						}
						next, exists := m[part]
						if !exists {
							cur = nil
							break
						}
						cur = next
					}
					if cur == nil {
						dangling++
						t.Errorf("dangling $ref: %s", ref)
					}
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(doc)

	paths, _ := doc["paths"].(map[string]any)
	if paths == nil {
		t.Fatal("openapi.yaml missing paths")
	}
	exemptBody := func(method, path string) bool {
		if method == "delete" {
			return true // 乐观锁 version 走 query
		}
		return method == "post" && strings.HasSuffix(path, "/logout")
	}
	for path, item := range paths {
		pi, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for method, opAny := range pi {
			switch method {
			case "get", "post", "put", "delete":
			default:
				continue
			}
			op, ok := opAny.(map[string]any)
			if !ok {
				t.Errorf("%s %s: operation is not a mapping", strings.ToUpper(method), path)
				continue
			}
			if _, has := op["responses"]; !has {
				t.Errorf("%s %s: missing responses", strings.ToUpper(method), path)
			}
			if method != "get" && !exemptBody(method, path) {
				if _, has := op["requestBody"]; !has {
					t.Errorf("%s %s: missing requestBody", strings.ToUpper(method), path)
				}
			}
		}
	}

	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for _, entity := range []string{"Device", "Rack", "Room", "DataCenter", "User", "Role",
		"PDU", "PDUSocket", "PDUConnection", "PDUImpactResult", "RackTemplate", "RackTemplateVersion",
		"ApprovalRecord", "OperationPolicy", "LDAPConfig", "ImportValidateResult", "ImportCommitResult",
		"DevicePositionHistory", "ULayout", "Envelope"} {
		if _, has := schemas[entity]; !has {
			t.Errorf("components.schemas missing entity: %s", entity)
		}
	}
	fmt.Printf("openapi schema contract: refs & bodies & entities verified\n")
}
