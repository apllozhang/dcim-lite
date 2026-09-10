
import base64, json, re
from playwright.sync_api import sync_playwright

B = "http://127.0.0.1:19173"
API = "http://127.0.0.1:19080"
env = open("/home/alec/cabinet-rebuild-deploy/.env").read()
pw = [l for l in env.splitlines() if l.startswith("ADMIN_PASSWORD=")][0].split("=", 1)[1]

def api(m, p, body=None, tok=None):
    import urllib.request, urllib.error
    h = {"Content-Type": "application/json"}
    if tok: h["Authorization"] = "Bearer " + tok
    d = None if body is None else json.dumps(body).encode()
    r = urllib.request.Request(API + p, data=d, headers=h, method=m)
    try:
        with urllib.request.urlopen(r, timeout=20) as x: return x.status, json.loads(x.read().decode())
    except urllib.error.HTTPError as e:
        try: return e.code, json.loads(e.read().decode())
        except Exception: return e.code, {}

res = []
def rec(n, ok, d=""):
    res.append((n, ok)); print(("  PASS " if ok else "  FAIL ") + n + "  " + str(d)[:130])

def free_start(occupied, height):
    """找一段能容纳 height 的空闲 U 位。"""
    used = set()
    for s, e in occupied:
        for u in range(s, e + 1):
            used.add(u)
    u = 1
    while u + height - 1 <= 42:
        if all((u + i) not in used for i in range(height)):
            return u
        u += 1
    return None

