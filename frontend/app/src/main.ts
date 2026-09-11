import { createApp } from "vue";
import { createPinia, setActivePinia } from "pinia";
import App from "@/app/App.vue";
import router from "@/app/router";
import { setUnauthorizedHandler, fetchReady } from "@/api/client";
import { reportError } from "@/api/errors";
import { useSessionStore } from "@/features/auth/session";
import "@/design-system/tokens.css";

const app = createApp(App);
const pinia = createPinia();
app.use(pinia);
app.use(router);
// 拦截器回调在 app 上下文之外触发,setActivePinia 保证可取 store
setActivePinia(pinia);

// 全局 401 收口(P1-R06):拦截器清 localStorage 后,这里同步清 store,
// 再回登录页——store 不同步会被路由守卫的"已登录"判断弹回。
setUnauthorizedHandler(() => {
  const session = useSessionStore();
  session.token = "";
  session.user = null;
  if (!location.pathname.includes("/login")) {
    router.push("/login");
  }
});

// 会话恢复:有 token 时校验 /auth/me;失效则由 401 收口处理
const session = (async () => {
  const store = useSessionStore();
  await store.whoami();
  return store;
})();

// 后端探活(P0-B:/health/ready 交付声明):不可达时上报
const readiness = fetchReady().then((ok) => {
  if (!ok) {
    reportError({ message: "backend /health/ready unreachable" });
  }
  return ok;
});

void Promise.all([session, readiness]).finally(() => {
  app.mount("#app");
});
