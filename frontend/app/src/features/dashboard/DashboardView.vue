<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { fetchDevices, fetchResourceTree } from "@/features/resource/api";
import type { TreeDataCenter } from "@/features/resource/api";

/**
 * 运行概览(第 6 轮 100% 复刻 v2 DashboardView):
 * 统计口径与 v2 相同——DC/机房/机柜数从 resource-tree 现算,设备数取 devices total;
 * 基线提示文案为 v2 原文。
 */
const router = useRouter();
const loading = ref(false);
const dcs = ref<TreeDataCenter[]>([]);
const deviceTotal = ref(0);

const roomCount = computed(() => dcs.value.reduce((sum, dc) => sum + (dc.rooms?.length ?? 0), 0));
const rackCount = computed(() =>
  dcs.value.reduce(
    (sum, dc) => sum + (dc.rooms ?? []).reduce((s, r) => s + (r.racks?.length ?? 0), 0),
    0,
  ),
);

const stats = computed(() => [
  { label: "数据中心", value: dcs.value.length, note: "基线 ≤ 15 个" },
  { label: "机房", value: roomCount.value, note: "每中心 ≤ 5 个" },
  { label: "机柜", value: rackCount.value, note: "总量 ≤ 200 个" },
  { label: "设备", value: deviceTotal.value, note: "总量 ≤ 3000 台" },
]);

onMounted(async () => {
  loading.value = true;
  try {
    const [tree, devices] = await Promise.all([
      fetchResourceTree(),
      fetchDevices({ page: 1, pageSize: 1 }),
    ]);
    dcs.value = tree;
    deviceTotal.value = devices.total ?? 0;
  } catch {
    ElMessage.error("运行统计加载失败");
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div v-loading="loading" class="dashboard-page" data-test="dashboard">
    <div class="page-header">
      <div>
        <h2>运行概览</h2>
        <p>展示当前数据中心、机房、机柜和设备资源统计。</p>
      </div>
      <el-tag type="success">服务在线</el-tag>
    </div>

    <el-row :gutter="16">
      <el-col v-for="s in stats" :key="s.label" :xs="24" :sm="12" :lg="6">
        <el-card class="stat-card">
          <div class="stat-label">{{ s.label }}</div>
          <div class="stat-value">{{ s.value }}</div>
          <div class="stat-note">{{ s.note }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="next-card">
      <template #header>
        <div class="card-header">
          <span>开发进度</span>
          <div class="header-actions">
            <el-button link @click="router.push('/data-centers')">进入资源管理</el-button>
            <el-button type="primary" link @click="router.push('/devices')">
              进入设备管理
            </el-button>
            <el-button type="success" link @click="router.push('/room-screen')">
              打开机房大屏
            </el-button>
          </div>
        </div>
      </template>
      <el-steps :active="4" finish-status="success">
        <el-step title="数据中心与机房" description="资源层级和基础台账" />
        <el-step title="机柜模板" description="42U模板和实例快照" />
        <el-step title="设备与U位" description="整U上架、迁移、审批和冲突校验" />
        <el-step title="机房大屏" description="数据中心与机房可视化展示" />
      </el-steps>
      <el-alert
        class="next-stage"
        type="info"
        :closable="false"
        title="下一阶段：PDU 插座明细、环境监控接口与外部系统集成适配器。"
      />
    </el-card>
  </div>
</template>

<style scoped>
/* 版面复刻自 v2 DashboardView-BKE-D84z.css(去 data-v 哈希) */
.dashboard-page {
  max-width: 1280px;
  margin: 0 auto;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}
.page-header h2 {
  margin: 0 0 8px;
}
.page-header p {
  margin: 0;
  color: #6b7280;
}
.stat-card {
  margin-bottom: 16px;
}
.stat-label,
.stat-note {
  color: #6b7280;
}
.stat-value {
  margin: 12px 0;
  font-size: 32px;
  font-weight: 700;
  color: #1f2937;
}
.next-card {
  margin-top: 8px;
}
.card-header,
.header-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-actions {
  gap: 8px;
}
.next-stage {
  margin-top: 24px;
}
</style>
