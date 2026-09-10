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
| S11 | 请求体解析 | 空 body 合法（EOF），格式错误统一 400；不再把坏 JSON 当空对象执行 | INTENTIONAL_FIX |

### 已知边界（保持现状，多实例部署前必须替换）

- 验证码、导入草稿、token 黑名单、登录限流均为单实例内存实现；水平扩展需迁移到 Redis。
- LDAP Test 仍为 TCP 连通性探测，不能证明 bind/搜索可用（接口语义见 docs/CODE-REVIEW-GUIDE.md）。
