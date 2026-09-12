import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import AutoImport from "unplugin-auto-import/vite";
import Components from "unplugin-vue-components/vite";
import { ElementPlusResolver } from "unplugin-vue-components/resolvers";

const API_PROXY = process.env.ALE_API_PROXY ?? "http://127.0.0.1:19080";
const LEGACY_PROXY = process.env.ALE_LEGACY_PROXY ?? "http://127.0.0.1:19173";

export default defineConfig({
  plugins: [
    vue(),
    // importStyle:false:EP 样式由 main.ts 全量引入一次(级联顺序确定),
    // 自动导入只管组件与 API 的 JS 部分
    AutoImport({ resolvers: [ElementPlusResolver({ importStyle: false })] }),
    Components({ resolvers: [ElementPlusResolver({ importStyle: false })] }),
  ],
  // P0-R2:ALE_ 前缀为历史兼容(旧 ALE_LEGACY_BASE);客户端注入一律用 VITE_ 前缀
  envPrefix: ["VITE_", "ALE_"],
  define: {
    // ErrorReporter 的 release 标识(P1-R05):构建时注入,缺省 dev
    __APP_RELEASE__: JSON.stringify(process.env.APP_RELEASE ?? "dev"),
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // 本地开发直连后端(与部署形态一致:同源 /api 反代)
      "/api": { target: API_PROXY, changeOrigin: true },
      "/health": { target: API_PROXY, changeOrigin: true },
      "/metrics": { target: API_PROXY, changeOrigin: true },
      // 旧 ALE bundle(feature flag 的 legacy 跳转目标;不覆盖 oracle)
      "/legacy": { target: LEGACY_PROXY, changeOrigin: true },
    },
  },
  preview: {
    port: 5173,
    // vite preview 继承 server.proxy;/api 与 /legacy 均可用
  },
  test: {
    environment: "jsdom",
    include: ["tests/**/*.test.ts"],
    setupFiles: ["tests/setup.ts"],
    // element-plus 按需导入链含 .css,inline 走 vite 管道以正确处理
    server: { deps: { inline: ["element-plus"] } },
  },
});
