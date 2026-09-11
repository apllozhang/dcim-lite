import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "@/app/App.vue";
import router from "@/app/router";
import "@/design-system/tokens.css";

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 会话恢复:有 token 时校验 /auth/me,失效则清除(路由守卫随后回登录页)
const session = (async () => {
  const { useSessionStore } = await import("@/features/auth/session");
  const store = useSessionStore();
  try {
    await store.whoami();
  } catch {
    store.token = "";
    localStorage.removeItem("ale.token");
  }
  return store;
})();

void session.finally(() => {
  app.mount("#app");
});
