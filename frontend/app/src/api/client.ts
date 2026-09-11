/**
 * API 客户端:唯一的 HTTP 出口(ADR §3)。页面禁止手写请求。
 *
 * 契约约束(P0-R03 → 第四轮 P1-R4 升级为 openapi-fetch 强类型客户端):
 * - 方法/路径/请求体/query/响应四者均由 OpenAPI 生成类型约束;
 *   GET 路径用于 POST、缺必填 body 字段、响应类型不符都会在编译期失败;
 * - Envelope.data 可选:GET 路径强制非空(缺失抛 EMPTY_DATA),写路径显式返回 D|undefined;
 * - 401 全局收口在 middleware 中实现(非 /auth/login 的 401 清 token 并触发注册的 handler)。
 */
import createClient from "openapi-fetch";
import type { paths } from "./generated/schema";

export interface Envelope<T = unknown> {
  code: string;
  message: string;
  data?: T;
  requestId?: string;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly requestId?: string,
  ) {
    super(`${code}: ${message}`);
    this.name = "ApiError";
  }
}

const TOKEN_KEY = "ale.token";

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? "";
}

export function setToken(token: string): void {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

type UnauthorizedHandler = () => void;
let onUnauthorized: UnauthorizedHandler = () => {};
/** 注册全局 401 处理(main.ts 里接路由跳转) */
export function setUnauthorizedHandler(fn: UnauthorizedHandler): void {
  onUnauthorized = fn;
}

// 同源部署形态:显式携带 origin(Node fetch 与测试环境不接受相对 URL)。
// fetch 经"调用时解引用"传递,保证测试期 vi.stubGlobal(fetch) 生效。
export const api = createClient<paths>({
  baseUrl: typeof window !== "undefined" ? window.location.origin : "",
  fetch: (...args) => globalThis.fetch(...(args as Parameters<typeof fetch>)),
});

const REQUEST_TIMEOUT_MS = 20000;

api.use({
  onRequest({ request }) {
    const token = getToken();
    if (token) request.headers.set("Authorization", `Bearer ${token}`);
    if (!request.signal) {
      return new Request(request, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
    }
    return request;
  },
  onResponse({ request, response }) {
    // 全局 401 收口:除登录本身外,任何 401 清 token 并交由注册的 handler 跳转
    if (response.status === 401 && !request.url.includes("/auth/login")) {
      setToken("");
      try {
        onUnauthorized();
      } catch {
        /* handler 异常不掩盖原始错误 */
      }
    }
    return response;
  },
});

/** 从 envelope 类型提取 data 部分 */
type DataOf<T> = T extends { data?: infer D } ? D : never;
/** openapi-fetch 的单次调用结果 */
interface CallResult<T> {
  data?: T | null;
  error?: unknown;
  response: Response;
}

function toApiError(res: { error?: unknown; response: Response }): ApiError {
  const body = res.error as Envelope | undefined;
  const status = res.response.status;
  return new ApiError(
    status,
    body?.code ?? (status === 0 ? "NETWORK_ERROR" : "HTTP_" + status),
    body?.message ?? res.response.statusText,
    body?.requestId,
  );
}

/** GET 并解开 Envelope.data;data 缺失/为空视为契约违例(EMPTY_DATA) */
export async function unwrapData<T>(
  res: CallResult<T>,
  op: string,
): Promise<NonNullable<DataOf<T>>> {
  if (!res.response.ok || !res.data) throw toApiError(res);
  const d = (res.data as { data?: DataOf<T> }).data;
  if (d === undefined || d === null) {
    throw new ApiError(res.response.status, "EMPTY_DATA", `响应缺少 data:${op}`);
  }
  return d as NonNullable<DataOf<T>>;
}

/** 写操作(POST/PUT/DELETE):data 可缺(如 logout),显式返回 D|undefined */
export async function unwrapOptional<T>(
  res: CallResult<T>,
  op: string,
): Promise<DataOf<T> | undefined> {
  if (!res.response.ok || !res.data) {
    if (!res.response.ok) throw toApiError(res);
    throw new ApiError(res.response.status, "EMPTY_DATA", `响应缺少 envelope:${op}`);
  }
  return (res.data as { data?: DataOf<T> }).data;
}

/** GET /health/ready 探活 */
export interface ReadyInfo {
  status?: string;
}

/** 后端探活(P0-B:补全 /health/ready 交付声明) */
export async function fetchReady(): Promise<boolean> {
  try {
    const resp = await fetch("/health/ready", { signal: AbortSignal.timeout(5000) });
    return resp.status === 200;
  } catch {
    return false;
  }
}
