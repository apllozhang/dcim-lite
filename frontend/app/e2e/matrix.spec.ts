import { expect, test, type Page } from "playwright/test";
import * as fs from "node:fs";

/**
 * 第 5 轮 P1-D 收口(复评 §5/§6):
 * 1) 五态种子(WAITING_RACK/RUNNING/MAINTENANCE/PENDING_REMOVAL/OFF_RACK)
 *    × 新旧双跑对照——状态列以旧 bundle 中文口径(待上架/运行中/…)断言;
 * 2) 筛选交互差分:生命周期筛选在新旧两侧结果集合一致;
 * 3) 三权限矩阵:admin/user × 新 UI(菜单+/admin 守卫) × 旧 UI(菜单+/admin 重定向)
 *    + 后端 API 矩阵(user GET /admin/users 403);
 * 4) 新 UI 关键页像素基准(toHaveScreenshot,阈值 2%,基准由 CI 首跑生成入库)。
 *
 * 种子前缀 MTX-:与 dualrun.spec 的 DUAL- 前缀隔离(同栈多 spec 编码不冲突)。
 * 状态可达性口径:WAITING_RACK=创建缺省,RUNNING=assign,OFF_RACK=assign→decommission,
 * MAINTENANCE/PENDING_REMOVAL=创建时显式指定(CreateDevice 接受合法枚举;
 * PUT 更新对状态收权 preserve existing,与厂商一致)。
 */

const ADMIN = process.env.E2E_USERNAME ?? "admin";
const ADMIN_PASS = process.env.E2E_PASSWORD ?? "";
// 旧 UI 对照源:CI 由 job 起在本机 19501;本地连远端栈时用 E2E_LEGACY_BASE 覆盖
const OLD = process.env.E2E_LEGACY_BASE ?? "http://localhost:19501";
const OLD_HOST = new URL(OLD).host;

/** 五态种子(确定性顺序,任何环境可复现) */
const FIVE = [
  { code: "MTX-DV1", status: "RUNNING", label: "运行中" },
  { code: "MTX-DV2", status: "WAITING_RACK", label: "待上架" },
  { code: "MTX-DV3", status: "MAINTENANCE", label: "维护中" },
  { code: "MTX-DV4", status: "OFF_RACK", label: "已下架" },
  { code: "MTX-DV5", status: "PENDING_REMOVAL", label: "待下架" },
];

let adminToken = "";

async function loginAs(page: Page, user: string, pass: string): Promise<string> {
  // 先清会话:已登录(localStorage 有 ale.token)时路由守卫把 /login 弹回首页,
  // 登录表单永远不出现(CI 第三跑用例2 fill 超时根因,aria 快照实证)
  await page.goto("/login");
  await page.evaluate(() => {
    localStorage.removeItem("ale.token");
    localStorage.setItem("ale.flags.debug", "1");
  });
  // 清 token 后重新完整导航,应用重载、守卫按匿名放行
  await page.goto("/login");
  await page.reload();
  await page.fill("input[placeholder='请输入用户名']", user);
  await page.fill("input[placeholder='请输入密码']", pass);
  const src = await page.locator("[data-test=captcha-img]").getAttribute("src");
  const code = await page.evaluate((b64) => {
    const svg = atob(b64.split(",", 2)[1]);
    return (svg.match(/>(\d)<\/text>/g) ?? []).map((m) => m[1]).join("");
  }, src ?? "");
  await page.fill("[data-test=captcha-input]", code);
  await page.click("[data-test=login-btn]");
  await page.waitForURL((u) => !u.pathname.includes("/login"), { timeout: 15000 });
  return page.evaluate(() => localStorage.getItem("ale.token") ?? "");
}