def login(pg):
    pg.goto(B + "/login", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(1800)
    pg.fill("input[placeholder='请输入用户名']", "admin")
    pg.fill("input[placeholder='请输入密码']", pw)
    cap = next((i.get_attribute("src") for i in pg.query_selector_all("img")
                if (i.get_attribute("src") or "").startswith("data:image/svg+xml;base64,")), None)
    pg.fill("#cmt-captcha-input", "".join(re.findall(r">(\d)</text>", base64.b64decode(cap.split(",",1)[1]).decode())))
    pg.click("button:has-text('登录')"); pg.wait_for_timeout(3500)

def open_assign(pg, code):
    pg.goto(B + "/devices", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(2600)
    row = next((tr for tr in pg.query_selector_all("tbody tr") if code in (tr.inner_text() or "")), None)
    if not row: return False
    row.query_selector(".cmt-ops-btn").click(); pg.wait_for_timeout(800)
    item = next((b for b in pg.query_selector_all("#cmt-ops-menu .cmt-ops-item") if "上架" in (b.inner_text() or "")), None)
    if not item: return False
    item.click(); pg.wait_for_selector("button:visible:has-text('提交')", timeout=15000); pg.wait_for_timeout(700)
    return True

def pick_rack(pg):
    submit = next(b for b in pg.query_selector_all("button") if b.is_visible() and "提交" in (b.inner_text() or ""))
    modal = submit.evaluate_handle("""el => { let n = el;
        while (n && !(n.querySelector && n.querySelector("input[placeholder='Select']"))) n = n.parentElement;
        return n; }""").as_element()
    inp = modal.query_selector("input[placeholder='Select']")
    wrapper = inp.evaluate_handle("el => el.closest('.el-cascader') || el.parentElement").as_element()
    wrapper.evaluate("""el => ['mousedown','mouseup','click'].forEach(t =>
        el.dispatchEvent(new MouseEvent(t, {bubbles:true, cancelable:true, view:window})))""")
    pg.wait_for_timeout(1300)
    path = []
    for _ in range(5):
        menus = [m for m in pg.query_selector_all(".el-cascader-menu") if m.is_visible()]
        if not menus: break
        nodes = [n for n in menus[-1].query_selector_all(".el-cascader-node") if n.is_visible()]
        if not nodes: break
        path.append((nodes[0].inner_text() or "").strip()[:18]); nodes[0].click(); pg.wait_for_timeout(900)
        if (inp.input_value() or "").strip(): break
    return inp.input_value().strip(), path

with sync_playwright() as p:
    br = p.chromium.launch(args=["--no-sandbox"])
    pg = br.new_context(viewport={"width": 1440, "height": 900}).new_page()
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)[:90]))
    login(pg)
    rec("UI 登录", "/login" not in pg.url)
    tok = pg.evaluate("() => localStorage.getItem('cabinet_access_token')")

    # ---------- 上架（UI 全链路）----------
    st, devs = api("GET", "/api/v1/devices?page=1&pageSize=50&lifecycleStatus=WAITING_RACK", tok=tok)
    t1 = ((devs.get("data") or {}).get("items") or [])[0]
    ok = open_assign(pg, t1["code"])
    rec("上架·弹窗打开", ok, t1["code"])
    if ok:
        val, path = pick_rack(pg)
        rec("上架·级联选定机柜", bool(val), " > ".join(path))
        nums = [n for n in pg.query_selector_all("input[type='number']") if n.is_visible()]
        if nums: nums[0].fill("1")
        next(b for b in pg.query_selector_all("button") if b.is_visible() and "提交" in (b.inner_text() or "")).click()
        pg.wait_for_timeout(3500)
        st, dev = api("GET", f"/api/v1/devices/{t1['id']}", tok=tok)
        st, h = api("GET", f"/api/v1/devices/{t1['id']}/history", tok=tok)
        rec("上架·lifecycle RUNNING", (dev.get("data") or {}).get("lifecycleStatus") == "RUNNING")
        rec("上架·履历写入", len(((h.get("data") or {}).get("items") or [])) > 0)

    # ---------- 移位（UI 无入口 -> ALE 同构载荷 + 空闲 U 位）----------
    st, rl = api("GET", "/api/v1/devices?page=1&pageSize=50&lifecycleStatus=RUNNING", tok=tok)
    rdev = ((rl.get("data") or {}).get("items") or [])[0]
    height = int(rdev.get("heightU") or 1)
    st, tree = api("GET", "/api/v1/resource-tree", tok=tok)
    rack_ids = []
    def walk(nodes):
        for n in nodes or []:
            for r in (n.get("racks") or []): rack_ids.append(r["id"])
            walk(n.get("rooms"))
    walk((tree.get("data") or {}).get("items") or [])
    st, h0 = api("GET", f"/api/v1/devices/{rdev['id']}/history", tok=tok)
    n0 = len(((h0.get("data") or {}).get("items") or []))
    moved = False
    for rid in rack_ids:
        st, lay = api("GET", f"/api/v1/racks/{rid}/u-layout", tok=tok)
        poss = ((lay.get("data") or {}).get("positions") or [])
        occupied = [(x["startU"], x["endU"]) for x in poss if x.get("deviceId") != rdev["id"]]
        start = free_start(occupied, height)
        if start is None: continue
        st, mv = api("POST", f"/api/v1/devices/{rdev['id']}/move",
                     {"rackId": rid, "startU": start, "orientation": "NORMAL",
                      "reason": "ALE 回归：移位（同构载荷）"}, tok=tok)
        if st == 200:
            moved = True
            rec("移位·ALE 同构载荷成功（空闲 U）", True, f"rack={rid[:8]} startU={start}")
            break
    if not moved:
        rec("移位·ALE 同构载荷成功（空闲 U）", False, "所有机柜均无可用 U 位")
    st, h1 = api("GET", f"/api/v1/devices/{rdev['id']}/history", tok=tok)
    rec("移位·履历增加", len(((h1.get("data") or {}).get("items") or [])) > n0,
        f"{n0} -> {len(((h1.get('data') or {}).get('items') or []))}")

    # ---------- 审批（UI 全链路：探路由 -> 点批准）----------
    api("PUT", "/api/v1/admin/approval-policy", {"assignApprovalEnabled": True}, tok)
    st, devs = api("GET", "/api/v1/devices?page=1&pageSize=50&lifecycleStatus=WAITING_RACK", tok=tok)
    t2 = ((devs.get("data") or {}).get("items") or [])[0]
    ok = open_assign(pg, t2["code"])
    if ok:
        val, path = pick_rack(pg)
        nums = [n for n in pg.query_selector_all("input[type='number']") if n.is_visible()]
        if nums: nums[0].fill("6")
        next(b for b in pg.query_selector_all("button") if b.is_visible() and "提交" in (b.inner_text() or "")).click()
        pg.wait_for_timeout(3000)
        st, pend = api("GET", "/api/v1/admin/approvals?status=PENDING", tok=tok)
        mine = [x for x in ((pend.get("data") or {}).get("items") or []) if x.get("deviceId") == t2["id"]]
        rec("审批·UI 上架生成 PENDING", len(mine) > 0, f"{len(mine)} 条")

    # 探后台路由：点侧边栏「系统管理」，列出子菜单
    pg.goto(B + "/", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(2000)
    try:
        pg.click("text=系统管理"); pg.wait_for_timeout(1200)
    except Exception as e:
        print("  点系统管理失败:", str(e)[:60])
    submenu = [e.inner_text().strip() for e in pg.query_selector_all("li, a, div[role=menuitem]")
               if e.is_visible() and e.inner_text().strip() and len(e.inner_text().strip()) < 12]
    print("  侧边栏可见项:", list(dict.fromkeys(submenu))[:20])
    # 找含「审批」的入口并点击
    clicked = False
    for el in pg.query_selector_all("li, a, span, div"):
        try:
            t = (el.inner_text() or "").strip()
            if el.is_visible() and "审批" in t and len(t) < 10:
                el.click(); clicked = True; break
        except Exception:
            continue
    rec("审批·后台入口可点", clicked)
    if clicked:
        pg.wait_for_timeout(2500)
        print("  当前 URL:", pg.url)
        pg.screenshot(path="/home/alec/ale-regress/approvals-ui.png")
        approve = next((b for b in pg.query_selector_all("button")
                        if b.is_visible() and ("批准" in (b.inner_text() or "") or "通过" in (b.inner_text() or ""))), None)
        rec("审批·页面上有批准按钮", approve is not None)
        if approve:
            approve.click(); pg.wait_for_timeout(1500)
            cf = next((b for b in pg.query_selector_all("button")
                       if b.is_visible() and "确定" in (b.inner_text() or "")), None)
            if cf: cf.click()
            pg.wait_for_timeout(2500)
            st, d2 = api("GET", f"/api/v1/devices/{t2['id']}", tok=tok)
            lc = (d2.get("data") or {}).get("lifecycleStatus")
            rec("审批·UI 批准后设备 RUNNING", lc == "RUNNING", lc)
    api("PUT", "/api/v1/admin/approval-policy", {"assignApprovalEnabled": False}, tok)
    # pageerror 回调里的 str(e) 是异常消息本身（如 "TypeError: ..."），不含 "pageerror" 字样，
    # 必须直接判空 errs，否则任何 JS 异常都会被漏报
    rec("无 JS 致命异常", not errs, errs[:1])
    br.close()

fails = [n for n, ok in res if not ok]
print("ALE_V6_" + ("PASS" if not fails else "FAIL " + ",".join(fails)))
# 任一断言失败必须非零退出，否则 CI 会把失败判为成功
raise SystemExit(1 if fails else 0)
