"""ALE 前端回归主套件（pytest + Playwright）。

流程分类（复评 P1-05 要求三类不可互替，这里明确标注）：
- test_login                UI E2E（登录）
- test_assign_full_ui       UI E2E（上架全链路：菜单 → 弹窗 → 级联 → 提交 → API 复核）
- test_move_api_compat      API 兼容验证（ALE 界面无移位入口，同构载荷直打后端）
- test_approval_ui_flow     UI E2E（审批上架 + 后台批准闭环）

所有用例只操作 conftest.env 自建的隔离数据（带 run ID），teardown 无条件清理。
"""
import re
import time

import pytest

from conftest import BASE_URL, api, data_of

WAIT_MODAL = "button:visible:has-text('提交')"


def open_assign_modal(pg, code: str, retries: int = 3) -> bool:
    """设备行「上架」→ 弹窗打开。先按编码搜索（自建设备不在首页），
    弹窗偶发不打开（历史时序问题）则显式等待 + 重试。"""
    for _ in range(retries):
        pg.goto(BASE_URL + "/devices", wait_until="networkidle", timeout=30000)
        pg.wait_for_timeout(2000)
        pg.fill("input[placeholder*='名称']", code)
        pg.click("button:has-text('查询')")
        pg.wait_for_timeout(1500)
        row = next((tr for tr in pg.query_selector_all("tbody tr") if code in (tr.inner_text() or "")), None)
        if row is None:
            continue
        row.query_selector(".cmt-ops-btn").click()
        pg.wait_for_timeout(800)
        item = next((b for b in pg.query_selector_all("#cmt-ops-menu .cmt-ops-item")
                     if "上架" in (b.inner_text() or "")), None)
        if item is None:
            continue
        item.click()
        try:
            pg.wait_for_selector(WAIT_MODAL, timeout=8000)
            pg.wait_for_timeout(500)
            return True
        except Exception:
            continue  # 弹窗未打开 → 重新进入重试
    return False


def pick_own_rack(pg, run_id: str):
    """级联选择器中只选自己创建的「数据中心>机房>机柜」（节点文本含 run_id）。

    交互要点（详见 README）：必须在 .el-cascader wrapper 上派发 mousedown 才能展开；
    多列结构每轮只操作最后一列。
    """
    submit = next(b for b in pg.query_selector_all("button")
                  if b.is_visible() and "提交" in (b.inner_text() or ""))
    modal = submit.evaluate_handle(
        """el => { let n = el;
            while (n && !(n.querySelector && n.querySelector("input[placeholder='Select']"))) n = n.parentElement;
            return n; }""").as_element()
    inp = modal.query_selector("input[placeholder='Select']")
    wrapper = inp.evaluate_handle("el => el.closest('.el-cascader') || el.parentElement").as_element()
    wrapper.evaluate("""el => ['mousedown','mouseup','click'].forEach(t =>
        el.dispatchEvent(new MouseEvent(t, {bubbles:true, cancelable:true, view:window})))""")
    pg.wait_for_timeout(1200)
    path = []
    for _ in range(5):
        menus = [m for m in pg.query_selector_all(".el-cascader-menu") if m.is_visible()]
        if not menus:
            break
        nodes = [n for n in menus[-1].query_selector_all(".el-cascader-node") if n.is_visible()]
        target = next((n for n in nodes if run_id in (n.inner_text() or "")), None)
        if target is None:
            break  # 本列没有自己的节点：不该发生（fixture 数据就在那），继续会选到别人的
        path.append((target.inner_text() or "").strip()[:20])
        target.click()
        pg.wait_for_timeout(900)
        if (inp.input_value() or "").strip():
            break
    return inp.input_value().strip(), path


def submit_assign(pg, start_u: str):
    nums = [n for n in pg.query_selector_all("input[type='number']") if n.is_visible()]
    if nums:
        nums[0].fill(start_u)
    next(b for b in pg.query_selector_all("button")
         if b.is_visible() and "提交" in (b.inner_text() or "")).click()
    pg.wait_for_timeout(3000)


def test_login(page):
    assert "/login" not in page.url
    assert not page.errors, f"JS 异常: {page.errors}"


def test_assign_full_ui(page, env):
    dev = env["make_device"]()
    assert open_assign_modal(page, dev["code"]), "上架弹窗未打开（重试后仍失败）"
    val, path = pick_own_rack(page, env["run_id"])
    assert val, f"级联未选定机柜，path={path}"
    assert env["run_id"] in val, f"级联选中的不是自建机柜: {val}"
    submit_assign(page, "1")

    st, d = api("GET", f"/api/v1/devices/{dev['id']}", None, env["tok"])
    assert st == 200
    assert data_of(d).get("lifecycleStatus") == "RUNNING", f"上架未生效: {data_of(d).get('lifecycleStatus')}"
    st, h = api("GET", f"/api/v1/devices/{dev['id']}/history", None, env["tok"])
    assert len(data_of(h).get("items", [])) > 0, "履历未写入"
    assert not page.errors, f"JS 异常: {page.errors}"


