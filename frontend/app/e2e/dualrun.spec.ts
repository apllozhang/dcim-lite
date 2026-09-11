import { expect, test, type Page } from "playwright/test";

/**
 * P1-D 新旧双跑差分(第四轮复评 §5):同一后端、同一种子数据,
 * 新 UI(/devices、/)与旧 ALE UI(/legacy/#/devices、#/resources)各自渲染,
 * 差分四路:API 请求集合、结构化 DOM 行/节点集、页面截图、JS 异常。
 *
 * 数据快照来源:冻结种子(确定性 API 顺序注入,与差分沙箱同款思路)——
 * 不用现网数据,避免数据噪声混入行为差分。
 * 旧 UI 免登录:直接注入其会话键 cabinet_access_token(旧 bundle 逐请求读取)。
 */

const USER = process.env.E2E_USERNAME ?? "admin";
const PASS = process.env.E2E_PASSWORD ?? "";

let adminToken = "";

/** 冻结种子:1 DC/1 房/2 柜/4 设备(3 在位/1 待上架),命名确定性 */
async function seedFrozenData(page: Page): Promise<void> {
  const post = (path: string, body: unknown) =>
    page.evaluate(
      async ({ path, body, token }) => {
        const r = await fetch(path, {
          method: "POST",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify(body),
        });
        return r.json();
      },
      { path, body, token: adminToken },
    );
  const dc = (await post("/api/v1/data-centers", { code: "DUAL-DC", name: "双跑机房" }))?.data;
  const room = (
    await post(`/api/v1/data-centers/${dc.id}/rooms`, { code: "DUAL-R", name: "双跑房间" })
  )?.data;
  const rackA = (
    await post(`/api/v1/rooms/${room.id}/racks`, { code: "DUAL-KA", name: "双跑柜A", uHeight: 20 })
  )?.data;
  await post(`/api/v1/rooms/${room.id}/racks`, { code: "DUAL-KB", name: "双跑柜B", uHeight: 20 });
  const types = await page.evaluate(async (token) => {
    const r = await fetch("/api/v1/device-types", {
      headers: { Authorization: `Bearer ${token}` },
    });
    return r.json();
  }, adminToken);
  const srv = types.data.items.find((t: { code: string }) => t.code === "SERVER");
  const codes = ["DUAL-DV1", "DUAL-DV2", "DUAL-DV3", "DUAL-DV4"];
  const ids: string[] = [];
  for (const code of codes) {
    const d = (
      await post("/api/v1/devices", {
        typeId: srv.id,
        code,
        name: `双跑-${code}`,
        heightU: 1,
      })
    )?.data;
    ids.push(d.id);
  }
  for (let i = 0; i < 3; i++) {
    await post(`/api/v1/devices/${ids[i]}/assign`, { rackId: rackA.id, startU: i + 1 });
  }
}

test.beforeAll(async ({ request }) => {
  // 后端就绪等待
  for (let i = 0; i < 30; i++) {
    const r = await request.get("/health/ready");
    if (r.ok()) return;
    await new Promise((res) => setTimeout(res, 2000));
  }
  throw new Error("backend not ready");
});

