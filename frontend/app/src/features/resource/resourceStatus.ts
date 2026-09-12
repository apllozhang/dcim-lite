/**
 * 资源状态显示映射(第 6 轮资源层级页;口径复刻 v2 bundle ResourceManagementView):
 * 数据中心/机房(OPERATING/RUNNING→运行中,PLANNING→规划中,MAINTENANCE→维护中,
 * DISABLED→停用,ARCHIVED→归档)与机柜(AVAILABLE→空闲,PARTIAL→部分使用,
 * FULL→已满,其余同上)分开映射;tag 颜色同 v2。
 */
export interface StatusView {
  label: string;
  tag: "success" | "warning" | "danger" | "info" | "primary";
}

const DC_ROOM: Record<string, StatusView> = {
  OPERATING: { label: "运行中", tag: "success" },
  RUNNING: { label: "运行中", tag: "success" },
  PLANNING: { label: "规划中", tag: "info" },
  MAINTENANCE: { label: "维护中", tag: "warning" },
  DISABLED: { label: "停用", tag: "info" },
  ARCHIVED: { label: "归档", tag: "info" },
};

const RACK: Record<string, StatusView> = {
  AVAILABLE: { label: "空闲", tag: "success" },
  PARTIAL: { label: "部分使用", tag: "warning" },
  FULL: { label: "已满", tag: "danger" },
  PLANNING: { label: "规划中", tag: "info" },
  MAINTENANCE: { label: "维护中", tag: "warning" },
  DISABLED: { label: "停用", tag: "info" },
};

/** 数据中心/机房状态视图;树接口未携带状态时按 v2 显示"运行中" */
export function dcRoomStatus(status: string | undefined | null): StatusView {
  return (status && DC_ROOM[status]) || { label: "运行中", tag: "success" };
}

/** 机柜状态视图;缺省按空闲 */
export function rackStatus(status: string | undefined | null): StatusView {
  return (status && RACK[status]) || { label: "空闲", tag: "success" };
}
