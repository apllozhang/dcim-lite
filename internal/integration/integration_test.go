//go:build integration

// 集成测试：需要真实 PostgreSQL（TEST_DATABASE_URL），无则跳过。
// 运行：go test -tags integration ./internal/integration/...
// 覆盖评审报告 P0 修复的验收标准：并发恰一次、事务零残留、乐观锁、软删唯一复用、
// 审批失败回滚保持 PENDING、机柜删除保护、最后管理员并发保护。
package integration

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"dcim-lite/internal/app"
	"dcim-lite/internal/config"
	"dcim-lite/internal/repository"
)

var (
	testApp  *app.App
	server   *httptest.Server
	adminTok string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println("TEST_DATABASE_URL not set; skipping integration tests")
		os.Exit(0)
	}
	db, err := repository.Open(dsn)
	if err != nil {
		fmt.Println("db open:", err)
		os.Exit(1)
	}
	if err := applyMigrations(db); err != nil {
		fmt.Println("migrations:", err)
		os.Exit(1)
	}
	users := repository.NewUserStore(db)
	if err := users.EnsureAdmin("itadmin", "ItAdmin#2026!", "集成测试管理员"); err != nil {
		fmt.Println("seed admin:", err)
		os.Exit(1)
	}
	if err := repository.NewDeviceStore(db).SeedDeviceTypesIfEmpty(); err != nil {
		fmt.Println("seed types:", err)
		os.Exit(1)
	}

	cfg := &config.Config{
		AppEnv: "development",
		// 与生产一致的强度要求（32+ 字节）
		JWTSecret:    "integration-test-secret-32bytes!!",
		JWTExpiresIn: 2 * time.Hour,
	}
	testApp = app.Build(db, cfg)
	server = httptest.NewServer(testApp.Engine)
	defer server.Close()

	adminTok = mustLogin("itadmin", "ItAdmin#2026!")
	os.Exit(m.Run())
}

// applyMigrations 在全新测试库上按文件名顺序执行 migrations/*.up.sql。
// 目录定位见 FindMigrationsDir（兼容 go test / CI 嵌套检出 / 独立二进制）。
func applyMigrations(db *gorm.DB) error {
	dir, err := FindMigrationsDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no .up.sql files under %s", dir)
	}
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if err := db.Exec(string(body)).Error; err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func mustLogin(user, pass string) string {
	// 取验证码并从 SVG 明文数字中解析（与 scripts/smoke-all.sh 同法）
	st, cap := call("GET", "/api/v1/auth/captcha", nil, "")
	if st != 200 {
		fmt.Printf("captcha failed: %d\n", st)
		os.Exit(1)
	}
	img := data(cap)["image"].(string)
	if i := strings.Index(img, ","); i >= 0 {
		img = img[i+1:]
	}
	svg, err := base64.StdEncoding.DecodeString(img)
	if err != nil {
		fmt.Println("captcha decode:", err)
		os.Exit(1)
	}
	var digits strings.Builder
	for _, m := range reDigits.FindAllStringSubmatch(string(svg), -1) {
		digits.WriteString(m[1])
	}
	st, body := call("POST", "/api/v1/auth/login", map[string]string{
		"username": user, "password": pass,
		"captchaId": data(cap)["id"].(string), "captcha": digits.String(),
	}, "")
	if st != 200 {
		fmt.Printf("login failed: %d %v\n", st, body)
		os.Exit(1)
	}
	return data(body)["token"].(string)
}

var reDigits = regexp.MustCompile(`>(\d)</text>`)

func call(method, path string, payload any, token string) (int, map[string]any) {
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	} else {
		body = []byte("{}")
	}
	req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, map[string]any{"err": err.Error()}
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func data(body map[string]any) map[string]any {
	if d, ok := body["data"].(map[string]any); ok {
		return d
	}
	return map[string]any{}
}

