/**
 * 路由级 feature flag(第三轮复评 P0-R04):新页面与旧 ALE 之间的模块级切换。
 *
 * 取值优先级:URL 参数 ale_flags(演示/应急强制)> localStorage(用户级持久)
 * > window.__ALE_ROUTE_OVERRIDES__(部署注入)> 默认 "new"。
 * 格式:"tree:legacy,devices:new"。
 *
 * 旧界面地址 = legacyBase + 旧路由;legacyBase 默认 "/legacy"(nginx/dev server
 * 反代到旧 bundle,不覆盖 oracle 的独立端口部署)。
 */
export type RouteFlag = "new" | "legacy";

const STORAGE_KEY = "ale.flags";
const URL_PARAM = "ale_flags";

export const LEGACY_BASE: string =
  (import.meta.env.ALE_LEGACY_BASE as string | undefined) ?? "/legacy";

interface FlagStore {
  [module: string]: RouteFlag;
}

function parse(raw: string | null | undefined): FlagStore {
  const out: FlagStore = {};
  if (!raw) return out;
  for (const part of raw.split(",")) {
    const [mod, val] = part.split(":");
    if (mod && (val === "new" || val === "legacy")) out[mod.trim()] = val;
  }
  return out;
}

function urlFlags(): FlagStore {
  try {
    return parse(new URLSearchParams(window.location.search).get(URL_PARAM));
  } catch {
    return {};
  }
}

function storedFlags(): FlagStore {
  try {
    return parse(localStorage.getItem(STORAGE_KEY));
  } catch {
    return {};
  }
}

function injectedFlags(): FlagStore {
  const w = window as unknown as { __ALE_ROUTE_OVERRIDES__?: FlagStore };
  return w.__ALE_ROUTE_OVERRIDES__ ?? {};
}

export function getRouteFlag(module: string, fallback: RouteFlag = "new"): RouteFlag {
  return urlFlags()[module] ?? storedFlags()[module] ?? injectedFlags()[module] ?? fallback;
}

export function setRouteFlag(module: string, value: RouteFlag): void {
  const cur = storedFlags();
  cur[module] = value;
  localStorage.setItem(
    STORAGE_KEY,
    Object.entries(cur)
      .map(([m, v]) => `${m}:${v}`)
      .join(","),
  );
}

/** 模块的新前端路径 → 旧 bundle 路径映射(旧 ALE 为 hash 路由) */
const LEGACY_PATHS: Record<string, string> = {
  tree: "/#/resources",
  devices: "/#/devices",
};

export function legacyUrlFor(module: string): string {
  return LEGACY_BASE + (LEGACY_PATHS[module] ?? "/");
}
