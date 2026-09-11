<script setup lang="ts">
import { onMounted, ref } from "vue";
import { fetchDevices } from "@/features/resource/api";
import type { DeviceRow } from "@/entities/resource";

const rows = ref<DeviceRow[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const search = ref("");
const loading = ref(false);
const error = ref("");

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const pg = await fetchDevices({
      page: page.value,
      pageSize: pageSize.value,
      search: search.value,
    });
    rows.value = pg.items ?? [];
    total.value = pg.total ?? 0;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <div class="toolbar">
      <el-input
        v-model="search"
        placeholder="按编码/名称搜索"
        clearable
        style="width: 260px"
        @change="((page = 1), load())"
      />
      <el-button type="primary" plain @click="((page = 1), load())"> 查询 </el-button>
    </div>
    <el-alert v-if="error" type="error" :title="error" :closable="false" />
    <el-table v-loading="loading" :data="rows" size="default">
      <el-table-column prop="code" label="编码" min-width="140" />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column
        prop="heightU"
        label="高度(U)"
        width="90"
        align="right"
        class-name="tabular-nums"
      />
      <el-table-column prop="lifecycleStatus" label="状态" width="130">
        <template #default="{ row }">
          <el-tag :type="row.lifecycleStatus === 'RUNNING' ? 'success' : 'info'" size="small">
            {{ row.lifecycleStatus }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="位置" min-width="120">
        <template #default="{ row }">
          <span v-if="row.currentPosition" class="tabular-nums">
            {{ row.currentPosition.rackCode }} U{{ row.currentPosition.startU }}-{{
              row.currentPosition.endU
            }}
          </span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="page"
      v-model:page-size="pageSize"
      :total="total"
      layout="total, prev, pager, next"
      @current-change="load"
    />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  gap: var(--ale-space-2);
  margin-bottom: var(--ale-space-3);
}
.muted {
  color: var(--ale-ink-500);
}
</style>
