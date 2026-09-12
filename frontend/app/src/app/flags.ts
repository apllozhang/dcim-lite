/**
 * 路由级 feature flag(P0-R04;P0-R2 治理修订)。
 *
 * 取值优先级:
 *   生产构建(PROD):部署注入 __ALE_ROUTE_OVERRIDES__ 始终生效(部署端强制策略);
 *     URL 参数 ale_flags 与 localStorage ale.flags 仅在调试令牌 ale.flags.debug=1
 *     (授权管理员)时生效,普通用户不可绕过灰度策略。
 *   开发/测试构建:URL > localStorage > 注入 > 默认。
 * 格式:"tree:legacy,devices:new"。
 *
 * 旧界面地址 = legacyBase + 旧路由;legacyBase 默认 "/legacy"(nginx/dev server
 * 反代到旧 bundle,不覆盖 oracle 的独立端口部署)。构建注入用 VITE_ALE_LEGACY_BASE
 * (P0-R2:Vite 只向客户端暴露 envPrefix 内变量,旧名 ALE_LEGACY_BASE 保留兼容)。
 */
export type RouteFlag = "new" | "legacy";

const STORAGE_KEY = "ale.flags";
const DEBUG_KEY = "ale.flags.debug";
const URL_PARAM = "ale_flags";

export const LEGACY_BASE: string =
  (import.meta.env.VITE_ALE_LEGACY_BASE as string | undefined) ??
  (import.meta.env.ALE_LEGACY_BASE as string | undefined) ??
  // 默认 = 同主机 19501:旧 ALE 为 HTML5 history 模式,必须以独立端口源根路径
  // 服务(/legacy/ 前缀下无法路由,P1-D 双跑与现场均已验证)
  (typeof location !== "undefined"
    ? `${location.protocol}//${location.hostname}:19501`
    : "http://127.0.0.1:19501");

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

/** 纯解析核心(可测):overridesAllowed=false 时 URL/localStorage 一律忽略,仅部署注入生效 */
export function resolveRouteFlag(
  module: string,
  src: { url: FlagStore; storage: FlagStore; injected: FlagStore; overridesAllowed: boolean },
  fallback: RouteFlag = "new",
): RouteFlag {
  if (!src.overridesAllowed) return src.injected[module] ?? fallback;
  return src.url[module] ?? src.storage[module] ?? src.injected[module] ?? fallback;
}

/** 调试令牌是否解锁(P0-R2:生产环境 override 治理) */
export function overridesUnlocked(): boolean {
  if (!(import.meta.env.PROD as boolean)) return true;
  try {
    return localStorage.getItem(DEBUG_KEY) === "1";
  } catch {
    return false;
  }
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
  return resolveRouteFlag(
    module,
    {
      url: urlFlags(),
      storage: storedFlags(),
      injected: injectedFlags(),
      overridesAllowed: overridesUnlocked(),
    },
    fallback,
  );
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

/** 模块的新前端路径 → 旧 bundle 路径(旧 ALE 为 HTML5 history 模式,根路径路由) */
const LEGACY_PATHS: Record<string, string> = {
  tree: "/data-centers",
  "room-screen": "/room-screen",
  racks: "/racks",
  "rack-templates": "/rack-templates",
  devices: "/devices",
};

export function legacyUrlFor(module: string): string {
  return LEGACY_BASE + (LEGACY_PATHS[module] ?? "/");
}
