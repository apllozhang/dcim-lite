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
    pg.fill("input[placeholder='请输入用户名']", E2E_USERNAME)
    pg.fill("input[placeholder='请输入密码']", E2E_PASSWORD)
    cap = next((i.get_attribute("src") for i in pg.query_selector_all("img")
                if (i.get_attribute("src") or "").startswith("data:image/svg+xml;base64,")), None)
    code = "".join(re.findall(r">(\d)</text>", base64.b64decode(cap.split(",", 1)[1]).decode()))
    pg.fill("#cmt-captcha-input", code)
    pg.click("button:has-text('登录')")
    pg.wait_for_function("() => !location.pathname.includes('login')", timeout=15000)
    pg.errors = errs  # 供用例断言无 JS 异常
    yield pg
    if errs:
        print(f"\n[pageerror] {errs}")
    pg.close()


@pytest.fixture
def env(admin_token):
    """自建隔离环境：唯一 run ID 的数据中心/机房/机柜 + 设备工厂。

    teardown 无条件清理（try/finally 语义），清理失败会打印警告但不掩盖用例结果。
    """
    run_id = uuid.uuid4().hex[:8]
    ctx = {
        "run_id": run_id, "tok": admin_token,
        "dc": None, "room": None, "rack": None, "devices": [],
        "policy_backup": None,
    }

    def _create(path, body, what):
        st, resp = api("POST", path, body, admin_token)
        assert st in (200, 201), f"fixture create {what}: {st} {resp}"
        return data_of(resp)

    dc = _create("/api/v1/data-centers", {"code": f"E2E-DC-{run_id}", "name": f"E2E数据中心{run_id}"}, "dc")
    ctx["dc"] = dc["id"]
    room = _create(f"/api/v1/data-centers/{dc['id']}/rooms", {"code": f"R-{run_id}", "name": f"E2E机房{run_id}"}, "room")
    ctx["room"] = room["id"]
    rack = _create(f"/api/v1/rooms/{room['id']}/racks",
                   {"code": f"K-{run_id}", "name": f"E2E机柜{run_id}", "uHeight": 12}, "rack")
    ctx["rack"] = rack["id"]

    st, types = api("GET", "/api/v1/device-types", None, admin_token)
    assert st == 200, "list device types failed"
    ctx["type_id"] = next(m["id"] for m in data_of(types)["items"] if m["code"] == "SERVER")

    def make_device(name_suffix=""):
        dev = _create("/api/v1/devices",
                      {"typeId": ctx["type_id"], "code": f"E2E-D{run_id}{name_suffix}",
                       "name": f"E2E设备{run_id}{name_suffix}", "heightU": 1}, "device")
        ctx["devices"].append(dev["id"])
        return dev

    ctx["make_device"] = make_device

    # 审批策略原值留档（用例内可开/关，teardown 恢复）
    st, pol = api("GET", "/api/v1/admin/approval-policy", None, admin_token)
    if st == 200:
        ctx["policy_backup"] = data_of(pol)

    yield ctx

    # ---- teardown：恢复策略 + 逆序清理 ----
    if ctx["policy_backup"] is not None:
        st, _ = api("PUT", "/api/v1/admin/approval-policy", ctx["policy_backup"], admin_token)
        if st != 200:
            print(f"\n[WARN] approval-policy restore failed: {st} (原值={ctx['policy_backup']})")
    for dev_id in list(ctx["devices"]):
        st, d = api("GET", f"/api/v1/devices/{dev_id}", None, admin_token)
        if st != 200:
            continue
        if data_of(d).get("lifecycleStatus") == "RUNNING":
            api("POST", f"/api/v1/devices/{dev_id}/decommission", {"reason": "E2E 清理"}, admin_token)
        st, d = api("GET", f"/api/v1/devices/{dev_id}", None, admin_token)
        ver = data_of(d).get("version")
        if ver is not None:
            s2, _ = api("DELETE", f"/api/v1/devices/{dev_id}?version={ver:.0f}", None, admin_token)
            if s2 != 200:
                print(f"\n[WARN] cleanup device {dev_id} failed: {s2}")
    st, r = api("GET", f"/api/v1/racks/{ctx['rack']}", None, admin_token)
    if st == 200:
        ver = data_of(r).get("version")
        st, resp = api("DELETE", f"/api/v1/racks/{ctx['rack']}?version={ver:.0f}", None, admin_token)
        if st != 200:
            print(f"\n[WARN] cleanup rack failed: {st} {resp}")
    st, r = api("GET", f"/api/v1/rooms/{ctx['room']}", None, admin_token)
    if st == 200:
        ver = data_of(r).get("version")
        api("DELETE", f"/api/v1/rooms/{ctx['room']}?version={ver:.0f}", None, admin_token)
    st, r = api("GET", f"/api/v1/data-centers/{ctx['dc']}", None, admin_token)
    if st == 200:
        ver = data_of(r).get("version")
        api("DELETE", f"/api/v1/data-centers/{ctx['dc']}?version={ver:.0f}", None, admin_token)


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
