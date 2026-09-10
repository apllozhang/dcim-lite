
import base64, json, re
from playwright.sync_api import sync_playwright
B = "http://127.0.0.1:19173"
env = open("/home/alec/cabinet-rebuild-deploy/.env").read()
pw = [l for l in env.splitlines() if l.startswith("ADMIN_PASSWORD=")][0].split("=", 1)[1]
with sync_playwright() as p:
    br = p.chromium.launch(args=["--no-sandbox"])
    pg = br.new_context(viewport={"width": 1440, "height": 900}).new_page()
    pg.goto(B + "/login", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(1800)
    pg.fill("input[placeholder='请输入用户名']", "admin")
    pg.fill("input[placeholder='请输入密码']", pw)
    cap = None
    for img in pg.query_selector_all("img"):
        s = img.get_attribute("src") or ""
        if s.startswith("data:image/svg+xml;base64,"): cap = s; break
    pg.fill("#cmt-captcha-input", "".join(re.findall(r">(\d)</text>", base64.b64decode(cap.split(",",1)[1]).decode())))
    pg.click("button:has-text('登录')"); pg.wait_for_timeout(3500)
    pg.goto(B + "/devices", wait_until="networkidle", timeout=30000); pg.wait_for_timeout(2800)
    row = pg.query_selector("tbody tr")
    row.query_selector(".cmt-ops-btn").click(); pg.wait_for_timeout(700)
    assign = next(b for b in pg.query_selector_all("#cmt-ops-menu .cmt-ops-item") if "上架" in (b.inner_text() or ""))
    assign.click(); pg.wait_for_timeout(3000)

    info = pg.evaluate(r"""() => {
      const vis = el => { const r = el.getBoundingClientRect(); return r.width > 1 && r.height > 1; };
      const out = {};
      // 1) 找到含「设备上架」文本的元素，回溯祖先链（带尺寸与定位）
      let node = Array.from(document.querySelectorAll('*')).find(e =>
          (e.children.length === 0) && (e.textContent || '').trim() === '设备上架');
      out.titleFound = !!node;
      out.chain = [];
      let cur = node;
      for (let i = 0; cur && i < 8; i++) {
        const r = cur.getBoundingClientRect();
        const cs = getComputedStyle(cur);
        out.chain.push({
          tag: cur.tagName, cls: (cur.className||'').toString().slice(0,70),
          rect: [Math.round(r.width), Math.round(r.height), Math.round(r.x), Math.round(r.y)],
          display: cs.display, visibility: cs.visibility, position: cs.position, zIndex: cs.zIndex,
        });
        cur = cur.parentElement;
      }
      // 2) 该容器内的输入控件
      const container = node ? node.closest('div[class*="drawer"], div[class*="dialog"], div[class*="modal"], div[class*="overlay"]') : null;
      out.containerClass = container ? (container.className||'').toString().slice(0,90) : null;
      if (container) {
        out.controls = Array.from(container.querySelectorAll('input,select,textarea')).map(e => {
          const r = e.getBoundingClientRect();
          return { tag: e.tagName, cls: (e.className||'').toString().slice(0,50), ph: e.placeholder,
                   type: e.type, val: e.value, wh: [Math.round(r.width), Math.round(r.height)], vis: vis(e) };
        });
        out.btns = Array.from(container.querySelectorAll('button')).map(e => (e.innerText||'').trim());
        out.selectLike = Array.from(container.querySelectorAll('*'))
          .filter(e => /select|picker/i.test((e.className||'').toString()) && e.children.length < 3)
          .map(e => e.tagName + '.' + (e.className||'').toString().replace(/\s+/g,'.')).slice(0, 10);
      }
      return out;
    }""")
    print(json.dumps(info, ensure_ascii=False, indent=1)[:3000])
    br.close()
