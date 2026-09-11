# FRONTEND-REBUILD-ADR:ALE 自主前端重建架构决策记录

> 日期:2026-09-11 | 状态:已接受(专家复评"有条件 GO",Phase 0 启动)
> 关联:《最新代码复评与ALE自主前端启动决策-20260911》§6/§7、`EXPERT-SUBMISSION-20260911.md` §6.3

## 1. 目标

1. **自主源码**:全部运行逻辑来自团队可读、可测试、可构建的源码;不复制混淆 bundle、不依赖对旧 DOM 打补丁。
2. **ALE 连续性**:视觉语言、信息架构、术语、关键操作体验与 ALE 产品保持一致(参照 `ALE-VISUAL-TOKENS.md` 与旧界面)。
3. **契约驱动**:直接消费后端正式 OpenAPI(`docs/openapi.yaml`),类型与 API client 由生成器产出,禁止页面手写散乱 HTTP 请求。
4. **可独立升级**:依赖可单独升级、安全漏洞可独立修复、可移植部署。

## 2. 非目标(Phase 0-1)

- 不做像素级机械复制;对齐的是业务信息、操作可发现性与视觉连续性。
- 不复刻厂商后端的历史怪异响应结构(见 `FIELD-DIFF-CLASSIFICATION-20260911.md` D/E 族区分)。
- 不在本阶段接管任何写操作。

## 3. 技术栈(已定)

| 层 | 选型 | 理由 |
|---|---|---|
| 框架 | Vue 3 + TypeScript | 与旧 ALE 生态一致,行为还原成本最低 |
| 构建 | Vite | 同上;构建/热更快 |
| 路由 | Vue Router 4 | 官方配套 |
| 状态 | Pinia | 官方推荐 |
| 组件库 | Element Plus + 自建 ALE token 主题层 | 旧界面即 EP 体系;主题收敛在 design-system,业务页面不散布样式覆盖 |
| API | openapi-typescript 生成类型 + 手写薄适配层 | 契约漂移在 CI 阻断 |
| 单测 | Vitest + Vue Test Utils | Vite 原生 |
| E2E | Playwright | 与现有 ale-regress 同栈 |
| 规范 | ESLint(flat config)+ Prettier + vue-tsc | CI 门禁 |

集成验收必须连接真实后端;MSW/契约 mock 仅用于前端本地开发。

## 4. 目录结构

```text
frontend/
├── legacy-ale/             # 冻结旧制品(manifest 见 legacy-ale-MANIFEST),仅作 oracle/过渡
├── app/                    # 自主前端(本 ADR 范围)
│   ├── src/
│   │   ├── app/            # 启动、路由、全局 provider、错误边界
│   │   ├── api/            # OpenAPI 生成代码(src/api/generated)+ 手写适配层
│   │   ├── design-system/  # ALE tokens、基础组件、布局
│   │   ├── features/       # auth / resource / device / ...按领域组织
│   │   ├── entities/       # 领域类型与展示模型
│   │   └── shared/         # 通用工具,不承载业务逻辑
│   ├── tests/              # Vitest
│   └── e2e/                # Playwright(新旧双跑脚本后续入 frontend/e2e)
└── docs → 见 docs/frontend/
```

## 5. 替换策略:绞杀者模式 + 路由级 feature flag

- 新页面按模块逐个上线,通过运行时 flag 在旧 ALE 与新页面之间切换,可单独回退;
- **实现状态(P0-B,`src/app/flags.ts`)**:取值优先级 URL 参数 `?ale_flags=module:legacy`
  > localStorage(`ale.flags`,用户级持久)> `window.__ALE_ROUTE_OVERRIDES__`(部署注入)> 默认 `new`;
  路由守卫在模块 flag=legacy 时整页跳转 `/legacy/<module 路径>`(dev server/nginx 反代旧 bundle,
  oracle 独立端口不受影响);应用壳提供模块级新旧切换下拉;`新→旧→新`回退由 e2e 用例固化;
- 旧 ALE bundle 保持原样运行,作为行为/视觉 oracle,直至 Phase 4 退出;
- 每个模块的完成定义遵循复评 §8(生成类型、三态权限、完整状态、409 冲突提示、防重复提交、单测+E2E、可单独回退、不依赖旧 bundle 运行时)。

## 6. 回滚原则

- flag 回退到旧路由 = 单模块秒级回滚;
- 新前端整体下线不影响旧 ALE 独立运行(两者静态目录隔离);
- 后端无任何为前端的破坏性改动;新前端只消费既有契约。

## 7. 里程碑与门禁

| 阶段 | 内容 | 出口标准 |
|---|---|---|
| Phase 0(本 ADR + 骨架) | 文档四件套、可构建骨架、类型生成、应用壳连 /auth/me 与 /health/ready | 干净 clone 可 lint/test/build;CI 全绿。**ready 声明已兑现(P0-B)**:启动时 `fetchReady()` 探活 `/health/ready`,不可达经 ErrorReporter 上报 |
| Phase 1 | 登录、布局、资源树、设备列表只读切片;新旧双跑 | 双跑通过,可 flag 回退 |
| Gate G1 | 复评 §7 六条件(含 D01 业务确认) | 通过后方可进入写操作预发布 |
| Phase 2/3/4 | 低风险写 → 高风险状态机 → 旧核心退出 | 复评 §7 各条 |
