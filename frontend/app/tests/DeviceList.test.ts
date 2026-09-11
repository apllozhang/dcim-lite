import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import ElementPlus from "element-plus";
import DeviceList from "@/features/device/DeviceList.vue";

vi.mock("@/features/resource/api", () => ({
  fetchDevices: vi.fn(),
}));

import { fetchDevices } from "@/features/resource/api";

const mockedFetch = vi.mocked(fetchDevices);

describe("DeviceList 组件", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
