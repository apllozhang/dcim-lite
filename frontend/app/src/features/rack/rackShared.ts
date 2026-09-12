/**
 * 机柜两屏(屏4 机柜管理 / 屏5 机柜模板)共享的类型与口径。
 * 全部逐项取自 v2 RackManagementView/RackTemplateView/rackDefaults 编译产物;
 * 差异点(后端契约如实化)集中在本文件,不散落组件:
 *   - 树/详情返回 rackRow/rackColumn,表单沿用 v2 命名 row/column,保存时映射;
 *   - v2 建柜传 templateVersionId,dcim-lite 后端按 templateId 取当前版本;
 *   - v2 依赖服务端 autoGenerateCode 生成编码,后端无此能力,改为保存时前端生成。
 */
import type { Rack, RackTemplate } from "@/features/resource/api";

export const RACK_STATUS_OPTIONS = [
  { label: "规划中", value: "PLANNING" },
  { label: "空闲", value: "AVAILABLE" },
  { label: "部分使用", value: "PARTIAL" },
  { label: "已满", value: "FULL" },
  { label: "维护中", value: "MAINTENANCE" },
  { label: "停用", value: "DISABLED" },
] as const;

export function rackStatusLabel(status: string): string {
  return RACK_STATUS_OPTIONS.find((o) => o.value === status)?.label ?? status;
}

export function rackStatusTagType(status: string): "success" | "warning" | "danger" | "info" {
  if (status === "AVAILABLE") return "success";
  if (status === "PARTIAL" || status === "PLANNING" || status === "MAINTENANCE") return "warning";
  if (status === "FULL" || status === "DISABLED") return "danger";
  return "info";
}

/** 机房内位置副标题:"A区 · 01排 · 01列 · 冷通道1";全空时"未设置机房内位置"(v2 ke 口径) */
export function rackLocationText(r: {
  zone?: string | null;
  rackRow?: string | null;
  rackColumn?: string | null;
  aisle?: string | null;
}): string {
  const parts = [
    r.zone && `${r.zone}区`,
    r.rackRow && `${r.rackRow}排`,
    r.rackColumn && `${r.rackColumn}列`,
    r.aisle && `${r.aisle}通道`,
  ].filter(Boolean);
  return parts.join(" · ") || "未设置机房内位置";
}

/* ── 机柜表单(v2 rackDefaults.createRackForm 同款字段) ── */
export interface RackForm {
  templateId?: string;
  code: string;
  name: string;
  version: number;
  status: string;
  sortOrder: number;
  type: string;
  uHeight: number;
  widthMm: number;
  depthMm: number;
  heightMm: number;
  rotation: number;
  dualPower: boolean;
  inputCircuits: number;
  pduCount: number;
  manufacturer: string;
  modelNumber: string;
  serialNumber: string;
  assetNumber: string;
  zone: string;
  row: string;
  column: string;
  aisle: string;
  manager: string;
  department: string;
  purpose: string;
  remarks: string;
  /** 服务端可空数值:新建时缺省(表单留空),编辑时从机柜对象带出 */
  loadCapacityKg?: number;
  xCoordinate?: number;
  yCoordinate?: number;
  ratedVoltage?: number;
  ratedCurrent?: number;
  ratedPowerKw?: number;
  peakPowerKw?: number;
}

export function rackFormDefaults(): RackForm {
  return {
    templateId: undefined,
    code: "",
    name: "",
    version: 0,
    status: "AVAILABLE",
    sortOrder: 0,
    type: "STANDARD",
    uHeight: 42,
    widthMm: 600,
    depthMm: 1200,
    heightMm: 2000,
    rotation: 0,
    dualPower: false,
    inputCircuits: 0,
    pduCount: 0,
    manufacturer: "",
    modelNumber: "",
    serialNumber: "",
    assetNumber: "",
    zone: "",
    row: "",
    column: "",
    aisle: "",
    manager: "",
    department: "",
    purpose: "",
    remarks: "",
  };
}

/** 模板版本规格(创建模板"初始版本参数"与发布新版本共用;TemplateVersionInput 同构) */
export type TemplateSpec = {
  type: string;
  manufacturer: string;
  modelNumber: string;
  uHeight: number;
  widthMm: number;
  depthMm: number;
  heightMm: number;
  loadCapacityKg?: number;
  dualPower: boolean;
  inputCircuits: number;
  ratedVoltage?: number;
  ratedCurrent?: number;
  ratedPowerKw?: number;
  peakPowerKw?: number;
  pduCount: number;
  changeNote: string;
};

/** 模板最新版本(versions 按 revision 降序取首;v2 rackDefaults.latestVersion 口径) */
export function latestVersion(
  t: RackTemplate,
): NonNullable<RackTemplate["versions"]>[number] | undefined {
  return [...(t.versions ?? [])].sort((a, b) => (b.revision ?? 0) - (a.revision ?? 0))[0];
}

