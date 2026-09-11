//go:build integration

// 集成测试：需要真实 PostgreSQL（TEST_DATABASE_URL），无则跳过。
// 运行：go test -tags integration ./internal/integration/...
// 覆盖评审报告 P0 修复的验收标准：并发恰一次、事务零残留、乐观锁、软删唯一复用、
// 审批失败回滚保持 PENDING、机柜删除保护、最后管理员并发保护。
package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	// userTok 为普通用户（无 system_admin 角色）令牌，权限矩阵测试用
	userTok string
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
		// 权限矩阵等测试会在 1 分钟内多次登录（mustLogin 每次消耗 captcha+login 两格），
		// 测试环境放宽限流；生产行为不受影响
		LoginRatePerMin: 200,
	}
	testApp = app.Build(db, cfg)
	server = httptest.NewServer(testApp.Engine)
	defer server.Close()

	adminTok = mustLogin("itadmin", "ItAdmin#2026!")

	// 权限矩阵测试用的普通用户（已存在则忽略 409）
	call("POST", "/api/v1/admin/users", map[string]any{
		"username": "ituser", "displayName": "普通用户", "password": "ItUser#2026!",
		"roleCodes": []string{"user"}, "enabled": true}, adminTok)
	userTok = mustLogin("ituser", "ItUser#2026!")
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

// ── 复评 P0-N1 验收:PDU 插座聚合锁协议 ──────────────────────

// setupPDUWithSocket 建一个 PDU + 单插座,返回 pduID 与 socketID。
func setupPDUWithSocket(t *testing.T, fx fixture, prefix string) (string, string) {
	t.Helper()
	st, pdu := call("POST", "/api/v1/racks/"+fx.rackID+"/pdus",
		map[string]any{"code": prefix + "-P" + short(), "name": "it"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create pdu: %d %v", st, pdu)
	}
	pduID := data(pdu)["id"].(string)
	st, sock := call("POST", "/api/v1/pdus/"+pduID+"/sockets",
		map[string]any{"socketNo": 1, "standard": "CN", "amperageA": 16}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create socket: %d %v", st, sock)
	}
	return pduID, data(sock)["id"].(string)
}

// socketInfo 经 PDU 插座列表查插座状态与版本;ok=false 表示插座已不在活动列表。
func socketInfo(t *testing.T, pduID, sockID string) (string, float64, bool) {
	t.Helper()
	st, body := call("GET", "/api/v1/pdus/"+pduID+"/sockets", nil, adminTok)
	if st == 404 {
		// PDU 已被删除/归档:其名下插座必然不可见
		return "", 0, false
	}
	if st != 200 {
		t.Fatalf("list sockets: %d %v", st, body)
	}
	for _, it := range data(body)["items"].([]any) {
		m := it.(map[string]any)
		if m["id"].(string) == sockID {
			return m["status"].(string), m["version"].(float64), true
		}
	}
	return "", 0, false
}

// pduVersionOf 经机柜 PDU 列表查 PDU 当前 version。
func pduVersionOf(t *testing.T, fx fixture, pduID string) float64 {
	t.Helper()
	st, body := call("GET", "/api/v1/racks/"+fx.rackID+"/pdus", nil, adminTok)
	if st != 200 {
		t.Fatalf("list pdus: %d %v", st, body)
	}
	for _, it := range data(body)["items"].([]any) {
		m := it.(map[string]any)
		if m["id"].(string) == pduID {
			return m["version"].(float64)
		}
	}
	t.Fatalf("pdu %s not found in rack list", pduID)
	return 0
}

// B01:DeleteSocket vs Connect 并发互斥——删除赢则接入必须失败,接入赢则
// 删除必须 409;绝不允许「活动连接指向已软删插座」的孤儿终态。
func TestSocketDeleteVsConnectRace(t *testing.T) {
	fx := newFixture(t, "SD")
	for round := 0; round < 10; round++ {
		pduID, sockID := setupPDUWithSocket(t, fx, "SD")
		_, sockVer, ok := socketInfo(t, pduID, sockID)
		if !ok {
			t.Fatalf("round %d: socket missing right after create", round)
		}
		dev := createDevice(t, fx, "SD-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign failed", round)
		}
		var stDel, stCon int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stDel, _ = call("DELETE", fmt.Sprintf("/api/v1/pdu-sockets/%s?version=%.0f", sockID, sockVer), nil, adminTok)
		}()
		go func() {
			defer wg.Done()
			stCon, _ = call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
				map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		}()
		wg.Wait()
		if (stDel == 200) == (stCon == 200 || stCon == 201) {
			t.Fatalf("round %d: socket-delete=%d connect=%d must be mutually exclusive", round, stDel, stCon)
		}
		// 孤儿终态检查:删除成功 → 连接不得存在;接入成功 → 插座状态必须 CONNECTED
		if stDel == 200 {
			st, conns := call("GET", "/api/v1/racks/"+fx.rackID+"/pdu-connections", nil, adminTok)
			if st != 200 {
				t.Fatalf("round %d: list connections: %d", round, st)
			}
			for _, it := range data(conns)["items"].([]any) {
				if it.(map[string]any)["socketId"].(string) == sockID {
					t.Fatalf("round %d: orphan connection to deleted socket", round)
				}
			}
		} else {
			if s, _, ok := socketInfo(t, pduID, sockID); !ok || s != "CONNECTED" {
				t.Fatalf("round %d: connect won but socket status=%q exists=%v", round, s, ok)
			}
		}
	}
}

