import { describe, expect, it, vi } from "vitest";
import {
  LIFECYCLE_STATUS_OPTIONS,
  lifecycleLabel,
  lifecycleTagType,
} from "@/features/device/statusLabel";

describe("statusLabel(旧 bundle zl/Rt 口径复刻)", () => {
  it("六态全覆盖(五态对照 + SCRAPPED)", () => {
    expect(LIFECYCLE_STATUS_OPTIONS.map((o) => o.value)).toEqual([
      "WAITING_RACK",
      "RUNNING",
      "MAINTENANCE",
      "PENDING_REMOVAL",
      "OFF_RACK",
      "SCRAPPED",
    ]);
  });

  it("枚举 → 中文显示名(与旧 UI 行文本一致)", () => {
    expect(lifecycleLabel("WAITING_RACK")).toBe("待上架");
    expect(lifecycleLabel("RUNNING")).toBe("运行中");
    expect(lifecycleLabel("MAINTENANCE")).toBe("维护中");
    expect(lifecycleLabel("PENDING_REMOVAL")).toBe("待下架");
    expect(lifecycleLabel("OFF_RACK")).toBe("已下架");
    expect(lifecycleLabel("SCRAPPED")).toBe("已报废");
  });

  it("未知值原样返回、空值空串(不吞数据)", () => {
    expect(lifecycleLabel("WEIRD")).toBe("WEIRD");
    expect(lifecycleLabel(undefined)).toBe("");
    expect(lifecycleLabel(null)).toBe("");
  });

  it("tag 颜色口径:RUNNING=success,MAINTENANCE/PENDING_REMOVAL=warning,SCRAPPED=danger,其余 info", () => {
    expect(lifecycleTagType("RUNNING")).toBe("success");
    expect(lifecycleTagType("MAINTENANCE")).toBe("warning");
    expect(lifecycleTagType("PENDING_REMOVAL")).toBe("warning");
    expect(lifecycleTagType("SCRAPPED")).toBe("danger");
    expect(lifecycleTagType("WAITING_RACK")).toBe("info");
    expect(lifecycleTagType("OFF_RACK")).toBe("info");
  });
});

describe("routeGuard(三权限守卫)", () => {
  // 动态 import 避免模块顶层 pinia 依赖
  async function makeGuard(session: { isLoggedIn: boolean; isAdmin: boolean }) {
    const mod = await import("@/app/router");
    // routeGuard 是纯函数,不需要 active pinia
    return mod.routeGuard(session);
  }

  it("匿名访问任意页 → /login", async () => {
    const g = await makeGuard({ isLoggedIn: false, isAdmin: false });
    expect(g({ path: "/devices", meta: {} })).toBe("/login");
    expect(g({ path: "/admin", meta: { requiresAdmin: true } })).toBe("/login");
  });

  it("user(已登录非 admin)直达 /admin → 重定向 /", async () => {
    const g = await makeGuard({ isLoggedIn: true, isAdmin: false });
    expect(g({ path: "/admin", meta: { requiresAdmin: true } })).toBe("/");
    expect(g({ path: "/devices", meta: {} })).toBe(true);
  });

  it("admin 直达 /admin 放行", async () => {
    const g = await makeGuard({ isLoggedIn: true, isAdmin: true });
    expect(g({ path: "/admin", meta: { requiresAdmin: true } })).toBe(true);
  });

  it("已登录访问 /login → /", async () => {
    const g = await makeGuard({ isLoggedIn: true, isAdmin: false });
    expect(g({ path: "/login" })).toBe("/");
  });

  it("legacy flag 模块跳出 SPA", async () => {
    // location 属性不可 redefine:整体 stub window.location(happy-dom 限制)
    const replace = vi.fn();
    const orig = window.location;
    vi.stubGlobal("location", { ...orig, replace });
    localStorage.setItem("ale.flags", "devices:legacy");
    try {
      const g = await makeGuard({ isLoggedIn: true, isAdmin: false });
      expect(g({ path: "/devices", meta: { flagModule: "devices" } })).toBe(false);
      expect(replace).toHaveBeenCalled();
    } finally {
      vi.unstubAllGlobals();
      localStorage.removeItem("ale.flags");
      void orig;
    }
  });
});
