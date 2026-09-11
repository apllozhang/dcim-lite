//go:build integration

package integration

import (
	"strings"
	"testing"
)

// ── 第四轮复评 P1-R6 验收:前端错误遥测端到端 ──

// TestFrontendTelemetry:匿名上报(白名单外的字段丢弃、超长 message 截断)
// → admin 检索接口必须能取回该事件;遥测检索接口仅 admin 可访问。
func TestFrontendTelemetry(t *testing.T) {
	// 匿名上报:含越权字段 tokenHint(必须被白名单丢弃)与超长 message
	longMsg := "e2e-telemetry-" + short() + " " + strings.Repeat("x", 800)
	st, body := call("POST", "/api/v1/telemetry/frontend-errors", map[string]any{
		"release":   "it-release",
		"route":     "/devices",
		"message":   longMsg,
		"role":      "anonymous",
		"tokenHint": "should-be-dropped",
	}, "")
	if st != 200 {
		t.Fatalf("telemetry report: %d %v", st, body)
	}

	// admin 检索:事件必须可达且完成收敛
	st, list := call("GET", "/api/v1/admin/telemetry/frontend-errors?limit=100", nil, adminTok)
	if st != 200 {
		t.Fatalf("admin telemetry list: %d", st)
	}
	items := data(list)["items"].([]any)
	var found map[string]any
	for _, it := range items {
		m := it.(map[string]any)
		if s, _ := m["message"].(string); strings.HasPrefix(s, "e2e-telemetry-") {
			found = m
		}
	}
	if found == nil {
		t.Fatal("reported event not found in admin retrieval")
	}
	msg, _ := found["message"].(string)
	if len(msg) > 500 {
		t.Fatalf("message not truncated: %d bytes", len(msg))
	}
	if _, leak := found["tokenHint"]; leak {
		t.Fatal("non-whitelisted field leaked into telemetry event")
	}
	if found["release"] != "it-release" {
		t.Fatalf("release mismatch: %v", found["release"])
	}
}
