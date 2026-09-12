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
          // 运行概览(第 6 轮;新 UI 独有页面,无 legacy 对应模块)
          path: "",
          component: () => import("@/features/dashboard/DashboardView.vue"),
        },
        {
          // 资源层级(复刻 v2 /data-centers;flag 模块 tree 的 legacy 跳转目标同为 /data-centers)
          path: "data-centers",
          component: () => import("@/features/resource/ResourceHierarchy.vue"),
          meta: { flagModule: "tree" },
        },
        {
          // 机房大屏(第 6 轮重建中,占位)
          path: "room-screen",
          component: () => import("@/features/screen/RoomScreenStub.vue"),
        },
        {
          path: "devices",
          component: () => import("@/features/device/DeviceList.vue"),
          meta: { flagModule: "devices" },
        },
        {
          path: "admin",
          component: () => import("@/features/admin/AdminUsers.vue"),
          // 三权限对照(P1-D 第 5 轮):与旧 bundle /admin 的 meta.requiresAdmin 同口径
          meta: { requiresAdmin: true },
        },
      ],
    },
  ],
});

// 权限守卫:匿名一律回登录页(与后端 401 口径一致);
// /admin 仅 system_admin(旧 bundle 同口径:非 admin 重定向首页)。
// flag 守卫(P0-R04):模块被切到 legacy 时整体跳出 SPA 到旧 bundle,
// location.replace 保证回退路径干净(不留 SPA 历史)。
// 导出纯函数便于单测(user 由 main.ts 在 mount 前 whoami 就绪)。
export function routeGuard(session: { isLoggedIn: boolean; isAdmin: boolean }) {
  return (to: {
    path: string;
    meta?: { requiresAdmin?: boolean; flagModule?: string };
  }): string | false | true => {
    if (to.path !== "/login" && !session.isLoggedIn) return "/login";
    if (to.path === "/login" && session.isLoggedIn) return "/";
    if (to.meta?.requiresAdmin && !session.isAdmin) return "/";
    const module = to.meta?.flagModule;
    if (module && getRouteFlag(module) === "legacy") {
      window.location.replace(legacyUrlFor(module));
      return false;
    }
    return true;
  };
}

// 延迟取 store(注册时 pinia 可能未安装;store 单例 getter 实时求值)
router.beforeEach((to) => routeGuard(useSessionStore())(to));

export default router;
