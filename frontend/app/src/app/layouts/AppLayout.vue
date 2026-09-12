<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useSessionStore } from "@/features/auth/session";
import {
  getRouteFlag,
  setRouteFlag,
  legacyUrlFor,
  overridesUnlocked,
  type RouteFlag,
} from "@/app/flags";
import { reportError } from "@/api/errors";

const session = useSessionStore();
const route = useRoute();
const router = useRouter();

const menu = computed(() => [
  { path: "/", label: "资源树", module: "tree", auth: true },
  { path: "/devices", label: "设备列表", module: "devices", auth: true },
  { path: "/admin", label: "系统管理", module: "admin", auth: session.isAdmin },
]);

const currentModule = computed(() => (route.meta.flagModule as string | undefined) ?? "tree");
/** 当前模块的界面版本(localStorage 持久;旧 bundle 侧用 ?ale_flags=xxx:new 切回) */
const uiVersion = ref<RouteFlag>(getRouteFlag(currentModule.value));

function switchUi(value: RouteFlag) {
  // 模块级回退审计(P1-D):经遥测通道留服务端痕迹(message 白名单化,不含业务数据)
  reportError({
    message: `ui-flag-switch: ${currentModule.value}→${value}`,
    apiCode: "FLAG_SWITCH",
  });
  setRouteFlag(currentModule.value, value);
  if (value === "legacy") {
    // 整页跳出至旧 bundle(SPA 无法渲染旧路由)
    window.location.href = legacyUrlFor(currentModule.value);
  }
}

async function doLogout() {
  await session.logout();
  router.push("/login");
}
</script>

<template>
  <el-container class="layout">
    <el-header class="header">
      <span class="brand"> <span class="brand-mark">ALE</span> 机柜管理 </span>
      <div class="header-right">
        <el-select
          v-if="overridesUnlocked()"
          :model-value="uiVersion"
          size="small"
          class="ui-switch"
          data-test="ui-version-switch"
          @change="switchUi"
        >
          <el-option label="新版界面" value="new" />
          <el-option label="旧版界面" value="legacy" />
        </el-select>
        <el-dropdown v-if="session.user">
          <span class="user" data-test="user-menu">
            {{ session.user.displayName || session.user.username }}
            ({{ session.isAdmin ? "管理员" : "用户" }})
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item data-test="logout" @click="doLogout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-header>
    <el-container>
      <el-aside :width="'220px'" class="sidebar">
        <el-menu :default-active="route.path" router>
          <el-menu-item v-for="m in menu.filter((x) => x.auth)" :key="m.path" :index="m.path">
            {{ m.label }}
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout {
  height: 100%;
}
.header {
  height: var(--ale-header-h);
  background: var(--ale-primary-deep);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--ale-space-4);
}
.brand {
  font-size: 18px;
  font-weight: 600;
}
.brand-mark {
  color: #fff;
  letter-spacing: 1px;
  margin-right: var(--ale-space-2);
  border-bottom: 2px solid var(--ale-primary-light);
}
.header-right {
  display: flex;
  align-items: center;
  gap: var(--ale-space-3);
}
.ui-switch {
  width: 120px;
}
.user {
  color: var(--ale-purple-100);
  cursor: pointer;
}
.sidebar {
  background: var(--ale-surface);
  border-right: 1px solid var(--ale-line-soft);
}
.main {
  background: var(--ale-surface-soft);
}
</style>
