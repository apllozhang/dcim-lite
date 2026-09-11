<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useSessionStore } from "@/features/auth/session";

const session = useSessionStore();
const route = useRoute();
const router = useRouter();

const menu = computed(() => [
  { path: "/", label: "资源树", auth: true },
  { path: "/devices", label: "设备列表", auth: true },
  { path: "/admin", label: "系统管理", auth: session.isAdmin },
]);

async function doLogout() {
  await session.logout();
  router.push("/login");
}
</script>

<template>
  <el-container class="layout">
    <el-header class="header">
      <span class="brand"> <span class="brand-mark">ALE</span> 机柜管理 </span>
      <el-dropdown v-if="session.user">
        <span class="user" data-test="user-menu">
          {{ session.user.displayName || session.user.username }}
          ({{ session.isAdmin ? "管理员" : "用户" }})
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item data-test="logout" @click="doLogout"> 退出登录 </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
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
