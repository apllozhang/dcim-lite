/**
 * 会话 store:登录/登出/当前用户。
 * 对应契约:GET /auth/captcha、POST /auth/login、GET /auth/me、POST /auth/logout。
 */
import { defineStore } from "pinia";
import { getData, sendData, setToken, getToken } from "@/api/client";

export interface SessionUser {
  id: string;
  username: string;
  displayName: string;
  roles: { code: string }[];
}

export interface Captcha {
  id: string;
  image: string; // data:image/svg+xml;base64,...
}

export const useSessionStore = defineStore("session", {
  state: () => ({
    user: null as SessionUser | null,
    token: getToken(),
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    isAdmin: (s) => !!s.user?.roles.some((r) => r.code === "system_admin"),
  },
  actions: {
    async captcha(): Promise<Captcha> {
      return getData<Captcha>("/api/v1/auth/captcha");
    },
    async login(username: string, password: string, captchaId: string, captcha: string) {
      const data = await sendData<{ token: string }>("post", "/api/v1/auth/login", {
        username,
        password,
        captchaId,
        captcha,
      });
      this.token = data.token;
      setToken(data.token);
      await this.whoami();
    },
    async whoami() {
      if (!this.token) return;
      this.user = await getData<SessionUser>("/api/v1/auth/me");
    },
    async logout() {
      try {
        await sendData("post", "/api/v1/auth/logout");
      } finally {
        this.token = "";
        this.user = null;
        setToken("");
      }
    },
  },
});