func code(body map[string]any) string {
	if c, ok := body["code"].(string); ok {
		return c
	}
	return ""
}

// ── fixture helper ────────────────────────────────────────────

type fixture struct {
	dcID, roomID, rackID, typeID string
}

func newFixture(t *testing.T, prefix string) fixture {
	t.Helper()
	st, dc := call("POST", "/api/v1/data-centers", map[string]any{"code": prefix + "-DC" + short(), "name": "it"}, adminTok)
	if st != 200 {
		t.Fatalf("create dc: %d %v", st, dc)
	}
	dcID := data(dc)["id"].(string)
	st, room := call("POST", "/api/v1/data-centers/"+dcID+"/rooms", map[string]any{"code": "R", "name": "it"}, adminTok)
	if st != 200 {
		t.Fatalf("create room: %d %v", st, room)
	}
	roomID := data(room)["id"].(string)
	st, rack := call("POST", "/api/v1/rooms/"+roomID+"/racks", map[string]any{"code": "K", "name": "it", "uHeight": 20}, adminTok)
	if st != 200 {
		t.Fatalf("create rack: %d %v", st, rack)
	}
	rackID := data(rack)["id"].(string)
	st, types := call("GET", "/api/v1/device-types", nil, adminTok)
	if st != 200 {
		t.Fatalf("list types: %d", st)
	}
	var typeID string
	for _, it := range data(types)["items"].([]any) {
		m := it.(map[string]any)
		if m["code"] == "SERVER" {
			typeID = m["id"].(string)
		}
	}
	return fixture{dcID: dcID, roomID: roomID, rackID: rackID, typeID: typeID}
}

func createDevice(t *testing.T, fx fixture, code string, heightU int) string {
	t.Helper()
	st, dev := call("POST", "/api/v1/devices", map[string]any{
		"typeId": fx.typeID, "code": code, "name": "it", "heightU": heightU,
	}, adminTok)
	if st != 200 {
		t.Fatalf("create device %s: %d %v", code, st, dev)
	}
	return data(dev)["id"].(string)
}

func short() string {
	return uuid.NewString()[:8]
}

// ── 测试用例 ──────────────────────────────────────────────────

