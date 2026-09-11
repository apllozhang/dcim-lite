import { createRouter, createWebHistory } from "vue-router";
import { useSessionStore } from "@/features/auth/session";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", component: () => import("@/features/auth/LoginView.vue") },
    {
      path: "/",
      component: () => import("@/app/layouts/AppLayout.vue"),
      children: [
        { path: "", component: () => import("@/features/resource/ResourceTree.vue") },
        { path: "devices", component: () => import("@/features/device/DeviceList.vue") },
      ],
    },
  ],
});

// 权限守卫:匿名一律回登录页(与后端 401 口径一致)
router.beforeEach((to) => {
  const session = useSessionStore();
  if (to.path !== "/login" && !session.isLoggedIn) return "/login";
  if (to.path === "/login" && session.isLoggedIn) return "/";
  return true;
});

export default router;
