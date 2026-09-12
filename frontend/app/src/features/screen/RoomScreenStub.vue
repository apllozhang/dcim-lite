<script setup lang="ts">
/**
 * 机房大屏占位(第 6 轮):暗色可视化组件按 v2 规格逐屏复刻中,
 * 主题层 ale-theme.css 的 .screen-page 暗色段已就位,随第 6 轮交付真身。
 */
</script>

<template>
  <div class="screen-stub" data-test="room-screen-stub">
    <div class="page-header">
      <div>
        <h2>机房资源大屏</h2>
        <p>数据中心与机房可视化展示。</p>
      </div>
    </div>
    <el-empty description="暗色大屏组件重建中（第 6 轮交付）——规格为 v2 逐屏截图与交互对照" />
  </div>
</template>

<style scoped>
.screen-stub {
  max-width: 1280px;
  margin: 0 auto;
}
.page-header {
  margin-bottom: 24px;
}
.page-header h2 {
  margin: 0 0 8px;
}
.page-header p {
  margin: 0;
  color: #6b7280;
}
</style>
