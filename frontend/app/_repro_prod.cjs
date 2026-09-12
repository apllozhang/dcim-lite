/* 本地复现 CI 挂起:DUAL 机房导出→导入校验。 */
const { chromium } = require("@playwright/test");
const path = require("path");
const fs = require("fs");

const BASE = "http://localhost:5174";
const OUT = "F:/AIwork/ZCode/Vibe Coding/cabinet-management-tool/_s6b_shots";

(async () => {
  const b = await chromium.launch({
    executablePath:
      "C:/Users/Administrator/AppData/Local/ms-playwright/chromium-1234/chrome-win64/chrome.exe",
  });
  const ctx = await b.newContext({ viewport: { width: 1600, height: 900 }, acceptDownloads: true });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("PAGEERROR:", String(e).slice(0, 300)));
  page.on("console", (m) => {
    if (m.type() === "error" || m.type() === "warning") console.log("CONSOLE:", m.type(), m.text().slice(0, 200));
  });

  await page.goto(BASE + "/login");
  await page.evaluate(() => localStorage.removeItem("ale.token"));
  await page.goto(BASE + "/login");
  await page.fill("input[placeholder='请输入用户名']", "admin");
  await page.fill("input[placeholder='请输入密码']", "admin123456");
  const src = await page.locator("[data-test=captcha-img]").getAttribute("src");
  const code = await page.evaluate((b64) => {
    const svg = atob(b64.split(",", 2)[1]);
    return (svg.match(/>(\d)<\/text>/g) ?? []).map((m) => m[1]).join("");
  }, src);
  await page.fill("[data-test=captcha-input]", code);
  await page.click("[data-test=login-btn]");
  await page.waitForURL((u) => !String(u).includes("/login"), { timeout: 15000 });

  await page.goto(BASE + "/room-screen");
  await page.waitForSelector(".rack-card", { timeout: 20000 });
  await page.waitForTimeout(1000);

  const [download] = await Promise.all([
    page.waitForEvent("download", { timeout: 30000 }),
    page.locator("[data-test=screen-export-btn]").click(),
  ]);
  const xlsxPath = path.join(OUT, "diagram-dual.xlsx");
  await download.saveAs(xlsxPath);
  console.log("exported:", fs.statSync(xlsxPath).size, "bytes");

  await page.locator("[data-test=screen-import-btn]").click();
  await page.waitForTimeout(700);
  await page.locator("button", { hasText: "选择机柜图" }).click();
  await page.setInputFiles("input[type=file][accept='.xlsx']", xlsxPath);
  await page.locator("[data-test=diagram-validate-btn]").click();
  console.log("validate clicked, waiting 12s...");
  await page.waitForTimeout(12000);
  console.log("summary count:", await page.locator(".validation-summary").count());
  console.log("toast:", await page.locator(".el-message").allInnerTexts().catch(() => []));
  console.log("dialog text:", (await page.locator(".el-dialog:visible").last().innerText().catch(() => "n/a")).slice(0, 500));
  await page.screenshot({ path: OUT + "/dual-validate.png" });
  await b.close();
  console.log("OK");
})().catch((e) => {
  console.error("FAIL", e.message);
  process.exit(1);
});
