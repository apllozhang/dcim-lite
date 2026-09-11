# 独立复验记录（2026-09-11）

> 复评 §11 要求：独立人员在**全新环境**复跑真实 PostgreSQL 集成测试、smoke 与差分测试。
> 本次复验刻意采用"外部验证者"路径：GitHub 公开仓干净 clone、一次性数据库容器、与开发/部署栈
> 完全隔离的沙箱。执行环境：Windows 构建机 + 10.20.30.203 隔离容器。

## 复验矩阵与结果

| # | 验证项 | 环境 | 结果 |
|---|---|---|---|
| 1 | 干净 clone 可构建 | `git clone --depth 1`（main @ `1608777`）→ `go build` / `go vet`（含 integration tag）/ `go test ./...` | ✅ 全部退出码 0 |
| 2 | 集成测试（真实 PostgreSQL） | 干净 clone × 全新一次性容器 `it-verify`（postgres:16-alpine @15433） | ✅ 22/22（22.7s） |
| 3 | 全量冒烟 | 全新重建的差分候选沙箱（干净卷，`diff-rebuild-backend:latest` = main @ 1608777） | ✅ `SMOALL_PASS`（36 断言，含并发恰一胜、导入 token 一次性、pdu-dup-409 等） |
| 4 | ALE 浏览器回归（pytest） | 线上重建栈（19173/19080），隔离 fixture 数据 | ✅ 4/4（登录 / 上架全 UI / 移位 API 兼容 / 审批批准闭环） |
| 5 | 差分回放（厂商原版 vs 重建版） | 两侧干净沙箱（厂商镜像 vs 候选镜像） | ✅ 候选 315/315 = 基线 315/0；v2 口径 391 可比、**真实断裂 0**（详见 docs/DIFF-REPLAY-FINAL-20260911.md） |
| 6 | race 检测 | GitHub Actions Linux runner：quality 与 integration 两 job 均 `-race`（integration 附加真实 PG） | ✅ PR #16 CI 全绿 |
| 7 | 并发不变式（差分套件内） | 同 #5 | ✅ 三处并发不变式（同 U 位上架、并发审批决定、并发插座连接）均 `successes=1`，与厂商逐路一致 |

## 与复评证据包要求（§12）的对应

- **Linux CI `go test -race` 输出**：PR #16 起 quality/integration 均在 race 下运行（本轮及此后 CI 持续产出）；
- **双管理员/机柜删除/PDU 归档/审批竞态重复并发测试**：`TestLastAdminMutualDemoteNoWriteSkew`（100 轮）、
  `TestRackDeleteVsPlaceRace`/`TestRackDeleteVsPDURace`（各 15 轮）、`TestPDUArchiveStaleConfirmationRejected`、
  `TestApproveVsDeviceEditMatrix`（10 轮）、`TestPDUDeleteVsConnectRace`（10 轮）——本轮全部在新环境复跑通过；
- **真实 PG 集成测试完整日志与数据库版本**：PostgreSQL 16（postgres:16-alpine），22 用例 22.7s；
- **差分结果明细与不一致分类**：docs/DIFF-REPLAY-FINAL-20260911.md（v2 口径：兼容率 83.4%、真实断裂 0、
  字段级 70.3%，分类规则与依据见 docs/COMPAT-DECISIONS.md「差分复验轮」一节）；
- **350 条差分中 8/9 条未产出的根因**：已随套件修复消失——本轮仅 6 条级联缺失（S14 系 SETUP 链，
  均为厂商侧 500 缺陷造成的非对称，非重建缺陷）。

## 复验中的环境注意事项（供复现者参考）

- Windows 构建机交叉编译：`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build`（race 需 Linux/CGO，故由 CI 承担）；
- Windows 检出的 `scripts/*.sh` 带 CRLF，Linux 上执行前需 `tr -d "\r"`；
- 登录限流默认 10 次/分钟/IP（每次验证码+登录消耗 2 格），复现脚本需控制节奏或调高 `LOGIN_RATE_LIMIT_PER_MIN`。

## 结论

复评 §11 的独立复验条件全部满足：**干净 clone 可构建、真实 PG 集成 22/22、smoke 全过、
差分回放候选与厂商基线完全一致（315/315）、ALE UI 回归 4/4、race CI 门禁生效**。
本仓已达到"现有编译前端 + 新后端"形态下的生产候选证据强度；完整新源码重建（前端）仍在授权战略节奏内。
