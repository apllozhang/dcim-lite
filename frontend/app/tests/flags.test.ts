import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { getRouteFlag, setRouteFlag, legacyUrlFor } from "@/app/flags";

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

  it("legacyUrlFor targets the legacy base", () => {
    expect(legacyUrlFor("tree")).toContain("/legacy/");
  });
});
