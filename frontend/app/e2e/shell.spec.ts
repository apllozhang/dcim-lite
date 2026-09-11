import { expect, test } from "playwright/test";

// Phase 0/P0-B 验收:匿名守卫 + 登录闭环(真后端 captcha)+ 会话恢复 + 401 收口 + flag 回退
const USER = process.env.E2E_USERNAME ?? "admin";
const PASS = process.env.E2E_PASSWORD ?? "";

test("anonymous is redirected to login with captcha", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.locator("[data-test=captcha-input]")).toBeVisible();
});

test("full login flow reaches read-only shell", async ({ page }) => {
  test.skip(!PASS, "E2E_PASSWORD 未提供时跳过");
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

test("legacy flag redirects module to legacy bundle(新→旧回退演练)", async ({ page }) => {
  test.skip(!PASS, "E2E_PASSWORD 未提供时跳过");
  // 登录
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

  // 模块切到 legacy → 访问该模块被整页重定向到旧 bundle 路径
  await page.evaluate(() => localStorage.setItem("ale.flags", "devices:legacy"));
  await page.goto("/devices");
  await page.waitForURL((u) => u.pathname.startsWith("/legacy/"), { timeout: 15000 });

  // URL 参数强制切回(新→旧→新):清 localStorage 后用 URL flag 回到新版
  await page.goto("/login");
  await page.evaluate(() => localStorage.removeItem("ale.flags"));
  await page.goto("/?ale_flags=devices:new");
  await page.waitForURL((u) => !u.pathname.startsWith("/legacy/"), { timeout: 15000 });
  await expect(page.locator("[data-test=resource-tree]")).toBeVisible();
});