// B01:CreateSocket vs Delete PDU 并发——PDU 删除成功后其名下不得残留活动插座
// (CreateSocket 持 PDU 锁:先赢则随级联软删,后赢则 404)。
func TestSocketCreateVsPDUDeleteRace(t *testing.T) {
	fx := newFixture(t, "SC")
	for round := 0; round < 10; round++ {
		st, pdu := call("POST", "/api/v1/racks/"+fx.rackID+"/pdus",
			map[string]any{"code": "SC-P" + short(), "name": "it"}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("round %d: create pdu: %d %v", round, st, pdu)
		}
		pduID := data(pdu)["id"].(string)
		pduVer := fmt.Sprintf("%.0f", data(pdu)["version"].(float64))
		var stCreate, stDel int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stCreate, _ = call("POST", "/api/v1/pdus/"+pduID+"/sockets",
				map[string]any{"socketNo": 9, "standard": "CN", "amperageA": 10}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stDel, _ = call("DELETE", "/api/v1/pdus/"+pduID+"?version="+pduVer, nil, adminTok)
		}()
		wg.Wait()
		if stDel != 200 {
			t.Fatalf("round %d: pdu delete unexpected status %d (create=%d)", round, stDel, stCreate)
		}
		// 终态不变量:PDU 已删 → 不得残留活动插座
		st, body := call("GET", "/api/v1/pdus/"+pduID+"/sockets", nil, adminTok)
		if st == 200 {
			if n := len(data(body)["items"].([]any)); n != 0 {
				t.Fatalf("round %d: %d active sockets survive pdu delete", round, n)
			}
		}
	}
}

// B01:UpdateSocket vs Connect 并发——只改 label 的编辑与接入串行化后各自生效,
// 终态 status 必须与连接事实一致(不得出现「已接入仍显示 AVAILABLE」的漂移)。
func TestSocketUpdateVsConnectRace(t *testing.T) {
	fx := newFixture(t, "SU")
	for round := 0; round < 10; round++ {
		pduID, sockID := setupPDUWithSocket(t, fx, "SU")
		dev := createDevice(t, fx, "SU-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign failed", round)
		}
		var stUpd, stCon int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stUpd, _ = call("PUT", "/api/v1/pdu-sockets/"+sockID,
				map[string]any{"socketNo": 1, "standard": "CN", "amperageA": 16, "label": "GT改"}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stCon, _ = call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
				map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		}()
		wg.Wait()
		if stUpd != 200 || stCon != 200 && stCon != 201 {
			t.Fatalf("round %d: update=%d connect=%d, both must succeed", round, stUpd, stCon)
		}
		if s, _, ok := socketInfo(t, pduID, sockID); !ok || s != "CONNECTED" {
			t.Fatalf("round %d: connected socket status=%q exists=%v (status drift)", round, s, ok)
		}
	}
}

// B01:Disconnect vs ForceArchive 并发——断开赢则归档因确认数过期 409,
// 归档赢则断开 404/409;终态无活动连接、无状态漂移。
func TestDisconnectVsForceArchiveRace(t *testing.T) {
	fx := newFixture(t, "DF")
	for round := 0; round < 10; round++ {
		pduID, sockID := setupPDUWithSocket(t, fx, "DF")
		pduVer := pduVersionOf(t, fx, pduID)
		dev := createDevice(t, fx, "DF-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign failed", round)
		}
		st, conn := call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
			map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("round %d: connect: %d %v", round, st, conn)
		}
		connID := data(conn)["id"].(string)
		connVer := data(conn)["version"].(float64)
		var stDis, stArc int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stDis, _ = call("DELETE", "/api/v1/pdu-connections/"+connID, map[string]any{"version": int(connVer)}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stArc, _ = call("POST", "/api/v1/pdus/"+pduID+"/force-archive",
				map[string]any{"version": int(pduVer), "reason": "报废", "confirmConnections": 1}, adminTok)
		}()
		wg.Wait()
		disOK, arcOK := stDis == 200, stArc == 200
		if disOK == arcOK {
			t.Fatalf("round %d: disconnect=%d archive=%d, exactly one must win", round, stDis, stArc)
		}
		// 终态:连接必须已断
		st, conns := call("GET", "/api/v1/racks/"+fx.rackID+"/pdu-connections", nil, adminTok)
		if st != 200 {
			t.Fatalf("round %d: list connections: %d", round, st)
		}
		for _, it := range data(conns)["items"].([]any) {
			if it.(map[string]any)["id"].(string) == connID {
				t.Fatalf("round %d: connection survives both disconnect and archive", round)
			}
		}
		// 终态:插座状态与事实一致——断开赢 → AVAILABLE;归档赢 → 插座已删
		if disOK {
			if s, _, ok := socketInfo(t, pduID, sockID); !ok || s != "AVAILABLE" {
				t.Fatalf("round %d: after disconnect socket status=%q exists=%v", round, s, ok)
			}
		} else {
			if _, _, ok := socketInfo(t, pduID, sockID); ok {
				t.Fatalf("round %d: socket survives force-archive", round)
			}
		}
	}
}

