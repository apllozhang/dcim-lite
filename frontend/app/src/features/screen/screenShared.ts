/**
 * 机房大屏(屏6)共享常量与口径,全部取自 v2 RoomScreenView 编译产物。
 * 布局常量:it=9(U 格像素)/Fl=218(卡宽)/tl=246(列距)/ll=38(左边距)/
 * At=72(顶边距)/Ol=32(行距)/zu=152(卡片固定段高)/Na=20(每排上限)。
 */
import type { Rack } from "@/features/resource/api";

export const U_PX = 9;
export const RACK_W = 218;
export const COL_PITCH = 246;
export const X_MARGIN = 38;
export const ROW_TOP = 72;
export const ROW_GAP = 32;
export const CARD_FIXED_H = 152;
export const MAX_PER_ROW = 20;

export const STATUS_LEGENDS = [
  { label: "空闲", value: "AVAILABLE", color: "#24d6a1" },
  { label: "部分使用", value: "PARTIAL", color: "#2dc8f0" },
  { label: "已满", value: "FULL", color: "#ffb547" },
  { label: "维护中", value: "MAINTENANCE", color: "#a78bfa" },
  { label: "规划中", value: "PLANNING", color: "#7188a8" },
  { label: "停用", value: "DISABLED", color: "#ef6470" },
] as const;

export const DEVICE_LEGENDS = [
  { label: "服务器", value: "SERVER", color: "#2d8cff" },
  { label: "网络设备", value: "NETWORK", color: "#18c7a2" },
  { label: "存储设备", value: "STORAGE", color: "#a56eff" },
  { label: "安全设备", value: "SECURITY", color: "#ff9f43" },
  { label: "动力环境", value: "POWER_ENVIRONMENT", color: "#ef6470" },
  { label: "其他设备", value: "OTHER", color: "#5f86a5" },
] as const;

export function statusLabel(s: string): string {
  return STATUS_LEGENDS.find((o) => o.value === s)?.label ?? s;
}
export function statusColor(s: string): string {
  return STATUS_LEGENDS.find((o) => o.value === s)?.color ?? "#7bdfff";
}
export function deviceColor(category: string | null | undefined): string {
  return DEVICE_LEGENDS.find((o) => o.value === category)?.color ?? "#5f86a5";
}
export function categoryLabel(category: string | null | undefined): string {
  if (category === "ACCESSORY") return "附件";
  return DEVICE_LEGENDS.find((o) => o.value === category)?.label ?? "其他设备";
}
/** 设备块分类 class:category-power-environment 等(v2 Xn 口径) */
export function deviceCategoryClass(category: string | null | undefined): string {
  const c = (category ?? "OTHER").toLowerCase().replace("_", "-");
  return `category-${c}`;
}
/** 卡内短名:>9 字截断(v2 Zn 口径) */
export function shortName(name: string | null | undefined, max = 9): string {
  const n = name ?? "";
  return n.length > max ? `${n.slice(0, max - 1)} …` : n;
}
export function rackCardHeight(rack: Pick<Rack, "uHeight">): number {
  return CARD_FIXED_H + (rack.uHeight ?? 0) * U_PX;
}
/** 位置文案:区域/排/列/通道,"自动排布"兜底(v2 oe 口径) */
export function rackPositionText(r: {
  zone?: string | null;
  rackRow?: string | null;
  rackColumn?: string | null;
  aisle?: string | null;
}): string {
  const parts = [
    r.zone,
    r.rackRow && `${r.rackRow}排`,
    r.rackColumn && `${r.rackColumn}列`,
    r.aisle,
  ].filter(Boolean);
  return parts.join(" · ") || "自动排布";
}
/** 卡片画布坐标:有有限 xCoordinate/yCoordinate 用持久值,否则行主序自动排布 */
export interface SlotPos {
  row: number;
  column: number;
}
export function autoSlot(index: number, perRow: number): SlotPos {
  const i = Math.max(0, index);
  return { row: Math.floor(i / perRow), column: i % perRow };
}
export function slotX(pos: SlotPos): number {
  return X_MARGIN + pos.column * COL_PITCH;
}
/** 行顶累计:首排 = At,后续 = 上一排顶 + 上一排最高卡 + 行距(v2 Je 口径) */
export function rowTops(maxHeights: number[]): number[] {
  const tops: number[] = [];
  let top = ROW_TOP;
  maxHeights.forEach((h) => {
    tops.push(top);
    top += h + ROW_GAP;
  });
  return tops;
}
export const CLOCK_WEEKDAYS = "日一二三四五六";

/** 从后端对象读取契约 schema 未声明但实际返回的扩展字段(与设备屏 ext 同口径) */
export function ext<T>(obj: unknown, key: string): T | undefined {
  return (obj as Record<string, unknown> | null)?.[key] as T | undefined;
}
