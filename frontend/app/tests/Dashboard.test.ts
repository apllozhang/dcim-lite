import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createRouter, createMemoryHistory } from "vue-router";
import ElementPlus, { ElMessage } from "element-plus";
import DashboardView from "@/features/dashboard/DashboardView.vue";

vi.mock("@/features/resource/api", () => ({
  fetchResourceTree: vi.fn(),
  fetchDevices: vi.fn(),
}));

import { fetchResourceTree, fetchDevices } from "@/features/resource/api";

const mockedTree = vi.mocked(fetchResourceTree);
const mockedDevices = vi.mocked(fetchDevices);

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", component: DashboardView },
      { path: "/data-centers", component: { template: "<div/>" } },
      { path: "/devices", component: { template: "<div/>" } },
      { path: "/room-screen", component: { template: "<div/>" } },
    ],
  });
}

describe("DashboardView(运行概览,复刻 v2)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("统计口径与 v2 一致:DC/机房/机柜由树现算,设备取 total", async () => {
    mockedTree.mockResolvedValueOnce([
      {
        id: "dc1",
        code: "DC-A",
        name: "源中心",
        rooms: [
          { id: "r1", code: "R1", name: "源机房", racks: [{ id: "k1" }, { id: "k2" }] },
          { id: "r2", code: "R2", name: "副本机房", racks: [] },
        ],
      },
    ]);
    mockedDevices.mockResolvedValueOnce({ items: [], total: 7 });
    const router = makeRouter();
    const wrapper = mount(DashboardView, {
      global: { plugins: [ElementPlus, router] },
    });
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("运行概览");
    expect(text).toContain("服务在线");
    // 四张统计卡:1 DC / 2 机房 / 2 机柜 / 7 设备
    expect(text).toContain("基线 ≤ 15 个");
    expect(wrapper.find("[data-test=dashboard]").exists()).toBe(true);
    const values = wrapper.findAll(".stat-value").map((v) => v.text());
    expect(values).toEqual(["1", "2", "2", "7"]);
    expect(wrapper.findAll(".stat-note")[0].text()).toBe("基线 ≤ 15 个");
    expect(wrapper.findAll(".stat-note")[1].text()).toBe("每中心 ≤ 5 个");
    expect(wrapper.findAll(".stat-note")[2].text()).toBe("总量 ≤ 200 个");
    expect(wrapper.findAll(".stat-note")[3].text()).toBe("总量 ≤ 3000 台");
    // 进度条与提示(v2 原文)
    expect(text).toContain("开发进度");
    expect(text).toContain("下一阶段：PDU 插座明细、环境监控接口与外部系统集成适配器。");
  });

  it("加载失败时报'运行统计加载失败'且不崩溃", async () => {
    mockedTree.mockRejectedValueOnce(new Error("boom"));
    const errSpy = vi.spyOn(ElMessage, "error");
    const router = makeRouter();
    mount(DashboardView, { global: { plugins: [ElementPlus, router] } });
    await flushPromises();
    expect(errSpy).toHaveBeenCalledWith("运行统计加载失败");
  });
});