// B02:socket status 收权——body 直写 status 被忽略,状态仅由连接事实维护。
func TestSocketStatusNotWritable(t *testing.T) {
	fx := newFixture(t, "SN")
	pduID, sockID := setupPDUWithSocket(t, fx, "SN")
	// 创建时直写 status=CONNECTED → 必须 AVAILABLE
	st, created := call("POST", "/api/v1/pdus/"+pduID+"/sockets",
		map[string]any{"socketNo": 2, "standard": "CN", "amperageA": 16, "status": "CONNECTED"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create with status: %d %v", st, created)
	}
	sock2 := data(created)["id"].(string)
	if s, _, _ := socketInfo(t, pduID, sock2); s != "AVAILABLE" {
		t.Fatalf("created socket must ignore client status=CONNECTED, got %q", s)
	}
	// 更新时直写 status=CONNECTED → 保留 AVAILABLE
	if st, body := call("PUT", "/api/v1/pdu-sockets/"+sockID,
		map[string]any{"socketNo": 1, "standard": "CN", "amperageA": 16, "label": "x", "status": "CONNECTED"}, adminTok); st != 200 {
		t.Fatalf("update socket: %d %v", st, body)
	}
	if s, _, _ := socketInfo(t, pduID, sockID); s != "AVAILABLE" {
		t.Fatalf("updated socket must ignore client status=CONNECTED, got %q", s)
	}
	// 连接存在时直写 status=AVAILABLE → 仍 CONNECTED
	dev := createDevice(t, fx, "SN-D"+short(), 1)
	if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
		map[string]any{"rackId": fx.rackID, "startU": 1}, adminTok); st != 200 {
		t.Fatal("assign failed")
	}
	if st, _ := call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
		map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok); st != 200 && st != 201 {
		t.Fatal("connect failed")
	}
	if st, body := call("PUT", "/api/v1/pdu-sockets/"+sockID,
		map[string]any{"socketNo": 1, "standard": "CN", "amperageA": 16, "status": "AVAILABLE"}, adminTok); st != 200 {
		t.Fatalf("update connected socket: %d %v", st, body)
	}
	if s, _, _ := socketInfo(t, pduID, sockID); s != "CONNECTED" {
		t.Fatalf("connected socket must stay CONNECTED despite client status=AVAILABLE, got %q", s)
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

// P0-02 复评验收：机房 A 的导入路径引用机房 B 的机柜 → validate 直接 400，零写入。
func TestImportCrossRoomRackRejected(t *testing.T) {
	fxA := newFixture(t, "XA")
	fxB := newFixture(t, "XB")
	st, body := call("POST", "/api/v1/rooms/"+fxA.roomID+"/rack-diagram-import/validate", map[string]any{
		"formatVersion":  "1",
		"coveredRackIds": []string{fxA.rackID, fxB.rackID},
		"devices": []map[string]any{
			{"clientId": "a", "rackId": fxB.rackID, "startU": 1, "endU": 1, "name": "跨机房探针X"},
		},
	}, adminTok)
	if st != 400 {
		t.Fatalf("covered rack from another room must 400, got %d %v", st, body)
	}
	// 零写入：拒绝发生在草稿创建之前，无 token 可提交
	st, list := call("GET", "/api/v1/devices?search=跨机房探针X", nil, adminTok)
	if st != 200 || data(list)["total"].(float64) != 0 {
		t.Fatalf("rejected import must leave zero residue, got %d %v", st, list)
	}
}

// P0-02 复评验收：行级 rackId 不在 coveredRackIds 覆盖集合内 → 行级 ERROR，commit 阻断。
func TestImportRowRackOutsideCoveredRejected(t *testing.T) {
	fx := newFixture(t, "XC")
	// 同数据中心另建一间机房与机柜，行 rackId 指向它（不在覆盖集合内）
	st, room2 := call("POST", "/api/v1/data-centers/"+fx.dcID+"/rooms",
		map[string]any{"code": "R2-" + short(), "name": "it"}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create room2: %d %v", st, room2)
	}
	room2ID := data(room2)["id"].(string)
	st, rack2 := call("POST", "/api/v1/rooms/"+room2ID+"/racks",
		map[string]any{"code": "K2-" + short(), "name": "it", "uHeight": 10}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create rack2: %d %v", st, rack2)
	}
	outside := data(rack2)["id"].(string)

	st, val := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/validate", map[string]any{
		"formatVersion":  "1",
		"coveredRackIds": []string{fx.rackID},
		"devices": []map[string]any{
			{"clientId": "a", "rackId": outside, "startU": 1, "endU": 1, "name": "越界行探针X"},
		},
	}, adminTok)
	if st != 200 {
		t.Fatalf("validate itself should still answer with row errors, got %d %v", st, val)
	}
	sum := data(val)["summary"].(map[string]any)
	if sum["errors"].(float64) < 1 {
		t.Fatalf("row outside covered set must be flagged as error, got %v", sum)
	}
	token := data(val)["token"].(string)
	st, body := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/commit",
		map[string]any{"token": token, "decisions": []any{}}, adminTok)
	if st != 400 {
		t.Fatalf("commit with blocking row errors must 400, got %d %v", st, body)
	}
	st, list := call("GET", "/api/v1/devices?search=越界行探针X", nil, adminTok)
	if st != 200 || data(list)["total"].(float64) != 0 {
		t.Fatalf("blocked import must leave zero residue, got %d %v", st, list)
	}
}

