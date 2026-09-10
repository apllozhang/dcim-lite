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
	// testDB 供个别测试做 API 之外的兜底恢复（如互降级测试中 itadmin 暂时失去管理员）
	testDB *gorm.DB
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
	testDB = db
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

// approvalStatus 取 assign 响应中的审批单状态（厂商形状：{executed,approval}）
func approvalStatus(body map[string]any) any {
	if ap, ok := data(body)["approval"].(map[string]any); ok {
		return ap["status"]
	}
	return data(body)["status"]
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
	if st != 200 && st != 201 {
		t.Fatalf("create dc: %d %v", st, dc)
	}
	dcID := data(dc)["id"].(string)
	st, room := call("POST", "/api/v1/data-centers/"+dcID+"/rooms", map[string]any{"code": "R", "name": "it"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create room: %d %v", st, room)
	}
	roomID := data(room)["id"].(string)
	st, rack := call("POST", "/api/v1/rooms/"+roomID+"/racks", map[string]any{"code": "K", "name": "it", "uHeight": 20}, adminTok)
	if st != 200 && st != 201 {
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
	if st != 200 && st != 201 {
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
	if st != 200 || approvalStatus(pend) != "PENDING" {
		t.Fatalf("assign should create PENDING, got %d %v", st, pend)
	}
	approvalID := data(pend)["approval"].(map[string]any)["id"].(string)

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
	approvalID := data(pend)["approval"].(map[string]any)["id"].(string)

	st, body := call("POST", "/api/v1/admin/approvals/"+approvalID+"/approve", map[string]any{}, adminTok)
	if st != 409 || code(body) != "RACK_U_CONFLICT" {
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
	if st != 200 && st != 201 {
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
	if st != 409 || code(body) != "RESOURCE_VERSION_CONFLICT" {
		t.Fatalf("stale update must 409 RESOURCE_VERSION, got %d %v", st, body)
	}
}

// 软删除 + 部分唯一索引：删除后同编码可复用。
func TestSoftDeleteCodeReuse(t *testing.T) {
	code := "SD-" + short()
	st, dc := call("POST", "/api/v1/data-centers", map[string]any{"code": code, "name": "first"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create: %d", st)
	}
	id := data(dc)["id"].(string)
	version := fmt.Sprintf("%.0f", data(dc)["version"].(float64))
	if st, _ := call("DELETE", "/api/v1/data-centers/"+id+"?version="+version, nil, adminTok); st != 200 {
		t.Fatalf("delete failed")
	}
	st, again := call("POST", "/api/v1/data-centers", map[string]any{"code": code, "name": "second"}, adminTok)
	if st != 200 && st != 201 {
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
	if st != 200 && st != 201 {
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
	if st != 409 || code(body) != "RESOURCE_HAS_CHILDREN" {
		t.Fatalf("delete occupied rack must 409 HAS_CHILDREN, got %d %v", st, body)
	}
}

// D5 验收：PDU 强制归档需二次确认（回报影响清单连接数）+ 必填原因 + 级联下线插座。
func TestPDUForceArchive(t *testing.T) {
	fx := newFixture(t, "FA")

	st, pdu := call("POST", "/api/v1/racks/"+fx.rackID+"/pdus",
		map[string]any{"code": "FA-PDU" + short(), "name": "强制归档柜"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create pdu: %d %v", st, pdu)
	}
	pduID := data(pdu)["id"].(string)
	pduVer := uint(data(pdu)["version"].(float64))

	st, sock := call("POST", "/api/v1/pdus/"+pduID+"/sockets",
		map[string]any{"socketNo": 1, "standard": "CN", "amperageA": 16}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create socket: %d %v", st, sock)
	}
	sockID := data(sock)["id"].(string)

	dev := createDevice(t, fx, "FA-D"+short(), 1)
	if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
		map[string]any{"rackId": fx.rackID, "startU": 1}, adminTok); st != 200 {
		t.Fatalf("assign device failed")
	}
	if st, _ := call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
		map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok); st != 200 && st != 201 {
		t.Fatalf("connect socket failed")
	}

	// 影响清单：1 插座 / 1 连接
	st, imp := call("GET", "/api/v1/pdus/"+pduID+"/archive-impact", nil, adminTok)
	if st != 200 {
		t.Fatalf("impact: %d %v", st, imp)
	}
	in := data(imp)
	if in["sockets"].(float64) != 1 || in["connections"].(float64) != 1 {
		t.Fatalf("unexpected impact: %v", in)
	}
	if len(in["devices"].([]any)) != 1 {
		t.Fatalf("impact must list the affected device")
	}

	// 普通删除：有连接 -> 409 PDU_IN_USE
	st, body := call("DELETE", "/api/v1/pdus/"+pduID+"?version="+fmt.Sprintf("%d", pduVer), nil, adminTok)
	if st != 409 || code(body) != "PDU_IN_USE" {
		t.Fatalf("plain delete with connections must 409 PDU_IN_USE, got %d %v", st, body)
	}

	// 强制归档：缺原因 -> 400
	st, _ = call("POST", "/api/v1/pdus/"+pduID+"/force-archive",
		map[string]any{"version": pduVer, "reason": "", "confirmConnections": 1}, adminTok)
	if st != 400 {
		t.Fatalf("force-archive without reason must 400, got %d", st)
	}
	// 强制归档：未正确回报连接数 -> 409 IMPACT_CONFIRMATION_REQUIRED
	st, body = call("POST", "/api/v1/pdus/"+pduID+"/force-archive",
		map[string]any{"version": pduVer, "reason": "现场搬迁", "confirmConnections": 0}, adminTok)
	if st != 409 || code(body) != "IMPACT_CONFIRMATION_REQUIRED" {
		t.Fatalf("force-archive without impact confirmation must 409, got %d %v", st, body)
	}
	// 正确确认 -> 200
	st, done := call("POST", "/api/v1/pdus/"+pduID+"/force-archive",
		map[string]any{"version": pduVer, "reason": "现场搬迁", "confirmConnections": 1}, adminTok)
	if st != 200 {
		t.Fatalf("force-archive with confirmation must succeed, got %d %v", st, done)
	}
	// 归档后 PDU 不可见；插座一并下线
	st, _ = call("GET", "/api/v1/pdus/"+pduID+"/sockets", nil, adminTok)
	if st != 404 {
		t.Fatalf("sockets of archived pdu must be gone (404), got %d", st)
	}
}

// ── 复评（2026-09-10）第一批 P0 修复验收 ─────────────────────────────────────

// userVersionByName 按用户名精确查找，返回 (id, version 字符串)。
func userVersionByName(t *testing.T, tok, username string) (string, string) {
	t.Helper()
	st, body := call("GET", "/api/v1/admin/users?search="+username, nil, tok)
	if st != 200 {
		t.Fatalf("list users: %d", st)
	}
	for _, it := range data(body)["items"].([]any) {
		m := it.(map[string]any)
		if m["username"] == username {
			return m["id"].(string), fmt.Sprintf("%.0f", m["version"].(float64))
		}
	}
	t.Fatal("user not found: " + username)
	return "", ""
}

// P0-01 复评验收：系统恰好两名启用管理员时并发互降级。
// 修复前为写偏斜：两个事务各自只锁目标用户行，各自读到对方"仍是管理员"的已提交旧
// 快照而双双放行，系统归零管理员。修复后管理员集合不变量锁（advisory xact lock）
// 串行化校验，每轮恰一成功，系统始终至少一名启用管理员。
func TestLastAdminMutualDemoteNoWriteSkew(t *testing.T) {
	const password = "MutAdmin#2026!"
	mk := func(name string) string {
		st, c := call("POST", "/api/v1/admin/users", map[string]any{
			"username": name, "displayName": name, "password": password,
			"roleCodes": []string{"system_admin"}, "enabled": true}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("create admin %s: %d %v", name, st, c)
		}
		return data(c)["id"].(string)
	}
	nameA, nameB := "mut-a-"+short(), "mut-b-"+short()
	idA, idB := mk(nameA), mk(nameB)
	tokA := mustLogin(nameA, password)
	tokB := mustLogin(nameB, password)

	demote := func(tok, id, ver string) int {
		st, _ := call("PUT", "/api/v1/admin/users/"+id+"?version="+ver,
			map[string]any{"authSource": "local", "enabled": true, "roleCodes": []string{"user"}}, tok)
		return st
	}
	promote := func(tok, id, ver string) {
		call("PUT", "/api/v1/admin/users/"+id+"?version="+ver,
			map[string]any{"authSource": "local", "enabled": true, "roleCodes": []string{"system_admin"}}, tok)
	}

	// 兜底恢复 itadmin：互降级期间 itadmin 暂为普通用户，若断言失败（bug 复现、
	// 系统归零管理员），API 通道已不可用，只能 DB 直连恢复。
	idIT, verIT := userVersionByName(t, adminTok, "itadmin")
	t.Cleanup(func() {
		if err := testDB.Exec(`UPDATE users SET enabled = true, failed_logins = 0, locked_until = NULL WHERE id = ?`, idIT).Error; err != nil {
			t.Logf("restore itadmin failed: %v", err)
		}
		testDB.Exec(`INSERT INTO user_roles (user_id, role_id)
			SELECT ?, id FROM roles WHERE code = 'system_admin'
			ON CONFLICT DO NOTHING`, idIT)
	})
	// 临时降级 itadmin，构造"系统恰好 A、B 两名启用管理员"的严格场景
	if st := demote(adminTok, idIT, verIT); st != 200 {
		t.Fatalf("demote itadmin: %d", st)
	}

	for round := 0; round < 100; round++ {
		_, verA := userVersionByName(t, tokA, nameA)
		_, verB := userVersionByName(t, tokA, nameB)
		var stA, stB int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); stA = demote(tokA, idB, verB) }() // A 降 B
		go func() { defer wg.Done(); stB = demote(tokB, idA, verA) }() // B 降 A
		wg.Wait()
		if (stA == 200) == (stB == 200) {
			t.Fatalf("round %d: mutual demote must be exclusive, got A-demotion=%d B-demotion=%d", round, stB, stA)
		}
		// 恢复被降级的一方为启用管理员，下一轮双方仍恰好是仅有的两名管理员
		if stA == 200 {
			promote(tokA, idB, func() string { _, v := userVersionByName(t, tokA, nameB); return v }())
		} else {
			promote(tokB, idA, func() string { _, v := userVersionByName(t, tokB, nameA); return v }())
		}
	}

	// 终态：系统仍有启用管理员（A、B 至少其一）
	st, body := call("GET", "/api/v1/admin/users", nil, adminTok)
	if st == 200 {
		admins := 0
		for _, it := range data(body)["items"].([]any) {
			m := it.(map[string]any)
			if enabled, ok := m["enabled"].(bool); ok && enabled {
				for _, r := range m["roles"].([]any) {
					if r.(map[string]any)["code"] == "system_admin" {
						admins++
						break
					}
				}
			}
		}
		if admins < 1 {
			t.Fatal("system must always retain at least one enabled admin")
		}
	}
}

// P0-03 复评验收：删除空机柜 vs 上架并发。两操作必须互斥——机柜删除成功则上架必须
// 失败，上架成功则删除必须 409；绝不允许"软删机柜里出现 active position"。
func TestRackDeleteVsPlaceRace(t *testing.T) {
	fx := newFixture(t, "DR")
	for round := 0; round < 15; round++ {
		st, rk := call("POST", "/api/v1/rooms/"+fx.roomID+"/racks",
			map[string]any{"code": "K" + short(), "name": "it", "uHeight": 10}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("create rack: %d %v", st, rk)
		}
		rackID := data(rk)["id"].(string)
		rackVer := fmt.Sprintf("%.0f", data(rk)["version"].(float64))
		dev := createDevice(t, fx, "DR-D"+short(), 1)

		var stDel, stAsg int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stDel, _ = call("DELETE", "/api/v1/racks/"+rackID+"?version="+rackVer, nil, adminTok)
		}()
		go func() {
			defer wg.Done()
			stAsg, _ = call("POST", "/api/v1/devices/"+dev+"/assign",
				map[string]any{"targetRackId": rackID, "startU": 1}, adminTok)
		}()
		wg.Wait()
		if (stDel == 200) == (stAsg == 200) {
			t.Fatalf("round %d: delete=%d assign=%d must be mutually exclusive", round, stDel, stAsg)
		}
		if stDel == 200 && stAsg != 404 && stAsg != 409 {
			t.Fatalf("round %d: assign onto deleted rack must 404/409, got %d", round, stAsg)
		}
	}
}

// P0-03 复评验收：删除空机柜 vs 创建 PDU 并发，互斥性同上。
func TestRackDeleteVsPDURace(t *testing.T) {
	fx := newFixture(t, "DP2")
	for round := 0; round < 15; round++ {
		st, rk := call("POST", "/api/v1/rooms/"+fx.roomID+"/racks",
			map[string]any{"code": "K" + short(), "name": "it", "uHeight": 10}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("create rack: %d %v", st, rk)
		}
		rackID := data(rk)["id"].(string)
		rackVer := fmt.Sprintf("%.0f", data(rk)["version"].(float64))

		var stDel, stPDU int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stDel, _ = call("DELETE", "/api/v1/racks/"+rackID+"?version="+rackVer, nil, adminTok)
		}()
		go func() {
			defer wg.Done()
			stPDU, _ = call("POST", "/api/v1/racks/"+rackID+"/pdus",
				map[string]any{"code": "DP2-P" + short(), "name": "it"}, adminTok)
		}()
		wg.Wait()
		pduOK := stPDU == 200 || stPDU == 201
		if (stDel == 200) == pduOK {
			t.Fatalf("round %d: delete=%d pdu-create=%d must be mutually exclusive", round, stDel, stPDU)
		}
	}
}

// P0-04 复评验收：影响清单过期确认必须被拒绝。客户端取得 impact(connections=1) 后、
// 提交归档前并发接入了第 2 条连接（串行模拟同一竞态），以过期数 confirmConnections=1
// 提交必须 409，且任何连接（含未确认的第 2 条）都不得被断开。
func TestPDUArchiveStaleConfirmationRejected(t *testing.T) {
	fx := newFixture(t, "SA")
	st, pdu := call("POST", "/api/v1/racks/"+fx.rackID+"/pdus",
		map[string]any{"code": "SA-P" + short(), "name": "it"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create pdu: %d %v", st, pdu)
	}
	pduID := data(pdu)["id"].(string)
	pduVer := uint(data(pdu)["version"].(float64))

	sockIDs := make([]string, 0, 2)
	for i := 1; i <= 2; i++ {
		st, sock := call("POST", "/api/v1/pdus/"+pduID+"/sockets",
			map[string]any{"socketNo": i, "standard": "CN", "amperageA": 16}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("create socket %d: %d %v", i, st, sock)
		}
		sockIDs = append(sockIDs, data(sock)["id"].(string))
	}
	dev1 := createDevice(t, fx, "SA-D1"+short(), 1)
	dev2 := createDevice(t, fx, "SA-D2"+short(), 1)
	if st, _ := call("POST", "/api/v1/devices/"+dev1+"/assign",
		map[string]any{"rackId": fx.rackID, "startU": 1}, adminTok); st != 200 {
		t.Fatal("assign dev1 failed")
	}
	if st, _ := call("POST", "/api/v1/devices/"+dev2+"/assign",
		map[string]any{"rackId": fx.rackID, "startU": 2}, adminTok); st != 200 {
		t.Fatal("assign dev2 failed")
	}
	connect1 := func() int {
		st, _ := call("POST", "/api/v1/pdu-sockets/"+sockIDs[0]+"/connection",
			map[string]any{"deviceId": dev1, "redundancyRole": "PRIMARY"}, adminTok)
		return st
	}
	if s := connect1(); s != 200 && s != 201 {
		t.Fatalf("connect dev1: %d", s)
	}
	st, imp := call("GET", "/api/v1/pdus/"+pduID+"/archive-impact", nil, adminTok)
	if st != 200 || int(data(imp)["connections"].(float64)) != 1 {
		t.Fatalf("impact must show 1 connection, got %d %v", st, imp)
	}

	// "确认之后、提交之前"竞态窗口内新增连接
	if st, _ := call("POST", "/api/v1/pdu-sockets/"+sockIDs[1]+"/connection",
		map[string]any{"deviceId": dev2, "redundancyRole": "PRIMARY"}, adminTok); st != 200 && st != 201 {
		t.Fatal("connect dev2 failed")
	}

	// 过期确认 → 409，归档不得执行
	st, body := call("POST", "/api/v1/pdus/"+pduID+"/force-archive",
		map[string]any{"version": pduVer, "reason": "现场搬迁", "confirmConnections": 1}, adminTok)
	if st != 409 || code(body) != "IMPACT_CONFIRMATION_REQUIRED" {
		t.Fatalf("stale confirmation must 409 IMPACT_CONFIRMATION_REQUIRED, got %d %v", st, body)
	}
	// 未确认连接未被断开：机柜下仍有 2 条活动连接
	st, conns := call("GET", "/api/v1/racks/"+fx.rackID+"/pdu-connections", nil, adminTok)
	if st != 200 || len(data(conns)["items"].([]any)) != 2 {
		t.Fatalf("2 connections must survive stale archive, got %d %v", st, conns)
	}
	// 以最新数量重新确认 → 200
	st, _ = call("POST", "/api/v1/pdus/"+pduID+"/force-archive",
		map[string]any{"version": pduVer, "reason": "现场搬迁", "confirmConnections": 2}, adminTok)
	if st != 200 {
		t.Fatalf("archive with fresh confirmation must succeed, got %d", st)
	}
	st, conns = call("GET", "/api/v1/racks/"+fx.rackID+"/pdu-connections", nil, adminTok)
	if st != 200 || len(data(conns)["items"].([]any)) != 0 {
		t.Fatalf("connections must be gone after archive, got %d %v", st, conns)
	}
}

// P0-04 复评验收：PDU 普通删除 vs 接入连接并发。互斥——删除成功则接入必须失败
// （PDU/插座已删），接入成功则删除必须 409 PDU_IN_USE。
func TestPDUDeleteVsConnectRace(t *testing.T) {
	fx := newFixture(t, "DC")
	for round := 0; round < 10; round++ {
		st, pdu := call("POST", "/api/v1/racks/"+fx.rackID+"/pdus",
			map[string]any{"code": "DC-P" + short(), "name": "it"}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("create pdu: %d %v", st, pdu)
		}
		pduID := data(pdu)["id"].(string)
		pduVer := fmt.Sprintf("%.0f", data(pdu)["version"].(float64))
		st, sock := call("POST", "/api/v1/pdus/"+pduID+"/sockets",
			map[string]any{"socketNo": 1, "standard": "CN", "amperageA": 16}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("create socket: %d %v", st, sock)
		}
		sockID := data(sock)["id"].(string)
		dev := createDevice(t, fx, "DC-D"+short(), 1)
		// fixture 机柜跨轮复用，U 位逐轮下移避开上一轮遗留的在位设备
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign failed", round)
		}

		var stDel, stCon int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stDel, _ = call("DELETE", "/api/v1/pdus/"+pduID+"?version="+pduVer, nil, adminTok)
		}()
		go func() {
			defer wg.Done()
			stCon, _ = call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
				map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		}()
		wg.Wait()
		if (stDel == 200) == (stCon == 200 || stCon == 201) {
			t.Fatalf("round %d: pdu-delete=%d connect=%d must be mutually exclusive", round, stDel, stCon)
		}
	}
}

// P0-05 复评验收：审批批准 vs 设备编辑并发。互斥——编辑赢则批准 409（锁内版本校验），
// 批准赢则编辑 409（乐观锁）；绝不出现"批准基于过期设备版本仍执行"。
func TestApproveVsDeviceEditMatrix(t *testing.T) {
	fx := newFixture(t, "AV")
	if st, _ := call("PUT", "/api/v1/admin/approval-policy", map[string]any{"assignApprovalEnabled": true}, adminTok); st != 200 {
		t.Fatal("enable policy failed")
	}
	defer call("PUT", "/api/v1/admin/approval-policy", map[string]any{"assignApprovalEnabled": false}, adminTok)

	for round := 0; round < 10; round++ {
		dev := createDevice(t, fx, "AV-D"+short(), 1)
		st, pend := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"targetRackId": fx.rackID, "startU": 3}, adminTok)
		if st != 200 || approvalStatus(pend) != "PENDING" {
			t.Fatalf("round %d: pending assign: %d %v", round, st, pend)
		}
		approvalID := data(pend)["approval"].(map[string]any)["id"].(string)
		st, dv := call("GET", "/api/v1/devices/"+dev, nil, adminTok)
		if st != 200 {
			t.Fatalf("get device: %d", st)
		}
		devVer := fmt.Sprintf("%.0f", data(dv)["version"].(float64))

		var stApp, stEdit int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stApp, _ = call("POST", "/api/v1/admin/approvals/"+approvalID+"/approve",
				map[string]any{"comment": "go"}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stEdit, _ = call("PUT", "/api/v1/devices/"+dev+"?version="+devVer,
				map[string]any{"name": "renamed-" + short()}, adminTok)
		}()
		wg.Wait()
		if (stApp == 200) == (stEdit == 200) {
			t.Fatalf("round %d: approve=%d edit=%d must be mutually exclusive", round, stApp, stEdit)
		}
		// 语义复核：批准赢 → 设备 RUNNING；编辑赢 → 设备保持 WAITING_RACK 且审批单回滚为 PENDING
		st, dv = call("GET", "/api/v1/devices/"+dev, nil, adminTok)
		if st != 200 {
			t.Fatalf("get device after race: %d", st)
		}
		if stApp == 200 && data(dv)["lifecycleStatus"] != "RUNNING" {
			t.Fatalf("round %d: approved device must be RUNNING, got %v", round, data(dv)["lifecycleStatus"])
		}
		if stEdit == 200 && data(dv)["lifecycleStatus"] != "WAITING_RACK" {
			t.Fatalf("round %d: edit-won device must stay WAITING_RACK, got %v", round, data(dv)["lifecycleStatus"])
		}
	}
}
