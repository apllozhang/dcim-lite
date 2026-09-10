
import json, base64, re
from playwright.sync_api import sync_playwright
B = "http://127.0.0.1:19173"
env = open("/home/alec/cabinet-rebuild-deploy/.env").read()
pw = [l for l in env.splitlines() if l.startswith("ADMIN_PASSWORD=")][0].split("=",1)[1]
with sync_playwright() as p:
    b = p.chromium.launch(args=["--no-sandbox"])
    pg = b.new_context(viewport={"width":1440,"height":900}).new_page()
    pg.goto(B + "/login", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(2000)
    pg.fill("input[placeholder='请输入用户名']", "admin")
    pg.fill("input[placeholder='请输入密码']", pw)
    cap = pg.eval_on_selector("img#cmt-captcha-img", "e => e.src") if pg.query_selector("img#cmt-captcha-img") else None
    if not cap:
        for img in pg.query_selector_all("img"):
            s = img.get_attribute("src") or ""
            if s.startswith("data:image/svg+xml;base64,"): cap = s; break
    svg = base64.b64decode(cap.split(",",1)[1]).decode()
    pg.fill("#cmt-captcha-input", "".join(re.findall(r">(\d)</text>", svg)))
    pg.click("button:has-text('登录')"); pg.wait_for_timeout(3500)
    print("login url:", pg.url)
    pg.goto(B + "/devices", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(3000)
    rows = pg.query_selector_all("tbody tr")
    print("device rows:", len(rows))
    # 第一行的操作入口（增强脚本折叠为「更多操作」三点按钮）
    btn = pg.query_selector(".cmt-ops-btn")
    print("ops button present:", btn is not None)
    if btn:
        btn.click(); pg.wait_for_timeout(800)
        menu = pg.query_selector("#cmt-ops-menu")
        items = [b.inner_text().strip() for b in pg.query_selector_all("#cmt-ops-menu .cmt-ops-item")] if menu else []
        print("ops menu items:", items)
    pg.screenshot(path="/home/alec/ale-regress/probe-device-ops.png")
    b.close()