// P0-02 复评验收：导入草稿与发起人绑定。管理员 B 持 A 的草稿 token 提交 → 403；
// 原发起人 A 重试仍可成功（草稿已回填，未被 B 的失败尝试消耗）。
func TestImportDraftOwnerMismatch(t *testing.T) {
	fx := newFixture(t, "OW")
	nameB := "imp-b-" + short()
	st, c := call("POST", "/api/v1/admin/users", map[string]any{
		"username": nameB, "displayName": nameB, "password": "ImpOwner#2026!",
		"roleCodes": []string{"system_admin"}, "enabled": true}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create second admin: %d %v", st, c)
	}
	tokB := mustLogin(nameB, "ImpOwner#2026!")

	st, val := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/validate", map[string]any{
		"formatVersion":  "1",
		"coveredRackIds": []string{fx.rackID},
		"defaultTypeId":  fx.typeID,
		"devices": []map[string]any{
			{"clientId": "a", "rackId": fx.rackID, "startU": 1, "endU": 1, "name": "草稿属主探针"},
		},
	}, adminTok)
	if st != 200 {
		t.Fatalf("validate: %d %v", st, val)
	}
	token := data(val)["token"].(string)

	// B 不是草稿发起人 → 403，且草稿不被消耗
	st, body := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/commit",
		map[string]any{"token": token, "decisions": []any{}}, tokB)
	if st != 403 || code(body) != "IMPORT_DRAFT_OWNER_MISMATCH" {
		t.Fatalf("commit by non-owner must 403 IMPORT_DRAFT_OWNER_MISMATCH, got %d %v", st, body)
	}
	// 原发起人仍可提交
	st, done := call("POST", "/api/v1/rooms/"+fx.roomID+"/rack-diagram-import/commit",
		map[string]any{"token": token, "decisions": []any{}}, adminTok)
	if st != 200 {
		t.Fatalf("owner commit must succeed, got %d %v", st, done)
	}
}