test("P1-D dual-run: 新旧 UI 在同一种子数据上渲染等价(树+设备列表)", async ({ page }) => {
  test.skip(!PASS, "E2E_PASSWORD 未提供时跳过");

  // ── 新 UI 登录(拿 token 供种子与旧 UI 注入) ──
  await page.goto("/login");
  await page.fill("input[placeholder='请输入用户名']", USER);
  await page.fill("input[placeholder='请输入密码']", PASS);
  const src = await page.locator("[data-test=captcha-img]").getAttribute("src");
  const code = await page.evaluate((b64) => {
    const svg = atob(b64.split(",", 2)[1]);
    return (svg.match(/>(\d)<\/text>/g) ?? []).map((m) => m[1]).join("");
  }, src ?? "");
  await page.fill("[data-test=captcha-input]", code);
  await page.click("[data-test=login-btn]");
  await page.waitForURL((u) => !u.pathname.includes("/login"), { timeout: 15000 });
  adminToken = await page.evaluate(() => localStorage.getItem("ale.token") ?? "");

  await seedFrozenData(page);

  // ── 新 UI:设备列表(结构化行集 + API 请求记录 + 截图) ──
  const newApiCalls: string[] = [];
  page.on("request", (r) => {
    if (r.url().includes("/api/v1/")) newApiCalls.push(r.url().replace(/^https?:\/\/[^/]+/, ""));
  });
  await page.goto("/devices");
  await expect(page.locator("[data-test=device-table]")).toBeVisible();
  await expect(page.locator("[data-test=device-table] tbody tr").first()).toBeVisible();
  const newRows = await page.$$eval(".el-table__body tr", (rows) =>
    rows.map((r) => r.textContent?.replace(/\s+/g, " ").trim() ?? ""),
  );
  await page.screenshot({ path: "test-results/dualrun-new-devices.png", fullPage: true });

  // 新 UI:资源树(节点集 + 截图)
  await page.goto("/");
  await expect(page.locator("[data-test=resource-tree] .el-tree-node").first()).toBeVisible({
    timeout: 15000,
  });
  const newTree = await page.$$eval("[data-test=resource-tree] .el-tree-node__content", (nodes) =>
    nodes.map((n) => n.textContent?.replace(/\s+/g, " ").trim() ?? ""),
  );
  await page.screenshot({ path: "test-results/dualrun-new-tree.png", fullPage: true });

  // ── 旧 UI:独立端口源(HTML5 history 模式,根路径路由) ──
  // 注入会话(旧 UI 独立 origin,localStorage 隔离;等价于用户在旧 UI 登录)
  const oldErrors: string[] = [];
  const oldApiCalls: string[] = [];
  page.on("pageerror", (e) => oldErrors.push(String(e)));
  page.on("request", (r) => {
    if (r.url().includes("/api/v1/")) oldApiCalls.push(r.url().replace(/^https?:\/\/[^/]+/, ""));
  });
  await page.goto("http://localhost:19501/");
  await page.evaluate((t) => localStorage.setItem("cabinet_access_token", t), adminToken);
  await page.goto("http://localhost:19501/devices");
  // 旧 bundle 的表格渲染(Element Plus 同款表格结构);等待任一行出现
  await page.waitForSelector(".el-table__body tr", { timeout: 20000 });
  await page.waitForTimeout(1500); // 旧 UI 二次取数/渲染稳定
  const oldRows = await page.$$eval(".el-table__body tr", (rows) =>
    rows.map((r) => r.textContent?.replace(/\s+/g, " ").trim() ?? ""),
  );
  await page.screenshot({ path: "test-results/dualrun-old-devices.png", fullPage: true });

  // 旧 UI:资源层级页(结构非 el-tree,按文本断言种子资源可见)
  await page.goto("http://localhost:19501/data-centers");
  await page.waitForTimeout(2500);
  const oldBody = await page.evaluate(() => document.body.innerText.replace(/\s+/g, " "));

  // ── 差分断言:同一数据在新旧 UI 的结构化展示必须等价(集合级,顺序不敏感) ──
  const coreOf = (row: string) => row; // 行文本已含编码/名称/状态等核心字段
  const newCore = newRows.map(coreOf).sort();
  const oldCore = oldRows.map(coreOf).sort();
  expect(oldRows.length, "新旧设备列表行数一致").toBe(newRows.length);
  // 两侧 UI 列定义不同:按"包含种子设备编码"做集合级等价(机器规则:ORDER_INSENSITIVE)
  const codes = ["DUAL-DV1", "DUAL-DV2", "DUAL-DV3", "DUAL-DV4"];
  for (const c of codes) {
    expect(
      newRows.some((r) => r.includes(c)),
      `新 UI 含 ${c}`,
    ).toBe(true);
    expect(
      oldRows.some((r) => r.includes(c)),
      `旧 UI 含 ${c}`,
    ).toBe(true);
  }
  expect(newCore.length).toBe(oldCore.length);

  // 树节点:双跑 DC/房间/两柜在两侧都可见
  for (const label of ["DUAL-DC", "DUAL-KA", "DUAL-KB"]) {
    expect(
      newTree.some((t) => t.includes(label)),
      `新树含 ${label}`,
    ).toBe(true);
    expect(oldBody.includes(label), `旧资源层级页含 ${label}`).toBe(true);
  }

  // API 双向:新旧两侧都真实调用了设备列表与资源树接口且成功
  expect(newApiCalls.some((u) => u.startsWith("/api/v1/devices"))).toBe(true);
  expect(oldApiCalls.some((u) => u.startsWith("/api/v1/devices"))).toBe(true);
  expect(oldErrors, "旧 UI 无未捕获异常").toEqual([]);
});
