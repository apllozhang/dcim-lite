# 第三轮复评响应提交材料(2026-09-12)

> 对应:第三轮完整复评与 Phase 1 执行决策(2026-0911)§8 P0-A/P0-B/P1-C 三批次。
> 所有证据在仓库内可复跑;六要素(修改文件/测试名/测试输出/commit SHA/CI 链接/前后对照)逐项列出。
> 四词纪律执行口径:每项按「实现→部署→验证→关闭」分别陈述,严禁提前关闭。

## 0. 材料索引

| 材料 | 位置 |
|---|---|
| 本文档 | docs/EXPERT-SUBMISSION-ROUND3-20260912.md |
| 字段差异机器台账(P1-C①) | docs/field-diff-decisions.yaml |
| 字段门禁校验器 | tests/diff/field_gate.py |
| 确定性门禁校验器 | tests/diff/determinism_gate.py |
| 双库验证原始输出(203 取回) | out/dualA、out/dualB、out/readmodel(会话工作区,nightly artifact 为 CI 侧等价物) |
| 第二轮材料 | docs/EXPERT-SUBMISSION-20260911.md、docs/EXPERT-COVER-LETTER-20260911.md |
| C01 人工分类(台账的分类依据) | docs/FIELD-DIFF-CLASSIFICATION-20260911.md |

## 1. P0-A:并发矩阵关闭(复评 §4.1,PR #32,main 5e49c27)

- **修改文件**:internal/service/template_pdu.go(Connect 第三次重写:统一锁序 device→rack/PDU→socket,锁内复检位置)、internal/service/device.go(placeInTx MOVE 分支带电阻断)、internal/repository/{device,template_pdu}.go(GetDeviceLock/CountActiveConnectionsByDevice/GetSocketLock/UpdateSocketStatus)
- **测试名称**:internal/integration:integration_test.go append 段 TestDeviceMoveVsConnectRace(20 轮互斥断言)、TestDecommissionVsConnectRace、TestDecommissionVsDisconnectRace、TestSocketDeleteVsConnectRace 等 + SQL 终态不变量断言(无跨机柜活动连接、已下架设备无连接)
- **测试输出**:go test -race -tags integration 全绿;203 部署后 E2E selfcheck 双绿
- **commit**:2620d30→5e49c27;**CI**:PR #32 四 job 绿
- **前后对照**:复评指出 D01 首实现的「与 Connect 串行化无竞态窗口」为错误结论(Connect 设备读取逃逸事务)→ 现.Connect 与 Move 共享设备行锁并统一锁序;COMPAT-DECISIONS D01 条目就地更正(§6.2 同款纪律)

## 2. P0-B:前端真基础(复评 §4.2,PR #33,main 1914502)

七项全部落地:keyof paths 类型约束+Envelope 收紧(EMPTY_DATA)/flags.ts 三源运行时+legacy 路由守卫/ErrorReporter/401 全局收口/Playwright 进 CI(真后端真 PG)/.gitattributes LF/独立部署。

- **测试名称**:frontend/app tests/(7 文件 25 用例)+ e2e/shell.spec.ts(3 用例:匿名跳转/登录闭环含 401 收口/新→旧→新回退演练)
- **测试输出**:vitest 25 通过;playwright 3 通过(CI frontend job 真后端)
- **commit**:7adbb42、d8ebb32、92998e7→1914502;**CI**:PR #33 四 job 绿
- **关键过程证据**:CI e2e 首跑抓到 401 死循环(拦截器清了 localStorage 但未清 Pinia store 副本,守卫把 /login 弹回 /)→ 修复为 handler 同时清 store+whoami 401 清 token,并补单测。该缺陷在浏览器冒烟阶段不可稳定复现,由 CI 环境首次运行暴露——e2e 进 CI 的直接价值案例。
- **独立部署**:203 ale-app@19500(nginx 反代 backend:8080 与旧 frontend:80;/assets try_files 404 回落旧端),回退演练五步全过(legacy 最终落点为旧 bundle 自身守卫行为,已记录)

## 3. P1-C:可信字段门禁(复评 §8.3,PR #34 main ee6439a + PR #35 main 66efee140)

### 3.1 ①逐字段机器决策台账

- docs/field-diff-decisions.yaml:键=caseId+normalizedFieldPath(`[n]→[*]`,与 diff_compare_v2 抽取口径一致),**287 条目覆盖四方并集 394 条具体差异**(来源报告 340+双库竞态翻转 14+读模型候选行错位 40),生成器带 394/394 全覆盖断言
- 分类=复评 §4 P1-N3 五族;状态 OPEN 97/ACCEPTED 110/CLOSED 80;每条带 reason/owner/targetPhase/backendTest/frontendTest/evidence,并附 raceDependent(现身与否取决于并发胜负)与 residualNoise(形状已复刻、残余为行错位噪声)两个机器标记

### 3.2 ②确定性双库 fixture 验证(203 实跑,2026-09-11)

- 方法:同一候选镜像、两套 `down -v` 全新库各回放全量 golden 套件(run A/run B)
- 结果:A/B 335/339 EQUIVALENT、真实断裂 0;仅余 4 例并发竞态次生(S14-PAGE-FILTER-LIFE/COMBO total、S16-DEV-WAITING total、S18-ASSIGN-LAYOUT-13 free[0])——两侧恰一次成功不变量均 PASS
- 结论:**B 族「数值漂移=库内数据状态/用例序差异,非行为分歧」的归因成立且边界收窄**——候选侧除竞态次生外逐字节确定;57 条 B 族翻 NOISE_VERIFIED(证据已写入台账)
- 附带修复:diff_compare_v2 新增 root_key 归一化配对。原口径下 AUTH 探针用例 caseId 内嵌资源 UUID(每库不同),产生 6 个假 MISSING 且被排除出兼容率分母;root 配对后 vendor 基线比对同样修正(dualA vs 基线与来源报告完全一致:51 例/70.3%)

