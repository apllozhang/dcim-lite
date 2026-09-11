import { afterEach, describe, expect, it, vi } from "vitest";
import { AxiosError, type AxiosAdapter, type AxiosResponse } from "axios";
import {
  getData,
  sendData,
  setToken,
  getToken,
  setUnauthorizedHandler,
  ApiError,
  http,
} from "@/api/client";

/**
 * 经 axios adapter mock:请求走完整管道(含响应拦截器),
 * 断言的是拦截器行为本身——直接 mock 实例方法会绕开拦截器。
 */
function useAdapter(fn: AxiosAdapter) {
  http.defaults.adapter = fn;
}

afterEach(() => {
  delete http.defaults.adapter;
  setToken("");
  setUnauthorizedHandler(() => {});
});

describe("getData(P0-R03/P1-R06)", () => {
  it("unwraps envelope data", async () => {
    useAdapter(
      async () =>
        ({
          status: 200,
          data: { code: "SUCCESS", message: "ok", data: { items: [], total: 0 } },
        }) as AxiosResponse,
    );
    const out = await getData("/api/v1/devices", { page: 1 });
    expect(out).toEqual({ items: [], total: 0 });
  });

  it("throws EMPTY_DATA when data missing(Envelope 收紧)", async () => {
    useAdapter(
      async () => ({ status: 200, data: { code: "SUCCESS", message: "ok" } }) as AxiosResponse,
    );
    await expect(getData("/api/v1/devices")).rejects.toMatchObject({ code: "EMPTY_DATA" });
  });

  it("wraps http error into ApiError with code(拦截器生效)", async () => {
    useAdapter(async (config) => {
      throw new AxiosError("req fail", undefined, config, undefined, {
        status: 409,
        data: { code: "RACK_U_CONFLICT", message: "overlap" },
      } as AxiosResponse);
    });
    await expect(getData("/api/v1/racks-page")).rejects.toMatchObject({
      status: 409,
      code: "RACK_U_CONFLICT",
    });
  });
});

describe("401 全局收口(P1-R06)", () => {
  it("clears token and invokes handler on non-login 401", async () => {
    setToken("fake-token");
    expect(getToken()).toBe("fake-token");
    const handler = vi.fn();
    setUnauthorizedHandler(handler);
    useAdapter(async (config) => {
      throw new AxiosError("unauth", undefined, config, undefined, {
        status: 401,
        data: { code: "UNAUTHORIZED", message: "expired" },
      } as AxiosResponse);
    });
    await expect(getData("/api/v1/devices")).rejects.toBeInstanceOf(ApiError);
    expect(getToken()).toBe("");
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("does not trigger handler for login endpoint 401", async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);
    useAdapter(async (config) => {
      throw new AxiosError("bad creds", undefined, config, undefined, {
        status: 401,
        data: { code: "INVALID_CREDENTIALS", message: "bad" },
      } as AxiosResponse);
    });
    await expect(sendData("post", "/api/v1/auth/login", {})).rejects.toBeInstanceOf(ApiError);
    expect(handler).not.toHaveBeenCalled();
  });
});

describe("sendData", () => {
  it("returns undefined when data absent(logout 容忍)", async () => {
    useAdapter(
      async () => ({ status: 200, data: { code: "SUCCESS", message: "ok" } }) as AxiosResponse,
    );
    await expect(sendData("post", "/api/v1/auth/logout")).resolves.toBeUndefined();
  });
});
