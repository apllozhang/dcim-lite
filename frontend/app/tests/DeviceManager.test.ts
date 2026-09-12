import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createMemoryHistory, createRouter, type Router } from "vue-router";
import DeviceManager from "@/features/device/DeviceManager.vue";

vi.mock("@/features/resource/api", () => ({
  fetchDevices: vi.fn(),
  fetchDeviceTypes: vi.fn(),
  createDevice: vi.fn(),
  updateDevice: vi.fn(),
  deleteDevice: vi.fn(),
  fetchDevice: vi.fn(),
  createDeviceType: vi.fn(),
  updateDeviceType: vi.fn(),
  deleteDeviceType: vi.fn(),
  fetchULayout: vi.fn(),
  fetchRacks: vi.fn(),
}));

import { fetchDevices, fetchDeviceTypes, fetchRacks, fetchULayout } from "@/features/resource/api";

const mockedDevices = vi.mocked(fetchDevices);
const mockedTypes = vi.mocked(fetchDeviceTypes);
const mockedRacks = vi.mocked(fetchRacks);
const mockedULayout = vi.mocked(fetchULayout);

function makeRouter(initial?: string): Router {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/:pathMatch(.*)*", component: { template: "<div/>" } }],
  });
  const to = initial ? router.push(initial) : Promise.resolve();
  return Object.assign(router, { __ready: to });
}

async function mountAt(initial?: string) {
  const router = makeRouter(initial);
  await (router as unknown as { __ready: Promise<unknown> }).__ready;
  const wrapper = mount(DeviceManager, { global: { plugins: [ElementPlus, router] } });
  await flushPromises();
  return wrapper;
}

describe("DeviceManager(设备台账, 复刻 v2 + 三 Tab)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedDevices.mockResolvedValue({ items: [], total: 0 });
    mockedTypes.mockResolvedValue([]);
    mockedRacks.mockResolvedValue({ items: [], total: 0 });
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("v2 页头与三个 Tab 标签渲染", async () => {
    const wrapper = await mountAt("/devices");
    const text = wrapper.text();
    expect(text).toContain("设备与 U 位管理");
    expect(text).toContain("新增设备");
    expect(text).toContain("批量导入");
    expect(wrapper.find("[data-test=device-manager]").exists()).toBe(true);
    expect(text).toContain("设备台账");
    expect(text).toContain("设备类型");
    expect(text).toContain("机柜 U 位");
  });

  it("设备表格含编码/名称/运行中/2U/位置列(v2 口径)", async () => {
    mockedDevices.mockResolvedValueOnce({
      items: [
        {
          id: "d1",
          code: "SRV01",
          name: "Web",
          heightU: 2,
          lifecycleStatus: "RUNNING",
          currentPosition: { rack: { code: "K1" }, startU: 3, endU: 4 },
        },
      ],
      total: 1,
    });
    const wrapper = await mountAt("/devices");
    const table = wrapper.find("[data-test=device-table]");
    expect(table.exists()).toBe(true);
    expect(table.text()).toContain("SRV01");
    expect(table.text()).toContain("运行中");
    expect(table.text()).toContain("2U");
    const pos = wrapper.find("[data-test=device-position]");
    expect(pos.exists()).toBe(true);
    expect(pos.text()).toContain("K1");
  });

  it("工具栏含设备类型和生命周期下拉", async () => {
    const wrapper = await mountAt("/devices");
    const toolbar = wrapper.find("[data-test=device-toolbar]");
    expect(toolbar.exists()).toBe(true);
    expect(toolbar.text()).toContain("生命周期");
  });

  it("状态列中文显示(v2 口径)", async () => {
    mockedDevices.mockResolvedValueOnce({
      items: [
        { id: "d1", code: "DV1", name: "a", heightU: 1, lifecycleStatus: "RUNNING" },
        { id: "d2", code: "DV2", name: "b", heightU: 1, lifecycleStatus: "WAITING_RACK" },
      ],
      total: 2,
    });
    const wrapper = await mountAt("/devices");
    expect(wrapper.text()).toContain("运行中");
    expect(wrapper.text()).toContain("待上架");
    expect(wrapper.text()).not.toContain("WAITING_RACK");
  });

  it("屏4 U 位入口:/devices?tab=layout&rackId 直切 U 位 Tab 并加载指定机柜", async () => {
    mockedRacks.mockResolvedValue({
      items: [{ id: "R1", code: "K1", name: "柜一", uHeight: 42 } as never],
      total: 1,
    });
    mockedULayout.mockResolvedValue({
      uHeight: 42,
      used: 2,
      free: 40,
      devices: [
        { id: "d1", code: "DV1", name: "Web", startU: 1, endU: 2, heightU: 2, color: "#8884d8" },
      ],
    });
    const wrapper = await mountAt("/devices?tab=layout&rackId=R1");
    await flushPromises();
    const active = wrapper.find(".el-tabs__item.is-active");
    expect(active.text()).toContain("机柜 U 位");
    expect(mockedULayout).toHaveBeenCalledWith("R1");
  });
});
