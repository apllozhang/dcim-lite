<script setup lang="ts">
import { onErrorCaptured, ref } from "vue";
import { ElAlert } from "element-plus";

const fatal = ref("");

// 错误边界:渲染/逻辑异常不白屏,统一展示并保留现场信息
onErrorCaptured((err) => {
  fatal.value = err instanceof Error ? err.message : String(err);
  console.error("[boundary]", err);
  return false;
});
</script>

<template>
  <el-alert
    v-if="fatal"
    type="error"
    :title="'页面出现异常,请刷新重试'"
    :description="fatal"
    :closable="false"
    style="margin: 24px"
  />
  <router-view v-else />
</template>
