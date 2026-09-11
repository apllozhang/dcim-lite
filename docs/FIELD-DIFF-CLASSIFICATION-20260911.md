# 字段级差异逐项分类(复评 C01,2026-09-11)

> 数据来源:与 nightly 同款冻结基线(`tests/diff/baseline`)的 PR25 验证跑
> (候选 = main `48636f3`,415 用例 golden 套件,真实断裂 0,兼容率 82.0%)。
> 分类口径按专家复评 §4 P1-N3:`REQUIRED_BY_NEW_UI / LEGACY_ONLY / INTENTIONAL_FIX / DATA_FIXTURE_NOISE / REAL_DEFECT`。

## 总口径:数据值差异 vs 形状差异分开统计

| 口径 | 数量 | 说明 |
|---|---:|---|
| 关键字段差异总条目(v2 比对器) | 53 | 与 nightly/main 一致(±2 为沙箱环境抖动) |
| 其中 **数据值差异**(同名字段、数值不同:total/version/heightU/startU/endU/lifecycleStatus) | ~12 | 全部为库内数据状态不同步(见 B 族),无业务语义分歧 |
| 其中 **响应形状差异**(字段缺失/嵌套深度/包装命名/排序) | ~41 | 嵌套填充深度为主,逐项修复属新前端重建范围 |

## A 族:嵌套关联对象填充缺失 —— REAL_DEFECT(新前端复刻范围,最高优先)

厂商返回、候选缺失的嵌套数据,旧 ALE 界面的设备列表位置列、冲突弹窗、PDU 插座面板、连接列表都依赖它们:

| 涉及用例 | 缺失字段 | 业务影响 |
|---|---|---|
| S05/S14/S16/S17/S18 设备列表与详情(~30 处) | `currentPosition.*`(startU/endU/heightU/orientation/rack 摘要) | 设备列表位置列无数据 |
| S06-ASSIGN-OVERLAP / S06C-CLIENT1(~24 处) | `details.conflictDevices[0].device.*` | U 位冲突弹窗无法显示冲突设备 |
| S10-PDU-LIST | `items[*].sockets[]` | PDU 面板插座表格无数据 |
| S10-CONNECTIONS-LIST | `items[*].device.* / socket.*` | 供电连接列表的设备/插座列 |
| S06-DECOMMISSION / S08-APPROVE-SUCCESS | `data.device.*` / `data.executed` | 下架/批准后的回显对象 |
| S06-ULAYOUT-READ | `positions[0].version / rack 摘要` | U 布局图的行级乐观锁 |

处置:**进入前端替代阶段的"契约复刻清单"**(新前端按厂商形状消费),后端按需补齐 Preload;不阻塞 Phase 0/1(只读切片可以先按现有响应开发,复刻在写操作迁移前完成)。

## B 族:数据值差异 —— DATA_FIXTURE_NOISE(登记,不修)

同名字段数值不同,根因是厂商基线库与候选沙箱库的数据状态/用例序差异,非行为分歧:

| 用例 | 字段 | 基线→候选 |
|---|---|---|
| S14-PAGE-FILTER-LIFE / COMBO | `heightU/version/total` | 5→4 等计数漂移 |
| S16-PAGE-1 / PAGE-2 / DEV-WAITING | `lifecycleStatus/version/heightU/total` | 首页条目内容不同 |
| S16-ASSIGN-OK / S18-ASSIGN-EXEC-13 | `position.startU/endU` | 选位 39→41、32→34(库内占用不同) |
| S18-ASSIGN-LAYOUT-13 | `free[0..1].*` | 空闲段连锁漂移 |
| S16-MOVE-READBACK / S17-DEV-PAGE-2 | `heightU/version/lifecycleStatus` | 条目序差异 |
| SETUP-S09-RACKB-CREATE | `uHeight` | 42→45(模板上下文) |

处置:维持 DATA_FIXTURE_NOISE 分类;nightly 门禁仍以"真实断裂=0"为准。

## C 族:厂商空壳值,候选填真值 —— INTENTIONAL_FIX(已完成,保持)

厂商返回空壳(0/空串),候选填充真实值——是对厂商缺陷的修复,不回退:

| 涉及用例 | 字段 | 基线→候选 |
|---|---|---|
| SETUP-S02/S09 + S02-TREE(~6 处) | `templateSnapshot.revision/uHeight` | 0→1/42(真值) |
| ~15 处 assign/move/approve/impact | `device.type.status/version` | 空/0→ACTIVE/1 |

## D 族:响应包装/排序差异 —— REAL_DEFECT(低危,随前端复刻逐一对齐)

| 用例 | 差异 | 处置 |
|---|---|---|
| S07-DC/ROOM/RACK-COPY(~14 处) | 厂商 `data.resource.*` vs 候选 `data.*`;`status` 枚举 PLANNING vs OPERATING | 前端复刻时按厂商包装对齐 |
| S09-TPL-NEW-VERSION / S15-TPL-DUP-REVISION(~25 处) | 版本发布响应:厂商返回版本对象(revision/uHeight),候选返回模板对象(isSystem/status) | 对齐响应主体 |
| S15-TPL-LIST | versions 数组排序:厂商 revision 逆序,候选升序 | 改为逆序对齐 |
| S11-USER-RESET-PASSWORD | 厂商返回完整 user(enabled/roles/version),候选简版 | 补齐 |
| S12-IMPORT-VALIDATE/COMMIT | `summary.total` 缺失、`decisions/allowedActions/ignored` 超集 | 补 total;超集保留 |

## E 族:候选超集 —— REQUIRED_BY_NEW_UI(保留)

| 用例 | 字段 | 说明 |
|---|---|---|
| S09/S15 | `data.isSystem=true` | 新前端模板管理需要 |
| S12-IMPORT-VALIDATE | `items[*].allowedActions` | 新前端导入决策 UI 需要 |
| S10C-CONCURRENT-CONNECT | `data.version` | 并发对称胜负互换的字段残影(等价) |

## 结论

1. **真实断裂 0 的结论不变**;53 项差异中无一条是业务语义分歧。
2. 需要"复刻动作"的差异(A 族 ~30 处 currentPosition + D 族包装/排序)全部划入**前端替代范围的契约复刻清单**,与 Gate G1 的"51 个字段差异逐项分类"要求闭合。
3. 数据值差异(B 族)与厂商空壳(C 族)维持现有分类口径,无需修复。
