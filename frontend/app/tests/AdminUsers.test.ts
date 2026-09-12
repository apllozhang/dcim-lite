import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import ElementPlus from "element-plus";
import AdminUsers from "@/features/admin/AdminUsers.vue";

vi.mock("@/features/admin/api", () => ({
  fetchAdminUsers: vi.fn(),
}));

import { fetchAdminUsers } from "@/features/admin/api";

const mockedFetch = vi.mocked(fetchAdminUsers);

describe("AdminUsers 组件(第 5 轮三权限对照最小切片)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders users with roles and enabled state", async () => {
    mockedFetch.mockResolvedValueOnce([
      {
        id: "u1",
        username: "admin",
        displayName: "管理员",
        email: "",
        authSource: "local",
        enabled: true,
        lastLoginAt: null,
        roles: [{ id: "r1", code: "system_admin", name: "系统管理员" }],
        version: 1,
      },
      {
        id: "u2",
        username: "op1",
        displayName: "操作员",
        email: "",
        authSource: "local",
        enabled: false,
        lastLoginAt: null,
        roles: [{ id: "r2", code: "user", name: "普通用户" }],
        version: 1,
      },
    ]);
    const wrapper = mount(AdminUsers, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("admin");
    expect(text).toContain("系统管理员");
    expect(text).toContain("op1");
    expect(text).toContain("停用");
    expect(wrapper.find("[data-test=admin-users-table]").exists()).toBe(true);
  });

  it("shows error state on 403/failure", async () => {
    mockedFetch.mockRejectedValueOnce(new Error("403 FORBIDDEN"));
    const wrapper = mount(AdminUsers, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(wrapper.text()).toContain("用户列表加载失败");
  });

  it("shows empty state", async () => {
    mockedFetch.mockResolvedValueOnce([]);
    const wrapper = mount(AdminUsers, { global: { plugins: [ElementPlus] } });
    await flushPromises();
    expect(wrapper.text()).toContain("暂无用户");
  });
});
