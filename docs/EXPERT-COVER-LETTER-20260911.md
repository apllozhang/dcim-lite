# 致评审专家:第二轮响应导读信(2026-09-11)

> 提交人:dcim-lite 重建团队
> 对应复评:《最新代码复评与ALE自主前端启动决策-20260911》(评估提交 9c2c0a5)
> 当前评估提交:`67af2e2`(main)
> 一页索引:`docs/EXPERT-SUBMISSION-20260911.md` §6

## 一、本轮响应一览(与您派工清单一一对应)

| 您的派工 | 我们的动作 | 快速核验点 |
|---|---|---|
| B01+B02(P0-N1 socket 锁协议 + status 收权) | PR #25:全聚合锁序「PDU 行 → socket 行」,5 类并发测试 | `internal/service/template_pdu.go` 五个写路径;集成测试 `TestSocket*` |
| T01(P1-N1 E2E 清理失效) | PR #26/#27:清理协议重写 + 故意失败自测 | `tests/ale-regress/conftest.py`、`test_teardown_selfcheck.py`;203 实测 4/4 + 双绿 |
| T02(P1-N2 Python 自测入 CI) | PR #26:更名 selfcheck_*.py + UTF-8 + quality job | `.github/workflows/ci.yml`;自测入库前已抓到 requirements 66 vs OpenAPI 72 漂移 |
| C01(P1-N3 字段差异分类) | PR #28:53 项五类逐一归类,数据值/形状分开统计 | `docs/FIELD-DIFF-CLASSIFICATION-20260911.md` |
| D01(P1-N4 移位与供电规则) | PR #30:业务确认方案 A,带电禁移 409 DEVICE_POWERED | `placeInTx` 移位分支;`TestMoveWithActiveConnectionBlocked` |
| F01-F03(前端启动) | PR #29:Phase 0 全套(文档四件套 + 可构建骨架 + CI 四 job) | `docs/frontend/`、`frontend/app/`、ci.yml frontend job |

## 二、建议阅读顺序

1. `EXPERT-SUBMISSION-20260911.md` §6(响应增补,含 commit 继承声明与 G1 进度)
2. `FIELD-DIFF-CLASSIFICATION-20260911.md`(字段差异五类归类,回应您 70.3% 口径的关切)
3. `docs/frontend/FRONTEND-REBUILD-ADR.md`(绞杀式替代 + flag 回退,Phase 0 交付物入口)
4. 差分证据:nightly 首次**定时**触发 SUCCESS(run 34575314523,main 338dd6e);PR25 验证跑(与 nightly 同款冻结基线,断裂 0,兼容率 82.0%)

## 三、希望您重点复核的三处

1. **P0-N1 锁协议实现**:`Connect` 在 PDU 行锁后经 `GetSocketLock` 锁内重读,`UpdateSocketStatus` 零行回滚——请审阅该实现是否仍有逃逸路径。
2. **C10 status 收权的 INTENTIONAL 登记**:您建议"status 由连接事实推导或仅允许 Connect/Disconnect 修改",我们取后者(缓存列 + 收权),差分零影响;请确认该偏离口径可接受。
3. **D01 的同族未决项**:decommission 与活动连接的关系我们暂维持厂商行为(允许下架、连接悬空),已登记待业务明确——请评估是否需要与移位同口径收紧。

## 四、运行态证据(可直接访问复核)

- 203 线上栈(8080/5173)与 dev 栈(19080/19173)均运行 main `67af2e2`(镜像 0.2.0);
- 线上实测:登录/资源树/设备/审批策略全 200,匿名 401,/metrics 200,D01 拦截实测通过(接电移位 409 → 断开后 200);
- 部署说明:线上库按 main 迁移 0001~0007 重建(旧库 `pd_uid` 列名漂移,测试环境无生产数据,业务方确认;dump 备份留存)。

## 五、待您裁定的开放项

1. D01 同族项(decommission × 供电连接)是否同口径收紧;
2. 字段差异 A/D 族复刻已划入前端替代范围——复刻的优先级排序是否与 Phase 1 并行;
3. Phase 1 只读切片双跑的验收口径(信息架构对齐 vs 视觉连续性的权重)。
