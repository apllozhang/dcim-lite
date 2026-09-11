# ale-app

ALE 自主前端核心源码(Phase 0 骨架)。架构决策见 `docs/frontend/FRONTEND-REBUILD-ADR.md`。

## 开发

```bash
npm install
npm run gen:api   # 从 docs/openapi.yaml 生成类型(契约漂移时 CI 会失败)
npm run dev       # http://127.0.0.1:5173(/api 代理到 127.0.0.1:19080,可用 ALE_API_PROXY 覆盖)
```

## 门禁

```bash
npm run lint      # eslint + prettier
npm test          # vitest
npm run build     # vue-tsc 类型检查 + vite build
```

## 目录

- `src/app/` 启动、路由、布局、错误边界
- `src/api/` HTTP 客户端与契约适配(`generated/` 为生成物,禁止手改)
- `src/design-system/` ALE tokens(唯一样式来源)
- `src/features/` 按领域组织的页面(auth / resource / device)
- `tests/` Vitest 单测;`e2e/` Playwright