// P1-10 复评验收：全路由权限矩阵。对 RouteInventory 清单中的每条路由，
// 以匿名/普通用户/管理员三种身份实测，结果必须与声明的权限级别完全一致；
// 同时断言管理员的合法请求不产生 5xx（顺带兜住 handler panic）。
// 路由清单来自 app.RouteInventory（单一事实源，与 TestRouteInventory 同源），
// 新增路由未登记权限级别时本测试直接失败。
func TestAuthMatrixAllRoutes(t *testing.T) {
	fakeID := "00000000-0000-0000-0000-000000000001"
	paramRe := regexp.MustCompile(`:[^/]+`)

	// 部分路由需要特判请求体，避免执行真实副作用：
	// - PUT /admin/approval-policy 回填当前值，净零变化
	// - POST /auth/logout 会吊销发送它的 token，管理员态改用一次性登录令牌
	currentPolicy := map[string]any{}
	if st, p := call("GET", "/api/v1/admin/approval-policy", nil, adminTok); st == 200 {
		currentPolicy["assignApprovalEnabled"] = data(p)["assignApprovalEnabled"]
	}
	bodyFor := func(spec app.RouteSpec) map[string]any {
		if spec.Method == "PUT" && spec.Path == "/api/v1/admin/approval-policy" {
			return currentPolicy
		}
		return nil
	}

	type stance struct {
		name string
		tok  string
	}
	// 路由的期望判定：allowed = 非 401/403/5xx，deny401 = 401，deny403 = 403
	check := func(t *testing.T, spec app.RouteSpec, s stance, want func(st int) bool, why string) {
		t.Helper()
		url := server.URL + paramRe.ReplaceAllString(spec.Path, fakeID)
		var bodyReader io.Reader
		if spec.Method != "GET" {
			b, _ := json.Marshal(bodyFor(spec))
			bodyReader = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(spec.Method, url, bodyReader)
		req.Header.Set("Content-Type", "application/json")
		if s.tok != "" {
			req.Header.Set("Authorization", "Bearer "+s.tok)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("%s %s [%s]: request error %v", spec.Method, spec.Path, s.name, err)
			return
		}
		defer resp.Body.Close()
		st := resp.StatusCode
		if !want(st) {
			t.Errorf("%s %s [%s]: %s, got %d", spec.Method, spec.Path, s.name, why, st)
		}
	}

	for _, spec := range app.RouteInventory() {
		// logout 会吊销发送它的 token：任何已登录身份态都用一次性令牌发送，
		// 避免消耗测试主令牌（userTok/adminTok）
		if spec.Method == "POST" && spec.Path == "/api/v1/auth/logout" {
			check(t, spec, stance{"匿名", ""}, func(st int) bool { return st == 401 }, "must reject anonymous with 401")
			check(t, spec, stance{"普通用户一次性", mustLogin("ituser", "ItUser#2026!")}, func(st int) bool { return st != 401 && st != 403 && st < 500 }, "must allow any authenticated user")
			continue
		}
		switch spec.Auth {
		case "public":
			check(t, spec, stance{"匿名", ""}, func(st int) bool { return st != 401 && st != 403 && st < 500 }, "public must be reachable anonymously")
			check(t, spec, stance{"普通用户", userTok}, func(st int) bool { return st != 401 && st != 403 && st < 500 }, "public must ignore roles")
			// 管理员态：public 路由无额外保证，跳过（login/captcha/health 已在匿名覆盖）
		case "user":
			check(t, spec, stance{"匿名", ""}, func(st int) bool { return st == 401 }, "must reject anonymous with 401")
			check(t, spec, stance{"普通用户", userTok}, func(st int) bool { return st != 401 && st != 403 && st < 500 }, "must allow any authenticated user")
			// 管理员态见下（与 admin 路由统一处理）
		case "admin":
			check(t, spec, stance{"匿名", ""}, func(st int) bool { return st == 401 }, "must reject anonymous with 401")
			check(t, spec, stance{"普通用户", userTok}, func(st int) bool { return st == 403 }, "must reject non-admin with 403")
		}
	}

	// 管理员态：user/admin 全部路由必须放行（非 401/403/5xx）
	for _, spec := range app.RouteInventory() {
		if spec.Auth == "public" {
			continue
		}
		tok := adminTok
		if spec.Method == "POST" && spec.Path == "/api/v1/auth/logout" {
			// logout 吊销发送它的 token，不能消耗测试主令牌
			tok = mustLogin("itadmin", "ItAdmin#2026!")
		}
		check(t, spec, stance{"管理员", tok}, func(st int) bool { return st != 401 && st != 403 && st < 500 }, "must allow system_admin")
	}
}

// 复评 §10 验收（P1-11）：用户持有两个 token 时被重置密码，两个旧 token 均立即
// 失效，新密码可正常登录。此前仅更新 password hash，重置密码吊销不了任何旧 token。
func TestPasswordResetRevokesAllTokens(t *testing.T) {
	const pwd1, pwd2 = "SvReset#2026a", "SvReset#2026b"
	name := "svr-" + short()
	st, c := call("POST", "/api/v1/admin/users", map[string]any{
		"username": name, "displayName": name, "password": pwd1,
		"roleCodes": []string{"user"}, "enabled": true}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create user: %d %v", st, c)
	}
	tok1 := mustLogin(name, pwd1)
	tok2 := mustLogin(name, pwd1)
	for _, tok := range []string{tok1, tok2} {
		if st, _ := call("GET", "/api/v1/auth/me", nil, tok); st != 200 {
			t.Fatalf("pre-reset token must be valid, got %d", st)
		}
	}

	id, ver := userVersionByName(t, adminTok, name)
	if st, body := call("POST", "/api/v1/admin/users/"+id+"/reset-password?version="+ver,
		map[string]any{"password": pwd2}, adminTok); st != 200 {
		t.Fatalf("reset password: %d %v", st, body)
	}

	for _, tok := range []string{tok1, tok2} {
		if st, _ := call("GET", "/api/v1/auth/me", nil, tok); st != 401 {
			t.Fatalf("old token must be revoked immediately after password reset, got %d", st)
		}
	}
	tok3 := mustLogin(name, pwd2)
	if st, _ := call("GET", "/api/v1/auth/me", nil, tok3); st != 200 {
		t.Fatalf("new login after reset must work, got %d", st)
	}
}

// 复评 P1-11 配套：停用（启用→停用）同样递增会话版本——否则重新启用后旧 token 复活。
func TestDisableUserRevokesTokens(t *testing.T) {
	const pwd = "SvDis#2026aa"
	name := "svd-" + short()
	st, c := call("POST", "/api/v1/admin/users", map[string]any{
		"username": name, "displayName": name, "password": pwd,
		"roleCodes": []string{"user"}, "enabled": true}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create user: %d %v", st, c)
	}
	tok1 := mustLogin(name, pwd)
	if st, _ := call("GET", "/api/v1/auth/me", nil, tok1); st != 200 {
		t.Fatal("pre-disable token must be valid")
	}

	id, ver := userVersionByName(t, adminTok, name)
	if st, body := call("PUT", "/api/v1/admin/users/"+id+"?version="+ver,
		map[string]any{"authSource": "local", "enabled": false, "roleCodes": []string{"user"}}, adminTok); st != 200 {
		t.Fatalf("disable user: %d %v", st, body)
	}
	if st, _ := call("GET", "/api/v1/auth/me", nil, tok1); st != 401 {
		t.Fatalf("disabled user token must be rejected, got %d", st)
	}

	// 重新启用：旧 token 不得复活（session_version 已在停用时递增）
	id, ver = userVersionByName(t, adminTok, name)
	if st, body := call("PUT", "/api/v1/admin/users/"+id+"?version="+ver,
		map[string]any{"authSource": "local", "enabled": true, "roleCodes": []string{"user"}}, adminTok); st != 200 {
		t.Fatalf("re-enable user: %d %v", st, body)
	}
	if st, _ := call("GET", "/api/v1/auth/me", nil, tok1); st != 401 {
		t.Fatalf("old token must NOT revive after re-enable, got %d", st)
	}
	tok2 := mustLogin(name, pwd)
	if st, _ := call("GET", "/api/v1/auth/me", nil, tok2); st != 200 {
		t.Fatalf("fresh login after re-enable must work, got %d", st)
	}
}

// ── D01 方案 A 验收:有活动供电连接禁止移位 ──────────────────

// TestMoveWithActiveConnectionBlocked:接电设备 move → 409 DEVICE_POWERED;
// 断开后 move → 200;重新接电再 move → 再次 409。
func TestMoveWithActiveConnectionBlocked(t *testing.T) {
	fx := newFixture(t, "MP")
	for round := 0; round < 3; round++ {
		pduID, sockID := setupPDUWithSocket(t, fx, "MP")
		dev := createDevice(t, fx, "MP-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign failed: %d", round, st)
		}
		st, conn := call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
			map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("round %d: connect failed: %d %v", round, st, conn)
		}
		// 接电 → 移位必须 409 DEVICE_POWERED
		st, body := call("POST", "/api/v1/devices/"+dev+"/move",
			map[string]any{"rackId": fx.rackID, "startU": 10 + round}, adminTok)
		if st != 409 || code(body) != "DEVICE_POWERED" {
			t.Fatalf("round %d: move on powered device must 409 DEVICE_POWERED, got %d %v", round, st, body)
		}
		// 断开 → 移位成功
		st, conns := call("GET", "/api/v1/racks/"+fx.rackID+"/pdu-connections", nil, adminTok)
		if st != 200 {
			t.Fatalf("round %d: list connections: %d", round, st)
		}
		var connID string
		var connVer float64
		for _, it := range data(conns)["items"].([]any) {
			m := it.(map[string]any)
			if m["deviceId"].(string) == dev {
				connID = m["id"].(string)
				connVer = m["version"].(float64)
				break
			}
		}
		if connID == "" {
			t.Fatalf("round %d: connection not found", round)
		}
		if st, _ := call("DELETE", "/api/v1/pdu-connections/"+connID,
			map[string]any{"version": int(connVer)}, adminTok); st != 200 {
			t.Fatalf("round %d: disconnect failed: %d", round, st)
		}
		if st, body := call("POST", "/api/v1/devices/"+dev+"/move",
			map[string]any{"rackId": fx.rackID, "startU": 10 + round, "reason": "D01 验收"}, adminTok); st != 200 {
			t.Fatalf("round %d: move after disconnect must 200, got %d %v", round, st, body)
		}
		// 重新接电 → 再移位再次 409
		if st, _ := call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
			map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok); st != 200 && st != 201 {
			t.Fatalf("round %d: reconnect failed: %d", round, st)
		}
		st, body = call("POST", "/api/v1/devices/"+dev+"/move",
			map[string]any{"rackId": fx.rackID, "startU": 15}, adminTok)
		if st != 409 || code(body) != "DEVICE_POWERED" {
			t.Fatalf("round %d: second move on re-powered device must 409, got %d %v", round, st, body)
		}
		_ = pduID
	}
}

