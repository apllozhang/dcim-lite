<script setup lang="ts">
import { onMounted, ref } from "vue";
import { fetchDevices, fetchDeviceTypes, type Device, type DeviceType } from "@/features/resource/api";
import { reportError } from "@/api/errors";
import {
  LIFECYCLE_STATUS_OPTIONS,
  lifecycleLabel,
  lifecycleTagType,
} from "@/features/device/statusLabel";

const rows = ref<Device[]>([]);
const types = ref<DeviceType[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const search = ref("");
const statusFilter = ref("");
const typeFilter = ref("");
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
      lifecycleStatus: statusFilter.value || undefined,
      typeId: typeFilter.value || undefined,
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

onMounted(async () => {
  load();
  // 类型筛选选项(独立加载,失败不阻断列表——下拉退化为空)
  try {
    types.value = await fetchDeviceTypes();
  } catch {
    types.value = [];
  }
});
</script>

<template>
  <div data-test="device-list">
    <div class="toolbar">
      <el-input
        v-model="search"
        placeholder="按编码/名称搜索"
        clearable
        style="width: 220px"
        data-test="device-search"
        @change="resetAndLoad"
      />
      <el-select
        v-model="statusFilter"
        placeholder="生命周期"
        clearable
        style="width: 150px"
        data-test="device-status-filter"
        @change="resetAndLoad"
      >
        <el-option
          v-for="o in LIFECYCLE_STATUS_OPTIONS"
          :key="o.value"
          :label="o.label"
          :value="o.value"
        />
      </el-select>
      <el-select
        v-model="typeFilter"
        placeholder="设备类型"
        clearable
        filterable
        style="width: 170px"
        data-test="device-type-filter"
        @change="resetAndLoad"
      >
        <el-option v-for="t in types" :key="t.id" :label="t.name ?? t.code" :value="t.id" />
      </el-select>
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
      <el-table-column prop="lifecycleStatus" label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="lifecycleTagType(row.lifecycleStatus)" size="small" data-test="device-status-tag">
            {{ lifecycleLabel(row.lifecycleStatus) }}
          </el-tag>
        </template>
      </el-table-column>
      <!-- A 族读模型首批:currentPosition 由后端读接口返回(未在位设备省略) -->
      <el-table-column label="位置" min-width="170">
        <template #default="{ row }">
          <span v-if="row.currentPosition" data-test="device-position" class="tabular-nums">
            {{ row.currentPosition.rack?.code ?? "" }} · U{{ row.currentPosition.startU }}-{{
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
  color: var(--ale-text-secondary, #909399);
}
</style>
