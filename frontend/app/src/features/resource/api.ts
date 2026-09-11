/** 资源树与设备列表的只读查询(Phase 1 切片:无任何写操作)。 */
import { getData } from "@/api/client";
import type { DataCenterNode, DeviceRow } from "@/entities/resource";

export async function fetchResourceTree(): Promise<DataCenterNode[]> {
  const data = await getData<{ items: DataCenterNode[] }>("/api/v1/resource-tree");
  return data.items ?? [];
}

export interface DevicePage {
  items: DeviceRow[];
  total: number;
}

export async function fetchDevices(params: {
  page?: number;
  pageSize?: number;
  search?: string;
}): Promise<DevicePage> {
  return getData<DevicePage>("/api/v1/devices", params);
}