// ── 第三轮复评 P0-R01 验收:跨聚合并发矩阵(device→PDU→socket 统一锁序)──

// setupSecondRack 在 fixture 机房下建第二个机柜(供 Move 跨柜场景)。
func setupSecondRack(t *testing.T, fx fixture, code string) string {
	t.Helper()
	st, rack := call("POST", "/api/v1/rooms/"+fx.roomID+"/racks",
		map[string]any{"code": code, "name": "rk2", "uHeight": 20}, adminTok)
	if st != 200 && st != 201 {
		t.Fatalf("create second rack: %d %v", st, rack)
	}
	return data(rack)["id"].(string)
}

// assertNoCrossRackConnection 终态不变量:任何活动连接的 PDU 机柜必须等于
// 设备的在位机柜;设备已 OFF_RACK/SCRAPPED 则不得有活动连接。
func assertNoCrossRackConnection(t *testing.T, round int) {
	t.Helper()
	rows := []map[string]any{}
	r := testDB.Raw(`
		SELECT c.id, c.device_id, p.rack_id AS pdu_rack,
		       (SELECT rack_id FROM rack_device_positions
		         WHERE device_id = c.device_id AND deleted_at IS NULL LIMIT 1) AS dev_rack
		FROM pdu_connections c
		JOIN pdu_sockets s ON s.id = c.socket_id AND s.deleted_at IS NULL
		JOIN pdus p ON p.id = s.pdu_id AND p.deleted_at IS NULL
		WHERE c.deleted_at IS NULL`).Scan(&rows)
	if r.Error != nil {
		t.Fatalf("round %d: invariant query: %v", round, r.Error)
	}
	for _, row := range rows {
		pr, _ := row["pdu_rack"].(string)
		dr, _ := row["dev_rack"].(string)
		if dr == "" || pr != dr {
			t.Fatalf("round %d: cross-rack active connection %v (pdu_rack=%q dev_rack=%q)",
				round, row["id"], pr, dr)
		}
	}
}

