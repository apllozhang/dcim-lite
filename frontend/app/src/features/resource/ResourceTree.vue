<script setup lang="ts">
import { onMounted, ref } from "vue";
import { fetchResourceTree } from "@/features/resource/api";
import type { DataCenterNode } from "@/entities/resource";

const tree = ref<DataCenterNode[]>([]);
const loading = ref(false);
const error = ref("");

onMounted(async () => {
  loading.value = true;
  try {
    tree.value = await fetchResourceTree();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div v-loading="loading" class="tree-wrap">
    <el-alert v-if="error" type="error" :title="error" :closable="false" />
    <el-empty v-else-if="!loading && tree.length === 0" description="暂无资源" />
    <el-tree
      v-else
      :data="tree"
      node-key="id"
      props="{ label: 'name', children: 'rooms' }"
      default-expand-all
    >
      <template #default="{ data }">
        <span class="node">
          <span class="node-name">{{ data.name }}</span>
          <span class="node-code tabular-nums">{{ data.code }}</span>
        </span>
      </template>
    </el-tree>
  </div>
</template>

<style scoped>
.tree-wrap {
  min-height: 200px;
}
.node {
  display: flex;
  gap: var(--ale-space-2);
  align-items: baseline;
}
.node-code {
  color: var(--ale-ink-500);
  font-size: 12px;
}
</style>
