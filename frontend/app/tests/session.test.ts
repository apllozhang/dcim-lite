import { afterEach, describe, expect, it, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import type { Mock } from "vitest";
import { useSessionStore } from "@/features/auth/session";
import { getToken, setToken, setUnauthorizedHandler } from "@/api/client";

/** P1-R4:经 fetch stub,登录/whoami/logout 走完整 openapi-fetch 管道 */
function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function useFetch(fn: (input: string) => Promise<Response>): Mock {
  const spy = vi.fn(fn as (input: unknown) => Promise<Response>);
  vi.stubGlobal("fetch", spy);
  return spy;
}

afterEach(() => {
  vi.unstubAllGlobals();
  setToken("");
  setUnauthorizedHandler(() => {});
});

describe("session store", () => {
  it("login persists token and loads user(isAdmin 派生)", async () => {
    setActivePinia(createPinia());
    let call = 0;
    useFetch(async () => {
      call += 1;
      if (call === 1) {
        return jsonResponse(200, { code: "SUCCESS", message: "ok", data: { token: "t-abc" } });
      }
      return jsonResponse(200, {
        code: "SUCCESS",
        message: "ok",
        data: { id: "u1", username: "admin", roles: [{ code: "system_admin" }] },
      });
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
    useFetch(async () => {
      throw new Error("net");
    });
    const s = useSessionStore();
    s.token = "t-x";
    await s.logout();
    expect(s.token).toBe("");
    expect(getToken()).toBe("");
  });

  it("whoami failure keeps anonymous state", async () => {
    setActivePinia(createPinia());
    useFetch(async () => {
      throw new Error("boom");
    });
    const s = useSessionStore();
    s.token = "t-y";
    await s.whoami();
    expect(s.user).toBeNull();
  });

  it("whoami 401 clears store token(e2e 实测守卫回弹根因)", async () => {
    setActivePinia(createPinia());
    useFetch(async () => jsonResponse(401, { code: "UNAUTHORIZED", message: "expired" }));
    const s = useSessionStore();
    s.token = "stale-token";
    await s.whoami();
    expect(s.token).toBe("");
    expect(s.isLoggedIn).toBe(false);
  });
});
