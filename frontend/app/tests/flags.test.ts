import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { getRouteFlag, setRouteFlag, legacyUrlFor, resolveRouteFlag } from "@/app/flags";

describe("feature flags(P0-R04)", () => {
  beforeEach(() => {
    localStorage.clear();
    window.history.replaceState({}, "", "/");
  });
  afterEach(() => {
    localStorage.clear();
    delete (window as { __ALE_ROUTE_OVERRIDES__?: unknown }).__ALE_ROUTE_OVERRIDES__;
  });

  it("defaults to new", () => {
    expect(getRouteFlag("tree")).toBe("new");
  });

  it("localStorage beats injected overrides", () => {
    (window as { __ALE_ROUTE_OVERRIDES__?: unknown }).__ALE_ROUTE_OVERRIDES__ = { tree: "legacy" };
    setRouteFlag("tree", "new");
    expect(getRouteFlag("tree")).toBe("new");
  });

  it("URL param beats everything", () => {
    setRouteFlag("tree", "new");
    window.history.replaceState({}, "", "/?ale_flags=tree:legacy");
    expect(getRouteFlag("tree")).toBe("legacy");
  });

  it("setRouteFlag persists per module", () => {
    setRouteFlag("devices", "legacy");
    expect(getRouteFlag("devices")).toBe("legacy");
    expect(getRouteFlag("tree")).toBe("new");
  });

  it("legacyUrlFor targets the legacy sibling origin(P1-D:旧 UI 独立端口)", () => {
    const u = legacyUrlFor("tree");
    expect(u).toContain("19501");
    expect(u).toContain("/data-centers");
    expect(legacyUrlFor("devices")).toContain("/devices");
  });
});

describe("flag 治理(P0-R2:生产 override 仅限调试令牌)", () => {
  const base = {
    url: { tree: "legacy" } as Record<string, "new" | "legacy">,
    storage: { tree: "new" } as Record<string, "new" | "legacy">,
    injected: { devices: "legacy" } as Record<string, "new" | "legacy">,
  };

  it("生产环境无调试令牌:URL/localStorage 被忽略,部署注入仍生效", () => {
    const f = resolveRouteFlag("tree", { ...base, overridesAllowed: false });
    expect(f).toBe("new"); // URL=legacy、storage=new 都不生效 → 默认 new
    expect(resolveRouteFlag("devices", { ...base, overridesAllowed: false })).toBe("legacy"); // 注入生效
  });

  it("生产环境持调试令牌:URL > storage > 注入照常", () => {
    expect(resolveRouteFlag("tree", { ...base, overridesAllowed: true })).toBe("legacy");
    expect(resolveRouteFlag("devices", { ...base, overridesAllowed: true })).toBe("legacy");
  });
});
