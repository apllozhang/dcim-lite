/**
 * 会话 store:登录/登出/当前用户。
 * 类型全部从 OpenAPI 生成 schema 派生(P0-R03);P1-R4 起方法/路径/请求体/响应
 * 全部由 openapi-fetch 强类型约束(登录 body 不再需要 as never)。
 */
import { defineStore } from "pinia";
import type { components } from "@/api/generated/schema";
import { api, unwrapData, unwrapOptional, ApiError, setToken, getToken } from "@/api/client";
import { reportError } from "@/api/errors";

type User = components["schemas"]["User"];
type Captcha = components["schemas"]["Captcha"];

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
      return unwrapData(await api.GET("/api/v1/auth/captcha"), "GET /api/v1/auth/captcha");
    },
    async login(username: string, password: string, captchaId: string, captcha: string) {
      const result = await unwrapOptional(
        await api.POST("/api/v1/auth/login", { body: { username, password, captchaId, captcha } }),
        "POST /api/v1/auth/login",
      );
      if (!result?.token) throw new Error("登录响应缺少 token");
      this.token = result.token;
      setToken(result.token);
      await this.whoami();
    },
    async whoami() {
      if (!this.token) return;
      try {
        this.user = await unwrapData(await api.GET("/api/v1/auth/me"), "GET /api/v1/auth/me");
      } catch (e) {
        // 会话失效:401 middleware 清了 localStorage,这里同步清 store 副本,
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
        await unwrapOptional(await api.POST("/api/v1/auth/logout"), "POST /api/v1/auth/logout");
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
