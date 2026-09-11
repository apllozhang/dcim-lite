import { afterEach, describe, expect, it, vi } from "vitest";
import {
  api,
  unwrapData,
  unwrapOptional,
  setToken,
  getToken,
  setUnauthorizedHandler,
  ApiError,
} from "@/api/client";

/**
 * P1-R4:经 fetch stub,请求走完整 openapi-fetch 管道(含 middleware:
 * 鉴权头、401 收口、错误归一化),断言的是管道行为本身。
 */
function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function useFetch(fn: (input: string) => Promise<Response>) {
  vi.stubGlobal("fetch", vi.fn(fn as (input: unknown) => Promise<Response>));
}

afterEach(() => {
  vi.unstubAllGlobals();
  setToken("");
  setUnauthorizedHandler(() => {});
});

describe("unwrapData(P1-R4 强类型客户端)", () => {
  it("unwraps envelope data(类型由 paths 推导)", async () => {
    useFetch(async () =>
      jsonResponse(200, { code: "SUCCESS", message: "ok", data: { items: [], total: 0 } }),
    );
    const out = await unwrapData(
      await api.GET("/api/v1/devices", { params: { query: { page: 1 } } }),
      "GET /api/v1/devices",
    );
    expect(out).toEqual({ items: [], total: 0 });
  });

  it("sends Authorization header from token(middleware)", async () => {
    setToken("tok-1");
    const spy = vi.fn(async (_input: unknown) =>
      jsonResponse(200, { code: "SUCCESS", message: "ok", data: { items: [], total: 0 } }),
    );
    vi.stubGlobal("fetch", spy);
    await unwrapData(await api.GET("/api/v1/devices"), "GET /api/v1/devices");
    const call = spy.mock.calls[0]?.[0] as Request;
    expect(call.headers.get("Authorization")).toBe("Bearer tok-1");
  });

  it("throws EMPTY_DATA when data missing(Envelope 收紧)", async () => {
    useFetch(async () => jsonResponse(200, { code: "SUCCESS", message: "ok" }));
    await expect(
      unwrapData(await api.GET("/api/v1/devices"), "GET /api/v1/devices"),
    ).rejects.toMatchObject({ code: "EMPTY_DATA" });
  });

  it("wraps http error into ApiError with code", async () => {
    useFetch(async () =>
      jsonResponse(409, { code: "RACK_U_CONFLICT", message: "overlap", requestId: "req_1" }),
    );
    await expect(
      unwrapData(await api.GET("/api/v1/devices"), "GET /api/v1/devices"),
    ).rejects.toMatchObject({ status: 409, code: "RACK_U_CONFLICT", requestId: "req_1" });
  });
});

describe("401 全局收口(P1-R06)", () => {
  it("clears token and invokes handler on non-login 401", async () => {
    setToken("fake-token");
    expect(getToken()).toBe("fake-token");
    const handler = vi.fn();
    setUnauthorizedHandler(handler);
    useFetch(async () => jsonResponse(401, { code: "UNAUTHORIZED", message: "expired" }));
    await expect(
      unwrapData(await api.GET("/api/v1/devices"), "GET /api/v1/devices"),
    ).rejects.toBeInstanceOf(ApiError);
    expect(getToken()).toBe("");
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("does not trigger handler for login endpoint 401", async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);
    useFetch(async () => jsonResponse(401, { code: "INVALID_CREDENTIALS", message: "bad" }));
    await expect(
      unwrapOptional(
        await api.POST("/api/v1/auth/login", {
          body: { username: "u", password: "p", captchaId: "cid", captcha: "1234" },
        }),
        "POST /api/v1/auth/login",
      ),
    ).rejects.toBeInstanceOf(ApiError);
    expect(handler).not.toHaveBeenCalled();
  });
});

describe("unwrapOptional", () => {
  it("returns undefined when data absent(logout 容忍)", async () => {
    useFetch(async () => jsonResponse(200, { code: "SUCCESS", message: "ok" }));
    await expect(
      unwrapOptional(await api.POST("/api/v1/auth/logout"), "POST /api/v1/auth/logout"),
    ).resolves.toBeUndefined();
  });
});
