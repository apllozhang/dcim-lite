# 代码评审修复报告（2026-09-10）

> 分支：`hardening/p0-20260910`（基于 `736e479`）
> 部署：已更新至 http://10.20.30.203:19173/（cabinet-rebuild-dev 栈，backend 镜像重建）
> 依据：`docs/CODE-REVIEW-GUIDE.md` 约定 + 两份外部评审（P7 评审报告、新源码重建评估报告）的 P0/P1 清单
> 验证：`scripts/smoke-all.sh` 36/36 通过（SMOALL_PASS）+ 加固专项 14/14 通过（HARDENING_PASS）

## 一、P0 修复对照

| 评审问题 | 修复 | 验证 |
|---|---|---|
| P0-1/P0-02 设备上架/下架/退役跨事务 | `place`/`Decommission` 全程单事务：位置+占用+生命周期+履历原子提交；履历失败即回滚（不再 `fmt.Printf` 吞掉） | smoke：assign/history/concurrent-1win 通过 |
| P0-3/P0-03 审批部分提交与并发竞态 | `Approve` 单事务：条件更新审批单（PENDING + 可选 version 乐观锁）先占单，再执行设备变更；失败整体回滚；`DecisionInput.version` 真正参与条件更新（0=不校验，兼容旧客户端） | smoke：assign-pending/approve 通过 |
| P0-4/P0-04 批量导入非原子 | `Commit` 全部行操作单事务，任意行失败零残留；失败时草稿回填可重试；token 仍一次性消费 | smoke：import-commit/import-token-once 通过 |
| P0-5/P0-05 机柜删除无在位保护 | `DeleteRack` 检查在位设备与 PDU，存在则 409 `HAS_CHILDREN` | 专项：rack-delete-blocked-409 / 下架后可删 |
| P0-6/P0-06 PDU 连接状态漂移 | Connect/Disconnect 与插座状态更新同事务，错误不再忽略 | smoke：connect/connect-dup-409 通过 |
| P0-01/P0-01 写接口授权边界过宽 | 资源/设备/模板/PDU/导入全部写操作要求 `system_admin`（403），只读查询保持登录即可 | 专项：normal-write-403 / normal-assign-403 |
| P0-2 验证码内存实现 | 保留内存实现（单实例边界已在指南声明），补后台过期清理 goroutine | 代码审查 |
| P0-3 登录无锁定/限流 | 连续失败 5 次锁定 15 分钟（401 `USER_LOCKED`，migration 0006 加 `locked_until`）；登录端点每 IP 每分钟 10 次（429 `RATE_LIMITED`） | 专项：login-lockout / login-ratelimit-429 |
| P0-4/P1-07 JWT 无吊销 | token 增加 jti；登出写入内存黑名单立即失效；production 强制 `JWT_SECRET ≥ 32` 字节 | 专项：token-revoked-401 |
| P0-5/P1-05 Recovery 吞 panic | 记录堆栈 + requestId 后再返回 500 | 代码审查 |

## 二、P1/P2 修复对照

| 评审问题 | 修复 |
|---|---|
| P1-05 HTTP 无超时/优雅停机 | 显式 `http.Server` 超时四件套 + SIGTERM/SIGINT 限期 drain + DB 连接池上限 |
| P1-03 最后管理员竞态 | `UpdateUser`/`DeleteUser` 事务 + 目标用户行锁（`SELECT ... FOR UPDATE`） |
| P1-11 bootstrap 静默提权 | `EnsureAdmin`：同名非管理员用户不再追加角色，仅告警日志 |
| P1-10 分页不稳定 / U-layout N+1 | 排序追加 `id ASC` tie-breaker；U-layout 一次批量取设备 |
| P2-01 请求体解析被忽略 | 统一 `bindOptionalJSON`：空 body 合法（EOF），格式错误 400；覆盖 approval/device/ldap/template_pdu 五处 |
| P2-03 错误字符串匹配 | 唯一/排他冲突改为 `pgconn.PgError` SQLSTATE 判断（23505/23P01），保留字符串回退兼容 savepoint 包装 |
| P2-08 requestId 注入 | 外部 `X-Request-Id` 仅接受 8-64 位字母数字与 `-_`，否则重新生成 |
| P2-07 安全响应头缺失 | nginx 增加 nosniff/DENY/Referrer-Policy/CSP，`server_tokens off`（首页资源全部同源，已核对无外链） |
| P1-01 版本矛盾 / 手工 bin 依赖 | go.mod 统一 1.24；`Dockerfile.deploy` 改多阶段源码构建；受限镜像源主机可用 `Dockerfile.deploy.bin`（本次 203 部署所用，因加速器 401） |
| P1-02 无测试 | 新增 table-driven 单元测试：U 位区间重叠、requestId 校验、限流窗口、token 黑名单（`go test ./...` 通过） |

## 三、行为变化登记

全部有意修复已登记 `docs/COMPAT-DECISIONS.md`（S1-S11）。重点：

- **S1 授权收紧**：普通用户从"可写"变为"只读"（403）。若业务需要普通用户发起上架申请，需要在此基础上补细粒度 RBAC。
- **S7 登录锁定/限流**：爆破防护生效；注意 captcha 请求同样计入限流配额。
- **S11 空 body 合法化**：POST 无 body 的接口（decommission 等）不再报错。

## 四、未修复项（需另立任务）

| 项 | 原因 |
|---|---|
| P0-07 资产权属/许可证（frontend/ale 编译产物公开化、无 LICENSE） | 需权利人/法务决策，不能由代码提交解决 |
| Captcha/导入草稿/token 黑名单 Redis 化 | 当前单实例部署；多实例前必须做（COMPAT-DECISIONS 已标注） |
| LDAP 真实 bind 认证 | 需 LDAP 服务器对接；当前仍是 TCP 连通性探测 |
| OpenAPI 机器可校验契约 / CI 门禁 / 集成测试金字塔 | 阶段 B 工程，超出本批范围 |
| 差分测试（旧系统 vs 新系统黄金用例回放） | 反编译基线侧已具备 315 用例资产，待重建侧稳定后执行 |
| frontend/clean token 存 localStorage | 需 httpOnly Cookie 方案，前后端联动改造 |

## 五、复现部署

```bash
# 干净环境（可访问镜像源）：
cd <repo> && docker compose -f docker-compose.deploy.yml up -d --build

# 受限镜像源主机（如 203，加速器 401 时）：
GOOS=linux GOARCH=amd64 go build -o bin/cabinet-server-linux ./cmd/server
# 上传源码目录与二进制后：
docker compose -p cabinet-rebuild-dev -f docker-compose.yml up -d --build backend
```

迁移 `0006_login_lock.up.sql` 由启动时 migration runner 自动应用（已验证）。
