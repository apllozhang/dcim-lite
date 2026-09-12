<script setup lang="ts">
import { onMounted, ref } from "vue";
import { fetchAdminUsers, type AdminUser } from "@/features/admin/api";
import { reportError } from "@/api/errors";

const users = ref<AdminUser[]>([]);
const loading = ref(false);
const error = ref("");
const loaded = ref(false);

onMounted(async () => {
  loading.value = true;
  try {
    users.value = await fetchAdminUsers();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
    reportError({ message: `admin users load failed: ${error.value}` });
  } finally {
    loading.value = false;
    loaded.value = true;
  }
});
</script>

<template>
  <div data-test="admin-users">
    <h2 class="page-title">系统管理 · 用户</h2>
    <el-alert v-if="error" type="error" :title="`用户列表加载失败:${error}`" :closable="false" />
    <el-empty v-else-if="loaded && users.length === 0" description="暂无用户" />
    <el-table
      v-else
      v-loading="loading"
      :data="users"
      size="default"
      data-test="admin-users-table"
    >
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column prop="displayName" label="显示名称" min-width="140" />
      <el-table-column prop="authSource" label="来源" width="100" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">
            {{ row.enabled ? "启用" : "停用" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="角色" min-width="160">
        <template #default="{ row }">
          <el-tag
            v-for="r in row.roles ?? []"
            :key="r.code"
            size="small"
            type="info"
            style="margin-right: 4px"
          >
            {{ r.name ?? r.code }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="lastLoginAt" label="最后登录" min-width="170" />
    </el-table>
  </div>
</template>

<style scoped>
.page-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 var(--ale-space-3);
}
</style>
