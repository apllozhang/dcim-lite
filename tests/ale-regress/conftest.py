"""ALE 前端回归（pytest + Playwright）公共设施。

与旧的一次性脚本（已移除的 ale_regress_flows.py）相比，本套件满足复评 P1-02/03/04/06：
- 环境变量化：地址与口令全部来自环境变量，缺省直接失败，杜绝硬编码；
- 变异门控：必须显式 E2E_ALLOW_MUTATION=true（这些用例会写业务数据），
  拒绝对未明确授权的环境执行；
- 数据隔离：每个用例自建带唯一 run ID 的数据中心/机房/机柜/设备，
  只操作自己创建的对象，结束按 ID 清理；
- 策略恢复：审批策略先读原值，finally 无条件恢复；
- 非零退出：任一用例失败 pytest 即返回非零（可直接做 CI 门禁）。

运行示例（部署主机）：
  export ALE_BASE_URL=http://127.0.0.1:19173 ALE_API_URL=http://127.0.0.1:19080
  export E2E_USERNAME=admin E2E_PASSWORD=... E2E_ALLOW_MUTATION=true
  ~/ale-venv/bin/python -m pytest test_ale_flows.py -v
"""
import base64
import json
import os
import re
import urllib.error
import urllib.request
import uuid

import pytest

ARTIFACT_DIR = os.environ.get("E2E_ARTIFACT_DIR", "ale-regress-artifacts")


def _require_env(name: str) -> str:
    v = os.environ.get(name, "").strip()
    if not v:
        pytest.exit(f"[ale-regress] missing required env: {name}", returncode=2)
    return v


BASE_URL = _require_env("ALE_BASE_URL")
API_URL = _require_env("ALE_API_URL")
E2E_USERNAME = os.environ.get("E2E_USERNAME", "admin")
E2E_PASSWORD = _require_env("E2E_PASSWORD")

# 变异门控：这些用例会真实写业务数据，必须显式授权
if os.environ.get("E2E_ALLOW_MUTATION", "").strip().lower() != "true":
    pytest.exit("[ale-regress] E2E_ALLOW_MUTATION=true is required (these tests mutate business data)",
                returncode=2)


def api(method: str, path: str, body=None, token: str = ""):
    """直打后端 API，返回 (status, dict)。"""
    h = {"Content-Type": "application/json"}
    if token:
        h["Authorization"] = "Bearer " + token
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(API_URL + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode())
        except Exception:
            return e.code, {}


def data_of(body: dict) -> dict:
    return body.get("data") if isinstance(body, dict) and isinstance(body.get("data"), dict) else {}


@pytest.fixture(scope="session")
def admin_token():
    """API 登录（含验证码解析）。"""
    st, cap = api("GET", "/api/v1/auth/captcha")
    assert st == 200, f"captcha failed: {st}"
    img = data_of(cap).get("image", "")
    if "," in img:
        img = img.split(",", 1)[1]
    svg = base64.b64decode(img).decode()
    code = "".join(re.findall(r">(\d)</text>", svg))
    st, resp = api("POST", "/api/v1/auth/login",
                   {"username": E2E_USERNAME, "password": E2E_PASSWORD,
                    "captchaId": data_of(cap).get("id", ""), "captcha": code})
    assert st == 200, f"login failed: {st} {resp}"
    tok = data_of(resp).get("token", "")
    assert tok, f"login response missing token: {resp}"
    return tok


@pytest.fixture(scope="session")
def browser():
    from playwright.sync_api import sync_playwright
    with sync_playwright() as p:
        br = p.chromium.launch(args=["--no-sandbox"])
        yield br
        br.close()


