import { defineConfig } from "playwright/test";

// E2E 冒烟:需要可访问的后端(ALE_API_BASE,默认本地 5173 dev server 代理)
export default defineConfig({
  testDir: "./e2e",
  timeout: 30000,
  use: {
    baseURL: process.env.ALE_APP_BASE ?? "http://localhost:5173",
  },
});
