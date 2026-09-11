/**
 * API 客户端:唯一的 HTTP 出口。页面禁止手写请求(ADR §3)。
 * Envelope 契约:{ code, message, data?, requestId? }(docs/openapi.yaml)。
 */
import axios, { AxiosError } from "axios";

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

export const http = axios.create({
  baseURL: "/",
  timeout: 20000,
});

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
    throw new ApiError(
      status,
      body?.code ?? (status === 0 ? "NETWORK_ERROR" : "HTTP_" + status),
      body?.message ?? err.message,
      body?.requestId,
    );
  },
);

/** GET 并解开 Envelope.data */
export async function getData<T>(path: string, params?: Record<string, unknown>): Promise<T> {
  const resp = await http.get<Envelope<T>>(path, { params });
  return resp.data.data as T;
}

/** 写操作(POST/PUT/DELETE):成功返回 data,失败抛 ApiError */
export async function sendData<T>(
  method: "post" | "put" | "delete",
  path: string,
  body?: unknown,
  params?: Record<string, unknown>,
): Promise<T> {
  const resp = await http.request<Envelope<T>>({ method, url: path, data: body, params });
  return resp.data.data as T;
}
