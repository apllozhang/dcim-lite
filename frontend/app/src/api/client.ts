/**
 * API 客户端:唯一的 HTTP 出口(ADR §3)。页面禁止手写请求。
 *
 * 契约约束(第三轮复评 P0-R03):
 * - 路径参数被 `keyof paths` 约束,写错路径无法通过编译;
 * - 响应类型从生成的 schema 派生,业务代码不得自行声明 T;
 * - Envelope.data 可选:getData 强制非空(缺失抛错),sendData 显式返回 D|undefined。
 */
import axios, { AxiosError, type AxiosInstance } from "axios";
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

/** GET /health/ready 探活 */
export interface ReadyInfo {
  status?: string;
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
/** 注册全局 401 处理(main.ts 里接路由跳转;P1-R06) */
export function setUnauthorizedHandler(fn: UnauthorizedHandler): void {
  onUnauthorized = fn;
}

export const http: AxiosInstance = axios.create({ baseURL: "/", timeout: 20000 });

http.interceptors.request.use((cfg) => {
  const token = getToken();
  if (token) cfg.headers.Authorization = `Bearer ${token}`;
  return cfg;
});

http.interceptors.response.use(
  (resp) => resp,
  (err: AxiosError<Envelope>) => {
    const body = err.response?.data;
    const status = err.response?.status ?? 0;
    const url = err.config?.url ?? "";
    // 全局 401 收口:除登录本身外,任何 401 清 token 并交由注册的 handler 跳转
    if (status === 401 && !url.includes("/auth/login")) {
      setToken("");
      try {
        onUnauthorized();
      } catch {
        /* handler 异常不掩盖原始错误 */
      }
    }
    throw new ApiError(
      status,
      body?.code ?? (status === 0 ? "NETWORK_ERROR" : "HTTP_" + status),
      body?.message ?? err.message,
      body?.requestId,
    );
  },
);

/** 从生成类型提取 GET 响应的 data 部分 */
type GetData<P extends keyof paths> = paths[P] extends {
  get: { responses: { 200: { content: { "application/json": infer R } } } };
}
  ? R extends { data?: infer D }
    ? D
    : never
  : never;

/** 从生成类型提取 POST 响应的 data 部分 */
type PostData<P extends keyof paths> = paths[P] extends {
  post: { responses: { 200: { content: { "application/json": infer R } } } };
}
  ? R extends { data?: infer D }
    ? D
    : never
  : never;

/** GET 并解开 Envelope.data;data 缺失/为空视为契约违例(收紧 P1-R06) */
export async function getData<P extends keyof paths>(
  path: P,
  params?: Record<string, unknown>,
): Promise<NonNullable<GetData<P>>> {
  const resp = await http.get<Envelope<GetData<P>>>(path, { params });
  const data = resp.data?.data;
  if (data === undefined || data === null) {
    throw new ApiError(resp.status, "EMPTY_DATA", `响应缺少 data:GET ${String(path)}`);
  }
  return data as NonNullable<GetData<P>>;
}

/** 写操作(POST/PUT/DELETE):data 可缺(如 logout),显式返回 D|undefined */
export async function sendData<P extends keyof paths>(
  method: "post" | "put" | "delete",
  path: P,
  body?: unknown,
  params?: Record<string, unknown>,
): Promise<PostData<P> | undefined> {
  const resp = await http.request<Envelope<PostData<P>>>({ method, url: path, data: body, params });
  return resp.data?.data;
}

/** 后端探活(P0-B:补全 /health/ready 交付声明) */
export async function fetchReady(): Promise<boolean> {
  try {
    const resp = await http.get<ReadyInfo>("/health/ready", { timeout: 5000 });
    return resp.status === 200;
  } catch {
    return false;
  }
}
