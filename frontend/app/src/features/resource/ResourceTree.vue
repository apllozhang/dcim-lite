<script setup lang="ts">
import { onMounted, ref } from "vue";
import { fetchResourceTree, type TreeNode } from "@/features/resource/api";
import { reportError } from "@/api/errors";

const tree = ref<TreeNode[]>([]);
const loading = ref(false);
const error = ref("");
const loaded = ref(false);

async function load() {
  loading.value = true;
  error.value = "";
  try {
    tree.value = await fetchResourceTree();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
    reportError({ message: `resource-tree load failed: ${error.value}` });
  } finally {
    loading.value = false;
    loaded.value = true;
  }
}

onMounted(load);
</script>

<template>
  <div v-loading="loading" data-test="resource-tree" class="tree-wrap">
    <el-alert v-if="error" type="error" :title="`资源树加载失败:${error}`" :closable="false">
      <el-button size="small" type="primary" plain @click="load">重试</el-button>
    </el-alert>
    <el-empty
      v-else-if="loaded && tree.length === 0"
      description="暂无资源"
      data-test="tree-empty"
    />
    <el-tree
      v-else
      :data="tree"
      node-key="id"
      :props="{ label: 'label', children: 'children' }"
      default-expand-all
      data-test="tree"
    >
      <template #default="{ data }">
        <span class="node" :data-test="`tree-${data.kind}`">
          <span class="node-name">{{ data.label }}</span>
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