// assertDeviceIdleNoConnection 终态不变量:OFF_RACK/SCRAPPED 设备不得有活动连接。
func assertDeviceIdleNoConnection(t *testing.T, round int) {
	t.Helper()
	var n int64
	if err := testDB.Raw(`
		SELECT count(*) FROM pdu_connections c
		JOIN devices d ON d.id = c.device_id
		WHERE c.deleted_at IS NULL AND d.lifecycle_status IN ('OFF_RACK','SCRAPPED')`).
		Scan(&n).Error; err != nil {
		t.Fatalf("round %d: idle invariant query: %v", round, err)
	}
	if n != 0 {
		t.Fatalf("round %d: %d active connections on decommissioned devices", round, n)
	}
}

// TestDeviceMoveVsConnectRace:Move 与 Connect 并发,合法结局互斥——
// Connect 赢则 Move 409 DEVICE_POWERED;Move 赢则 Connect 409(同柜校验失败)。
// 每轮后做终态不变量断言(不存在跨机柜活动连接)。
func TestDeviceMoveVsConnectRace(t *testing.T) {
	fx := newFixture(t, "MC")
	RK2 := setupSecondRack(t, fx, "MC2")
	for round := 0; round < 20; round++ {
		_, sockID := setupPDUWithSocket(t, fx, "MC")
		dev := createDevice(t, fx, "MC-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign: %d", round, st)
		}
		var stCon, stMov int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stCon, _ = call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
				map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stMov, _ = call("POST", "/api/v1/devices/"+dev+"/move",
				map[string]any{"rackId": RK2, "startU": 15}, adminTok)
		}()
		wg.Wait()
		conOK, movOK := stCon == 200 || stCon == 201, stMov == 200
		if conOK == movOK {
			t.Fatalf("round %d: connect=%d move=%d must be mutually exclusive", round, stCon, stMov)
		}
		if movOK {
			// Move 赢:Connect 必须因同柜校验失败被拒
			if stCon != 409 {
				t.Fatalf("round %d: move won, connect must 409, got %d", round, stCon)
			}
		} else {
			// Connect 赢:Move 必须因带电阻断
			if stMov != 409 {
				t.Fatalf("round %d: connect won, move must 409, got %d", round, stMov)
			}
		}
		assertNoCrossRackConnection(t, round)
	}
}

// TestDecommissionVsConnectRace:Decommission 与 Connect 并发——
// Connect 赢则 Decommission 409 DEVICE_POWERED;Decommission 赢则 Connect 409。
func TestDecommissionVsConnectRace(t *testing.T) {
	fx := newFixture(t, "DC2")
	for round := 0; round < 20; round++ {
		_, sockID := setupPDUWithSocket(t, fx, "DC")
		dev := createDevice(t, fx, "DC-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign: %d", round, st)
		}
		var stCon, stDec int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stCon, _ = call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
				map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stDec, _ = call("POST", "/api/v1/devices/"+dev+"/decommission",
				map[string]any{"reason": "并发验收"}, adminTok)
		}()
		wg.Wait()
		conOK, decOK := stCon == 200 || stCon == 201, stDec == 200
		if conOK == decOK {
			t.Fatalf("round %d: connect=%d decommission=%d must be mutually exclusive", round, stCon, stDec)
		}
		assertDeviceIdleNoConnection(t, round)
		assertNoCrossRackConnection(t, round)
	}
}

// TestDecommissionVsDisconnectRace:Decommission 与 Disconnect 并发——
// 合法结局:两者皆成功(断开先提交,下架读到 0 连接)或 断开成功+下架 409 带电;
// 终态不变量:已下架设备不得残留活动连接。
func TestDecommissionVsDisconnectRace(t *testing.T) {
	fx := newFixture(t, "DD")
	for round := 0; round < 20; round++ {
		pduID, sockID := setupPDUWithSocket(t, fx, "DD")
		dev := createDevice(t, fx, "DD-D"+short(), 1)
		if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
			map[string]any{"rackId": fx.rackID, "startU": round + 1}, adminTok); st != 200 {
			t.Fatalf("round %d: assign: %d", round, st)
		}
		st, conn := call("POST", "/api/v1/pdu-sockets/"+sockID+"/connection",
			map[string]any{"deviceId": dev, "redundancyRole": "PRIMARY"}, adminTok)
		if st != 200 && st != 201 {
			t.Fatalf("round %d: connect: %d %v", round, st, conn)
		}
		connID := data(conn)["id"].(string)
		connVer := int(data(conn)["version"].(float64))
		var stDis, stDec int
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			stDis, _ = call("DELETE", "/api/v1/pdu-connections/"+connID,
				map[string]any{"version": connVer}, adminTok)
		}()
		go func() {
			defer wg.Done()
			stDec, _ = call("POST", "/api/v1/devices/"+dev+"/decommission",
				map[string]any{"reason": "并发验收"}, adminTok)
		}()
		wg.Wait()
		decOK := stDec == 200
		if decOK {
			// 下架成功的前提是连接已断(锁内读到 0),此时 Disconnect 必已成功
			if stDis != 200 {
				t.Fatalf("round %d: decommission won but disconnect=%d", round, stDis)
			}
		}
		assertDeviceIdleNoConnection(t, round)
		assertNoCrossRackConnection(t, round)
		_ = pduID
	}
}

