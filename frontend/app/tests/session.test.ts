import { afterEach, describe, expect, it } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import type { AxiosAdapter, AxiosResponse } from "axios";
import { useSessionStore } from "@/features/auth/session";
import { http, getToken, setToken } from "@/api/client";

/** 经 adapter mock,走完整拦截器链 */
function useAdapter(fn: AxiosAdapter) {
  http.defaults.adapter = fn;
}

afterEach(() => {
  delete http.defaults.adapter;
  setToken("");
});

describe("session store", () => {
  it("login persists token and loads user(isAdmin 派生)", async () => {
    setActivePinia(createPinia());
    let call = 0;
    useAdapter(async () => {
      call += 1;
      if (call === 1) {
        return {
          status: 200,
          data: { code: "SUCCESS", message: "ok", data: { token: "t-abc" } },
        } as AxiosResponse;
      }
      return {
        status: 200,
        data: {
          code: "SUCCESS",
          message: "ok",
          data: { id: "u1", username: "admin", roles: [{ code: "system_admin" }] },
        },
      } as AxiosResponse;
    });
    const s = useSessionStore();
    await s.login("admin", "pw", "cid", "1234");
    expect(s.token).toBe("t-abc");
    expect(getToken()).toBe("t-abc");
    expect(s.isAdmin).toBe(true);
  });

  it("logout clears token even when api fails", async () => {
    setActivePinia(createPinia());
    setToken("t-x");
    useAdapter(async (config) => {
      const err = new Error("net");
      throw Object.assign(err, { config, isAxiosError: false });
    });
    const s = useSessionStore();
    s.token = "t-x";
    await s.logout();
    expect(s.token).toBe("");
    expect(getToken()).toBe("");
  });

  it("whoami failure keeps anonymous state", async () => {
    setActivePinia(createPinia());
    useAdapter(async (config) => {
      throw Object.assign(new Error("boom"), { config });
    });
    const s = useSessionStore();
    s.token = "t-y";
    await s.whoami();
    expect(s.user).toBeNull();
  });

  it("whoami 401 clears store token(e2e 实测守卫回弹根因)", async () => {
    setActivePinia(createPinia());
    useAdapter(async (config) => {
      const { AxiosError } = await import("axios");
      throw new AxiosError("unauth", undefined, config, undefined, {
        status: 401,
        data: { code: "UNAUTHORIZED", message: "expired" },
      } as AxiosResponse);
    });
    const s = useSessionStore();
    s.token = "stale-token";
    await s.whoami();
    expect(s.token).toBe("");
    expect(s.isLoggedIn).toBe(false);
  });
});
