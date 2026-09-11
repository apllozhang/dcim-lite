import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import ElementPlus from "element-plus";
import ResourceTree from "@/features/resource/ResourceTree.vue";

vi.mock("@/features/resource/api", () => ({
  fetchResourceTree: vi.fn(),
}));

import { fetchResourceTree } from "@/features/resource/api";

const mockedFetch = vi.mocked(fetchResourceTree);

describe("ResourceTree 组件(P0-R02 回归)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders three levels via :props binding(修复前为字符串绑定,树形失效)", async () => {
    mockedFetch.mockResolvedValueOnce([
      {
        id: "dc1",
        label: "一号数据中心",
        code: "DC1",
        kind: "dc",
        children: [
          {
            id: "rm1",
            label: "机房一",
            code: "R1",
            kind: "room",
            children: [{ id: "rk1", label: "机柜一", code: "K1", kind: "rack" }],
          },
        ],
      },
    ]);
    const wrapper = mount(ResourceTree, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    const html = wrapper.html();
    expect(html).toContain("一号数据中心");
    expect(html).toContain("机房一");
    expect(html).toContain("机柜一");
    // 规整后的树形数据被 el-tree 以 label 渲染(P0-R02:props 为对象绑定)
    expect(wrapper.find("[data-test=tree]").exists()).toBe(true);
  });

  it("shows empty state when tree is empty", async () => {
    mockedFetch.mockResolvedValueOnce([]);
    const wrapper = mount(ResourceTree, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(wrapper.find("[data-test=tree-empty]").exists()).toBe(true);
  });

  it("shows error state with retry on failure", async () => {
    mockedFetch.mockRejectedValueOnce(new Error("backend down"));
    const wrapper = mount(ResourceTree, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(wrapper.text()).toContain("资源树加载失败");
  });
});
