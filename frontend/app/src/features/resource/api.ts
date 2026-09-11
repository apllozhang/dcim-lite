/**
 * 资源树与设备列表的只读查询(Phase 1 切片:无写操作)。
 * 类型从生成 schema 派生;el-tree 数据规整在此层完成(组件不做数据变换)。
 */
import type { components } from "@/api/generated/schema";
import { api, unwrapData } from "@/api/client";

export type TreeDataCenter = components["schemas"]["TreeDataCenter"];
export type Device = components["schemas"]["Device"];

/** el-tree 节点:两级子字段(rooms/racks)规整为统一 children */
export interface TreeNode {
  id: string;
  label: string;
  code: string;
  kind: "dc" | "room" | "rack";
  children?: TreeNode[];
}

export function toTreeNodes(tree: TreeDataCenter[]): TreeNode[] {
  return tree.map((dc) => ({
    id: dc.id ?? "",
    label: dc.name ?? dc.code ?? "",
    code: dc.code ?? "",
    kind: "dc" as const,
    children: (dc.rooms ?? []).map((room) => ({
      id: room.id ?? "",
      label: room.name ?? room.code ?? "",
      code: room.code ?? "",
      kind: "room" as const,
      children: (room.racks ?? []).map((rack) => ({
        id: rack.id ?? "",
        label: rack.name ?? rack.code ?? "",
        code: rack.code ?? "",
        kind: "rack" as const,
      })),
    })),
  }));
}

export async function fetchResourceTree(): Promise<TreeNode[]> {
  const data = await unwrapData(
    await api.GET("/api/v1/resource-tree"),
    "GET /api/v1/resource-tree",
  );
  return toTreeNodes((data.items ?? []) as TreeDataCenter[]);
}

export interface DevicePage {
  items: Device[];
  total: number;
}

export async function fetchDevices(params: {
  page?: number;
  pageSize?: number;
  search?: string;
}): Promise<DevicePage> {
  const data = await unwrapData(
    await api.GET("/api/v1/devices", { params: { query: params } }),
    "GET /api/v1/devices",
  );
  return { items: data.items ?? [], total: data.total ?? 0 };
}