@pytest.fixture
def page(browser):
    """每个用例独立页面（登录后的 ALE 界面）。"""
    pg = browser.new_context(viewport={"width": 1440, "height": 900}).new_page()
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)[:200]))
    pg.goto(BASE_URL + "/login", wait_until="networkidle", timeout=30000)
    pg.wait_for_timeout(1800)

    def _read_captcha():
        return next((i.get_attribute("src") for i in pg.query_selector_all("img")
                     if (i.get_attribute("src") or "").startswith("data:image/svg+xml;base64,")), None)

    cap = _read_captcha()
    if cap is None:
        # 间歇性渲染时序：验证码 svg 未就绪，刷新重取一次
        pg.reload(wait_until="networkidle", timeout=30000)
        pg.wait_for_timeout(1500)
        cap = _read_captcha()
    assert cap, "登录页验证码图片未渲染（重试后仍失败）"
    code = "".join(re.findall(r">(\d)</text>", base64.b64decode(cap.split(",", 1)[1]).decode()))
    pg.fill("input[placeholder='请输入用户名']", E2E_USERNAME)
    pg.fill("input[placeholder='请输入密码']", E2E_PASSWORD)
    pg.fill("#cmt-captcha-input", code)
    pg.click("button:has-text('登录')")
    pg.wait_for_function("() => !location.pathname.includes('login')", timeout=15000)
    pg.errors = errs  # 供用例断言无 JS 异常
    yield pg
    if errs:
        print(f"\n[pageerror] {errs}")
    pg.close()


