/**
 * 资源域查询与写操作(第 6 轮资源层级页)。
 * 类型从生成 schema 派生;方法/路径/请求体/query 全部由 openapi-fetch 契约约束。
 */
import type { components } from "@/api/generated/schema";
import type { paths } from "@/api/generated/schema";
import { api, unwrapData, unwrapOptional } from "@/api/client";
import { deviceColor } from "@/features/screen/screenShared";

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

/* ── 模板写操作(第 6 轮 屏5 机柜模板) ── */
export type TemplateVersionInput = NonNullable<components["schemas"]["TemplateVersionInput"]>;
type TemplateCreateInput = NonNullable<
  paths["/api/v1/rack-templates"]["post"]["requestBody"]
>["content"]["application/json"];
type TemplateUpdateInput = NonNullable<
  paths["/api/v1/rack-templates/{id}"]["put"]["requestBody"]
>["content"]["application/json"];
export type { TemplateCreateInput, TemplateUpdateInput };

export async function createRackTemplate(body: TemplateCreateInput): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/rack-templates", { body }),
    "POST /api/v1/rack-templates",
  );
}
export async function updateRackTemplate(
  id: string,
  version: number,
  body: TemplateUpdateInput,
): Promise<void> {
  await unwrapOptional(
    await api.PUT("/api/v1/rack-templates/{id}", {
      params: { path: { id }, query: { version } },
      body,
    }),
    "PUT /api/v1/rack-templates/{id}",
  );
}
export async function deleteRackTemplate(id: string, version: number): Promise<void> {
  await unwrapOptional(
    await api.DELETE("/api/v1/rack-templates/{id}", {
      params: { path: { id }, query: { version } },
    }),
    "DELETE /api/v1/rack-templates/{id}",
  );
}
export async function publishRackTemplateVersion(
  id: string,
  version: number,
  spec: TemplateVersionInput,
): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/rack-templates/{id}/versions", {
      params: { path: { id }, query: { version } },
      body: spec,
    }),
    "POST /api/v1/rack-templates/{id}/versions",
  );
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

/* ── 设备上架/移位/下架(第 6 轮 屏6 机房大屏 U 位操作) ── */
export async function assignDevice(
  id: string,
  body: { rackId: string; startU: number },
): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/devices/{id}/assign", {
      params: { path: { id } },
      body,
    } as never),
    "POST /api/v1/devices/{id}/assign",
  );
}
export async function moveDevice(
  id: string,
  body: { rackId: string; startU: number },
): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/devices/{id}/move", {
      params: { path: { id } },
      body,
    } as never),
    "POST /api/v1/devices/{id}/move",
  );
}
export async function decommissionDevice(id: string, reason: string): Promise<void> {
  await unwrapOptional(
    await api.POST("/api/v1/devices/{id}/decommission", {
      params: { path: { id } },
      body: { reason },
    } as never),
    "POST /api/v1/devices/{id}/decommission",
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

/* ── 机柜U位视图(屏3 第三个Tab;屏6 大屏复用) ──
 * 后端实际返回 {positions:[{...,device:摘要}], free:[空闲段]},无 used/devices
 * 字段(2026-09-12 屏6 实测);此处统一映射为视图模型,used 与 devices 由
 * positions 派生——屏3 U 位 Tab 此前因此一直显示 0,本映射顺带修复。 */
export interface ULayoutDevice {
  id: string;
  code: string;
  name: string;
  startU: number;
  endU: number;
  heightU: number;
  category?: string;
  color: string;
}
export interface ULayoutResponse {
  rackId?: string;
  rackCode?: string;
  rackName?: string;
  uHeight: number;
  used: number;
  free: number;
  devices: ULayoutDevice[];
}
export async function fetchULayout(rackId: string): Promise<ULayoutResponse> {
  const data = (await unwrapData(
    await api.GET("/api/v1/racks/{id}/u-layout", { params: { path: { id: rackId } } }),
    "GET /api/v1/racks/{id}/u-layout",
  )) as {
    rackId?: string;
    rackCode?: string;
    rackName?: string;
    uHeight?: number;
    positions?: {
      deviceId: string;
      startU: number;
      endU: number;
      heightU?: number;
      device?: {
        id: string;
        code?: string;
        name?: string;
        heightU?: number;
        type?: { category?: string };
      };
    }[];
  };
  const devices: ULayoutDevice[] = (data.positions ?? [])
    .filter((p) => p.device)
    .map((p) => ({
      id: p.device!.id,
      code: p.device?.code ?? "",
      name: p.device?.name ?? "",
      startU: p.startU,
      endU: p.endU,
      heightU: p.device?.heightU ?? p.endU - p.startU + 1,
      category: p.device?.type?.category,
      color: deviceColor(p.device?.type?.category),
    }));
  const used = (data.positions ?? []).reduce((s, p) => s + Math.max(1, p.endU - p.startU + 1), 0);
  return {
    rackId: data.rackId,
    rackCode: data.rackCode,
    rackName: data.rackName,
    uHeight: data.uHeight ?? 0,
    used,
    free: Math.max(0, (data.uHeight ?? 0) - used),
    devices,
  };
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