def test_move_api_compat(page, env):
    """移位：UI 无入口（已实测确认），此用例是 API 兼容验证，非 UI E2E。"""
    dev = env["make_device"]()
    st, _ = api("POST", f"/api/v1/devices/{dev['id']}/assign",
                {"rackId": env["rack"], "startU": 2}, env["tok"])
    assert st == 200, "预置上架失败"

    st, h0 = api("GET", f"/api/v1/devices/{dev['id']}/history", None, env["tok"])
    n0 = len(data_of(h0).get("items", []))
    st, mv = api("POST", f"/api/v1/devices/{dev['id']}/move",
                 {"rackId": env["rack"], "startU": 5, "orientation": "NORMAL",
                  "reason": "E2E：移位（同构载荷）"}, env["tok"])
    assert st == 200, f"移位失败: {st} {mv}"
    st, h1 = api("GET", f"/api/v1/devices/{dev['id']}/history", None, env["tok"])
    assert len(data_of(h1).get("items", [])) == n0 + 1, "移位履历未增加"


def test_approval_ui_flow(page, env):
    """审批上架 → 后台 /admin 页批准 → 设备 RUNNING（审批批准闭环首次走通）。"""
    tok = env["tok"]
    dev = env["make_device"]()
    st, _ = api("PUT", "/api/v1/admin/approval-policy", {"assignApprovalEnabled": True}, tok)
    assert st == 200, "开启审批策略失败"

    assert open_assign_modal(page, dev["code"]), "上架弹窗未打开（重试后仍失败）"
    val, _ = pick_own_rack(page, env["run_id"])
    assert val
    submit_assign(page, "6")

    st, pend = api("GET", "/api/v1/admin/approvals?status=PENDING", None, tok)
    mine = [x for x in data_of(pend).get("items", []) if x.get("deviceId") == dev["id"]]
    assert mine, "UI 上架未生成 PENDING 审批单"

    # 后台审批页：侧边栏「系统管理」→ 含「审批」的菜单项
    page.goto(BASE_URL + "/", wait_until="networkidle", timeout=30000)
    page.wait_for_timeout(2000)
    page.click("text=系统管理")
    page.wait_for_timeout(1200)
    clicked = False
    for el in page.query_selector_all("li, a, span, div"):
        try:
            t = (el.inner_text() or "").strip()
            if el.is_visible() and "审批" in t and len(t) < 10:
                el.click()
                clicked = True
                break
        except Exception:
            continue
    assert clicked, "未找到后台审批入口"
    page.wait_for_timeout(2500)

    # 定位自己的审批行：设备列有时显示名称、有时显示 UUID（编译产物行为不一），
    # 用 run_id 匹配最稳（自建设备名称/编码都含它，存量单不含）；
    # UUID 在单元格内折行会带入换行符，统一去空白后匹配
    rows = [tr for tr in page.query_selector_all("tbody tr")
            if env["run_id"] in re.sub(r"\s+", "", tr.inner_text() or "")]
    assert rows, f"待批列表中未找到自己的审批单（设备 {dev['id']}）"
    approve = next((b for b in rows[0].query_selector_all("button")
                    if b.is_visible() and ("通过" in (b.inner_text() or "") or "批准" in (b.inner_text() or ""))), None)
    assert approve is not None, "审批行无通过按钮"
    approve.click()
    page.wait_for_timeout(1200)
    # 确认弹窗按钮是英文 OK（编译产物文案），兼容「确定」
    confirm = next((b for b in page.query_selector_all("button")
                    if b.is_visible() and (b.inner_text() or "").strip() in ("OK", "确定", "确 定")), None)
    assert confirm is not None, "批准确认弹窗未出现"
    confirm.click()
    page.wait_for_timeout(3000)

    st, d = api("GET", f"/api/v1/devices/{dev['id']}", None, tok)
    assert data_of(d).get("lifecycleStatus") == "RUNNING", \
        f"批准后设备未 RUNNING: {data_of(d).get('lifecycleStatus')}"
    st, pend = api("GET", "/api/v1/admin/approvals?status=PENDING", None, tok)
    left = [x for x in data_of(pend).get("items", []) if x.get("deviceId") == dev["id"]]
    assert not left, "审批单仍为 PENDING"
    assert not page.errors, f"JS 异常: {page.errors}"
