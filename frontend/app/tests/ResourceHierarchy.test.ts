import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createRouter, createMemoryHistory } from "vue-router";
import ElementPlus from "element-plus";
import ResourceHierarchy from "@/features/resource/ResourceHierarchy.vue";

vi.mock("@/features/resource/api", () => ({
  fetchResourceTree: vi.fn(),
  fetchRackTemplates: vi.fn(),
  createDataCenter: vi.fn(),
  updateDataCenter: vi.fn(),
  deleteDataCenter: vi.fn(),
  copyDataCenter: vi.fn(),
  createRoom: vi.fn(),
  updateRoom: vi.fn(),
  deleteRoom: vi.fn(),
  copyRoom: vi.fn(),
  moveRoom: vi.fn(),
  createRack: vi.fn(),
  updateRack: vi.fn(),
  deleteRack: vi.fn(),
  copyRack: vi.fn(),
  moveRack: vi.fn(),
}));

import { fetchResourceTree, fetchRackTemplates } from "@/features/resource/api";

const mockedTree = vi.mocked(fetchResourceTree);
const mockedTemplates = vi.mocked(fetchRackTemplates);

const TREE = [
  {
    id: "dc1",
    code: "DC-A",
    name: "源中心",
    status: "OPERATING",
    version: 3,
    rooms: [
      {
        id: "r1",
        code: "R1",
        name: "源机房",
        status: "OPERATING",
        version: 2,
        racks: [
          { id: "k1", code: "K1", name: "源机柜", uHeight: 42, status: "AVAILABLE", version: 5 },
          { id: "k2", code: "K2", name: "副本机柜", uHeight: 42, status: "PARTIAL", version: 1 },
        ],
      },
    ],
  },
  {
    id: "dc2",
    code: "DC-B",
    name: "副本中心",
    status: "OPERATING",
    version: 1,
    rooms: [],
  },
];

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/", component: ResourceHierarchy }],
  });
}

describe("ResourceHierarchy(资源层级,复刻 v2)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedTree.mockResolvedValue(TREE);
    mockedTemplates.mockResolvedValue([]);
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("页头/统计条/三栏与级联选择(默认选中第一个 DC 与机房)", async () => {
    const router = makeRouter();
    const wrapper = mount(ResourceHierarchy, {
      global: { plugins: [ElementPlus, router] },
    });
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("基础资源管理");
    expect(text).toContain("新增数据中心");
    // 统计条:2 DC / 1 机房 / 2 机柜
    expect(wrapper.find("[data-test=summary-dc]").text()).toBe("2");
    expect(wrapper.find("[data-test=summary-room]").text()).toBe("1");
    expect(wrapper.find("[data-test=summary-rack]").text()).toBe("2");
    // DC 列:两张卡,默认第一张 active
    const dcs = wrapper.findAll("[data-test=dc-item]");
    expect(dcs.length).toBe(2);
    expect(dcs[0].classes()).toContain("active");
    expect(dcs[0].text()).toContain("源中心");
    expect(dcs[0].text()).toContain("DC-A · 1 个机房");
    expect(dcs[0].text()).toContain("运行中");
    // 机柜表:默认机房的两台,状态映射 v2 口径
    const table = wrapper.find("[data-test=rack-table]");
    expect(table.exists()).toBe(true);
    expect(table.text()).toContain("自定义参数");
    expect(table.text()).toContain("42U");
    expect(table.text()).toContain("空闲");
    expect(table.text()).toContain("部分使用");
  });

  it("点击第二个 DC → 机房列切换为空态提示(暂无机房)", async () => {
    const router = makeRouter();
    const wrapper = mount(ResourceHierarchy, {
      global: { plugins: [ElementPlus, router] },
    });
    await flushPromises();
    await wrapper.findAll("[data-test=dc-item]")[1].trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("暂无机房");
    // 机柜列提示请先选择机房
    expect(wrapper.text()).toContain("请先选择机房");
  });

  it("机房卡显示'未设置物理位置'元信息(v2 口径)", async () => {
    const router = makeRouter();
    const wrapper = mount(ResourceHierarchy, {
      global: { plugins: [ElementPlus, router] },
    });
    await flushPromises();
    const room = wrapper.find("[data-test=room-item]");
    expect(room.exists()).toBe(true);
    expect(room.text()).toContain("R1 · 2 个机柜");
    expect(room.text()).toContain("未设置物理位置");
  });
});