### 3.3 ③nightly 字段门禁(unknown_field_breaks==0)

- tests/diff/field_gate.py:①台账 schema/唯一键/枚举校验;②报告每条 FIELD_BREAK 必须有台账记录(**新增默认失败**);③OPEN 须仍在报告(raceDependent 豁免);④CLOSED 不得重现(residualNoise 豁免,回归防护由关联测试承担);⑤字段兼容率≥meta 基线 68.0(观测区间 68.6~71.0,竞态配对与形状复刻都会移动该值,下限设在观测最小值之下)
- tests/diff/determinism_gate.py:双库两轮回放,除台账登记噪声(CLOSED+residualNoise 同样放行)外任何 A/B 差异即失败
- 接线:ci.yml quality job 用冻结报告校验台账一致性(PR 即拦,不等 nightly);diff-replay.yml 抹卷重启+二次回放+两道门禁,artifacts 上报 field-gate.json(台账 sha256/commit/镜像 digest)与 determinism-gate.json
- 负向自测(全部按预期拦截):伪造未知差异 S99 / OPEN 改 CLOSED 后复现 / AB 报告注入 REAL_DEFECT 键

### 3.4 ④A 族读模型首批:currentPosition

- 后端:model.CurrentPositionView+PositionRackSummary(复刻厂商形状:仓位行+rack 摘要,Device 挂 gorm:"-" 视图字段);ListActivePositionsWithRack 批量装配(防 N+1);仅设备列表/详情读接口返回,写操作响应不带(与厂商 assign/move 响应形状一致,差分证实)
- 契约:openapi.yaml 增 Device.currentPosition+CurrentPosition+PositionRackSummary;前端 gen:api 类型再生,DeviceList 位置列消费
- 测试:集成 TestDeviceCurrentPositionReadModel(未在位省略/在位完整含 rack 摘要/下架后省略;32/32,203 一次性真 PG);前端 DeviceList.test.ts currentPosition 渲染(26/26)
- 回放验证:读模型候选 vs 冻结基线,字段兼容率 70.3→71.0,currentPosition 形状缺口消失;残余差异为行错位噪声(两库设备码含 run 标签,列表排序错位,与 lifecycleStatus 同类)→ 台账 80 条翻 **CLOSED(residualNoise=true)**,按四词纪律:关闭依据=回放证据+关联测试,残余噪声的回归防护由 backendTest/frontendTest 承担

### 3.5 ⑤差异↔测试关联

台账条目携带 backendTest/frontendTest;currentPosition 族已回填(internal/integration: TestDeviceCurrentPositionReadModel;frontend/app: DeviceList.test.ts);其余 OPEN 条目在对应页面/接口落地时回填,未回填的条目 targetPhase 均已登记。

## 4. Gate G1 清单(§9,13 项)

| # | 项 | 状态 |
|---|---|---|
| 1 | Move×Connect 并发漏洞关闭 | ✅ PR#32 |
| 2 | Decommission×active connection 规则 | ✅ PR#32(§4.3 裁定落地) |
| 3 | 51 个 FIELD_BREAK 用例及字段路径机器可追溯 | ✅ PR#34(394 差异/287 目,含四方并集) |
| 4 | A 族首批读模型 | ✅ PR#34(currentPosition) |
| 5 | OpenAPI 生成类型真正进入调用链 | ✅ PR#33 |
| 6 | feature flag 与回退演练 | ✅ PR#33 |
| 7 | ErrorReporter 与全局 401 | ✅ PR#33 |
| 8 | 新前端独立部署 | ✅ 19500 |
| 9 | 资源树+设备列表新旧双跑 | ⬜ P1-D |
| 10 | Playwright 进 CI | ✅ PR#33 |
| 11 | Windows/Linux lint/test/build 可复现 | ✅ .gitattributes+CI |
| 12 | nightly 阻断未知字段漂移 | ✅ PR#34,nightly #7 首验 SUCCESS |
| 13 | 供应链 action/工具/镜像 SHA/digest 固定 | ⬜ G1 尾项 |

## 5. 请专家裁定的开放项

1. **residualNoise 语义**:形状复刻后残余行错位噪声以 CLOSE+residualNoise 标记处理(而非永久 OPEN)——请评审该口径是否接受;若要求更严,可改为保留 OPEN+噪声豁免,台账语义等价但「已复刻」状态不可见。
2. **B 族翻转的证据强度**:双库验证在 203 实跑一次(335/339);nightly 将每日常态化产出该证据。是否需要 CI 内二次独立复跑再翻转?
3. **G1 尾项顺序**:P1-D 双跑(§7 验收口径)与供应链 SHA 固定,先做哪个?
4. **A 族后续优先序**:按复评建议为 conflictDevices→sockets→connections→写回显→包装/排序;如需配合 P1-D 页面节奏调整请指示。

## 6. 复现指引

```bash
# 台账一致性(CI quality job 同款)
python3 tests/diff/field_gate.py --ledger docs/field-diff-decisions.yaml \
  --report docs/DIFF-REPLAY-FINAL-20260911.json
# 集成套件(一次性真 PG)
docker run -d --name it-pg -e POSTGRES_USER=it -e POSTGRES_PASSWORD=it -e POSTGRES_DB=it -p 15432:5432 postgres:16-alpine
TEST_DATABASE_URL=postgres://it:it@127.0.0.1:15432/it?sslmode=disable go test -tags integration ./internal/integration/ -count=1
# 前端
cd frontend/app && npm ci && npm run gen:api && npm test && npm run build
```
