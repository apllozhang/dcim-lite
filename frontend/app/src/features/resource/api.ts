/**
 * 资源域查询与写操作(第 6 轮资源层级页)。
 * 类型从生成 schema 派生;方法/路径/请求体/query 全部由 openapi-fetch 契约约束。
 */
import type { components } from "@/api/generated/schema";
import type { paths } from "@/api/generated/schema";
import { api, unwrapData, unwrapOptional } from "@/api/client";

export type TreeDataCenter = components["schemas"]["TreeDataCenter"];
export type Device = components["schemas"]["Device"];
export type DeviceType = components["schemas"]["DeviceType"];
export type Rack = components["schemas"]["Rack"];
export type RackTemplate = components["schemas"]["RackTemplate"];

type DataCenterInput = NonNullable<
  paths["/api/v1/data-centers"]["post"]["requestBody"]
>["content"]["application/json"];
type RoomInput = NonNullable<
  paths["/api/v1/data-centers/{id}/rooms"]["post"]["requestBody"]
>["content"]["application/json"];
type RackInput = NonNullable<
  paths["/api/v1/rooms/{id}/racks"]["post"]["requestBody"]
>["content"]["application/json"];
type CopyMoveInput = NonNullable<
  paths["/api/v1/data-centers/{id}/copy"]["post"]["requestBody"]
>["content"]["application/json"];

export type { DataCenterInput, RoomInput, RackInput, CopyMoveInput };

export async function fetchResourceTree(): Promise<TreeDataCenter[]> {
  const data = await unwrapData(
    await api.GET("/api/v1/resource-tree"),
    "GET /api/v1/resource-tree",
  );
  return data.items ?? [];
}

/* ── 数据中心 ── */
export async function createDataCenter(body: DataCenterInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/data-centers", { body }),
    "POST /api/v1/data-centers",
  );
}
export async function updateDataCenter(
  id: string,
  version: number,
  body: DataCenterInput,
): Promise<void> {
  await unwrapOptional(
    await api.PUT("/api/v1/data-centers/{id}", {
      params: { path: { id }, query: { version } },
      body,
    }),
    "PUT /api/v1/data-centers/{id}",
  );
}
export async function deleteDataCenter(id: string, version: number): Promise<void> {
  await unwrapOptional(
    await api.DELETE("/api/v1/data-centers/{id}", {
      params: { path: { id }, query: { version } },
    }),
    "DELETE /api/v1/data-centers/{id}",
  );
}
export async function copyDataCenter(
  id: string,
  version: number,
  body: CopyMoveInput,
): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/data-centers/{id}/copy", {
      params: { path: { id } },
      body: { ...body, version },
    }),
    "POST /api/v1/data-centers/{id}/copy",
  );
}

/* ── 机房 ── */
export async function createRoom(dcId: string, body: RoomInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/data-centers/{id}/rooms", { params: { path: { id: dcId } }, body }),
    "POST /api/v1/data-centers/{id}/rooms",
  );
}
export async function updateRoom(id: string, version: number, body: RoomInput): Promise<void> {
  await unwrapOptional(
    await api.PUT("/api/v1/rooms/{id}", { params: { path: { id }, query: { version } }, body }),
    "PUT /api/v1/rooms/{id}",
  );
}
export async function deleteRoom(id: string, version: number): Promise<void> {
  await unwrapOptional(
    await api.DELETE("/api/v1/rooms/{id}", { params: { path: { id }, query: { version } } }),
    "DELETE /api/v1/rooms/{id}",
  );
}
export async function copyRoom(id: string, version: number, body: CopyMoveInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/rooms/{id}/copy", {
      params: { path: { id } },
      body: { ...body, version },
    }),
    "POST /api/v1/rooms/{id}/copy",
  );
}
export async function moveRoom(id: string, version: number, body: CopyMoveInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/rooms/{id}/move", {
      params: { path: { id }, query: { version } },
      body,
    }),
    "POST /api/v1/rooms/{id}/move",
  );
}

