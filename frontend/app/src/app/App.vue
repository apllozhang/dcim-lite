<script setup lang="ts">
import { onErrorCaptured, ref } from "vue";
import { reportError } from "@/api/errors";

const fatal = ref("");

// 错误边界 + 上报(P1-R05):渲染/逻辑异常不白屏,并经 ErrorReporter 落日志
onErrorCaptured((err) => {
  fatal.value = err instanceof Error ? err.message : String(err);
  reportError({ message: `boundary: ${fatal.value}` });
  return false;
});
</script>

<template>
  <el-alert
    v-if="fatal"
    type="error"
    title="页面出现异常,请刷新重试"
    :description="fatal"
    :closable="false"
    style="margin: 24px"
    data-test="fatal-boundary"
  />
  <router-view v-else />
</template>
