# dcim-lite 代码审查说明

> 给同事 / 评审同学：先看本文，再进目录，避免在编译产物里浪费时间。  
> 仓库：https://github.com/apllozhang/dcim-lite  
> 日期：2026-09-10

---

## 1. 这个仓库是什么

**dcim-lite**：轻量 DCIM（数据中心基础设施台账）后端 + 两套前端。

| 层 | 技术 | 位置 |
| --- | --- | --- |
| 后端 API | Go 1.24 · Gin · GORM · PostgreSQL 16 | 根目录 `cmd/` `internal/` `migrations/` |
| 开源 UI | 原生 JS，无 CDN | `frontend/clean/` |
| 内网 ALE UI | 编译 Vue3 包 + ALE 主题 + 增强脚本 | `frontend/ale/` |

运行方式见根目录 `README.md`（Docker / 本地 `go run`）。

---

## 2. 目录怎么读（建议顺序）

```text
1. README.md                     产品与启动
2. docs/R0-BASELINE.md           基线与证据优先级
3. docs/COMPAT-DECISIONS.md      有意修复项（500→409 等）
4. internal/router/router.go     全部路由一览
5. internal/service/**           业务规则（审查重点）
6. internal/handler/**           HTTP 入参/出参
7. internal/repository/**        SQL 与乐观锁
8. migrations/*.sql              库结构与排他约束
9. frontend/clean/**             自研 UI
10. frontend/ale/ 三个增强文件    见第 4 节
11. scripts/smoke-all.sh         集成回归清单
```

---

## 3. 后端：请重点审什么

| 优先级 | 文件/主题 | 看什么 |
| --- | --- | --- |
| P0 | `service/device.go` | U 位区间校验、生命周期、履历 |
| P0 | `migrations/0002_devices.up.sql` | `EXCLUDE gist` 排他约束是否正确 |
| P0 | `service/auth.go` + `middleware` | JWT、角色、未授权 401 |
| P0 | `response/` + `apperr/` | 统一信封、业务错误码 |
| P1 | `service/resource.go` | 编码唯一、有子资源禁删、父级停用禁建 |
| P1 | `service/resource_copy_move.go` | 深拷贝、跨中心迁移 |
| P1 | `service/admin.go` | 最后管理员、重置密码 |
| P1 | `service/approval.go` | 审批策略、版本冲突拒绝批准 |
| P1 | `service/template_pdu.go` | 模板快照；PDU 冲突 409 |
| P1 | `service/rack_import.go` | 两阶段导入、草稿一次性 |
| P2 | `service/captcha.go` | 4 位数字码、一次性、SVG |
| P2 | `service/ldap.go` | 配置读写；bind 仅 TCP 探测 |
| P2 | `cmd/server/main.go` | migration runner、种子角色/管理员 |

### 约定（和契约对齐）

- 成功：`{ "code":"SUCCESS", "message":"操作成功", "data", "requestId" }`
- 失败：无 `data`，带业务 `code`；头 `X-Request-Id`
- 乐观锁：资源 `PUT/DELETE` 用 `?version=`，冲突 `409 RESOURCE_VERSION`
- 软删除：`deleted_at`；唯一索引带 `WHERE deleted_at IS NULL`

### 有意修复（不要当 bug 报）

见 `docs/COMPAT-DECISIONS.md`：PDU/插座重复编码等由旧 500 改为 **409**。

---

## 4. 前端两套：审什么、别审什么

### 4.1 `frontend/clean/` — 全部可审（自研）

| 文件 | 内容 |
| --- | --- |
| `index.html` | 登录 + 主壳 |
| `app.js` | 验证码登录、资源树、机柜/设备表、分页排序 |
| `styles.css` | 中性紫主题（非 ALE 商标） |

对接同一套 API；可用 `window.DCIM_API_BASE` 指到任意后端。

### 4.2 `frontend/ale/` — 内网包，**只审增强层**

| 文件 | 是否本项目源码 | 说明 |
| --- | --- | --- |
| `assets/captcha-login.js` | **是** | 登录页注入验证码；XHR 拦截补 `captchaId` |
| `assets/table-ux.js` | **是** | 表头排序、列宽拖拽、上下横条、操作列折叠 |
| `assets/ale-theme.css` | **是** | ALE v4.1 色板、LOGO、表格换行/滚动条样式 |
| `index-BHgR2UnN.js`、`vue-core-*.js`、各 `*View-*.js` | **否** | 原产品**编译产物**，仅作行为参考 |
| `element-plus-*.js/css` | **否** | 第三方组件库打包 |
| `ale-logo*.png` | **否** | ALE 品牌资产 |

详见 `frontend/ale/REVIEW-NOTES.md`。

**审查增强脚本时注意：**

- 禁止在 `MutationObserver` 里无节制改 DOM（曾导致登录页卡死，已改成防抖/幂等 + history 钩子）
- 所有注入必须可重入、可跳过已处理节点

---

## 5. 必测场景（可对照 smoke）

`scripts/smoke-all.sh` 已覆盖，本地：

```bash
BASE=http://127.0.0.1:8080 ENV_FILE=.env bash scripts/smoke-all.sh
```

| 场景 | 期望 |
| --- | --- |
| 无 token 访问业务 API | 401 `UNAUTHORIZED` |
| 登录无验证码 | 400 `CAPTCHA_REQUIRED` |
| 重复编码 | 409 `DUPLICATE_CODE` |
| 错误 version | 409 `RESOURCE_VERSION` |
| 有子资源删除 | 409 `HAS_CHILDREN` |
| U 位重叠 | 409 `U_SLOT_CONFLICT` |
| 并发同 U 双上架 | 恰 1 成功 + 1 冲突 |
| 最后管理员停用/删除 | 409 `LAST_ADMIN` |
| 审批开启后 assign | 返回 PENDING，批准后 RUNNING |
| 导入草稿复用 | 400 `...DRAFT_EXPIRED` |

---

## 6. 安全与合规（审查必读）

| 项 | 说明 |
| --- | --- |
| `.env` | 不入库；示例见 `.env.example` |
| 默认口令 | 仅开发示例，生产必须改 |
| Postgres | 建议不对办公网暴露 |
| `frontend/ale` | 含商标与编译包，**不建议 Public 仓**；审查后可转 Private |
| Captcha | 仅基础防刷，不能替代限流/WAF |

---

## 7. 已知边界 / 未完成

- LDAP：仅配置 + TCP 探测，无真实目录 bind  
- 数据迁移：旧库 → rebuild 未做正式演练  
- 管理域审批/LDAP 页在 ALE UI 上仍依赖旧前端逻辑  
- 前端多语言：ALE 壳层可切中英，业务文案未全量 i18n  

---

## 8. 建议的 Review 输出格式

对每个问题请写清：

```text
文件:行号
严重级: P0/P1/P2
问题: ...
建议: ...
是否阻塞上线: 是/否
```

---

## 9. 联系与环境

| 项 | 值 |
| --- | --- |
| 开发部署 | 10.20.30.203 内网 rebuild 栈（UI :19173 / API :19080） |
| 隔离说明 | 与主栈 cabinet 5173/8080/5433 独立 |
| 冒烟 | `scripts/smoke-all.sh`（约 36 项） |

审查过程中改代码请走 PR；**不要**直接 force-push `main`。