/** API 直调:返回 {status, envelope};envelope.data 即资源(Envelope 单层包装) */
async function api(
  page: Page,
  method: string,
  path: string,
  body?: unknown,
  token: string = adminToken,
): Promise<{ status: number; envelope: { data?: unknown; code?: string } | null }> {
  return page.evaluate(
    async ({ method, path, body, token }) => {
      const r = await fetch(path, {
        method,
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      let j = null;
      try {
        j = await r.json();
      } catch {
        /* 非 JSON */
      }
      return { status: r.status, envelope: j };
    },
    { method, path, body, token },
  );
}

async function tableRows(page: Page): Promise<string[]> {
  return page.$$eval(".el-table__body tr", (rows) =>
    rows.map((r) => r.textContent?.replace(/\s+/g, " ").trim() ?? ""),
  );
}

test.beforeAll(async ({ request }) => {
  for (let i = 0; i < 30; i++) {
    const r = await request.get("/health/ready");
    if (r.ok()) return;
    await new Promise((res) => setTimeout(res, 2000));
  }
  throw new Error("backend not ready");
});

test("五态种子:新旧 UI 状态口径对照 + 生命周期筛选差分", async ({ page }) => {
  test.skip(!ADMIN_PASS, "E2E_PASSWORD 未提供时跳过");
  test.setTimeout(180000); // 种子 API 多次往返 + 双 UI 渲染 + 两轮筛选 + 屏6b 导出回导闭环

  // ── 种子:1 DC/1 房/2 柜 + 五态各一 ──
  adminToken = await loginAs(page, ADMIN, ADMIN_PASS);
  const resOf = (r: { envelope: { data?: unknown } | null }) =>
    (r.envelope?.data ?? {}) as { id?: string };
  const dc = resOf(
    await api(page, "POST", "/api/v1/data-centers", { code: "MTX-DC", name: "矩阵机房" }),
  );
  const room = resOf(
    await api(page, "POST", `/api/v1/data-centers/${dc.id}/rooms`, {
      code: "MTX-R",
      name: "矩阵房间",
    }),
  );
  const rackA = resOf(
    await api(page, "POST", `/api/v1/rooms/${room.id}/racks`, {
      code: "MTX-KA",
      name: "矩阵柜A",
      uHeight: 20,
    }),
  );
  const rackB = resOf(
    await api(page, "POST", `/api/v1/rooms/${room.id}/racks`, {
      code: "MTX-KB",
      name: "矩阵柜B",
      uHeight: 20,
    }),
  );
  const types = (await api(page, "GET", "/api/v1/device-types")).envelope?.data as {
    items?: { code: string; id: string }[];
  };
  const srv = types?.items?.find((t) => t.code === "SERVER");

  const idOf: Record<string, string> = {};
  for (const d of FIVE) {
    const dev = resOf(
      await api(page, "POST", "/api/v1/devices", {
        typeId: srv?.id,
        code: d.code,
        name: `矩阵-${d.code}`,
        heightU: 1,
        ...(d.status === "MAINTENANCE" || d.status === "PENDING_REMOVAL"
          ? { lifecycleStatus: d.status }
          : {}),
      }),
    );
    idOf[d.code] = dev.id ?? "";
  }
  // 状态机路径:DV1 上架→RUNNING;DV4 上架再下架→OFF_RACK
  await api(page, "POST", `/api/v1/devices/${idOf["MTX-DV1"]}/assign`, {
    rackId: rackA.id,
    startU: 1,
  });
  await api(page, "POST", `/api/v1/devices/${idOf["MTX-DV4"]}/assign`, {
    rackId: rackB.id,
    startU: 1,
  });
  await api(page, "POST", `/api/v1/devices/${idOf["MTX-DV4"]}/decommission`, {
    reason: "matrix-e2e",
  });

  // ── 新 UI:五台设备 + 中文状态标签(旧 bundle zl 口径) ──
  await page.goto("/devices");
  await expect(page.locator("[data-test=device-table] tbody tr").first()).toBeVisible();
  // 测试库可能含存量设备(脏库兼容):先按 MTX- 前缀收窄,种子断言与截图只含种子行
  await page.fill("[data-test=device-search]", "MTX-");
  await page.locator("[data-test=device-search-btn]").click();
  await page.waitForTimeout(800);
  let rows = await tableRows(page);
  for (const d of FIVE) {
    const row = rows.find((r) => r.includes(d.code));
    expect(row, `新 UI 含 ${d.code}`).toBeTruthy();
    expect(row, `新 UI ${d.code} 状态列中文口径`).toContain(d.label);
  }
  await expect(page).toHaveScreenshot("mtx-new-devices.png", {
    fullPage: true,
    maxDiffPixelRatio: 0.02,
  });

  // ── 新 UI:生命周期筛选"待上架" → 只剩 DV2 ──
  // 先注册监听再交互:click 后请求可能瞬间完成,晚注册会 miss(快环境竞态)
  const newFiltered = page.waitForResponse((r) => r.url().includes("lifecycleStatus=WAITING_RACK"));
  await page.locator("[data-test=device-status-filter]").click();
  await page.locator(".el-select-dropdown__item", { hasText: "待上架" }).click();
  await newFiltered;
  rows = await tableRows(page);
  const mtxVisible = rows.filter((r) => r.includes("MTX-DV"));
  expect(mtxVisible.length, "新 UI 筛选后 MTX 设备只剩待上架一台").toBe(1);
  expect(mtxVisible[0]).toContain("MTX-DV2");

  // ── 新 UI:搜索差分(编码搜索 → 收窄到一台;后端六列 ILIKE 同语义) ──
  const newSearch = page.waitForResponse(
    (r) => r.url().includes("search=MTX-DV2") && r.request().method() === "GET",
  );
  await page.fill("[data-test=device-search]", "MTX-DV2");
  await page.locator("[data-test=device-search-btn]").click();
  await newSearch;
  rows = await tableRows(page);
  const newSearched = rows.filter((r) => r.includes("MTX-DV"));
  expect(newSearched.length, "新 UI 搜索 MTX-DV2 只剩一台").toBe(1);
  expect(newSearched[0]).toContain("MTX-DV2");

  // ── 第 7 轮 P1-01:主表三列恢复(设备类型/资产编号/管理 IP) ──
  const deviceHead = await page.locator("[data-test=device-table] thead").innerText();
  for (const col of ["设备类型", "资产编号", "管理 IP"]) {
    expect(deviceHead, `设备表恢复 ${col} 列`).toContain(col);
  }

  // ── 第 7 轮 P0-02:导出设备信息——Node 侧校验文件本身(评审验收:不能只看 download 事件)。
  // 此刻筛选=搜索 MTX-DV2,导出应遵循当前筛选口径:仅 1 台。 ──
  const [devExport] = await Promise.all([
    page.waitForEvent("download", { timeout: 30000 }),
    page.locator("button", { hasText: "导出设备信息" }).click(),
  ]);
  expect(devExport.suggestedFilename(), "导出文件名").toMatch(/^设备信息_.+\.xlsx$/);
  const XLSX = await import("xlsx").then(
    (m) => ("default" in m ? m.default : m) as typeof import("xlsx"),
  );
  const devWb = XLSX.readFile(await devExport.path());
  const devGrid = XLSX.utils.sheet_to_json<string[]>(devWb.Sheets[devWb.SheetNames[0]], {
    header: 1,
  });
  expect(devGrid[0]?.length, "导出表头 43 列(v2 Hl 集合)").toBe(43);
  for (const col of ["设备编码", "设备类型", "资产编号", "管理IP", "设备分类", "上架状态"]) {
    expect(devGrid[0], `导出表头含 ${col}`).toContain(col);
  }
  const devDataRows = devGrid.slice(1).filter((r) => String(r?.[0] ?? "").trim());
  expect(devDataRows.length, "导出遵循当前筛选口径(仅 MTX-DV2)").toBe(1);
  expect(String(devDataRows[0][0])).toBe("MTX-DV2");
  expect(String(devDataRows[0][4]), "生命周期中文口径").toBe("待上架");
  // 导出文件改名回传导入(应用校验 .xlsx 扩展名,path() 是无后缀 GUID 名)
  const devXlsxRaw = await devExport.path();
  const devXlsxPath = `${devXlsxRaw}.xlsx`;
  fs.renameSync(devXlsxRaw, devXlsxPath);

  // ── 第 7 轮 P0-01:批量导入——对话框 + xlsx 模板下载 + 导出文件回导校验预览(不提交) ──
  await page.locator("[data-test=batch-import-btn]").click();
  const deviceImportDialog = page.locator(".el-overlay:visible .el-dialog", {
    hasText: "批量导入设备",
  });
  await expect(deviceImportDialog).toBeVisible({ timeout: 8000 });
  const [devTpl] = await Promise.all([
    page.waitForEvent("download", { timeout: 30000 }),
    page.locator("[data-test=device-import-template-btn]").click(),
  ]);
  expect(devTpl.suggestedFilename(), "模板文件名").toBe("设备批量导入模板.xlsx");
  const tplWb = XLSX.readFile(await devTpl.path());
  const tplGrid = XLSX.utils.sheet_to_json<string[]>(tplWb.Sheets[tplWb.SheetNames[0]], {
    header: 1,
  });
  expect(tplGrid[0], "模板 35 列(v2 st 同序)").toHaveLength(35);
  // 表头带"（必填）"标记,解析端已容忍该后缀(matchColumn 剔除后匹配)
  expect(
    tplGrid[0].some((h) => String(h).startsWith("设备类型编码")),
    "模板表头",
  ).toBe(true);
  expect(String(tplGrid[1]?.[0] ?? ""), "模板第二行为字段说明").toContain("启用");
  await page.setInputFiles('input[accept=".xlsx,.xls"]', devXlsxPath);
  await expect(page.locator("[data-test=device-import-stats]")).toBeVisible({ timeout: 20000 });
  const importStats = await page.locator("[data-test=device-import-stats]").innerText();
  expect(importStats, "回导总行数 1").toContain("总行数 1");
  expect(importStats, "回导校验全过").toContain("可导入 1");
  const previewRow = deviceImportDialog.locator(".el-table__body tr").first();
  await expect(previewRow, "按编码匹配 → 预览为更新").toContainText("UPDATE");
  await expect(previewRow, "匹配到 MTX-DV2").toContainText("MTX-DV2");
  await deviceImportDialog.locator("button", { hasText: "关闭" }).click();
  await page.waitForTimeout(400);

  // ── 旧 UI:五台设备 + 中文状态(独立源注入会话) ──
  const oldErrors: string[] = [];
  page.on("pageerror", (e) => oldErrors.push(String(e)));
  await page.goto(`${OLD}/`);
  await page.evaluate((t) => localStorage.setItem("cabinet_access_token", t), adminToken);
  await page.goto(`${OLD}/devices`);
  await page.waitForSelector(".el-table__body tr", { timeout: 20000 });
  // 脏库兼容:同新 UI,先按 MTX- 前缀搜索收窄到种子行(查询点击读取当前输入框值)
  await page.fill("input[placeholder*='编码']", "MTX-");
  await page.locator("button", { hasText: "查询" }).click();
  await page.waitForTimeout(1500);
  let oldRows = await tableRows(page);
  for (const d of FIVE) {
    const row = oldRows.find((r) => r.includes(d.code));
    expect(row, `旧 UI 含 ${d.code}`).toBeTruthy();
    expect(row, `旧 UI ${d.code} 状态中文`).toContain(d.label);
  }

  // ── 旧 UI:生命周期筛选"待上架" → 同样只剩 DV2 ──
  // 旧 bundle 为 EP 2.x 新结构:placeholder 渲染为 span 文本而非 input 属性
  // (error-context aria 快照实证:combobox + generic"生命周期"),
  // 故用 .el-select hasText 定位而非 [placeholder=...]。
  // 且旧 UI 筛选变更不自动查询——须点"查询"按钮(CI 第三跑 waitForResponse 超时根因)。
  const filtered = page.waitForResponse(
    (r) => r.url().includes("lifecycleStatus=WAITING_RACK") && r.request().method() === "GET",
  );
  await page.locator(".el-select", { hasText: "生命周期" }).click();
  await page.locator(".el-select-dropdown__item", { hasText: "待上架" }).click();
  await page.locator("button", { hasText: "查询" }).click();
  await filtered;
  await page.waitForTimeout(1200);
  oldRows = await tableRows(page);
  const oldVisible = oldRows.filter((r) => r.includes("MTX-DV"));
  expect(oldVisible.length, "旧 UI 筛选后 MTX 设备只剩待上架一台(与新 UI 一致)").toBe(1);
  expect(oldVisible[0]).toContain("MTX-DV2");

  // ── 旧 UI:搜索差分(aria 实证有搜索框:placeholder=名称、编码、资产号、序列号或 IP) ──
  const oldSearch = page.waitForResponse(
    (r) => r.url().includes("search=MTX-DV2") && r.request().method() === "GET",
  );
  await page.locator("input[placeholder*='编码']").fill("MTX-DV2");
  await page.locator("button", { hasText: "查询" }).click();
  await oldSearch;
  await page.waitForTimeout(1200);
  oldRows = await tableRows(page);
  const oldSearched = oldRows.filter((r) => r.includes("MTX-DV"));
  expect(oldSearched.length, "旧 UI 搜索 MTX-DV2 只剩一台(与新 UI 一致)").toBe(1);
  expect(oldSearched[0]).toContain("MTX-DV2");
  expect(oldErrors, "旧 UI 全程无未捕获异常").toEqual([]);

  // ── 像素基准:新 UI 资源层级页(MTX 种子后;第 6 轮起为三栏资源页) ──
  await page.goto("/data-centers");
  await expect(page.locator("[data-test=resource-page]")).toBeVisible({
    timeout: 15000,
  });
  await expect(page.locator("[data-test=dc-item]", { hasText: "MTX-DC" })).toBeVisible();
  await expect(page).toHaveScreenshot("mtx-new-hierarchy.png", {
    fullPage: true,
    maxDiffPixelRatio: 0.02,
  });

  // ── 像素基准:新 UI 机柜管理(MTX 两柜入台账;状态/规格/模板口径与 v2 一致) ──
  await page.goto("/racks");
  await expect(page.locator("[data-test=rack-table] tbody tr").first()).toBeVisible({
    timeout: 15000,
  });
  // 脏库兼容:存量机柜多,先按 MTX- 前缀搜索收窄到种子行
  await page.fill("[data-test=rack-search]", "MTX-");
  await page.locator("[data-test=rack-search-btn]").click();
  await page.waitForTimeout(800);
  rows = await tableRows(page);
  for (const code of ["MTX-KA", "MTX-KB"]) {
    const row = rows.find((r) => r.includes(code));
    expect(row, `机柜台账含 ${code}`).toBeTruthy();
    expect(row, `${code} 状态中文口径`).toContain("空闲");
  }
  expect(await page.locator("[data-test=rack-page]").innerText()).toContain("机柜总数");
  await expect(page).toHaveScreenshot("mtx-new-racks.png", {
    fullPage: true,
    maxDiffPixelRatio: 0.02,
  });

  // ── 像素基准:新 UI 机柜模板(系统内置 42U 模板在列) ──
  await page.goto("/rack-templates");
  await expect(page.locator("[data-test=rack-template-table] tbody tr").first()).toBeVisible({
    timeout: 15000,
  });
  const tplText = await page.locator("[data-test=rack-template-table]").innerText();
  expect(tplText, "模板列表含系统内置标准 42U 模板").toContain("标准 42U");
  expect(tplText, "系统内置模板带系统标记").toContain("系统");
  await expect(page).toHaveScreenshot("mtx-new-rack-templates.png", {
    fullPage: true,
    maxDiffPixelRatio: 0.02,
  });

  // ── 像素基准:新 UI 机房大屏(暗色;独立全屏路由) ──
  // 时钟每秒走字,基准必须 mask 时钟区域,否则永久假红。
  await page.goto("/room-screen");
  await expect(page.locator("[data-test=room-screen]")).toBeVisible({ timeout: 15000 });
  await expect(page.locator(".rack-card").first()).toBeVisible({ timeout: 20000 });
  const screenText = await page.locator("[data-test=room-screen]").innerText();
  expect(screenText, "大屏品牌区").toContain("机房资源大屏");
  expect(screenText, "画布工具栏").toContain("机柜 U 位总览");
  expect(screenText, "状态图例").toContain("机柜状态图例");
  expect(screenText, "设备类型图例").toContain("设备类型图例");
  // 大屏常有一台当前选中(v2 同款自动选中)
  await expect(page.locator(".selected-card")).toBeVisible({ timeout: 10000 });
  await expect(page).toHaveScreenshot("mtx-new-room-screen.png", {
    fullPage: true,
    maxDiffPixelRatio: 0.02,
    mask: [page.locator(".clock")],
  });

  // ── 第 7 轮 P1-03:大屏设备菜单(查看详情/编辑)——切到 MTX 机房(DV1 在矩阵柜A) ──
  await page.locator("[data-test=screen-dc-select]").click();
  await page.locator(".el-select-dropdown__item", { hasText: "矩阵机房" }).click();
  const devBlock = page.locator(".device-block").first();
  await expect(devBlock).toBeVisible({ timeout: 15000 });
  await devBlock.click({ button: "right" });
  const screenMenu = page.locator(".rack-context-menu");
  await expect(screenMenu).toBeVisible({ timeout: 5000 });
  const menuText = await screenMenu.innerText();
  for (const item of ["查看设备详情", "编辑设备信息", "调整 / 迁移位置", "下架设备"]) {
    expect(menuText, `设备菜单含「${item}」(v2 文案)`).toContain(item);
  }
  // 详情抽屉(v2 同名标题+分组)
  await screenMenu.locator("[data-test=screen-device-detail-btn]").click();
  const deviceDrawerEl = page.locator(".el-drawer", { hasText: "设备详细信息" });
  await expect(deviceDrawerEl).toBeVisible({ timeout: 10000 });
  await expect(deviceDrawerEl, "资产分组").toContainText("资产与规格");
  await expect(deviceDrawerEl, "网络分组").toContainText("网络与管理");
  await deviceDrawerEl.locator("button", { hasText: "关闭" }).click();
  await page.waitForTimeout(500);
  // 编辑对话框打开即关(980px screen-device-editor-dialog;不保存不写数据)
  await devBlock.click({ button: "right" });
  await page.locator("[data-test=screen-device-edit-btn]").click();
  const screenEditDialog = page.locator(".el-dialog", { hasText: "编辑设备信息" });
  await expect(screenEditDialog).toBeVisible({ timeout: 10000 });
  await expect(screenEditDialog).toContainText("设备编码");
  await screenEditDialog.locator("button", { hasText: "取消" }).click();
  await page.waitForTimeout(400);

  // ── 屏6b:容量对话框 + 机柜图导出→回导校验闭环(不 commit,不写数据) ──
  await page.locator(".u-button", { hasText: "查看完整 U 位详情" }).click();
  // 标题在 el-dialog header 插槽,body 内容在 .rack-detail-shell,分开断言
  const capDialog = page.locator(".el-overlay:visible .el-dialog", {
    hasText: "机柜详情与容量分析",
  });
  await expect(capDialog).toBeVisible({ timeout: 10000 });
  const shell = page.locator(".rack-detail-shell");
  await expect(shell).toBeVisible({ timeout: 10000 });
  const capText = await shell.innerText();
  expect(capText, "利用率指标卡").toContain("U 位利用率");
  expect(capText, "PDU 面板").toContain("PDU 与供电连接");
  await page.keyboard.press("Escape");
  await page.waitForTimeout(600);

  const [download] = await Promise.all([
    page.waitForEvent("download", { timeout: 30000 }),
    page.locator("[data-test=screen-export-btn]").click(),
  ]);
  // download.path() 是无后缀 GUID 临时名,应用的扩展名校验会正确拒绝——
  // 必须改名为 .xlsx 再回传(应用的防护是对的,测试要遵守)
  const xlsxRaw = await download.path();
  const xlsxPath = `${xlsxRaw}.xlsx`;
  fs.renameSync(xlsxRaw, xlsxPath);
  expect(xlsxPath, "导出机柜图落地").toBeTruthy();

  await page.locator("[data-test=screen-import-btn]").click();
  await page.waitForTimeout(600);
  page.on("console", (m) => {
    if (m.text().includes("[diagram]")) console.log("PAGE:", m.text().slice(0, 160));
  });
  await page.locator("button", { hasText: "选择机柜图" }).click();
  await page.setInputFiles("input[type=file][accept='.xlsx']", xlsxPath);
  await page.locator("[data-test=diagram-validate-btn]").click();
  // 诊断:validate 请求状态与响应体直打日志(失败时 error-context 只有页面快照,看不到 toast)
  const vrespPromise = page.waitForResponse(
    (r) => r.url().includes("rack-diagram-import/validate"),
    { timeout: 30000 },
  );
  let vdesc = "no-request";
  try {
    const vresp = await vrespPromise;
    vdesc = `${vresp.status()} ${(await vresp.text()).slice(0, 300)}`;
  } catch {
    vdesc = "no-request-within-30s";
  }
  console.log("DIAGRAM-VALIDATE:", vdesc);
  console.log(
    "IMPORT-DIALOG-OPEN:",
    await page.locator(".el-overlay:visible .el-dialog", { hasText: "导入机柜图" }).count(),
  );
  console.log(
    "FILE-STATUS:",
    await page
      .locator(".file-control > span")
      .first()
      .innerText()
      .catch(() => "n/a"),
  );
  console.log(
    "TOAST:",
    await page
      .locator(".el-message")
      .allInnerTexts()
      .catch(() => []),
  );
  // 同源导出秒级回读:全部设备"保持不变",0 错误 0 待确认(dualrun 种子确定性)
  await expect(page.locator(".validation-summary")).toBeVisible({ timeout: 20000 });
  const sumText = await page.locator(".validation-summary").innerText();
  expect(sumText, "回导校验无错误").toMatch(/错误\s*\n?\s*0/);
  expect(sumText, "回导校验无待确认").toMatch(/待人工确认\s*\n?\s*0/);
  await expect(page.locator(".el-overlay:visible .el-dialog", { hasText: "校验通过" })).toBeVisible(
    {
      timeout: 10000,
    },
  );
});

test("三权限矩阵:/admin 守卫与菜单的新旧对照 + API 403", async ({ page }) => {
  test.skip(!ADMIN_PASS, "E2E_PASSWORD 未提供时跳过");
  test.setTimeout(120000); // 账号创建 + 三轮登录 + 双 UI 直达/重定向

  // 页面必须真实登录 admin(模块级 adminToken 复用会跳过登录,页面仍是匿名态——
  // CI 第四轮菜单断言挂败根因:每个 test 是新 context,会话不跨用例继承)
  adminToken = await loginAs(page, ADMIN, ADMIN_PASS);

  // 创建 user 测试账号(用户名带时间戳,重跑不冲突;自建临时口令)
  const stamp = `${Date.now() % 100000}`;
  const username = `e2e-mtx-${stamp}`;
  const userPass = `Mtx#E2e${stamp}a`;
  const created = await api(page, "POST", "/api/v1/admin/users", {
    username,
    displayName: "矩阵只读用户",
    password: userPass,
    roleCodes: ["user"],
    enabled: true,
  });
  // 后端 response.Created → 201(openapi 原写 200,契约漂移已修)
  expect(created.status, "user 测试账号创建").toBe(201);

  // ── 新 UI admin:菜单含系统管理 + /admin 用户表渲染(页面此刻是 admin 会话) ──
  await page.goto("/");
  await expect(page.locator("[data-test=dashboard]").first()).toBeVisible();
  expect(await page.locator(".el-menu").innerText()).toContain("系统管理");
  await page.locator(".el-menu-item", { hasText: "系统管理" }).click();
  await expect(page.locator("[data-test=admin-users-table]").first()).toBeVisible();
  // 数据行异步加载:先等首行出现,否则 innerText 只抓到表头(flaky 根因)
  await expect(page.locator("[data-test=admin-users-table] tbody tr").first()).toBeVisible({
    timeout: 10000,
  });
  expect(await page.locator("[data-test=admin-users-table]").innerText()).toContain("admin");
  expect(await page.locator("[data-test=admin-users-table]").innerText()).toContain(username);

  // ── user 登录(页面会话切换为 user)──
  const userToken = await loginAs(page, username, userPass);

  // ── API 矩阵(后端裁决,两侧同源) ──
  const adminList = await api(page, "GET", "/api/v1/admin/users");
  expect(adminList.status, "admin GET /admin/users = 200").toBe(200);
  const userList = await api(page, "GET", "/api/v1/admin/users", undefined, userToken);
  expect(userList.status, "user GET /admin/users = 403").toBe(403);

  // ── 新 UI user:菜单无系统管理,直达 /admin 被守卫重定向 ──
  await page.goto("/");
  expect(await page.locator(".el-menu").innerText(), "user 菜单无系统管理").not.toContain(
    "系统管理",
  );
  await page.goto("/admin");
  await page.waitForURL((u) => u.pathname === "/" || u.pathname === "", { timeout: 10000 });
  expect(page.url().includes("/admin"), "user 直达 /admin 不停留").toBe(false);

  // ── 旧 UI user:菜单无系统管理,直达 /admin 重定向 dashboard(旧守卫同口径) ──
  await page.goto(`${OLD}/`);
  await page.evaluate((t) => localStorage.setItem("cabinet_access_token", t), userToken);
  await page.goto(`${OLD}/admin`);
  await page.waitForURL((u) => u.host === OLD_HOST && (u.pathname === "/" || u.pathname === ""), {
    timeout: 15000,
  });
  expect(await page.locator(".el-menu").innerText(), "旧 UI user 菜单无系统管理").not.toContain(
    "系统管理",
  );

  // ── 旧 UI admin:菜单含系统管理(正向对照) ──
  await page.evaluate((t) => localStorage.setItem("cabinet_access_token", t), adminToken);
  await page.goto(`${OLD}/admin`);
  await page.waitForTimeout(2000);
  expect(await page.locator(".el-menu").innerText(), "旧 UI admin 菜单含系统管理").toContain(
    "系统管理",
  );
});
