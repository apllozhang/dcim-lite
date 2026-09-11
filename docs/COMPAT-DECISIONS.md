# Compatibility decisions

| ID | Topic | Decision | Class |
| --- | --- | --- | --- |
| D1 | Duplicate PDU code | 409 `DUPLICATE_CODE` | INTENTIONAL_FIX |
| D2 | Duplicate socket number | 409 | INTENTIONAL_FIX |
| D3 | Duplicate power-role connect | 409 `SOCKET_CONNECTED` | INTENTIONAL_FIX |
| D4 | Rack template snapshot | Persist jsonb on create | INTENTIONAL_FIX |
| V1 | Optimistic lock | `?version=` on PUT/DELETE | MATCH |

## 2026-09-10 安全加固批次（INTENTIONAL_FIX）

按代码评审报告（P0/P1）实施，以下行为变化均为有意修复，不是回归：

| ID | Topic | Decision | Class |
| --- | --- | --- | --- |
| S1 | 写接口授权边界 | 资源/设备/模板/PDU/导入的全部写操作要求 `system_admin`，普通用户仅保留只读查询（403 FORBIDDEN） | INTENTIONAL_FIX |
| S2 | 设备上架/移位/下架/退役 | 位置+占用+生命周期+履历单事务提交；履历写入失败视为业务失败整体回滚 | INTENTIONAL_FIX |
| S3 | 审批决定 | 单事务：条件更新审批单（PENDING + 可选 version 乐观锁）→ 设备变更；并发批准恰一次成功，失败方 409 `APPROVAL_STATE` | INTENTIONAL_FIX |
| S4 | 批量导入 commit | 全部行操作单事务；失败零残留且草稿回填可重试；token 仍一次性消费 | INTENTIONAL_FIX |
| S5 | 机柜删除保护 | 机柜内存在在位设备或 PDU 时拒绝删除（409 `HAS_CHILDREN`） | INTENTIONAL_FIX |
| S6 | PDU 连接一致性 | 连接创建/删除与插座状态更新同事务，不再出现状态漂移 | INTENTIONAL_FIX |
| S7 | 登录防爆破 | 连续失败 5 次锁定 15 分钟（401 `USER_LOCKED`）；登录端点每 IP 每分钟限 10 次（429 `RATE_LIMITED`） | INTENTIONAL_FIX |
| S8 | 登出即时生效 | JWT 增加 jti，登出后旧 token 进入黑名单立即失效（单实例内存） | INTENTIONAL_FIX |
| S9 | JWT 密钥强度 | production 环境强制 JWT_SECRET ≥ 32 字节，否则拒绝启动 | INTENTIONAL_FIX |
| S10 | 种子管理员 | 已存在的同名非管理员用户不再被静默提权，仅打告警日志 | INTENTIONAL_FIX |
| D5 | PDU 删除语义 | **有意偏离厂商**：厂商允许删除仍有插座的 PDU（200）；重建版把删除边界画在「连接」上——有活动连接则 409 `PDU_IN_USE`（提示先断开），无连接则允许删除并**级联软删插座** | INTENTIONAL_FIX |
| S11 | 请求体解析 | 空 body 合法（EOF），格式错误统一 400；不再把坏 JSON 当空对象执行 | INTENTIONAL_FIX |

### 已知边界（保持现状，多实例部署前必须替换）

- 验证码、导入草稿、token 黑名单、登录限流均为单实例内存实现；水平扩展需迁移到 Redis。
- LDAP Test 仍为 TCP 连通性探测，不能证明 bind/搜索可用（接口语义见 docs/CODE-REVIEW-GUIDE.md）。
### D5 补充：强制归档出口（现场处置）

正常删除在「有活动连接」时返回 409 `PDU_IN_USE`。对「PDU 报废但设备仍在用」（如分批搬迁先拆 PDU）这一低频但真实的场景，提供受控出口：

- `GET /api/v1/pdus/{id}/archive-impact`：先取影响清单（插座数、活动连接数、受影响设备明细）
- `POST /api/v1/pdus/{id}/force-archive`：须回报 `confirmConnections`（等于清单中的连接数，否则 409 `IMPACT_CONFIRMATION_REQUIRED`）且 `reason` 必填
- 执行：断开全部连接 + 下线插座 + 归档 PDU（单事务）
- 留痕：影响清单与原因写入 `audit_logs`（action=`FORCE_ARCHIVE`）

## 差分复验轮（2026-09-11）新增对齐与登记

第四轮差分回放（双侧干净沙箱 + 415 用例 golden 套件 + 关键字段扩展比对，报告见 docs/DIFF-REPLAY-FINAL-20260911.md）修复的兼容缺陷与新增登记：

### 本轮修复的行为对齐（厂商可实测）

| 编号 | 端点 | 修复 |
|---|---|---|
| C1 | POST /devices | 负/零 heightU 拒绝（400），此前静默回退默认高度 |
| C2 | POST /devices/{id}/assign | 在位设备重复上架返回 409 `DEVICE_POSITIONED`（此前 INVALID_RESOURCE） |
| C3 | DELETE /pdu-connections/{id} | 修复响应体双写（缺 query version 时 body 兜底成功仍追加错误段）；缺 version 统一 400 `INVALID_REQUEST` |
| C4 | POST /rack-templates/{id}/versions | stale version 校验（此前恒以当前版本条件更新，过期版本被接受） |
| C5 | POST /admin/approvals/{id}/approve、/reject | version 校验优先于状态检查：过期 version → `RESOURCE_VERSION_CONFLICT`；申请后设备被修改 → `RESOURCE_VERSION_CONFLICT`（此前混返 APPROVAL_STATE_CONFLICT） |
| C6 | POST /rooms/{id}/move、/racks/{id}/move | version 校验先于其他参数校验（厂商顺序） |
| C7 | GET /health/live | data.status 对齐厂商 `alive`（此前 `live`） |
| C8 | GET /racks/{id}/u-layout | 补齐 `free` 连续空闲段数组（厂商形状必需；缺失使依赖 free 选位的客户端全部失效） |
| C9 | 登录 | 停用账户 403（厂商状态码；此前 401） |

### 差异分类口径（v2 比对器，tests/sandbox/diff_compare_v2.py）

- **SYMMETRIC_SWAP**：并发对称用例（CLIENT0/1 胜负互换、同一审批单 approve/reject 决策互换）——胜负分配是时序，两侧「恰一次生效」不变量断言均各自 PASS；
- **INTENTIONAL（D 系列/D5）**：厂商 500 竞争缺陷修复为确定性响应；带电删除 PDU 的 D5 偏离；
- **SHAPE_OR_DRIFT**：厂商在嵌套对象/模板快照中返回空壳值（0/空串），候选填充真实值；以及厂商自身 500 缺陷导致其库内数据统计 ±N 的漂移。逐项消除属新前端/新源码重建范围。

### 残留差异（v2 口径实测，不阻塞）

- 状态码+错误码口径兼容率 **83.4%**（可比 391），**真实断裂 0**；
- 关键字段（响应形状）口径 **70.3%**——剩余差异集中在嵌套关联对象的填充深度与个别响应字段命名（如导入 summary 的 `unchanged`/`ignored`），属厂商响应形状的完整复刻工程。
