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
    <!-- 顶栏复刻 v2:浅底 + 左侧平台名 + 右侧工具区;紫色签名下边线由 ale-theme.css .topbar 提供 -->
    <el-header class="topbar" height="56px">
      <span class="topbar-title">基础资源管理平台</span>
      <div class="topbar-right">
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
        <!-- 文字被 ale-theme.css .sidebar .logo 置零,由伪元素渲染官方彩色 logo(与 v2 同源) -->
        <div class="logo">ALE 机柜管理</div>
        <el-menu :default-active="route.path" router>
          <el-menu-item v-for="m in menu.filter((x) => x.auth)" :key="m.path" :index="m.path">
            {{ m.label }}
          </el-menu-item>
        </el-menu>
      </el-aside>
      <el-main class="main main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout {
  height: 100%;
}
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #ffffff;
  padding: 0 var(--ale-space-4);
}
.topbar-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary, #1a1a1a);
}
.topbar-right {
  display: flex;
  align-items: center;
  gap: var(--ale-space-3);
}
.ui-switch {
  width: 120px;
}
.user {
  color: var(--el-text-color-regular, #4b4d50);
  cursor: pointer;
}
.sidebar {
  background: #ffffff;
  border-right: 1px solid var(--el-border-color-light, #e9e8e4);
}
.main {
  background: var(--el-bg-color-page, #f7f7f5);
}
</style>
