/**
 * 机柜图 Excel 导出/解析(屏6b;规格取自 v2 RoomScreenView 编译产物行为)。
 *
 * 工作簿两表:
 *  1. "机柜图"(可见):画布。每柜 6 列块(名称/U 标识/设备 3 列合并/U 标识),
 *     排内块距 1 列,排间空 4 行;U 从上到下递减;配色板见 COLORS。
 *  2. "__CABINET_IMPORT_META__"(隐藏):导入记录行 + U1 起的格式元数据
 *     (formatVersion="1"/dataCenterId/roomId/generatedAt,位于第 21 列即 "U" 列)。
 *
 * 解析:只收本系统导出的 .xlsx;读 META 记录还原每柜表头行/首列,再从画布
 * 读设备文本块("名称/SN/功耗W")与单元格填充色(反查分类);产出
 * ImportValidateInput 调后端 validate。
 */
import * as XLSX from "xlsx";
import type { Rack } from "@/features/resource/api";
import type { ULayoutResponse } from "@/features/resource/api";

export const META_SHEET = "__CABINET_IMPORT_META__";
export const FORMAT_VERSION = "1";

const COLORS: Record<string, string> = {
  black: "000000",
  white: "FFFFFF",
  header: "FFC000",
  headerSide: "F4B183",
  uBar: "00E5E5",
  SERVER: "FFFF00",
  NETWORK: "92D050",
  STORAGE: "8EA9DB",
  SECURITY: "F4B183",
  POWER_ENVIRONMENT: "F8696B",
  ACCESSORY: "D9EAD3",
  OTHER: "D9D2E9",
};
/** 画布填充色 → 设备分类(导入解析反查;v2 lu 表) */
const COLOR_TO_CATEGORY: Record<string, string> = {
  FFFF00: "SERVER",
  "92D050": "NETWORK",
  "8EA9DB": "STORAGE",
  F4B183: "SECURITY",
  F8696B: "POWER_ENVIRONMENT",
  D9EAD3: "ACCESSORY",
  D9D2E9: "OTHER",
};

const U_BLOCK = 6; // 每柜列块宽
const BLOCK_GAP = 1; // 排内块间距
const START_ROW = 1; // 画布起始行(0-based)
const START_COL = 1; // 画布起始列
const MAX_PER_ROW = 20;

const THIN_BLACK = {
  style: "thin",
  color: { rgb: COLORS.black },
};
const BORDER = {
  top: THIN_BLACK,
  bottom: THIN_BLACK,
  left: THIN_BLACK,
  right: THIN_BLACK,
};

function fill(rgb: string) {
  return { patternType: "solid", fgColor: { rgb } };
}
function baseStyle(rgb = COLORS.white) {
  return {
    font: { name: "微软雅黑", sz: 10, color: { rgb: COLORS.black } },
    alignment: { horizontal: "center", vertical: "center", wrapText: true },
    border: BORDER,
    fill: fill(rgb),
  };
}
function put(sheet: XLSX.WorkSheet, r: number, c: number, v: string | number, s?: object) {
  sheet[XLSX.utils.encode_cell({ r, c })] = {
    t: typeof v === "number" ? "n" : "s",
    v,
    s: s ?? baseStyle(),
  };
}
function merge(sheet: XLSX.WorkSheet, r1: number, c1: number, r2: number, c2: number) {
  (sheet["!merges"] ??= []).push({ s: { r: r1, c: c1 }, e: { r: r2, c: c2 } });
}
function clampUHeight(rack: Pick<Rack, "uHeight">): number {
  const n = Math.round(Number(rack.uHeight));
  return Number.isFinite(n) ? Math.max(1, n) : 42;
}
function clampRange(
  p: { startU: number; endU?: number; heightU?: number },
  uHeight: number,
): { startU: number; endU: number } {
  const start = Math.max(1, Math.min(uHeight, Math.round(Number(p.startU))));
  const rawEnd = Number(p.endU) || start + Math.max(1, Number(p.heightU)) - 1;
  const end = Math.max(start, Math.min(uHeight, Math.round(rawEnd)));
  return { startU: start, endU: end };
}
function categoryColor(category: string | null | undefined): string {
  return COLORS[category ?? "OTHER"] ?? COLORS.OTHER;
}
function rackTitle(rack: Rack): string {
  const code = rack.code?.trim();
  const name = rack.name?.trim();
  return code && name && code !== name ? `${code}  ${name}` : code || name || "未命名机柜";
}
function deviceText(d: {
  name?: string;
  serialNumber?: string | null;
  ratedPowerW?: number | null;
}): string {
  const parts = [
    d.name?.trim() || "未命名设备",
    d.serialNumber?.trim(),
    Number(d.ratedPowerW) > 0 ? `${Number(d.ratedPowerW!.toFixed(2))} W` : "",
  ].filter(Boolean);
  return parts.join("/");
}
function fileNameSafe(s: string): string {
  // 文件名非法字符(含控制字符)替换为"-";不用正则避免 no-control-regex
  const cleaned = Array.from(s)
    .map((ch) => {
      const code = ch.charCodeAt(0);
      if (code < 0x20 || '<>:"/\\|?*'.includes(ch)) return "-";
      return ch;
    })
    .join("");
  return cleaned.replace(/[. ]+$/g, "").trim() || "机柜图";
}

