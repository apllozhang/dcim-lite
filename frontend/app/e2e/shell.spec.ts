import { expect, test } from "playwright/test";

// Phase 1 双跑冒烟:匿名访问 → 回登录页 → 验证码渲染
test("anonymous is redirected to login with captcha", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.locator("[data-test=captcha-input]")).toBeVisible();
});
