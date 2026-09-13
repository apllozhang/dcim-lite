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

// 菜单与图标复刻 v2(layout-enhance.js MENU 表的 SVG path;顺序与 v2 一致)
const menu = computed(() => [
  {
    path: "/",
    label: "运行概览",
    icon: "M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z",
    auth: true,
  },
  {
    path: "/data-centers",
    label: "资源层级",
    icon: "M4 6h16v2H4zm0 5h16v2H4zm0 5h10v2H4z",
    auth: true,
  },
  {
    path: "/room-screen",
    label: "机房大屏",
    icon: "M3 5h18v12H3zm4 15h10v2H7z",
    auth: true,
  },
  {
    path: "/racks",
    label: "机柜管理",
    icon: "M4 3h16v18H4zm2 2v3h12V5zm0 5v3h12v-3zm0 5v3h12v-3z",
    auth: true,
  },
  {
    path: "/rack-templates",
    label: "机柜模板",
    icon: "M4 4h7v7H4zm9 0h7v7h-7zM4 13h7v7H4zm9 0h7v7h-7z",
    auth: true,
  },
  {
    path: "/devices",
    label: "设备台账",
    icon: "M4 5h16v10H4zm2 12h12v2H6z",
    auth: true,
  },
  {
    path: "/admin",
    label: "系统管理",
    icon: "M12 8a4 4 0 100 8 4 4 0 000-8zm8.94 5l1.5 1.3-1.5 2.6-1.8-.5a7 7 0 01-1.7 1l-.3 1.9h-3l-.3-1.9a7 7 0 01-1.7-1l-1.8.5-1.5-2.6L5.06 13l-1.5-1.3 1.5-2.6 1.8.5a7 7 0 011.7-1L8.8 6.7h3l.3 1.9a7 7 0 011.7 1l1.8-.5 1.5 2.6L20.94 13z",
    auth: session.isAdmin,
  },
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
  <!-- 应用壳对齐 v2 DefaultLayout(评审 UI-P1-02):左侧栏全高(顶部 logo 块与顶栏同高
       60px 齐平,logo 占左上角),顶栏从侧栏右缘开始(紫色签名线只横跨右侧段)。
       菜单图标为评审已批准增强(v2 纯文字),保留。 -->
  <el-container class="layout">
    <el-aside :width="'220px'" class="sidebar">
      <!-- 文字被 ale-theme.css .sidebar .logo 置零,由伪元素渲染官方彩色 logo(与 v2 同源) -->
      <div class="logo">ALE 机柜管理</div>
      <el-menu :default-active="route.path" router>
        <el-menu-item v-for="m in menu.filter((x) => x.auth)" :key="m.path" :index="m.path">
          <svg
            class="menu-icon"
            viewBox="0 0 24 24"
            width="18"
            height="18"
            fill="currentColor"
            aria-hidden="true"
          >
            <path :d="m.icon" />
          </svg>
          {{ m.label }}
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container class="right-col">
      <el-header class="topbar" height="60px">
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
.menu-icon {
  display: inline-block;
  margin-right: 8px;
  vertical-align: -3px;
  opacity: 0.85;
}
:deep(.el-menu-item.is-active) .menu-icon {
  color: #6b489d;
  opacity: 1;
}
.main {
  background: var(--el-bg-color-page, #f7f7f5);
}
</style>
