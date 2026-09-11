import { defineConfig } from "playwright/test";

// E2E:由 webServer 显式启动前端(vite preview);后端由运行环境另行启动
// (CI 中由 frontend job 起 Go 二进制 + PostgreSQL;本地用 ALE_API_PROXY 指向已有后端)
export default defineConfig({
  testDir: "./e2e",
  timeout: 30000,
  use: {
    baseURL: process.env.ALE_APP_BASE ?? "http://localhost:5173",
  },
  webServer: {
    command: "npm run preview",
    url: "http://localhost:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 60000,
  },
});