// P0-02 验收：同 U 位双并发上架，恰一成功。
func TestConcurrentPlaceOneWinner(t *testing.T) {
	fx := newFixture(t, "CP")
	d1 := createDevice(t, fx, "CP-A"+short(), 2)
	d2 := createDevice(t, fx, "CP-B"+short(), 2)

	var wg sync.WaitGroup
	results := make([]int, 2)
	for i, dev := range []string{d1, d2} {
		wg.Add(1)
		go func(i int, dev string) {
			defer wg.Done()
			st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
				map[string]any{"targetRackId": fx.rackID, "startU": 5}, adminTok)
			results[i] = st
		}(i, dev)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, st := range results {
		switch st {
		case 200:
			ok++
		case 409:
			conflict++
		default:
			t.Fatalf("unexpected status %d", st)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("want exactly 1 winner + 1 conflict, got ok=%d conflict=%d (%v)", ok, conflict, results)
	}
}

// P0-03 验收：并发批准同一审批单（20 路），决定与副作用严格一次。
func TestApprovalConcurrentApproveExactlyOnce(t *testing.T) {
	fx := newFixture(t, "AP")
	dev := createDevice(t, fx, "AP-D"+short(), 1)

	if st, _ := call("PUT", "/api/v1/admin/approval-policy", map[string]any{"assignApprovalEnabled": true}, adminTok); st != 200 {
		t.Fatal("enable policy failed")
	}
	defer call("PUT", "/api/v1/admin/approval-policy", map[string]any{"assignApprovalEnabled": false}, adminTok)

	st, pend := call("POST", "/api/v1/devices/"+dev+"/assign",
		map[string]any{"targetRackId": fx.rackID, "startU": 3}, adminTok)
	if st != 200 || data(pend)["status"] != "PENDING" {
		t.Fatalf("assign should create PENDING, got %d %v", st, pend)
	}
	approvalID := data(pend)["id"].(string)

	const n = 20
	var wg sync.WaitGroup
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st, _ := call("POST", "/api/v1/admin/approvals/"+approvalID+"/approve",
				map[string]any{"comment": "go"}, adminTok)
			statuses[i] = st
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, s := range statuses {
		if s == 200 {
			ok++
		} else if s != 409 {
			t.Fatalf("unexpected approve status %d", s)
		}
	}
	if ok != 1 {
		t.Fatalf("exactly one approve must win, got %d winners", ok)
	}

	// 履历恰一条 ASSIGN（副作用严格一次）
	st, hist := call("GET", "/api/v1/devices/"+dev+"/history", nil, adminTok)
	if st != 200 {
		t.Fatalf("history: %d", st)
	}
	if n := len(data(hist)["items"].([]any)); n != 1 {
		t.Fatalf("exactly one ASSIGN history expected, got %d", n)
	}
}

// P0-03 原子性证明：批准时目标 U 位已被占 → 执行失败 → 审批单回滚保持 PENDING。
func TestApproveRollbackKeepsPending(t *testing.T) {
	fx := newFixture(t, "RB")
	dev := createDevice(t, fx, "RB-D"+short(), 1)
	blocker := createDevice(t, fx, "RB-B"+short(), 1)

	// blocker 必须在开启审批策略前占位，否则它的 assign 也会进审批流
	if st, _ := call("POST", "/api/v1/devices/"+blocker+"/assign",
		map[string]any{"targetRackId": fx.rackID, "startU": 8}, adminTok); st != 200 {
		t.Fatalf("blocker assign failed")
	}

	if st, _ := call("PUT", "/api/v1/admin/approval-policy", map[string]any{"assignApprovalEnabled": true}, adminTok); st != 200 {
		t.Fatal("enable policy failed")
	}
	defer call("PUT", "/api/v1/admin/approval-policy", map[string]any{"assignApprovalEnabled": false}, adminTok)

	st, pend := call("POST", "/api/v1/devices/"+dev+"/assign",
		map[string]any{"targetRackId": fx.rackID, "startU": 8}, adminTok)
	if st != 200 {
		t.Fatalf("pending assign: %d", st)
	}
	approvalID := data(pend)["id"].(string)

	st, body := call("POST", "/api/v1/admin/approvals/"+approvalID+"/approve", map[string]any{}, adminTok)
	if st != 409 || code(body) != "U_SLOT_CONFLICT" {
		t.Fatalf("approve should hit U_SLOT_CONFLICT, got %d %v", st, body)
	}

	// 审批单必须仍为 PENDING（决定写入随事务回滚）
	st, list := call("GET", "/api/v1/admin/approvals?status=PENDING", nil, adminTok)
	if st != 200 {
		t.Fatalf("list pending: %d", st)
	}
	still := false
	for _, it := range data(list)["items"].([]any) {
		if it.(map[string]any)["id"] == approvalID {
			still = true
		}
	}
	if !still {
		t.Fatal("approval must remain PENDING after failed approve (transaction rollback)")
	}
	// 被审批设备必须仍未上架
	st, devBody := call("GET", "/api/v1/devices/"+dev, nil, adminTok)
	if st != 200 || data(devBody)["lifecycleStatus"] != "WAITING_RACK" {
		t.Fatalf("device must stay WAITING_RACK, got %v", data(devBody)["lifecycleStatus"])
	}
}

// P0-04 验收：导入提交中途失败 → 全量回滚零残留，且 token 回填可重试。
func TestImportCommitRollbackZeroResidue(t *testing.T) {
	fx := newFixture(t, "IM")
	src := createDevice(t, fx, "IM-S"+short(), 2)
	if st, _ := call("POST", "/api/v1/devices/"+src+"/assign",
		map[string]any{"targetRackId": fx.rackID, "startU": 1}, adminTok); st != 200 {
		t.Fatalf("seed assign failed")
	}

	st, val := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/validate", map[string]any{
		"formatVersion": "1", "dataCenterId": fx.dcID, "roomId": fx.roomID,
		"coveredRackIds": []string{fx.rackID}, "defaultTypeId": fx.typeID,
		"devices": []map[string]any{
			{"clientId": "a", "rackId": fx.rackID, "startU": 1, "endU": 2, "name": "改名后",
				"sourceDeviceId": src},
			{"clientId": "b", "rackId": fx.rackID, "startU": 5, "endU": 5, "name": "新建残留探针X"},
		},
	}, adminTok)
	if st != 200 {
		t.Fatalf("validate: %d %v", st, val)
	}
	token := data(val)["token"].(string)
	sum := data(val)["summary"].(map[string]any)
	if sum["create"].(float64) != 1 || sum["update"].(float64) != 1 {
		t.Fatalf("unexpected plan: %v", sum)
	}

	// 破坏更新项：提交前删除源设备 → commitUpdate 失败 → 整体回滚
	// （decommission 与删除都会递增 version，最后再取一次）
	if st, _ := call("POST", "/api/v1/devices/"+src+"/decommission", map[string]any{"reason": "破坏"}, adminTok); st != 200 {
		t.Fatalf("decommission src failed")
	}
	st, devBody := call("GET", "/api/v1/devices/"+src, nil, adminTok)
	if st != 200 {
		t.Fatalf("get src: %d", st)
	}
	version := fmt.Sprintf("%.0f", data(devBody)["version"].(float64))
	if st, _ := call("DELETE", "/api/v1/devices/"+src+"?version="+version, nil, adminTok); st != 200 {
		t.Fatalf("delete src failed")
	}

	st, _ = call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/commit",
		map[string]any{"token": token, "decisions": []any{}}, adminTok)
	if st == 200 {
		t.Fatal("commit should fail when a planned row breaks mid-flight")
	}

	// 零残留：计划新建的设备不得落库
	st, list := call("GET", "/api/v1/devices?search=新建残留探针X", nil, adminTok)
	if st != 200 {
		t.Fatalf("search: %d", st)
	}
	if total := data(list)["total"].(float64); total != 0 {
		t.Fatalf("rolled-back import must leave zero residue, found %d rows", int(total))
	}

	// token 已回填：重试不得返回 DRAFT_EXPIRED
	st, retry := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/commit",
		map[string]any{"token": token, "decisions": []any{}}, adminTok)
	if st == 400 && code(retry) == "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED" {
		t.Fatal("draft must be refilled after rollback for retry")
	}
}

