import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import ElementPlus from "element-plus";
import DeviceList from "@/features/device/DeviceList.vue";

vi.mock("@/features/resource/api", () => ({
  fetchDevices: vi.fn(),
  fetchDeviceTypes: vi.fn(),
}));

import { fetchDevices, fetchDeviceTypes } from "@/features/resource/api";

const mockedFetch = vi.mocked(fetchDevices);
const mockedTypes = vi.mocked(fetchDeviceTypes);

type DeviceLike = Parameters<typeof mockedFetch.mockResolvedValueOnce>[0]["items"][number];

function device(p: Partial<DeviceLike> & { id: string; code: string }): DeviceLike {
  return {
    name: p.code,
    heightU: 1,
    lifecycleStatus: "RUNNING",
    ...p,
  } as DeviceLike;
}

describe("DeviceList 组件", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedTypes.mockResolvedValue([]);
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders rows from generated schema types", async () => {
    mockedFetch.mockResolvedValueOnce({
      items: [
        { id: "d1", code: "SRV-001", name: "Web 服务器", heightU: 2, lifecycleStatus: "RUNNING" },
        { id: "d2", code: "SRV-002", name: "备份机", heightU: 1, lifecycleStatus: "WAITING_RACK" },
      ],
      total: 2,
    });
    const wrapper = mount(DeviceList, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("SRV-001");
    expect(text).toContain("备份机");
    expect(wrapper.find("[data-test=device-table]").exists()).toBe(true);
  });

  it("状态列显示中文标签(第 5 轮:与旧 bundle zl 口径一致)", async () => {
    mockedFetch.mockResolvedValueOnce({
      items: [
        device({ id: "d1", code: "DV1", lifecycleStatus: "RUNNING" }),
        device({ id: "d2", code: "DV2", lifecycleStatus: "WAITING_RACK" }),
        device({ id: "d3", code: "DV3", lifecycleStatus: "MAINTENANCE" }),
        device({ id: "d4", code: "DV4", lifecycleStatus: "OFF_RACK" }),
        device({ id: "d5", code: "DV5", lifecycleStatus: "PENDING_REMOVAL" }),
      ],
      total: 5,
    });
    const wrapper = mount(DeviceList, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("运行中");
    expect(text).toContain("待上架");
    expect(text).toContain("维护中");
    expect(text).toContain("已下架");
    expect(text).toContain("待下架");
    // 不再显示英文枚举(旧 UI 行文本口径为中文)
    expect(text).not.toContain("WAITING_RACK");
  });

  it("状态筛选交互:lifecycleStatus 参数传后端查询", async () => {
    mockedFetch.mockResolvedValue({ items: [], total: 0 });
    const wrapper = mount(DeviceList, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(mockedFetch).toHaveBeenLastCalledWith(
      expect.objectContaining({ lifecycleStatus: undefined }),
    );
    // 模拟筛选下拉变更(组件通过 resetAndLoad 重新查询)
    const vm = wrapper.vm as unknown as {
      statusFilter: string;
      resetAndLoad: () => void;
    };
    vm.statusFilter = "WAITING_RACK";
    vm.resetAndLoad();
    await flushPromises();
    expect(mockedFetch).toHaveBeenLastCalledWith(
      expect.objectContaining({ lifecycleStatus: "WAITING_RACK", page: 1 }),
    );
  });

  it("renders currentPosition (A 族读模型) and omits it for idle devices", async () => {
    mockedFetch.mockResolvedValueOnce({
      items: [
        {
          id: "d1",
          code: "SRV-001",
          name: "Web 服务器",
          heightU: 2,
          lifecycleStatus: "RUNNING",
          currentPosition: {
            id: "p1",
            version: 1,
            deviceId: "d1",
            rackId: "r1",
            roomId: "rm1",
            dataCenterId: "dc1",
            startU: 3,
            endU: 4,
            heightU: 2,
            orientation: "NORMAL",
            installedAt: "2026-09-11T00:00:00Z",
            rack: {
              id: "r1",
              code: "RKA",
              name: "机柜A",
              uHeight: 42,
              status: "PARTIAL",
              version: 4,
            },
          },
        },
        { id: "d2", code: "SRV-002", name: "备份机", heightU: 1, lifecycleStatus: "WAITING_RACK" },
      ],
      total: 2,
    });
    const wrapper = mount(DeviceList, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    const cells = wrapper.findAll("[data-test=device-position]");
    expect(cells.length).toBe(1);
    expect(cells[0].text()).toContain("RKA");
    expect(cells[0].text()).toContain("U3-4");
  });

  it("shows empty state when no devices", async () => {
    mockedFetch.mockResolvedValueOnce({ items: [], total: 0 });
    const wrapper = mount(DeviceList, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(wrapper.find("[data-test=device-empty]").exists()).toBe(true);
  });

  it("shows error state with retry on failure", async () => {
    mockedFetch.mockRejectedValueOnce(new Error("500"));
    const wrapper = mount(DeviceList, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(wrapper.text()).toContain("设备列表加载失败");
  });
});
