# 差分回放报告：厂商原版 vs 重建版（2026-09-10）

## 方法

- **基线**：厂商原版镜像 `cabinet-management-tool-backend:0.1.0`，全新 sandbox（干净卷，`127.0.0.1:18380`）
- **候选**：重建版镜像 `cabinet-rebuild-dev-backend:latest`，全新 sandbox（干净卷，`127.0.0.1:18280`）
- **用例**：同一套 golden 套件（`tests/sandbox/gt_final_v3.py`，场景 S01-S18 + 并发/不变式），
  逐用例比对 HTTP 状态码 + 业务错误码；`--pg` 数据库级断言两边同库同用户，均为 `cabinet/cabinet`
- **兼容层**：`GT_CAPTCHA=off|auto` 环境变量门控（基线原生不带验证码；重建版需验证码，由兼容层自动附加）
  差异本身由 `DIFF-*` 探针用例显式记录，不被兼容层掩盖

## 结果

- 基线正式用例（TEST/SETUP）：350
- 候选可比用例：342；未产生（夹具失败级联）：8
- 逐用例完全一致：285
- 语义等价（仅错误码命名差异）：1
- 断言文本差异（非错误码类）：14
- **真实断裂：42**

**可比用例兼容率 = 286/342 = 83.6%**

## 差异发现的能力（探针）

| 探针 | 基线（厂商） | 候选（重建） | 性质 |
|---|---|---|---|
| DIFF-LOGIN-NO-CAPTCHA | 200 SUCCESS | 400 CAPTCHA_REQUIRED | 重建版新增强制验证码（有意加固，需调用方适配） |
| DIFF-NOAUTH-RESOURCE-TREE | 401 UNAUTHORIZED | 401 UNAUTHORIZED | 一致 |
| DIFF-UNKNOWN-ROUTE | 404 NOT_FOUND | 404 RESOURCE_NOT_FOUND | 命名对齐后一致 |

## 本轮据差分结果修复的兼容性缺陷

| # | 问题 | 影响面 | 修复 |
|---|---|---|---|
| 1 | POST 创建/拷贝返回 200，厂商为 **201** | 全部 35 个 golden 夹具失败 → 14 场景中断、约 150 用例未执行 | 14 个创建类端点改用 201 |
| 2 | assign/move 只认 `targetRackId`，厂商/ALE 前端发 **`rackId`** | 设备上架、移位全线失败（**ALE 界面实际点击也会失败**） | 兼容两种字段名 |
| 3 | move 的 `version` 只认 query，厂商放 **body** | 机房/机柜迁移失败 | query 优先、body 兜底 |
| 4 | 用户更新强制要求 username/displayName | 停用/降级/部分更新失败 | 改为部分更新语义 |
| 5 | 错误码命名与厂商不同（10 组） | 依赖错误码分支的调用方行为不一致 | 全部对齐厂商码 |
| 6 | 停用账户登录返回 INVALID_CREDENTIALS | 前端无法区分停用与密码错 | 返回 USER_DISABLED |
| 7 | 审批单插入用 `in.TargetRackID`（传 rackId 时为零 UUID） | 开启审批后上架 **500 外键违约** | 走 `desiredRackID()` |
| 8 | assign/move 响应为裸设备对象 | 厂商返回 `{executed, device, position}`、审批流返回 `{executed, approval}` | 对齐响应形状 |
| 9 | 设备分类未校验枚举、模板名称允许空 | 厂商 400 / 重建 200 | 补校验 |
| 10 | 缺失设备类型返回 400 | 厂商 404 | 对齐为 404 |

## 尚未修复（需后续批次，均有实测证据）

