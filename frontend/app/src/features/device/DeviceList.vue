<script setup lang="ts">
import { onMounted, ref } from "vue";
import { fetchDevices, type Device } from "@/features/resource/api";
import { reportError } from "@/api/errors";

const rows = ref<Device[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const search = ref("");
const loading = ref(false);
const error = ref("");
const loaded = ref(false);

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const pg = await fetchDevices({
      page: page.value,
      pageSize: pageSize.value,
      search: search.value,
    });
    rows.value = pg.items;
    total.value = pg.total;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
    reportError({ message: `device list load failed: ${error.value}` });
  } finally {
    loading.value = false;
    loaded.value = true;
  }
}

function resetAndLoad() {
  page.value = 1;
  load();
}

onMounted(load);
</script>

<template>
  <div data-test="device-list">
    <div class="toolbar">
      <el-input
        v-model="search"
        placeholder="按编码/名称搜索"
        clearable
        style="width: 260px"
        data-test="device-search"
        @change="resetAndLoad"
      />
      <el-button type="primary" plain data-test="device-search-btn" @click="resetAndLoad">
        查询
      </el-button>
    </div>
    <el-alert v-if="error" type="error" :title="`设备列表加载失败:${error}`" :closable="false">
      <el-button size="small" type="primary" plain @click="load">重试</el-button>
    </el-alert>
    <el-empty
      v-else-if="loaded && rows.length === 0"
      description="暂无设备"
      data-test="device-empty"
    />
    <el-table v-else v-loading="loading" :data="rows" size="default" data-test="device-table">
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
      <!-- 位置列待 A 族读模型补齐(Device schema 暂无 currentPosition,P1-C) -->
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
</style>