interface DrawResult {
  sheet: XLSX.WorkSheet;
  records: Record<string, unknown>[];
  maxRow: number;
  maxCol: number;
}

/** 画单个机柜块,返回块底行 */
/** 画布构造(v2 La 同规格) */
function buildCanvas(input: {
  dataCenter: { id?: string; code?: string; name?: string };
  room: { id?: string; code?: string; name?: string; racksPerRow?: number };
  racks: Rack[];
  layouts: Record<string, ULayoutResponse | undefined>;
  generatedAt: Date;
}): DrawResult {
  const sheet: XLSX.WorkSheet = {};
  const records: Record<string, unknown>[] = [];
  const racks = input.racks ?? [];
  const perRowRaw = Number(input.room.racksPerRow);
  const perRow = Number.isFinite(perRowRaw)
    ? Math.min(MAX_PER_ROW, Math.max(1, Math.round(perRowRaw)))
    : Math.min(8, Math.max(1, racks.length));
  let top = START_ROW;
  let maxRow = START_ROW;
  let maxCol = START_COL;

  for (let y = 0; y < racks.length; y += perRow) {
    const rowRacks = racks.slice(y, y + perRow);
    const rowMaxU = Math.max(...rowRacks.map(clampUHeight));
    rowRacks.forEach((rack, idx) => {
      const layout = input.layouts[rack.id ?? ""];
      const uHeight = clampUHeight(rack);
      const cName = START_COL + idx * (U_BLOCK + BLOCK_GAP); // 名称列
      const cU1 = cName + 1;
      const cDev = cName + 2; // 设备 3 列起点
      const cDevEnd = cName + 4;
      const cU2 = cName + 5;

      records.push({
        recordType: "RACK",
        rackId: rack.id,
        rackVersion: rack.version,
        rackCode: rack.code,
        rackName: rack.name,
        uHeight,
        headerRow: top + 1,
        firstColumn: cName + 1,
      });

      // 表头行:两侧 headerSide、设备 3 列 header 橙黄 + 标题合并
      for (let c = cName; c <= cU2; c += 1) {
        const bg = c >= cDev && c <= cDevEnd ? COLORS.header : COLORS.headerSide;
        put(sheet, top, c, "", {
          ...baseStyle(bg),
          font: {
            name: "微软雅黑",
            sz: 11,
            bold: true,
            italic: true,
            color: { rgb: COLORS.black },
          },
        });
      }
      put(sheet, top, cDev, rackTitle(rack), {
        ...baseStyle(COLORS.header),
        font: { name: "微软雅黑", sz: 11, bold: true, italic: true, color: { rgb: COLORS.black } },
      });
      merge(sheet, top, cDev, top, cDevEnd);

      // U 行:从上到下递减
      const positions = (layout?.devices ?? []).map((d) => ({
        startU: d.startU,
        endU: d.endU,
        heightU: d.heightU,
        device: d,
      }));
      const byU = new Map<number, (typeof positions)[number]>();
      for (const p of positions) {
        const { startU, endU } = clampRange(p, uHeight);
        for (let u = startU; u <= endU; u += 1) {
          if (!byU.has(u)) byU.set(u, { ...p, startU, endU });
        }
      }
      for (let u = uHeight; u >= 1; u -= 1) {
        const row = top + 1 + (uHeight - u);
        const hit = byU.get(u);
        put(sheet, row, cName, u, baseStyle());
        put(sheet, row, cU1, "1U", baseStyle(COLORS.uBar));
        for (let c = cDev; c <= cDevEnd; c += 1) {
          put(
            sheet,
            row,
            c,
            "",
            baseStyle(hit ? categoryColor(hit.device.category) : COLORS.white),
          );
        }
        put(sheet, row, cU2, "", baseStyle(COLORS.uBar));
        if (!hit) {
          merge(sheet, row, cDev, row, cDevEnd);
          continue;
        }
        if (u !== hit.endU) continue; // 只在设备顶端行写文本并纵跨合并
        const rowTop = top + 1 + (uHeight - hit.endU);
        const rowBottom = top + 1 + (uHeight - hit.startU);
        put(sheet, rowTop, cDev, deviceText(hit.device), {
          ...baseStyle(categoryColor(hit.device.category)),
          font: { name: "微软雅黑", sz: 10, italic: true, color: { rgb: COLORS.black } },
        });
        merge(sheet, rowTop, cDev, rowBottom, cDevEnd);
        records.push({
          recordType: "DEVICE",
          rackId: rack.id,
          rackVersion: rack.version,
          rackCode: rack.code,
          rackName: rack.name,
          uHeight,
          headerRow: top + 1,
          firstColumn: cName + 1,
          deviceId: hit.device.id,
          deviceVersion: hit.device.version,
          deviceCode: hit.device.code,
          deviceName: hit.device.name,
          serialNumber: hit.device.serialNumber,
          ratedPowerW: hit.device.ratedPowerW,
          typeId: hit.device.typeId,
          typeCategory: hit.device.category,
          startU: hit.startU,
          endU: hit.endU,
          cellAddress: XLSX.utils.encode_cell({ r: rowTop, c: cDev }),
        });
      }
      // 封底
      const bottom = top + uHeight + 1;
      for (let c = cName; c <= cU2; c += 1) put(sheet, bottom, c, "", baseStyle(COLORS.uBar));
      merge(sheet, bottom, cU1, bottom, cU2);

      maxRow = Math.max(maxRow, bottom);
      maxCol = Math.max(maxCol, cU2);
    });
    top += rowMaxU + 4;
  }
  if (!racks.length) {
    put(sheet, START_ROW, START_COL, "当前机房暂无机柜", baseStyle(COLORS.headerSide));
    maxRow = START_ROW;
    maxCol = START_COL;
  }
  return { sheet, records, maxRow, maxCol };
}

