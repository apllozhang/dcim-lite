/**
 * 设备信息导出(第 7 轮 UI-P0-02;列集合/样式/命名复刻 v2 DeviceManagementView
 * Hl/Jl 导出规格):43 列表头,设备类型/分类显示名,位置三段来自 currentPosition
 * 与资源树反查;当前筛选口径由调用方传入已过滤的全量设备。
 * xlsx 库按需动态加载,不进设备页主 chunk。
 */
import type { Device } from "@/features/resource/api";
import { lifecycleLabel } from "@/features/device/statusLabel";

/** 设备分类 → 中文(v2 Gl 表) */
export const CATEGORY_CN: Record<string, string> = {
  SERVER: "服务器",
  NETWORK: "网络设备",
  STORAGE: "存储设备",
  SECURITY: "安全设备",
  POWER_ENVIRONMENT: "配电与环境设备",
  ACCESSORY: "机柜配件",
  OTHER: "其他",
};

/** rackId → {数据中心, 机房}(资源树反查;v2 Kl 同口径) */
export type RackIndex = Map<string, { dcName: string; roomName: string }>;

export function buildRackIndex(tree: unknown[]): RackIndex {
  const idx: RackIndex = new Map();
  for (const dc of tree as {
    name?: string;
    rooms?: { name?: string; racks?: { id?: string }[] }[];
  }[]) {
    for (const room of dc.rooms ?? []) {
      for (const rack of room.racks ?? []) {
        if (rack.id) idx.set(rack.id, { dcName: dc.name ?? "", roomName: room.name ?? "" });
      }
    }
  }
  return idx;
}

/** v2 Hl 导出表头(43 列) */
export const EXPORT_HEADERS = [
  "设备编码",
  "设备名称",
  "设备类型",
  "设备分类",
  "生命周期状态",
  "资产编号",
  "序列号",
  "厂商",
  "型号",
  "规格",
  "固件版本",
  "采购批次",
  "保修到期日",
  "设备高度U",
  "宽度mm",
  "深度mm",
  "高度mm",
  "重量kg",
  "额定功耗W",
  "峰值功耗W",
  "输入电压V",
  "是否双路供电",
  "管理IP",
  "业务IP",
  "MAC地址",
  "管理协议",
  "监控状态",
  "外部二维码",
  "二维码链接",
  "标签",
  "组织",
  "负责人",
  "联系电话",
  "业务系统",
  "应用名称",
  "数据中心",
  "机房",
  "机柜编码",
  "机柜名称",
  "起始U",
  "结束U",
  "上架状态",
  "备注",
];

type Rec = Record<string, unknown>;
const s = (v: unknown): string => (v === null || v === undefined ? "" : String(v).trim());
const n = (v: unknown): string | number =>
  v === null || v === undefined || v === "" || !Number.isFinite(Number(v)) ? "" : Number(v);

export interface DeviceTypeLite {
  id?: string | null;
  name?: string | null;
  category?: string | null;
}

