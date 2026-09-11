# 专家评审提交材料（2026-09-11）

> 致：外部评审专家
> 事由：按《PR6-PR7 新源码重建复评与下一轮指导-20260910》的三批整改顺序，第一、二、三批
> （工程与安全部分）已全部完成并通过独立复验，现提交全部原始证据供复核。
> **新前端源码替代的启动节奏，待本次评审意见后由业务方确定**（见 §4 开放项）。

## 1. 材料索引（全部在仓库内，均为可复跑证据而非摘要）

| 材料 | 位置 |
|---|---|
| 自评报告（含 §0/§0.1 两轮复评响应、三列式证据表） | `docs/SELF-REVIEW-FOR-EXPERT-20260910.md` |
| 差分回放最终报告（v2 口径：状态码+错误码+关键字段） | `docs/DIFF-REPLAY-FINAL-20260911.md`（+同名 .json 机器可读版） |
| 独立复验记录（复评 §11 七项矩阵） | `docs/INDEPENDENT-VERIFY-20260911.md` |
| 行为对齐与差异分类登记 | `docs/COMPAT-DECISIONS.md`（S1-S11、D1-D5、C1-C9、SHAPE_OR_DRIFT） |
| golden 套件与比对器 | `tests/sandbox/`（gt_final_v3.py 415 用例、diff_compare_v2.py） |
| 冻结厂商基线 | `tests/diff/baseline/` |

## 2. 复评问题关闭状态

### 第一批（P0，2026-09-10 关闭，PR #8/#9）

P0-01 最后管理员写偏斜、P0-02 导入越机房、P0-03 机柜删除 TOCTOU、P0-04 PDU 归档确认
TOCTOU、P0-05 审批版本竞态——全部以行锁/事务级 advisory lock 关闭，验收测试见自评报告 §0。

### 第二批（可信门禁，2026-09-11 关闭，PR #11/#12/#13/#14）

- 权限矩阵：`TestAuthMatrixAllRoutes` 全部 72 条路由 × 匿名/普通用户/管理员三态实测；
- `go test -race`：quality 与 integration 两 job 均在 Linux CI 的 race 检测下运行；
- E2E：ale-regress 重写为隔离 pytest 套件（环境变量化、变异门控、自建数据、零残留、非零退出），
  审批批准闭环首次端到端走通；
- 并发不变式差异（successes=1 vs 0）：**已关闭**——根因是候选 u-layout 缺 `free` 空闲段，
  修复后三处并发不变式与厂商逐路一致（`successes=1`，note 含每路明细）。

### 第三批（2026-09-11 关闭，PR #15/#16/#19/#22/#23）

- **session version**（P1-11）：密码重置/停用/软删递增 `users.session_version`，全部旧 token
  立即失效（`TestPasswordResetRevokesAllTokens` 等 2 个测试）；
- **迁移锁绑连接**（P1-07）：单事务 + 事务级 advisory lock；双副本并发冷启动实测恰一次；
- **Go 工具链统一**（P1-08）：全链路 1.25（依赖链强制，见 PR #19）；
- **OpenAPI 完整 schema**（P1-09）：71 操作 requestBodies + 全部实体/DTO schemas +
  错误码登记，`TestOpenAPISchemaContract` 断言 $ref 零悬空与 requestBody 全覆盖；
- **/metrics**：Prometheus 请求计数/时延（路由模板 label 防高基数）；
- **audit outbox 语义**：用户管理四操作、审批 approve/reject、导入提交的业务审计与
  业务变更**同事务**；载荷脱敏不含密码材料；
- **SBOM/漏洞扫描**：ci.yml supply-chain job（govulncheck + SPDX SBOM），**首跑即发现并
  修复三个可达漏洞**：GO-2026-5970（x/text DoS）、GO-2026-5960（excelize 解析 OOM，
  用户上传路径）、GO-2026-5004（pgx SQL 注入）。

### PR #7 的定位修正（复评 §11）

`tests/ale-regress/` 已改为「实验性诊断脚本」定位声明；浏览器回归由 `tests/ale-regress`
pytest 套件（隔离/幂等/非零退出）承担，并已在 203 线上栈两轮 4/4。

## 3. 复评 §12 证据包对照

| §12 要求 | 现物 |
|---|---|
| P0 对应 commit/测试名/前后结果 | 自评报告 §0 表 + 各 PR 描述 |
| Linux CI `go test -race` 输出 | PR #12 起每次 CI（最近：PR #23，三 job 全绿） |
| 并发竞态重复测试日志 | 集成测试 22 用例（含 100 轮互降级、15 轮删除竞态）本轮独立复验重跑 |
| 真实 PG 集成完整日志与版本 | PostgreSQL 16（postgres:16-alpine），22/22，22.7s |
| 350 条差分明细与未产出根因 | DIFF-REPLAY-FINAL：415 用例（含新增语义/证据场景），候选 315/315=基线，断裂 0，缺失仅 6 条且均为厂商自身 500 的非对称 |
| 87.1% 不一致项分类表 | v2 口径重列：兼容率 83.4%（可比 391），SYMMETRIC_SWAP/INTENTIONAL/SHAPE_OR_DRIFT 三类规则各有登记依据（COMPAT-DECISIONS） |
| E2E run id/退出码/截图/清理 | pytest 套件（run id、非零退出、E2E_ARTIFACT_DIR 截图、teardown 零残留实测） |
| 干净构建日志与镜像 digest | 独立复验 §1（GitHub clone → build）+ nightly workflow 构建 |
| 迁移锁双实例竞争测试 | PR #12：双进程真 PG 并发冷启动，双副本正常、迁移恰一次 6/6 |
| 权限矩阵全路由执行结果 | `TestAuthMatrixAllRoutes` 72×3 态 |

## 4. 开放项（请专家给出意见）

1. **新前端源码替代的启动节奏**——工程侧已备好替代所需的全部后端契约（openapi.yaml 全量
   schema + 权限矩阵 + UI 回归基线），按授权战略待评审意见后启动；
2. **字段级差异（70.3% → 更高）**——剩余为厂商响应嵌套深度/字段命名的完整复刻，工作量
   等同部分前端重建，是否纳入新前端替代范围一并解决，请专家裁定；
3. 第三批未纳入本轮的两项（共享限流/撤销的多实例化、可信代理设置）依赖部署形态决策
   （单实例形态下不阻塞，边界见 COMPAT-DECISIONS「已知边界」）。

## 5. 当前定位

与复评一致：**后端重建取得决定性进展（差分 315/315 断裂 0、并发不变式与厂商一致）、
旧前端可兼容运行的单实例内部试运行版**。复评提出的生产候选门禁中，除「新前端替代」外的
各项（5 个 P0、race、独立复验、备份演练除外）均已具备可复核证据。
