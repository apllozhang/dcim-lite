# 专家评审提交材料（2026-09-11，含第二轮复评响应增补）

> 致：外部评审专家
> 事由：按《PR6-PR7 新源码重建复评与下一轮指导-20260910》的三批整改顺序，第一、二、三批
> （工程与安全部分）已全部完成并通过独立复验，现提交全部原始证据供复核。
> **更新（2026-09-11 第二轮）**：专家复评意见（《最新代码复评与ALE自主前端启动决策-20260911》）
> 已给出"有条件 GO"——Phase 0/Phase 1 立即启动，生产写切换受 Gate G1 约束。本轮响应：
> P0-N1 已关闭（PR #25）、P1-N1/P1-N2 已关闭（PR #26/#27）、字段差异逐项分类完成
> （`docs/FIELD-DIFF-CLASSIFICATION-20260911.md`），详见 §6。

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
| E2E run id/退出码/截图/清理 | pytest 套件（run id、非零退出、E2E_ARTIFACT_DIR 截图）。**更正（复评 §10.1）**：原「teardown 零残留实测」表述不实——清理逻辑调用了不存在的 GET 路由、404 后静默跳过删除。复评 P1-N1 指出后已重写清理协议（创建即存 id+version 直删、addfinalizer、409 重试、失败即 fail、run ID 零残留断言），并以故意失败自测（`test_teardown_selfcheck.py`）证明用例异常时仍彻底清理；203 实测 flows 4/4 + selfcheck 双绿（PR #26/#27） |
| 干净构建日志与镜像 digest | 独立复验 §1（GitHub clone → build）+ nightly workflow 构建 |
| 迁移锁双实例竞争测试 | PR #12：双进程真 PG 并发冷启动，双副本正常、迁移恰一次 6/6 |
| 权限矩阵全路由执行结果 | `TestAuthMatrixAllRoutes` 72×3 态 |

## 4. 开放项（请专家给出意见）

1. **~~新前端源码替代的启动节奏~~** → 已裁定（复评 §1）：有条件 GO，Phase 0/Phase 1 启动中（见 §6）；
2. **字段级差异（70.3% → 更高）**——已按复评 §10.3 完成数据值差异与形状差异的分开统计和
   逐项分类（`docs/FIELD-DIFF-CLASSIFICATION-20260911.md`）：需要复刻动作的全部划入前端
   替代范围（A 族嵌套填充 + D 族包装/排序），数据值噪声与厂商空壳维持登记不修；
3. 第三批未纳入本轮的两项（共享限流/撤销的多实例化、可信代理设置）依赖部署形态决策
   （单实例形态下不阻塞，边界见 COMPAT-DECISIONS「已知边界」）。

## 5. 当前定位

与复评一致：**后端重建取得决定性进展（差分 315/315 断裂 0、并发不变式与厂商一致）、
旧前端可兼容运行的单实例内部试运行版**。复评提出的生产候选门禁中，除「新前端替代」外的
各项（5 个 P0、race、独立复验、备份演练除外）均已具备可复核证据。

## 6. 第二轮复评响应增补（2026-09-11）

### 6.1 复评 §10.2：独立复验 commit 继承声明

独立复验（`INDEPENDENT-VERIFY-20260911.md` 七项矩阵）执行于 commit `1608777`；专家评估
提交为 `9c2c0a5`。两者间的增量提交为三个 PR，全部有 CI 三 job 绿 + nightly 差分绿覆盖：

| 增量 | 内容 | 继承证据 |
|---|---|---|
| PR #22 | OpenAPI 全量 schema（纯文档 + schema 契约测试） | CI quality（含 TestOpenAPISchemaContract）+ integration 绿 |
| PR #23 | /metrics + 事务审计 + govulncheck/SBOM（3 个可达漏洞修复） | CI 三 job 绿 + supply-chain 首跑抓漏记录 |
| PR #24 | 证据地图（本文档） | 纯文档 |

此后 `9c2c0a5` → `7ce8a57`（本轮四批）的增量见 §6.2,均逐项带验收证据；203 侧另以
「PR25 差分验证跑」(与 nightly 同款冻结基线,真实断裂 0,兼容率 82.0%)复核了
`48636f3` 的运行时行为。

### 6.2 本轮四批整改（复评派工 B01/B02/T01/T02/C01）

| 派工 | 状态 | 证据 |
|---|---|---|
| B01+B02(P0-N1 PDU 插座聚合锁协议 + status 收权) | **已关闭**(PR #25,main `48636f3`) | 5 类并发集成测试(race + 真 PG);golden 套件 315/315;nightly 同款基线差分断裂 0;status 收权按 INTENTIONAL 登记为 C10(套件用例不传 status,门禁零影响) |
| T01(P1-N1 pytest 清理) | **已关闭**(PR #26/#27,main `7ce8a57`) | 重写清理协议 + `test_teardown_selfcheck.py` 故意失败自测;203 实测 flows 4/4 + selfcheck 双绿 |
| T02(P1-N2 Python 自测入 CI) | **已关闭**(PR #26) | 三工具更名 selfcheck_*.py + UTF-8 输出;quality job 逐个执行 + pytest 零收集断言;**自测入库前即抓到真实漂移**:coverage-requirements 66 vs OpenAPI 72(PR #22 后),已对齐 |
| C01(P1-N3 字段差异分类) | **已关闭** | `FIELD-DIFF-CLASSIFICATION-20260911.md`:53 项 = 数据值噪声 ~12 + 形状差异 ~41,复刻项全部划入前端替代范围 |

### 6.3 Gate G1 门禁进度（复评 §7)

| G1 条件 | 状态 |
|---|---|
| P0-N1 修复 + 并发测试 | ✅ PR #25 |
| pytest 清理修复 + 故意失败零残留 | ✅ PR #26/#27 |
| 字段差异逐项分类 + 数据值差异关闭 | ✅ 分类完成;数据值差异实为库状态噪声(非缺陷) |
| 设备移位与供电连接规则(D01) | ⏳ 待业务确认(工程侧建议方案 A:有活动连接禁止移位) |
| 新前端 CI、错误监控、feature flag 回退 | ⏳ Phase 0 交付物 |
| 隔离环境只读双跑验收 | ⏳ Phase 1 交付物 |

### 6.4 复评 §10.4/§10.5 落实

- Python 证据工具自测已入 CI(`.github/workflows/ci.yml` quality job),三工具 Linux 全绿、
  Windows 经 UTF-8 输出保护通过(本地实测 17+11+14 全过),pytest 零误收集有 CI 断言;
- 本材料定位从「新前端替代待评审」更新为「专家有条件 GO,Phase 0/Phase 1 执行中,
  生产写切换受 Gate G1 约束」。