| 项 | 基线 | 候选 | 说明 |
|---|---|---|---|
| 系统模板保护不完整 | 409 SYSTEM_TEMPLATE_PROTECTED | 200 SUCCESS | 停用系统模板未被拦 |
| copy 类接口未校验 version | 409 RESOURCE_VERSION_CONFLICT | 201 SUCCESS | 过期版本仍可拷贝 |
| 插座制式未校验 | 400 INVALID_RESOURCE | 201 SUCCESS | 非法 standard 被接受 |
| 设备管理 IP 未校验 | 400 INVALID_RESOURCE | 200 SUCCESS | 非法 IP 被接受 |
| 用户密码强度 | 400 INVALID_USER | 201 SUCCESS | 短密码被接受 |
| PDU 删除语义 | 200 允许 | 409 RESOURCE_HAS_CHILDREN | 重建版更严格，需业务确认 |
| LDAP 测试 | 400 LDAP_UNAVAILABLE | 200 SUCCESS | 无 LDAP 时重建版仍报成功 |
| 导入草稿过期状态码 | 410 | 400 | 状态码语义差异 |
| assign 审批响应形状 | `{executed, approval}` | 已对齐 | ✓ |
| 并发上架/连接的不变式断言 | successes=1 | successes=0 | 场景前置状态不同导致，需单独复核 |

## 复现方式

```bash
# 基线（厂商镜像）
cd tests/sandbox && GT_CAPTCHA=off python3 gt_final_v3.py \
  --env-file <vendor-env> --run-id DIFF-YYYYMMDD \
  --pg <vendor-pg-container> --out <out-baseline> --api http://<vendor-api>

# 候选（重建版）
GT_CAPTCHA=auto python3 gt_final_v3.py \
  --env-file <rebuild-env> --run-id DIFF-YYYYMMDD \
  --pg <rebuild-pg-container> --out <out-candidate> --api http://<rebuild-api>

# 比对
python3 diff_compare.py <baseline>/results.jsonl <candidate>/results.jsonl report.md
```

---

# 第二批：校验缺失补齐（2026-09-10）

第一批差分回放发现「重建版比厂商更宽松」的校验缺失，本批全部补齐，并用同一套差分复验。

| 用例 | 厂商规则 | 修复 |
|---|---|---|
| `S15-DEV-PUT-BAD-IP` | 设备管理/业务 IP 非法 → 400 `INVALID_RESOURCE`「IP 地址格式无效」 | `net.ParseIP` 校验（空值放行） |
| `S14-VAL-USER-SHORTPW` / `S15-USER-NO-PASSWORD` | 本地用户密码 < 8 位或缺省 → 400 `INVALID_USER` | 密码必填且 ≥ 8 位；错误码改为 `INVALID_USER` |
| `S14-VAL-USER-EMPTYNAME` | 用户名/显示名为空 → 400 `INVALID_USER` | 同上（原先返回 `INVALID_RESOURCE`） |
| `S10-SOCKET-BAD-STD` / `S15-SOCKET-PUT-EMPTY-STD` | 插座制式非 `CN`/`EU`（含空）→ 400 `INVALID_RESOURCE`「插座制式必须为 CN 或 EU」 | 创建与更新均校验枚举 |
| `S07-DC-COPY-STALE-VERSION` / `S15-ROOM-COPY-STALE` / `S15-RACK-COPY-STALE` | copy 类接口请求体 `version` 过期 → 409 `RESOURCE_VERSION_CONFLICT` | 三种 copy 均校验（未传 version 时按旧行为放行） |
| `S09-SYS-TPL-DISABLE-PROTECTED` / `S15-SYS-TPL-UPDATE-CONFLICT` | 系统内置模板停用 → 409 `SYSTEM_TEMPLATE_PROTECTED` | 名称校验（400）先于保护检查（409），与厂商顺序一致 |

## 复验结果

- 目标用例 **9/9 与厂商完全一致**（含 4 个同规则相邻用例）
- 候选回放：292 PASS / 19 FAIL（上轮 285 / 26）
- **可比用例兼容率 83.6% → 87.1%**
- 线上栈冒烟 `SMOALL_PASS`（36/36）；因插座制式收紧，smoke 夹具由 `GB` 改为 `CN`

## 附带影响

- `scripts/smoke-all.sh` 插座制式 `GB` → `CN`
- 重置密码最低长度 6 → 8 位，错误码改 `INVALID_USER`