/** 导出机柜图 xlsx 并触发下载;返回文件名 */
export function exportRackDiagram(input: {
  dataCenter: { id?: string; code?: string; name?: string };
  room: { id?: string; code?: string; name?: string; racksPerRow?: number };
  racks: Rack[];
  layouts: Record<string, ULayoutResponse | undefined>;
  generatedAt?: Date;
}): string {
  const generatedAt = input.generatedAt ?? new Date();
  const wb = XLSX.utils.book_new();
  const { sheet, records, maxRow, maxCol } = buildCanvas({ ...input, generatedAt });
  sheet["!ref"] = XLSX.utils.encode_range({ s: { r: 0, c: 0 }, e: { r: maxRow, c: maxCol } });
  const cols = maxCol + 1;
  sheet["!cols"] = Array.from({ length: cols }, (_, c) => {
    if (c < START_COL) return { wch: 2 };
    const w = (c - START_COL) % (U_BLOCK + BLOCK_GAP);
    if (w === U_BLOCK) return { wch: 4 };
    if (w === 0 || w === 1 || w === 5) return { wch: 5 };
    return { wch: 14 };
  });
  sheet["!rows"] = Array.from({ length: maxRow + 1 }, (_, r) => ({
    hpt: r === START_ROW ? 24 : 18,
  }));
  sheet["!margins"] = {
    left: 0.25,
    right: 0.25,
    top: 0.35,
    bottom: 0.35,
    header: 0.1,
    footer: 0.1,
  };
  sheet["!pageSetup"] = { orientation: "landscape", fitToWidth: 1, fitToHeight: 0, paperSize: 9 };
  wb.Props = {
    Title: `${input.dataCenter.name}-${input.room.name}-机柜图`,
    Subject: "机柜 U 位与设备占用图",
    Author: "机柜管理工具",
    CreatedDate: generatedAt,
  };
  XLSX.utils.book_append_sheet(wb, sheet, "机柜图");

  const meta = XLSX.utils.json_to_sheet(records, {
    header: [
      "recordType",
      "rackId",
      "rackVersion",
      "rackCode",
      "rackName",
      "uHeight",
      "headerRow",
      "firstColumn",
      "deviceId",
      "deviceVersion",
      "deviceCode",
      "deviceName",
      "serialNumber",
      "ratedPowerW",
      "typeId",
      "typeCategory",
      "startU",
      "endU",
      "cellAddress",
    ],
  });
  XLSX.utils.sheet_add_aoa(
    meta,
    [
      ["formatVersion", FORMAT_VERSION],
      ["dataCenterId", input.dataCenter.id],
      ["roomId", input.room.id],
      ["generatedAt", generatedAt.toISOString()],
    ],
    { origin: "U1" },
  );
  XLSX.utils.book_append_sheet(wb, meta, META_SHEET);
  wb.Workbook = {
    ...wb.Workbook,
    Sheets: wb.SheetNames.map((n) => ({ name: n, Hidden: n === META_SHEET ? 2 : 0 })),
  };

  const stamp = (d: Date) => {
    const p = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`;
  };
  const name = `${fileNameSafe(
    `${input.dataCenter.code || input.dataCenter.name}-${input.room.code || input.room.name}-机柜图-${stamp(generatedAt)}`,
  )}.xlsx`;
  const data = XLSX.write(wb, { bookType: "xlsx", type: "array", cellStyles: true });
  const blob = new Blob([data], {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.style.display = "none";
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
  return name;
}

/* ── 导入解析 ── */
export interface ParsedDiagramRow {
  clientId: string;
  rackId: string;
  rackCode: string;
  startU: number;
  endU: number;
  name: string;
  serialNumber?: string;
  ratedPowerW?: number;
  typeId?: string;
  typeCategory?: string;
  sourceDeviceId?: string;
  sourceDeviceVersion?: number;
  sourceDeviceCode?: string;
  sourceRackId?: string;
  sourceStartU?: number;
  sourceEndU?: number;
}
export interface ParsedDiagram {
  formatVersion: string;
  dataCenterId: string;
  roomId: string;
  exportedAt?: string;
  coveredRackIds: string[];
  devices: ParsedDiagramRow[];
}

function text(v: unknown): string {
  return String(v ?? "").trim();
}
function num(v: unknown): number | undefined {
  if (v === null || v === undefined || text(v) === "") return;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
}
/** META 元数据行位于第 21 列(U 列) */
function metaCell(sheet: XLSX.WorkSheet, row: number): string {
  return text(sheet[XLSX.utils.encode_cell({ r: row, c: 21 })]?.v);
}
function cellFillColor(sheet: XLSX.WorkSheet, r: number, c: number): string {
  const s = (
    sheet[XLSX.utils.encode_cell({ r, c })] as {
      s?: { fill?: { fgColor?: { rgb?: string } }; fgColor?: { rgb?: string } };
    }
  )?.s;
  const rgb = s?.fill?.fgColor?.rgb ?? s?.fgColor?.rgb ?? "";
  return String(rgb)
    .replace(/^FF(?=[0-9A-F]{6}$)/i, "")
    .toUpperCase();
}
function findMerge(
  sheet: XLSX.WorkSheet,
  r: number,
  c: number,
): { s: { r: number; c: number }; e: { r: number; c: number } } | undefined {
  return ((sheet["!merges"] as unknown[]) ?? []).find(
    (m): m is { s: { r: number; c: number }; e: { r: number; c: number } } =>
      (m as { s: { r: number } }).s.r <= r &&
      r <= (m as { e: { r: number } }).e.r &&
      (m as { s: { c: number } }).s.c <= c &&
      c <= (m as { e: { c: number } }).e.c,
  );
}
/** 设备文本"名称/SN/功耗W" → 结构化(画布文本解析兜底) */
function parseDeviceText(t: string, fallbackCategory?: string): ParsedDiagramRow {
  const parts = t
    .split("/")
    .map((s) => s.trim())
    .filter(Boolean);
  let ratedPowerW: number | undefined;
  const last = parts[parts.length - 1];
  const pw = last?.match(/^([0-9]+(?:\.[0-9]+)?)\s*[wW]$/);
  if (pw) {
    ratedPowerW = Number(pw[1]);
    parts.pop();
  }
  let serialNumber = "";
  if (parts.length > 1) serialNumber = parts.pop() ?? "";
  return {
    clientId: "",
    rackId: "",
    rackCode: "",
    startU: 0,
    endU: 0,
    name: parts.join("/").trim(),
    serialNumber: serialNumber || undefined,
    ratedPowerW,
    typeCategory: fallbackCategory,
  };
}

/** 解析机柜图 xlsx → validate 输入(不调 API) */
export async function parseRackDiagram(
  file: File,
  expectRoomId: string,
  expectDcId: string,
): Promise<ParsedDiagram> {
  if (!/\.xlsx$/i.test(file.name)) {
    throw new Error("请选择由“导出机柜图”生成的 .xlsx 文件");
  }
  const wb = XLSX.read(await file.arrayBuffer(), { type: "array", cellStyles: true });
  const canvas = wb.Sheets["机柜图"];
  const meta = wb.Sheets[META_SHEET];
  if (!canvas || !meta) {
    throw new Error("文件缺少机柜图或系统元数据，必须使用本系统导出的机柜图");
  }
  const formatVersion = metaCell(meta, 0);
  const dataCenterId = metaCell(meta, 1);
  const roomId = metaCell(meta, 2);
  const exportedAt = metaCell(meta, 3) || undefined;
  if (formatVersion !== FORMAT_VERSION) {
    throw new Error(`不支持的机柜图版本：${formatVersion || "未知"}`);
  }
  if (roomId !== expectRoomId || dataCenterId !== expectDcId) {
    throw new Error("机柜图所属数据中心或机房与当前选择不一致");
  }
  // META 记录行:json_to_sheet 表头在第 0 行,数据从第 1 行起;列序同 header 数组
  const headerRow: string[] = [];
  for (let c = 0; ; c += 1) {
    const v = text(meta[XLSX.utils.encode_cell({ r: 0, c })]?.v);
    if (!v) break;
    headerRow.push(v);
  }
  const records: Record<string, unknown>[] = [];
  for (let r = 1; ; r += 1) {
    const first = meta[XLSX.utils.encode_cell({ r, c: 0 })]?.v;
    if (first === undefined) break;
    const rec: Record<string, unknown> = {};
    headerRow.forEach((key, c) => {
      rec[key] = meta[XLSX.utils.encode_cell({ r, c })]?.v;
    });
    records.push(rec);
    void first;
  }
  const rackRecords = records.filter((r) => r.recordType === "RACK");
  if (!rackRecords.length) {
    throw new Error("机柜图元数据中没有机柜记录");
  }
  const deviceBySource = new Map(
    records
      .filter((r) => r.recordType === "DEVICE" && r.sourceDeviceId)
      .map((r) => [text(r.sourceDeviceId), r]),
  );
  const devices: ParsedDiagramRow[] = [];
  rackRecords.forEach((rk, idx) => {
    const rackId = text(rk.rackId);
    const rackCode = text(rk.rackCode);
    const uHeight = Math.max(1, Math.round(Number(rk.uHeight)));
    const headerRowN = Number(rk.headerRow) - 1; // 0-based
    const firstColumnN = Number(rk.firstColumn) - 1;
    const cDev = firstColumnN + 2;
    for (let u = uHeight; u >= 1; u -= 1) {
      const r = headerRowN + 1 + (uHeight - u);
      const cellRef = XLSX.utils.encode_cell({ r, c: cDev });
      const name = text(canvas[cellRef]?.v);
      if (!name) continue;
      // 合并块:找该单元格所在合并,块顶即设备顶端行
      const m = findMerge(canvas, r, cDev);
      // Excel 行从上到下 U 递减:块顶行 = endU,块底行 = startU
      const topR = m?.s.r ?? r;
      const bottomR = m?.e.r ?? r;
      const endU = uHeight - (topR - headerRowN - 1);
      const startU = uHeight - (bottomR - headerRowN - 1);
      const category = COLOR_TO_CATEGORY[cellFillColor(canvas, r, cDev)];
      // 优先 META 记录(原设备回导),否则解析画布文本
      const byCell = records.find(
        (rec) => rec.recordType === "DEVICE" && text(rec.cellAddress) === cellRef,
      );
      const src = byCell?.sourceDeviceId
        ? deviceBySource.get(text(byCell.sourceDeviceId))
        : undefined;
      const base = src
        ? {
            name: text(src.deviceName),
            serialNumber: text(src.serialNumber) || undefined,
            ratedPowerW: num(src.ratedPowerW),
          }
        : parseDeviceText(name, category);
      devices.push({
        clientId: `${rackId}:${startU}:${endU}:${devices.length}`,
        rackId,
        rackCode,
        startU,
        endU,
        name: base.name,
        serialNumber: base.serialNumber,
        ratedPowerW: base.ratedPowerW,
        typeId: (src?.typeId as string) ?? undefined,
        typeCategory: (byCell?.typeCategory as string) ?? category,
        sourceDeviceId: text(byCell?.sourceDeviceId) || undefined,
        sourceDeviceVersion: num(byCell?.deviceVersion),
        sourceDeviceCode: text(byCell?.deviceCode) || undefined,
        sourceRackId: rackId,
        sourceStartU: startU,
        sourceEndU: endU,
      });
      void idx;
    }
  });
  return {
    formatVersion,
    dataCenterId,
    roomId,
    exportedAt,
    coveredRackIds: rackRecords.map((r) => text(r.rackId)),
    devices,
  };
}
