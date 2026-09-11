# ALE 行为地图(Phase 0 冻结版,2026-09-11)

> 旧 ALE bundle 的页面、路由、角色、动作与后端 API 映射。
> 来源:`docs/frontend-routes-evidence.json`(bundle 反推)、`docs/API-CONTRACT.md`、
> 后端 `internal/app/routes.go`(RouteInventory 72 条)与差分沙箱实测。
> 用途:新前端 Phase 1+ 的模块拆分与双跑对照基准。

## 1. 角色与全局行为

| 角色 | 可见范围 | 后端口径 |
|---|---|---|
| 匿名 | 登录页;任何数据接口 401 | Auth 中间件 |
| 普通用户(user) | 全部只读 + 无管理入口;写操作按策略需审批 | `/api/v1/**` 非 /admin 路由 |
| 系统管理员(system_admin) | 全部 + 管理(用户/角色/审批策略/LDAP/模板/PDU 归档) | `/admin/**` admin 态 |

全局:登录含 SVG 验证码(`/auth/captcha` → login 携带 captchaId+captcha);会话 JWT(TTL 默认 7200s);
密码重置/停用会吊销全部旧 token(session version);操作审批策略对"上架"默认可开。

## 2. 页面 → API 映射(按旧 bundle 视图名)

| 页面(旧视图证据) | 主要动作 | 后端 API |
|---|---|---|
| LoginView | 登录(验证码) | GET /auth/captcha, POST /auth/login, GET /auth/me, POST /auth/logout |
| DashboardView | 仪表只读卡片 | GET /resource-tree, GET /devices(统计), GET /metrics(新) |
| ResourceTreeView / DataCenterView | 资源树、数据中心 CRUD | GET /resource-tree; POST/PUT/DELETE /data-centers; POST /data-centers/{id}/rooms|copy |
| RoomScreenView | 机房、机柜 CRUD、移位 | PUT/DELETE /rooms/{id}; POST /rooms/{id}/racks|copy|move|rack-diagram-import/* |
| RackScreenView | 机柜详情、U 布局 | GET /racks/{id}/u-layout|pdus|pdu-connections; GET /racks-page |
| device(设备视图) | 设备 CRUD、上架/移位/下架 | GET /devices(+分页/筛选); POST /devices; PUT/DELETE /devices/{id}; POST /devices/{id}/assign|move|decommission; GET /devices/{id}|{id}/history; GET /devices/import-template |
| PduView | PDU/插座/接电 | POST /racks/{id}/pdus; PUT/DELETE /pdus/{id}; POST /pdus/{id}/sockets|force-archive; PUT/DELETE /pdu-sockets/{id}; POST /pdu-sockets/{id}/connection; DELETE /pdu-connections/{id} |
| TemplateView | 机柜模板 | GET/POST /rack-templates; PUT/DELETE /rack-templates/{id}; POST /rack-templates/{id}/versions |
| ApprovalView | 审批单 | GET /admin/approvals?status=; POST /admin/approvals/{id}/approve|reject |
| AdminView | 用户/角色/策略 | GET/POST /admin/users; PUT/DELETE /admin/users/{id}; POST /admin/users/{id}/reset-password; GET /admin/roles; GET/PUT /admin/approval-policy; LDAP /admin/ldap* |
| 设备导入 | Excel 导入 | POST /devices/import(校验+提交), GET /devices/import-template |

## 3. 已确认的行为口径(双跑必须一致)

- 停用账户登录 → 403(C9);错误码统一 `code` 字段(Envelope);
- 上架在位冲突 → 409 `RACK_U_CONFLICT`;跨柜接电 → 409 `PDU_DEVICE_RACK_MISMATCH`;
- 已连接插座删除 → 409 `PDU_SOCKET_CONNECTED`;带电 PDU 删除 → 409 `PDU_IN_USE`;
- 乐观锁:PUT/DELETE 携带 `version`,过期 → 409 `RESOURCE_VERSION_CONFLICT`(UI 必须提示刷新);
- 上架审批开启时:assign 产生 PENDING 单,批准后设备才 RUNNING;
- 审批 approve/reject:stale version → `RESOURCE_VERSION_CONFLICT` 优先于状态冲突;
- 强制归档:先 GET archive-impact 展示影响清单,提交须回填 `confirmConnections`,过期 → 409 `IMPACT_CONFIRMATION_REQUIRED`;
- 移位(D01 方案 A):设备有活动供电连接时 move → 409 `DEVICE_POWERED`,UI 提示先断开;新前端移位表单须处理该冲突态。

## 4. 已知差异决策(新前端不复制的历史怪异)

见 `docs/COMPAT-DECISIONS.md` 与 `FIELD-DIFF-CLASSIFICATION-20260911.md`:
socket status 不可直写(C10);厂商空壳字段(templateSnapshot=0、type.status 空)以真值替换;
导入 summary 以候选超集(allowedActions/decisions/ignored)为准。

## 5. D01 开放决策(影响后续模块)

设备移位与活动供电连接的领域规则待业务确认(建议方案 A:有活动连接禁止移位,提示先断开)。
该决策前,"移位"页面交互不冻结;设备查询/详情不受影响。