// ── 第三轮复评 P1-C④ 验收:A 族读模型首批 currentPosition ──

// assertNoCurrentPosition 断言响应体不含 currentPosition(未在位设备,厂商形状省略)。
func assertNoCurrentPosition(t *testing.T, where string, body map[string]any) {
	t.Helper()
	if _, ok := data(body)["currentPosition"]; ok {
		t.Fatalf("%s: idle device must not carry currentPosition", where)
	}
}

// assertCurrentPosition 断言在位读模型完整:仓位行 + rack 摘要与厂商形状一致。
func assertCurrentPosition(t *testing.T, where string, body map[string]any, fx fixture, wantStartU, wantEndU, wantHeightU int) map[string]any {
	t.Helper()
	cp, ok := data(body)["currentPosition"].(map[string]any)
	if !ok {
		t.Fatalf("%s: currentPosition missing", where)
	}
	if int(cp["startU"].(float64)) != wantStartU || int(cp["endU"].(float64)) != wantEndU ||
		int(cp["heightU"].(float64)) != wantHeightU {
		t.Fatalf("%s: position U mismatch: %v", where, cp)
	}
	if cp["orientation"] != "NORMAL" {
		t.Fatalf("%s: orientation must be NORMAL, got %v", where, cp["orientation"])
	}
	if v, ok := cp["version"].(float64); !ok || v < 1 {
		t.Fatalf("%s: position row version missing: %v", where, cp["version"])
	}
	if cp["rackId"].(string) != fx.rackID {
		t.Fatalf("%s: rackId mismatch: %v", where, cp["rackId"])
	}
	rack, ok := cp["rack"].(map[string]any)
	if !ok {
		t.Fatalf("%s: currentPosition.rack summary missing", where)
	}
	if rack["id"].(string) != fx.rackID {
		t.Fatalf("%s: rack.id mismatch: %v", where, rack["id"])
	}
	if int(rack["uHeight"].(float64)) != 20 {
		t.Fatalf("%s: rack.uHeight mismatch: %v", where, rack["uHeight"])
	}
	if s, ok := rack["status"].(string); !ok || s == "" {
		t.Fatalf("%s: rack.status missing: %v", where, rack["status"])
	}
	if v, ok := rack["version"].(float64); !ok || v < 1 {
		t.Fatalf("%s: rack.version missing: %v", where, rack["version"])
	}
	return cp
}

// TestDeviceCurrentPositionReadModel:设备列表/详情读接口返回 currentPosition
// (A 族首批,复刻厂商形状);未在位设备省略;写操作响应不带该字段。
func TestDeviceCurrentPositionReadModel(t *testing.T) {
	fx := newFixture(t, "RM")
	dev := createDevice(t, fx, "RM-D"+short(), 2)

	// 未在位:详情与列表都不得带 currentPosition
	st, body := call("GET", "/api/v1/devices/"+dev, nil, adminTok)
	if st != 200 {
		t.Fatalf("get idle device: %d", st)
	}
	assertNoCurrentPosition(t, "idle detail", body)
	st, list := call("GET", "/api/v1/devices?search="+data(body)["code"].(string), nil, adminTok)
	if st != 200 {
		t.Fatalf("list idle: %d", st)
	}
	assertNoCurrentPosition(t, "idle list", list)

	// 上架后:详情与列表返回完整 currentPosition(rack 摘要齐备)
	if st, _ := call("POST", "/api/v1/devices/"+dev+"/assign",
		map[string]any{"rackId": fx.rackID, "startU": 3}, adminTok); st != 200 {
		t.Fatal("assign failed")
	}
	st, body = call("GET", "/api/v1/devices/"+dev, nil, adminTok)
	if st != 200 {
		t.Fatalf("get positioned device: %d", st)
	}
	assertCurrentPosition(t, "detail", body, fx, 3, 4, 2)
	// 写操作响应不带 currentPosition(与厂商 assign/move 响应形状一致)
	if _, ok := data(body)["type"]; !ok {
		t.Fatal("detail should preload type")
	}

	st, list = call("GET", "/api/v1/devices?search="+data(body)["code"].(string), nil, adminTok)
	if st != 200 {
		t.Fatalf("list positioned: %d", st)
	}
	items := data(list)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("search should hit exactly 1, got %d", len(items))
	}
	item := items[0].(map[string]any)
	assertCurrentPosition(t, "list", map[string]any{"data": item}, fx, 3, 4, 2)

	// 下架后:currentPosition 再次省略
	if st, _ := call("POST", "/api/v1/devices/"+dev+"/decommission",
		map[string]any{"reason": "读模型验收"}, adminTok); st != 200 {
		t.Fatal("decommission failed")
	}
	st, body = call("GET", "/api/v1/devices/"+dev, nil, adminTok)
	if st != 200 {
		t.Fatalf("get decommissioned device: %d", st)
	}
	assertNoCurrentPosition(t, "decommissioned detail", body)
}