/** 单台设备 → 43 列行(与 EXPORT_HEADERS 等长;纯函数可单测) */
export function deviceExportRow(
  d: Device,
  rackIndex: RackIndex,
  typeFallback?: DeviceTypeLite,
): (string | number)[] {
  const pos = d.currentPosition as Rec | null | undefined;
  const rack = (pos?.rack ?? null) as Rec | null;
  const loc = rack?.id ? rackIndex.get(String(rack.id)) : undefined;
  const type = (d.type ?? null) as Rec | null;
  const typeName = s(type?.name) || s(typeFallback?.name) || s(d.typeId);
  const categoryRaw = s(type?.category) || s(typeFallback?.category);
  const startU = pos?.startU;
  const endU = pos?.endU;
  return [
    s(d.code),
    s(d.name),
    typeName,
    categoryRaw ? (CATEGORY_CN[categoryRaw] ?? categoryRaw) : "",
    lifecycleLabel(s(d.lifecycleStatus)) || s(d.lifecycleStatus),
    s(d.assetNumber),
    s(d.serialNumber),
    s(d.manufacturer),
    s(d.modelNumber),
    s(d.specification),
    s(d.firmwareVersion),
    s(d.purchaseBatch),
    s(d.warrantyExpiresAt),
    n(d.heightU),
    n(d.widthMm),
    n(d.depthMm),
    n(d.heightMm),
    n(d.weightKg),
    n(d.ratedPowerW),
    n(d.peakPowerW),
    n(d.inputVoltage),
    d.dualPowerRequired ? "是" : "否",
    s(d.managementIp),
    s(d.businessIp),
    s(d.macAddress),
    s(d.managementProtocol),
    s(d.monitoringStatus),
    s(d.externalQrCode),
    s(d.externalQrCodeUrl),
    s(d.tags),
    s(d.organization),
    s(d.manager),
    s(d.contact),
    s(d.businessSystem),
    s(d.applicationName),
    loc?.dcName ?? "",
    loc?.roomName ?? "",
    s(rack?.code),
    s(rack?.name),
    startU === null || startU === undefined ? "" : Number(startU),
    endU === null || endU === undefined ? "" : Number(endU),
    pos ? "已上架" : "未上架",
    s(d.remarks),
  ];
}

/** 全部设备 → AoA(首行表头) */
export function deviceExportAoA(
  devices: Device[],
  rackIndex: RackIndex,
  typeById?: Map<string, DeviceTypeLite>,
): unknown[][] {
  return [
    EXPORT_HEADERS,
    ...devices.map((d) =>
      deviceExportRow(d, rackIndex, d.type ? undefined : typeById?.get(s(d.typeId))),
    ),
  ];
}

/** 导出文件名:v2 Xl 口径——设备信息_时间戳_数量台 */
export function deviceExportFileName(d: Date, count: number): string {
  const p = (x: number) => String(x).padStart(2, "0");
  return `设备信息_${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}_${count}台.xlsx`;
}

const EXPORT_HEADER_STYLES = {
  header: {
    font: { name: "微软雅黑", sz: 10, bold: true, color: { rgb: "FFFFFF" } },
    alignment: { horizontal: "center", vertical: "center", wrapText: true },
    fill: { patternType: "solid", fgColor: { rgb: "1F4E78" } },
  },
  cell: {
    font: { name: "微软雅黑", sz: 10, color: { rgb: "1F2937" } },
    alignment: { vertical: "center", wrapText: true },
  },
} as const;

/** 生成并下载导出 xlsx;返回文件名(v2 Jl 口径) */
export async function downloadDeviceExport(
  devices: Device[],
  rackIndex: RackIndex,
  typeById?: Map<string, DeviceTypeLite>,
): Promise<string> {
  const XLSX = await import("xlsx");
  const aoa = deviceExportAoA(devices, rackIndex, typeById);
  const sheet = XLSX.utils.aoa_to_sheet(aoa);
  const cols = EXPORT_HEADERS.map((h) => ({ wch: Math.max(10, Math.min(28, h.length * 2 + 4)) }));
  sheet["!cols"] = cols;
  const range = XLSX.utils.decode_range(sheet["!ref"] ?? "A1");
  for (let c = range.s.c; c <= range.e.c; c += 1) {
    sheet[XLSX.utils.encode_cell({ r: 0, c })] = {
      ...(sheet[XLSX.utils.encode_cell({ r: 0, c })] ?? { v: "", t: "s" }),
      s: EXPORT_HEADER_STYLES.header,
    };
  }
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, sheet, "设备信息");
  const buf = XLSX.write(wb, { bookType: "xlsx", type: "array", cellStyles: true });
  const blob = new Blob([buf], {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
  const fileName = deviceExportFileName(new Date(), devices.length);
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = fileName;
  a.style.display = "none";
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
  return fileName;
}
