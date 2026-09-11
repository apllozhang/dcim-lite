/**
 * 会话 store:登录/登出/当前用户。
 * 类型全部从 OpenAPI 生成 schema 派生(P0-R03);端点路径受 keyof paths 约束。
 */
import { defineStore } from "pinia";
import type { components } from "@/api/generated/schema";
import { ApiError, getData, sendData, setToken, getToken } from "@/api/client";
import { reportError } from "@/api/errors";

type User = components["schemas"]["User"];
type Captcha = components["schemas"]["Captcha"];
type LoginResult = components["schemas"]["LoginResult"];

export const useSessionStore = defineStore("session", {
  state: () => ({
    user: null as User | null,
    token: getToken(),
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    isAdmin: (s) => !!s.user?.roles?.some((r) => r.code === "system_admin"),
    /** 匿名化角色标签(错误上报用,不含用户名) */
    roleLabel: (s): string =>
      s.user?.roles?.some((r) => r.code === "system_admin")
        ? "system_admin"
        : s.user
          ? "user"
          : "anonymous",
  },
  actions: {
    async captcha(): Promise<Captcha> {
      return getData("/api/v1/auth/captcha");
    },
    async login(username: string, password: string, captchaId: string, captcha: string) {
      const data = await sendData("post", "/api/v1/auth/login", {
        username,
        password,
        captchaId,
        captcha,
      } as never);
      const result = data as LoginResult | undefined;
      if (!result?.token) throw new Error("登录响应缺少 token");
      this.token = result.token;
      setToken(result.token);
      await this.whoami();
    },
    async whoami() {
      if (!this.token) return;
      try {
        this.user = await getData("/api/v1/auth/me");
      } catch (e) {
        // 会话失效:401 拦截器清了 localStorage,这里同步清 store 副本,
        // 否则路由守卫仍按"已登录"放行(P1-R06 e2e 实测踩中)
        if (e instanceof ApiError && e.status === 401) {
          this.token = "";
        }
        reportError({
          message: `whoami failed: ${e instanceof Error ? e.message : String(e)}`,
          role: "anonymous",
        });
        this.user = null;
      }
    },
    async logout() {
      try {
        await sendData("post", "/api/v1/auth/logout");
      } catch (e) {
        // 登出接口失败(网络/后端 5xx)不阻断本地会话清理
        reportError({
          message: `logout api failed: ${e instanceof Error ? e.message : String(e)}`,
          role: this.roleLabel,
        });
      } finally {
        this.token = "";
        this.user = null;
        setToken("");
      }
    },
  },
});
