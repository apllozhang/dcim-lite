import { expect, test, type Page } from "playwright/test";

// Phase 0/P0-B 验收:匿名守卫 + 登录闭环(真后端 captcha)+ 会话恢复 + 401 收口 + flag 回退
const USER = process.env.E2E_USERNAME ?? "admin";
const PASS = process.env.E2E_PASSWORD ?? "";

/** 登录助手:真后端 captcha 解析 + 提交;P0-R2 下预览构建需先解锁调试令牌 */
async function login(page: Page): Promise<void> {
  await page.goto("/login");
  await page.evaluate(() => localStorage.setItem("ale.flags.debug", "1"));
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
}

test("anonymous is redirected to login with captcha", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.locator("[data-test=captcha-input]")).toBeVisible();
});

test("full login flow reaches read-only shell", async ({ page }) => {
  test.skip(!PASS, "E2E_PASSWORD 未提供时跳过");
  await login(page);
  await expect(page.locator("[data-test=resource-tree]")).toBeVisible();
  await expect(page.locator("[data-test=user-menu]")).toContainText("管理员");

  // 会话恢复:刷新后仍登录
  await page.reload();
  await expect(page.locator("[data-test=user-menu]")).toBeVisible();

  // 401 收口:伪造 token 后任何数据接口 401 → 回登录页
  await page.evaluate(() => localStorage.setItem("ale.token", "invalid-token"));
  await page.goto("/devices");
  await page.waitForURL((u) => u.pathname.includes("/login"), { timeout: 15000 });
});

test("frontend error telemetry reaches admin retrieval(P1-R6)", async ({ page }) => {
  test.skip(!PASS, "E2E_PASSWORD 未提供时跳过");
  await login(page);

  // 注入受控异常 → 前端 remote reporter 上报 → 断言上报请求 2xx
  const reportPromise = page.waitForResponse(
    (r) => r.url().includes("/api/v1/telemetry/frontend-errors"),
    { timeout: 15000 },
  );
  const marker = `p1r6-e2e-${Date.now()}`;
  await page.evaluate((m) => window.__ALE_FORCE_REPORT__?.(m), marker);
  const report = await reportPromise;
  expect(report.status()).toBe(200);

  // 管理端检索:环形缓冲里必须能查到这条受控事件(端到端闭环,而非仅 console)
  const items = await page.evaluate(async () => {
    const token = localStorage.getItem("ale.token") ?? "";
    const r = await fetch("/api/v1/admin/telemetry/frontend-errors?limit=100", {
      headers: { Authorization: `Bearer ${token}` },
    });
    return (await r.json())?.data?.items ?? [];
  }, marker);
  expect(
    items.some((i: { message?: string }) => i.message?.includes(marker)),
    "受控异常必须能在 admin 检索接口中找到",
  ).toBe(true);
});

test("legacy flag redirects module to legacy origin(新→旧→新回退演练,真加载验证)", async ({
  page,
}) => {
  test.skip(!PASS, "E2E_PASSWORD 未提供时跳过");
  // P1-D 修订:旧 ALE 为 HTML5 history 模式,回退目标 = 独立端口源(同主机:19501)。
  // 监听:旧源资源 200、主文档状态、pageerror、API 成功计数
  const pageErrors: string[] = [];
  const badMainDocs: string[] = [];
  let legacyAsset200 = 0;
  let api200 = 0;
  page.on("pageerror", (e) => pageErrors.push(String(e)));
  page.on("response", (r) => {
    if (r.url().includes(":19501/assets/") && r.status() === 200) legacyAsset200++;
    if (r.url().includes(":19501") && r.request().resourceType() === "document") {
      if (r.status() !== 200) badMainDocs.push(`${r.status()} ${r.url()}`);
    }
    if (r.url().includes("/api/v1/") && r.status() === 200) api200++;
  });

  // 登录(helper 内先解锁调试令牌:P0-R2 治理下预览构建属生产模式)
  await login(page);

  // 旧源注入会话(旧 UI 独立 origin,localStorage 隔离;真实用户在旧 UI 登录,等价会话移交)
  await page.goto("http://localhost:19501/");
  await page.evaluate(() =>
    localStorage.setItem("cabinet_access_token", localStorage.getItem("ale.token") ?? ""),
  );

  // 模块切到 legacy → 整页重定向到旧源(HTML5 路径)
  await page.evaluate(() => localStorage.setItem("ale.flags", "devices:legacy"));
  await page.goto("/devices");
  await page.waitForURL((u) => u.port === "19501", { timeout: 15000 });

  // 真加载断言:旧标题、旧资源 200、主文档 200、旧导航真实渲染、无 JS 异常
  await expect(page).toHaveTitle(/机柜管理工具/, { timeout: 15000 });
  expect(legacyAsset200, "旧 bundle 静态资源必须有真实 200 加载").toBeGreaterThan(0);
  expect(badMainDocs, "旧源主文档不得出现 4xx/5xx").toEqual([]);
  await expect(page.getByText("设备台账", { exact: false }).first()).toBeVisible({
    timeout: 15000,
  });
  expect(pageErrors, "旧页面不得有未捕获 JS 异常").toEqual([]);

  // 新→旧→新:回到新源,URL 参数切回 devices:new,断言新树真实渲染且 API 成功
  await page.goto("/?ale_flags=devices:new");
  await page.waitForURL((u) => u.port !== "19501", { timeout: 15000 });
  await expect(page).toHaveTitle(/ALE 机柜管理/, { timeout: 15000 });
  await expect(page.locator("[data-test=resource-tree]")).toBeVisible();
  expect(api200, "切回新版后 API 必须真实成功").toBeGreaterThan(0);
  expect(pageErrors, "新页面不得有未捕获 JS 异常").toEqual([]);
});