// V1 验收：乐观锁 version 不匹配 → 409 RESOURCE_VERSION。
func TestOptimisticLockConflict(t *testing.T) {
	st, dc := call("POST", "/api/v1/data-centers", map[string]any{"code": "OL-DC" + short(), "name": "it"}, adminTok)
	if st != 200 {
		t.Fatalf("create dc: %d", st)
	}
	id := data(dc)["id"].(string)
	version := fmt.Sprintf("%.0f", data(dc)["version"].(float64))

	if st, _ := call("PUT", "/api/v1/data-centers/"+id+"?version="+version,
		map[string]any{"code": "OL-DC2" + short(), "name": "v2"}, adminTok); st != 200 {
		t.Fatalf("first update failed")
	}
	st, body := call("PUT", "/api/v1/data-centers/"+id+"?version="+version,
		map[string]any{"code": "OL-DC3" + short(), "name": "v3"}, adminTok)
	if st != 409 || code(body) != "RESOURCE_VERSION" {
		t.Fatalf("stale update must 409 RESOURCE_VERSION, got %d %v", st, body)
	}
}

// 软删除 + 部分唯一索引：删除后同编码可复用。
func TestSoftDeleteCodeReuse(t *testing.T) {
	code := "SD-" + short()
	st, dc := call("POST", "/api/v1/data-centers", map[string]any{"code": code, "name": "first"}, adminTok)
	if st != 200 {
		t.Fatalf("create: %d", st)
	}
	id := data(dc)["id"].(string)
	version := fmt.Sprintf("%.0f", data(dc)["version"].(float64))
	if st, _ := call("DELETE", "/api/v1/data-centers/"+id+"?version="+version, nil, adminTok); st != 200 {
		t.Fatalf("delete failed")
	}
	st, again := call("POST", "/api/v1/data-centers", map[string]any{"code": code, "name": "second"}, adminTok)
	if st != 200 {
		t.Fatalf("code reuse after soft delete must succeed, got %d %v", st, again)
	}
}

