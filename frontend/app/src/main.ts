import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "@/app/App.vue";
import router from "@/app/router";
import { setUnauthorizedHandler, fetchReady } from "@/api/client";
import { reportError } from "@/api/errors";
import "@/design-system/tokens.css";

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 全局 401 收口(P1-R06):任意接口 401(非登录)→ 清 token → 回登录页
setUnauthorizedHandler(() => {
  if (!location.pathname.includes("/login")) {
    router.push("/login");
  }
});

// 会话恢复:有 token 时校验 /auth/me;失效则由 401 handler 收口
const session = (async () => {
  const { useSessionStore } = await import("@/features/auth/session");
  const store = useSessionStore();
  await store.whoami();
  return store;
})();

// 后端探活(P0-B:/health/ready 交付声明):不可达时上报(登录页与横幅消费)
const readiness = fetchReady().then((ok) => {
  if (!ok) {
    reportError({ message: "backend /health/ready unreachable" });
  }
  return ok;
});

void Promise.all([session, readiness]).finally(() => {
  app.mount("#app");
});