/* ── 机柜 ── */
export async function createRack(roomId: string, body: RackInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/rooms/{id}/racks", { params: { path: { id: roomId } }, body }),
    "POST /api/v1/rooms/{id}/racks",
  );
}
export async function updateRack(id: string, version: number, body: RackInput): Promise<void> {
  await unwrapOptional(
    await api.PUT("/api/v1/racks/{id}", { params: { path: { id }, query: { version } }, body }),
    "PUT /api/v1/racks/{id}",
  );
}
export async function deleteRack(id: string, version: number): Promise<void> {
  await unwrapOptional(
    await api.DELETE("/api/v1/racks/{id}", { params: { path: { id }, query: { version } } }),
    "DELETE /api/v1/racks/{id}",
  );
}
export async function copyRack(id: string, version: number, body: CopyMoveInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/racks/{id}/copy", {
      params: { path: { id } },
      body: { ...body, version },
    }),
    "POST /api/v1/racks/{id}/copy",
  );
}
export async function moveRack(id: string, version: number, body: CopyMoveInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/racks/{id}/move", {
      params: { path: { id }, query: { version } },
      body,
    }),
    "POST /api/v1/racks/{id}/move",
  );
}

export async function fetchRackTemplates(): Promise<RackTemplate[]> {
  const data = await unwrapData(
    await api.GET("/api/v1/rack-templates"),
    "GET /api/v1/rack-templates",
  );
  return data.items ?? [];
}

/* ── 设备(列表/类型;台账页与运行概览共用) ── */
export interface DevicePage {
  items: Device[];
  total: number;
}

export async function fetchDevices(params: {
  page?: number;
  pageSize?: number;
  search?: string;
  lifecycleStatus?: string;
  typeId?: string;
}): Promise<DevicePage> {
  const data = await unwrapData(
    await api.GET("/api/v1/devices", { params: { query: params } }),
    "GET /api/v1/devices",
  );
  return { items: data.items ?? [], total: data.total ?? 0 };
}

export async function fetchDeviceTypes(): Promise<DeviceType[]> {
  const data = await unwrapData(await api.GET("/api/v1/device-types"), "GET /api/v1/device-types");
  return data.items ?? [];
}

/* ── 设备写操作(第6轮 屏3) ── */
export async function createDevice(body: Record<string, unknown>): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/devices", { body } as never),
    "POST /api/v1/devices",
  );
}
export async function updateDevice(
  id: string,
  version: number,
  body: Record<string, unknown>,
): Promise<void> {
  await unwrapOptional(
    await api.PUT("/api/v1/devices/{id}", {
      params: { path: { id }, query: { version } },
      body,
    } as never),
    "PUT /api/v1/devices/{id}",
  );
}
export async function deleteDevice(id: string, version: number): Promise<void> {
  await unwrapOptional(
    await api.DELETE("/api/v1/devices/{id}", { params: { path: { id }, query: { version } } }),
    "DELETE /api/v1/devices/{id}",
  );
}
export async function fetchDevice(id: string): Promise<Device> {
  return unwrapData(
    await api.GET("/api/v1/devices/{id}", { params: { path: { id } } }),
    "GET /api/v1/devices/{id}",
  );
}

/* ── 设备类型写操作 ── */
export async function createDeviceType(body: Record<string, unknown>): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/device-types", { body } as never),
    "POST /api/v1/device-types",
  );
}
export async function updateDeviceType(
  id: string,
  version: number,
  body: Record<string, unknown>,
): Promise<void> {
  await unwrapOptional(
    await api.PUT("/api/v1/device-types/{id}", {
      params: { path: { id }, query: { version } },
      body,
    } as never),
    "PUT /api/v1/device-types/{id}",
  );
}
export async function deleteDeviceType(id: string, version: number): Promise<void> {
  await unwrapOptional(
    await api.DELETE("/api/v1/device-types/{id}", { params: { path: { id }, query: { version } } }),
    "DELETE /api/v1/device-types/{id}",
  );
}

/* ── 机柜U位视图(屏3 第三个Tab) ── */
export interface ULayoutDevice {
  id: string;
  code: string;
  name: string;
  startU: number;
  endU: number;
  heightU: number;
  color: string;
}
export interface ULayoutResponse {
  uHeight: number;
  used: number;
  free: number;
  devices: ULayoutDevice[];
}
export async function fetchULayout(rackId: string): Promise<ULayoutResponse> {
  const data = await unwrapData(
    await api.GET("/api/v1/racks/{id}/u-layout", { params: { path: { id: rackId } } }),
    "GET /api/v1/racks/{id}/u-layout",
  );
  return data as unknown as ULayoutResponse;
}
export async function fetchRacks(params?: {
  page?: number;
  pageSize?: number;
}): Promise<{ items: Rack[]; total: number }> {
  const data = await unwrapData(
    await api.GET("/api/v1/racks-page", { params: { query: params ?? {} } }),
    "GET /api/v1/racks-page",
  );
  return { items: data.items ?? [], total: data.total ?? 0 };
}
