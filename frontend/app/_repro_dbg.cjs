/* 建调试机房(20U 柜×2 + 3 台 1U 设备同 DUAL 形态)→导出→导入,复现 CI 挂起。 */
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
  const token = await page.evaluate(() => localStorage.getItem("ale.token"));

  // 种子(幂等):DBG-DC/DBG-R + 2×20U 柜 + 3×1U 设备(对齐 DUAL 形态)
  const post = (p, body) =>
    page.evaluate(async ({ p, body, t }) => {
      const r = await fetch(p, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: "Bearer " + t },
        body: JSON.stringify(body),
      });
      const j = await r.json().catch(() => ({}));
      return { status: r.status, data: j.data };
    }, { p, body, t: token });
  const tree0 = await page.evaluate(async (t) => {
    const x = await (await fetch("/api/v1/resource-tree", { headers: { Authorization: "Bearer " + t } })).json();
    return (x.data.items ?? []).find((d) => d.code === "DBG-DC");
  }, token);
  let dc, room;
  if (tree0) {
    dc = { data: { id: tree0.id, version: tree0.version } };
    const rm = tree0.rooms[0];
    room = { data: { id: rm.id } };
    console.log("reuse existing DBG-DC");
  } else {
    dc = await post("/api/v1/data-centers", { code: "DBG-DC", name: "调试中心" });
    room = await post(`/api/v1/data-centers/${dc.data.id}/rooms`, { code: "DBG-R", name: "调试房间" });
  }
  const existRacks = tree0 ? tree0.rooms[0].racks ?? [] : [];
  let ka;
  const kaExist = existRacks.find((k) => k.code === "DBG-KA");
  if (kaExist) ka = { data: { id: kaExist.id } };
  else ka = await post(`/api/v1/rooms/${room.data.id}/racks`, { code: "DBG-KA", name: "调试柜A", uHeight: 20 });
  const kbExist = existRacks.find((k) => k.code === "DBG-KB");
  if (!kbExist) await post(`/api/v1/rooms/${room.data.id}/racks`, { code: "DBG-KB", name: "调试柜B", uHeight: 20 });
  const type = await page.evaluate(async (t) => {
    const r = await (await fetch("/api/v1/device-types", { headers: { Authorization: "Bearer " + t } })).json();
    return (r.data.items ?? []).find((x) => x.code === "SERVER") ?? (r.data.items ?? [])[0];
  }, token);
  const devIds = [];
  for (let i = 1; i <= 3; i++) {
    const exist = await page.evaluate(async ({ t, code }) => {
      const r = await (await fetch(`/api/v1/devices?search=${code}`, { headers: { Authorization: "Bearer " + t } })).json();
      return (r.data.items ?? []).find((d) => d.code === code);
    }, { t: token, code: `DBG-DV${i}` });
    if (exist) {
      devIds.push(exist.id);
      continue;
    }
    const d = await post("/api/v1/devices", {
      code: `DBG-DV${i}`,
      name: `双跑-DUAL-${i}`,
      typeId: type.id,
      heightU: 1,
    });
    devIds.push(d.data.id);
    await post(`/api/v1/devices/${d.data.id}/assign`, { rackId: ka.data.id, startU: i });
  }
  console.log("seeded");

  // 切到调试机房
  await page.goto(BASE + "/room-screen");
  await page.waitForSelector(".rack-card", { timeout: 20000 });
  await page.waitForTimeout(800);
  await page.locator("[data-test=screen-dc-select]").click();
  await page.locator(".el-select-dropdown__item", { hasText: "调试中心" }).first().click();
  await page.waitForTimeout(800);
  await page.locator("[data-test=screen-room-select]").click();
  await page.locator(".el-select-dropdown__item", { hasText: "调试房间" }).first().click();
  await page.waitForTimeout(1500);

  const [download] = await Promise.all([
    page.waitForEvent("download", { timeout: 30000 }),
    page.locator("[data-test=screen-export-btn]").click(),
  ]);
  const xlsxPath = path.join(OUT, "diagram-dbg.xlsx");
  await download.saveAs(xlsxPath);
  console.log("exported:", fs.statSync(xlsxPath).size);

  await page.locator("[data-test=screen-import-btn]").click();
  await page.waitForTimeout(700);
  await page.locator("button", { hasText: "选择机柜图" }).click();
  await page.setInputFiles("input[type=file][accept='.xlsx']", xlsxPath);
  await page.locator("[data-test=diagram-validate-btn]").click();
  console.log("validate clicked...");
  for (let i = 0; i < 6; i++) {
    await page.waitForTimeout(5000);
    const n = await page.locator(".validation-summary").count();
    console.log(`t+${(i + 1) * 5}s summary:`, n);
    if (n) break;
  }
  console.log("toast:", await page.locator(".el-message").allInnerTexts().catch(() => []));
  await page.screenshot({ path: OUT + "/dbg-validate.png" });

  // 清理:删设备(下架+删)→删柜→删房→删DC
  for (const id of devIds) {
    await post(`/api/v1/devices/${id}/decommission`, { reason: "dbg" });
  }
  const del = (p, q) =>
    page.evaluate(async ({ p, q, t }) => {
      const r = await fetch(p + (q ? `?version=${q}` : ""), {
        method: "DELETE",
        headers: { Authorization: "Bearer " + t },
      });
      return r.status;
    }, { p, q, t: token });
  const racks = await page.evaluate(async (t) => {
    const x = await (await fetch("/api/v1/resource-tree", { headers: { Authorization: "Bearer " + t } })).json();
    const dc2 = (x.data.items ?? []).find((d) => d.code === "DBG-DC");
    const rm = dc2.rooms[0];
    return rm.racks.map((k) => ({ id: k.id, v: k.version }));
  }, token);
  for (const k of racks) await del(`/api/v1/racks/${k.id}`, k.v);
  const rooms = await page.evaluate(async (t) => {
    const x = await (await fetch("/api/v1/resource-tree", { headers: { Authorization: "Bearer " + t } })).json();
    const dc2 = (x.data.items ?? []).find((d) => d.code === "DBG-DC");
    return dc2.rooms.map((r) => ({ id: r.id, v: r.version }));
  }, token);
  for (const r of rooms) await del(`/api/v1/rooms/${r.id}`, r.v);
  await del(`/api/v1/data-centers/${dc.data.id}`, dc.data.version);
  console.log("cleaned");

  await b.close();
  console.log("OK");
})().catch((e) => {
  console.error("FAIL", e.message);
  process.exit(1);
});