// P1-03 验收：8 路并发降级第二个管理员（itadmin 保持管理员），恰一成功（其余乐观锁 409）。
func TestLastAdminConcurrentDemote(t *testing.T) {
	// 建第二个管理员：它的降级是合法操作（itadmin 仍在），竞争发生在 version 乐观锁
	suffix := short()
	st, created := call("POST", "/api/v1/admin/users", map[string]any{
		"username": "itadmin2-" + suffix, "displayName": "二号管理员",
		"password": "ItAdmin2#2026!", "roleCodes": []string{"system_admin", "user"}, "enabled": true,
	}, adminTok)
	if st != 200 {
		t.Fatalf("create second admin: %d %v", st, created)
	}
	adminID := data(created)["id"].(string)
	version := fmt.Sprintf("%.0f", data(created)["version"].(float64))

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, _ := call("PUT", "/api/v1/admin/users/"+adminID+"?version="+version,
				map[string]any{"username": "itadmin2-" + suffix, "displayName": "二号管理员",
					"authSource": "local", "enabled": true, "roleCodes": []string{"user"}}, adminTok)
			mu.Lock()
			defer mu.Unlock()
			if st == 200 {
				ok++
			} else if st != 409 {
				t.Errorf("unexpected demote status %d", st)
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("exactly one demote must win, got %d", ok)
	}

	// 收尾：删除二号管理员，保持环境干净
	st, me := call("GET", "/api/v1/admin/users?search=itadmin2-"+suffix, nil, adminTok)
	if st == 200 {
		for _, it := range data(me)["items"].([]any) {
			m := it.(map[string]any)
			if m["id"] == adminID {
				v := fmt.Sprintf("%.0f", m["version"].(float64))
				call("DELETE", "/api/v1/admin/users/"+adminID+"?version="+v, nil, adminTok)
			}
		}
	}
}

// P0-05 验收：机柜在位设备存在时删除被拒（409 HAS_CHILDREN）。
func TestRackDeleteProtection(t *testing.T) {
	fx := newFixture(t, "DP")
	dev := createDevice(t, fx, "DP-D"+short(), 1)
	if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
		map[string]any{"targetRackId": fx.rackID, "startU": 2}, adminTok); st != 200 {
		t.Fatalf("assign failed")
	}

	st, rack := call("GET", "/api/v1/racks-page?search=K&pageSize=200", nil, adminTok)
	if st != 200 {
		t.Fatalf("list racks: %d", st)
	}
	var version string
	for _, it := range data(rack)["items"].([]any) {
		m := it.(map[string]any)
		if m["id"] == fx.rackID {
			version = fmt.Sprintf("%.0f", m["version"].(float64))
		}
	}
	if version == "" {
		t.Fatal("rack not found in list")
	}
	st, body := call("DELETE", "/api/v1/racks/"+fx.rackID+"?version="+version, nil, adminTok)
	if st != 409 || code(body) != "HAS_CHILDREN" {
		t.Fatalf("delete occupied rack must 409 HAS_CHILDREN, got %d %v", st, body)
	}
}
