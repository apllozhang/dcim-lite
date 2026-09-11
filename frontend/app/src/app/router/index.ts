import { createRouter, createWebHistory } from "vue-router";
import { useSessionStore } from "@/features/auth/session";
import { getRouteFlag, legacyUrlFor } from "@/app/flags";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", component: () => import("@/features/auth/LoginView.vue") },
    {
      path: "/",
      component: () => import("@/app/layouts/AppLayout.vue"),
      children: [
        {
          path: "",
          component: () => import("@/features/resource/ResourceTree.vue"),
          meta: { flagModule: "tree" },
        },
        {
          path: "devices",
          component: () => import("@/features/device/DeviceList.vue"),
          meta: { flagModule: "devices" },
        },
      ],
    },
  ],
});

// 权限守卫:匿名一律回登录页(与后端 401 口径一致)
// flag 守卫(P0-R04):模块被切到 legacy 时整体跳出 SPA 到旧 bundle,
// location.replace 保证回退路径干净(不留 SPA 历史)。
router.beforeEach((to) => {
  const session = useSessionStore();
  if (to.path !== "/login" && !session.isLoggedIn) return "/login";
  if (to.path === "/login" && session.isLoggedIn) return "/";
  const module = to.meta.flagModule as string | undefined;
  if (module && getRouteFlag(module) === "legacy") {
    window.location.replace(legacyUrlFor(module));
    return false;
  }
  return true;
});

export default router;
