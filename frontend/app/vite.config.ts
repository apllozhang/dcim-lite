import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import AutoImport from "unplugin-auto-import/vite";
import Components from "unplugin-vue-components/vite";
import { ElementPlusResolver } from "unplugin-vue-components/resolvers";

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({ resolvers: [ElementPlusResolver()] }),
    Components({ resolvers: [ElementPlusResolver()] }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // 本地开发直连后端(与旧 ALE 部署形态一致:同源 /api 反代)
      "/api": { target: process.env.ALE_API_PROXY ?? "http://127.0.0.1:19080", changeOrigin: true },
      "/health": {
        target: process.env.ALE_API_PROXY ?? "http://127.0.0.1:19080",
        changeOrigin: true,
      },
      "/metrics": {
        target: process.env.ALE_API_PROXY ?? "http://127.0.0.1:19080",
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "jsdom",
    include: ["tests/**/*.test.ts"],
  },
});
