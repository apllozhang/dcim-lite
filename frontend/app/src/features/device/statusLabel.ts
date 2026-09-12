/**
 * 设备生命周期状态的显示映射(第 5 轮 P1-D)。
 * 口径复刻旧 ALE bundle(DeviceManagementView zl/Rt):
 * 显示中文名 + tag 颜色(RUNNING=success,MAINTENANCE/PENDING_REMOVAL=warning,
 * SCRAPPED=danger,其余 info)——新旧双跑差分的状态列文本口径以此为准。
 */
export const LIFECYCLE_STATUS_OPTIONS = [
  { value: "WAITING_RACK", label: "待上架" },
  { value: "RUNNING", label: "运行中" },
  { value: "MAINTENANCE", label: "维护中" },
  { value: "PENDING_REMOVAL", label: "待下架" },
  { value: "OFF_RACK", label: "已下架" },
  { value: "SCRAPPED", label: "已报废" },
] as const;

export type LifecycleStatus = (typeof LIFECYCLE_STATUS_OPTIONS)[number]["value"];

const LABEL: Record<string, string> = Object.fromEntries(
  LIFECYCLE_STATUS_OPTIONS.map((o) => [o.value, o.label]),
);

/** 枚举值 → 中文显示名(未知值原样返回,不吞数据) */
export function lifecycleLabel(v: string | undefined | null): string {
  return v ? (LABEL[v] ?? v) : "";
}

/** 枚举值 → el-tag type(旧 UI Rt 口径) */
export function lifecycleTagType(
  v: string | undefined | null,
): "success" | "warning" | "danger" | "info" {
  if (v === "RUNNING") return "success";
  if (v === "MAINTENANCE" || v === "PENDING_REMOVAL") return "warning";
  if (v === "SCRAPPED") return "danger";
  return "info";
}