@pytest.fixture
def env(request, admin_token):
    """自建隔离环境：唯一 run ID 的数据中心/机房/机柜 + 设备工厂。

    清理协议（复评 P1-N1 修复，T01）：
    - 创建即保存 id+version，teardown 直接逆序 DELETE——不再调用不存在的
      GET /racks|rooms|data-centers/{id}（旧清理因此 404 跳过删除，从未生效）；
    - request.addfinalizer 注册：fixture 在 yield 前失败也清理已创建资源；
    - 乐观锁 409 时经 resource-tree / 设备详情重取最新 version 重试一次；
    - 清理失败让 teardown 报错（不再 WARN 掩盖），末尾按 run ID 做零残留断言。
    """
    run_id = uuid.uuid4().hex[:8]
    ctx = {
        "run_id": run_id, "tok": admin_token,
        "dc": None, "room": None, "rack": None, "devices": [],
        "policy_backup": None,
        "_dc_row": None, "_room_row": None, "_rack_row": None, "_device_rows": [],
    }

    def _create(path, body, what):
        st, resp = api("POST", path, body, admin_token)
        assert st in (200, 201), f"fixture create {what}: {st} {resp}"
        d = data_of(resp)
        assert d.get("id") and d.get("version") is not None, \
            f"fixture create {what}: response missing id/version: {d}"
        return d

    def _version_from_tree(obj_id):
        """resource-tree 全树定位 dc/room/rack 的最新 version；不在树中返回 None。"""
        st, tree = api("GET", "/api/v1/resource-tree", None, admin_token)
        if st != 200:
            return None

        def walk(node):
            if isinstance(node, dict):
                if node.get("id") == obj_id:
                    return node.get("version")
                for v in node.values():
                    r = walk(v)
                    if r is not None:
                        return r
            elif isinstance(node, list):
                for it in node:
                    r = walk(it)
                    if r is not None:
                        return r
            return None

        return walk(data_of(tree))

    def _purge(kind, row):
        """DELETE {kind}/{id}?version=；409 重取 version 重试一次。返回错误消息或 None。"""
        obj_id, ver = row["id"], row["version"]
        for _ in range(2):
            st, body = api("DELETE", f"/api/v1/{kind}/{obj_id}?version={ver:.0f}", None, admin_token)
            if st in (200, 404):
                return None
            if st == 409:
                latest = _version_from_tree(obj_id)
                if latest is None:
                    return f"{kind} {obj_id}: 409 but not found in resource-tree"
                ver = latest
                continue
            return f"{kind} {obj_id}: cleanup DELETE got {st} {body}"
        return f"{kind} {obj_id}: cleanup failed after version retry"

    def _purge_device(row):
        obj_id = row["id"]
        st, d = api("GET", f"/api/v1/devices/{obj_id}", None, admin_token)
        if st == 200 and data_of(d).get("lifecycleStatus") == "RUNNING":
            st2, b2 = api("POST", f"/api/v1/devices/{obj_id}/decommission",
                          {"reason": "E2E 清理"}, admin_token)
            if st2 != 200:
                return f"device {obj_id}: decommission got {st2} {b2}"
        ver = row["version"]
        for _ in range(2):
            st, body = api("DELETE", f"/api/v1/devices/{obj_id}?version={ver:.0f}", None, admin_token)
            if st in (200, 404):
                return None
            if st == 409:
                st, d = api("GET", f"/api/v1/devices/{obj_id}", None, admin_token)
                if st == 404:
                    return None
                if st != 200:
                    return f"device {obj_id}: refetch got {st}"
                latest = data_of(d).get("version")
                if latest is None:
                    return f"device {obj_id}: refetch missing version"
                ver = latest
                continue
            return f"device {obj_id}: cleanup DELETE got {st} {body}"
        return f"device {obj_id}: cleanup failed after version retry"

    def _finalizer():
        problems = []
        if ctx["policy_backup"] is not None:
            st, _ = api("PUT", "/api/v1/admin/approval-policy", ctx["policy_backup"], admin_token)
            if st != 200:
                problems.append(f"approval-policy restore failed: {st}")
        for row in reversed(ctx["_device_rows"]):
            err = _purge_device(row)
            if err:
                problems.append(err)
        for kind, key in (("racks", "_rack_row"), ("rooms", "_room_row"), ("data-centers", "_dc_row")):
            row = ctx[key]
            if row:
                err = _purge(kind, row)
                if err:
                    problems.append(err)
        # 零残留断言：树中无 dc、设备列表无本 run 前缀
        if ctx["dc"] and _version_from_tree(ctx["dc"]) is not None:
            problems.append(f"residue: dc {ctx['dc']} still in resource-tree")
        st, devs = api("GET", "/api/v1/devices?page=1&pageSize=500", None, admin_token)
        if st == 200:
            prefix = f"E2E-D{run_id}"
            hit = [m.get("code") for m in (data_of(devs).get("items") or [])
                   if str(m.get("code", "")).startswith(prefix)]
            if hit:
                problems.append(f"residue devices with prefix {prefix}: {hit}")
        if problems:
            raise AssertionError("[env teardown] 清理不彻底：\n" + "\n".join(problems))

    request.addfinalizer(_finalizer)

    dc = _create("/api/v1/data-centers", {"code": f"E2E-DC-{run_id}", "name": f"E2E数据中心{run_id}"}, "dc")
    ctx["dc"] = dc["id"]
    ctx["_dc_row"] = dc
    room = _create(f"/api/v1/data-centers/{dc['id']}/rooms", {"code": f"R-{run_id}", "name": f"E2E机房{run_id}"}, "room")
    ctx["room"] = room["id"]
    ctx["_room_row"] = room
    rack = _create(f"/api/v1/rooms/{room['id']}/racks",
                   {"code": f"K-{run_id}", "name": f"E2E机柜{run_id}", "uHeight": 12}, "rack")
    ctx["rack"] = rack["id"]
    ctx["_rack_row"] = rack

    st, types = api("GET", "/api/v1/device-types", None, admin_token)
    assert st == 200, "list device types failed"
    ctx["type_id"] = next(m["id"] for m in data_of(types)["items"] if m["code"] == "SERVER")

    def make_device(name_suffix=""):
        dev = _create("/api/v1/devices",
                      {"typeId": ctx["type_id"], "code": f"E2E-D{run_id}{name_suffix}",
                       "name": f"E2E设备{run_id}{name_suffix}", "heightU": 1}, "device")
        ctx["devices"].append(dev["id"])
        ctx["_device_rows"].append(dev)
        return dev

    ctx["make_device"] = make_device

    # 审批策略原值留档（用例内可开/关，teardown 恢复）
    st, pol = api("GET", "/api/v1/admin/approval-policy", None, admin_token)
    if st == 200:
        ctx["policy_backup"] = data_of(pol)

    return ctx


@pytest.hookimpl(hookwrapper=True)
def pytest_runtest_makereport(item, call):
    """失败留痕：截图 + 页面 URL + JS 异常，写入 E2E_ARTIFACT_DIR。"""
    outcome = yield
    rep = outcome.get_result()
    if rep.when == "call" and rep.failed and "page" in item.funcargs:
        pg = item.funcargs["page"]
        os.makedirs(ARTIFACT_DIR, exist_ok=True)
        path = os.path.join(ARTIFACT_DIR, f"failure-{item.name}.png")
        try:
            pg.screenshot(path=path)
            print(f"\n[artifact] screenshot: {path} url={pg.url}")
        except Exception as e:
            print(f"\n[artifact] screenshot failed: {e}")