/** 把模板版本规格落到表单(v2 rackDefaults.applyTemplateToForm 口径;空值不覆盖) */
export function applyTemplateToForm(
  form: RackForm,
  v: NonNullable<RackTemplate["versions"]>[number],
) {
  form.type = v.type ?? form.type;
  form.manufacturer = v.manufacturer ?? "";
  form.modelNumber = v.modelNumber ?? "";
  form.uHeight = v.uHeight ?? form.uHeight;
  form.widthMm = v.widthMm ?? form.widthMm;
  form.depthMm = v.depthMm ?? form.depthMm;
  form.heightMm = v.heightMm ?? form.heightMm;
  form.dualPower = v.dualPower ?? form.dualPower;
  form.inputCircuits = v.inputCircuits ?? form.inputCircuits;
  form.pduCount = v.pduCount ?? form.pduCount;
}

/** 启用且有可用版本的模板选项(v2 ve computed 口径) */
export function activeTemplateOptions(templates: RackTemplate[]) {
  return templates
    .filter((t) => t.status === "ACTIVE")
    .map((t) => ({ template: t, version: latestVersion(t) }))
    .filter((o) => !!o.version);
}

/**
 * 客户端自动编码:dcim-lite 后端无 autoGenerateCode(v2 为服务端生成),
 * 保存时前端生成,前缀区分域,base36 时间戳保证短且不重复。
 */
export function autoCode(prefix: string): string {
  return `${prefix}-${Date.now().toString(36).toUpperCase()}`;
}

/** 编辑回填:服务端 Rack → 表单(剔除关联字段,rackRow/rackColumn 映射回 row/column;v2 ce 口径) */
export function rackToForm(r: Rack): RackForm {
  const base = rackFormDefaults();
  const num = (v: number | null | undefined): number | undefined =>
    v === null || v === undefined ? undefined : v;
  return {
    ...base,
    code: r.code ?? "",
    name: r.name ?? "",
    version: r.version ?? 0,
    status: r.status ?? base.status,
    sortOrder: r.sortOrder ?? 0,
    type: r.type ?? base.type,
    uHeight: r.uHeight ?? base.uHeight,
    widthMm: r.widthMm ?? base.widthMm,
    depthMm: r.depthMm ?? base.depthMm,
    heightMm: r.heightMm ?? base.heightMm,
    rotation: r.rotation ?? 0,
    dualPower: r.dualPower ?? false,
    inputCircuits: r.inputCircuits ?? 0,
    pduCount: r.pduCount ?? 0,
    manufacturer: r.manufacturer ?? "",
    modelNumber: r.modelNumber ?? "",
    serialNumber: r.serialNumber ?? "",
    assetNumber: r.assetNumber ?? "",
    zone: r.zone ?? "",
    row: r.rackRow ?? "",
    column: r.rackColumn ?? "",
    aisle: r.aisle ?? "",
    manager: r.manager ?? "",
    department: r.department ?? "",
    purpose: r.purpose ?? "",
    remarks: r.remarks ?? "",
    loadCapacityKg: num(r.loadCapacityKg),
    xCoordinate: num(r.xCoordinate),
    yCoordinate: num(r.yCoordinate),
    ratedVoltage: num(r.ratedVoltage),
    ratedCurrent: num(r.ratedCurrent),
    ratedPowerKw: num(r.ratedPowerKw),
    peakPowerKw: num(r.peakPowerKw),
  };
}

/** 保存载荷:表单 → 后端 RackInput(code 由调用方决定;row/column 映射为 rackRow/rackColumn) */
export function formToRackPayload(form: RackForm, code: string): Record<string, unknown> {
  return {
    code,
    name: form.name,
    status: form.status,
    type: form.type,
    uHeight: form.uHeight,
    widthMm: form.widthMm,
    depthMm: form.depthMm,
    heightMm: form.heightMm,
    loadCapacityKg: form.loadCapacityKg,
    manufacturer: form.manufacturer,
    modelNumber: form.modelNumber,
    serialNumber: form.serialNumber,
    assetNumber: form.assetNumber,
    zone: form.zone,
    rackRow: form.row,
    rackColumn: form.column,
    aisle: form.aisle,
    xCoordinate: form.xCoordinate,
    yCoordinate: form.yCoordinate,
    rotation: form.rotation,
    manager: form.manager,
    department: form.department,
    purpose: form.purpose,
    dualPower: form.dualPower,
    inputCircuits: form.inputCircuits,
    ratedVoltage: form.ratedVoltage,
    ratedCurrent: form.ratedCurrent,
    ratedPowerKw: form.ratedPowerKw,
    peakPowerKw: form.peakPowerKw,
    pduCount: form.pduCount,
    remarks: form.remarks,
    sortOrder: form.sortOrder,
    ...(form.templateId ? { templateId: form.templateId } : {}),
  };
}

/** 错误消息提取(catch 子句统一 unknown;与其它屏同口径) */
export function rackErrMsg(e: unknown): string {
  if (e instanceof Error) return e.message;
  const anyErr = e as { response?: { data?: { message?: string } }; message?: string };
  return anyErr?.response?.data?.message ?? anyErr?.message ?? "操作失败";
}

/** 资源树行(树内机柜 + 补挂 DC/房名;与 v2 flatMap 行同构) */
export type TreeRackRow = Rack & {
  dataCenterName: string;
  roomName: string;
  dataCenterStatus?: string;
  roomStatus?: string;
};
