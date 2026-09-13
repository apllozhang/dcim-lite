# 专家评审总导航（2026-09-13）

> 本文档是整个评审的**唯一入口**：三分钟建立全貌，再按 §10 建议动线深入。
> 评审对象 = 同一仓库内的三大件：**公共后端（Go）+ 新 UI（自主前端源码）+ 旧 UI（厂商编译产物，对照规格）**。
> 历史：厂商只授权二进制部署、不给源码；本项目目标 = 后端/前端/数据库/部署全链路自主可构建，产品界面与交互保持 ALE 风格与行为。

---

## 1. 仓库目录地图

```
dcim-lite/
├── cmd/server/              # 后端入口(Go 1.25)
├── internal/                # 后端全部业务:app路由/domain/model/repository/service
│   ├── integration/         #   并发与事务集成测试(race+真PG,-tags integration)
│   └── ...
├── migrations/              # DB 迁移(PG16)
├── docs/                    # ★ 全部文档(本文件所在;索引见 §6)
│   ├── openapi.yaml         #   API 契约(单一事实源,含如实化标注)
│   ├── frontend/            #   前端 ADR/行为地图/视觉令牌/旧产物清单
│   └── review/              #   ★ 新旧 UI 对照截图(本次新增,见 §4)
├── frontend/
│   ├── app/                 # ★ 新 UI 自主源码(Vue3+TS+Vite+ElementPlus 2.9
│   │                        #   +Pinia+VueRouter+openapi-fetch+Vitest+Playwright)
│   │   ├── src/features/    #   按屏分域:dashboard/resource/device/rack/screen/admin/auth
│   │   ├── src/design-system/ale-theme.css  # v2 主题逐字节移植(:root:root 适配)
│   │   ├── e2e/             #   matrix(五态/双跑/三权限/像素基准) dualrun shell
│   │   └── tests/           #   Vitest 单测(71 例)
│   ├── ale/                 # ★ 旧 UI = 厂商 v2 编译产物(46 文件,只读)
│   │                        #   用途:①对照规格 ②e2e 双跑对照 ③flag 回退源
│   │                        #   红线:严禁任何字节复制进 frontend/app(clean-room)
│   └── clean/ README.md
├── deploy/                  # 部署即代码(compose + nginx server 块)
├── .github/workflows/       # CI(ci.yml 8 项 + diff-replay nightly)
├── tests/                   # 后端证据工具自检(field_gate 等负向自测)
└── simulate/                # B 族台账模拟三连证据(20260912)
```

## 2. 现场环境（10.20.30.203，测试环境）

| 端口 | 内容 | 说明 |
|---|---|---|
| 19500 | **主入口 = 旧 UI**（v2 换肤版） | 生产替身,新 UI 评审期不动它 |
| 19501 | 旧 UI 独立源 | e2e 新旧双跑对照 + flag 回退目标 |
| 19502 | **新 UI 预览** | 本次评审对象,镜像 dcim-ale-app:2ff888a(main) |
| 19080 | dev 后端 API | 19500-19502 共用同一后端与库 |
| 5173 | 线上旧栈(0.1.0) | 遗留,弱默认口令待第 9 轮收口 |

凭证：dev 栈 `admin / admin123456`（测试期临时口令,第 9 轮生产加固须收口——已知项,非遗漏）。
CI：GitHub Actions,每个 PR 8 项检查（见 §8）；另有 nightly 差分回放。

## 3. 三大件边界与 clean-room 声明

1. **公共后端**：`cmd/ + internal/ + migrations/` 全自主源码,行为以厂商二进制差分回放校准（见 §6 差分文档族）。API 契约 = `docs/openapi.yaml`,前端经 openapi-typescript 生成类型 + openapi-fetch 强类型调用,CI 有契约漂移门。
2. **旧 UI**（`frontend/ale`）：厂商编译产物。在授权范围内**仅作三用**：部署运行、e2e 对照、新 UI 的规格参照（文案/列宽/交互行为/暗色样式）。**新 UI 未复制其任何字节**——新 UI 全部源码可读、可构建、可审计,与旧产物的唯一联系是"行为与观感对齐"。
3. **新 UI**（`frontend/app`）：Vue3 全源码。规格来源 = v2 编译产物的行为提取（文案/列宽/布局常量/色板/交互分支逐项反编译记录）+ 203 实测截图；每屏实现细节与差异如实记录在各 PR 描述与 §5。

## 4. 新旧 UI 对照截图（`docs/review/`）

六屏 × 新旧双份（同库同数据同视口）+ 两张关键浮层,索引见 **`docs/review/README.md`**。
数据量参考：72 数据中心 / 69 机房 / 90 机柜 / 2 模板；e2e 种子带 `MTX-*`/`DUAL-*` 前缀。

## 5. 六屏复刻清单与契约如实化（评审重点）

每屏规格均从 v2 编译产物逐项提取（文案、列宽、布局常量、色板、空态、交互分支），差异**全部如实登记**、无一隐瞒：

| 屏 | 路由 | 关键文件（frontend/app/src/features/） | 对照要点 |
|---|---|---|---|
| 1 运行概览 | / | dashboard/DashboardView.vue | v2 DashboardView 有对应旧页(docs/review/old-ui/01),非新 UI 独有;文案取自 v2 编译源 |
| 2 资源层级 | /data-centers | resource/ResourceHierarchy.vue + resourceStatus.ts | 三栏级联/全套 CRUD/复制移动对话框 |
| 3 设备台账 | /devices | device/DeviceManager.vue + DeviceImportDialog.vue + deviceImport.ts + deviceExport.ts | 三 Tab/六段 37 字段表单/U 位视图/批量导入与导出(均真 xlsx)/主表 9 列 |
| 4 机柜管理 | /racks | rack/CabinetManager.vue + RackImportDialog.vue + rackShared.ts | 四统计卡/八列台账/三段式建柜/批量导入(CSV) |
| 5 机柜模板 | /rack-templates | rack/CabinetTemplate.vue + TemplateSpecForm.vue | 系统内置标记/初始版本参数/发布新版本 |
| 6 机房大屏 | /room-screen | screen/RoomScreenView.vue + ScreenDeviceDrawer.vue + ScreenDeviceEditDialog.vue + screenShared.ts | 暗色(ale-theme 自动套)/U 位立面 9px 格/拖动换位/U 位上架/设备详情抽屉+编辑对话框 |

**六屏能力状态表**（IMPLEMENTED=已实现 / PARTIAL=部分 / NOT_IMPLEMENTED=未实现 / ACCEPTED_DIFFERENCE=已批准差异）：

| 能力 | 状态 | 说明 |
|---|---|---|
| 设备批量导入 | IMPLEMENTED | 真 xlsx(v2 st 35 字段同序/别名/匹配规则同口径);模板/预览/提交/结果回执全链路 |
| 设备信息导出 | IMPLEMENTED | 真 xlsx 43 列(v2 Hl 集合),遵循当前筛选口径;e2e 校验文件内容 |
| 设备主表三列(类型/资产号/管理IP) | IMPLEMENTED | 第 7 轮恢复,列序对齐 v2 |
| 大屏设备查看详情/编辑 | IMPLEMENTED | 720px 抽屉 + 980px 编辑对话框(v2 同名同规格) |
| 机柜批量导入 | IMPLEMENTED_DIFFERENT_FORMAT | CSV(可另存为 xlsx),v2 为 xlsx;校验口径同 |
| 大屏机柜右键菜单 | NOT_IMPLEMENTED(等价入口已存在) | 编辑/删除从选中卡与工具栏可达;P2 延后 |
| 行操作直接文字按钮(机柜/模板/设备) | ACCEPTED_DIFFERENCE(待产品确认) | 旧 UI 为省略号菜单;新 UI 直接铺开,误触风险略高 |
| 应用壳(顶栏/侧栏结构) | VISUAL_DEVIATION_PENDING(待 UI 负责人裁定) | 旧 logo 占左上/标题自侧栏起;新顶栏全宽——六普通页共有差异 |

**契约如实化差异总表**（v2 前端行为 vs 本仓后端契约,以契约为准,前端适配）：

| # | v2 行为 | 本仓实现 | 位置 |
|---|---|---|---|
| 1 | 表单字段 row/column | 后端 `rackRow`/`rackColumn`,保存/回显双向映射 | rackShared.ts |
| 2 | 建柜传 templateVersionId | 后端按 `templateId` 取当前版本,选项文案不变;UI 明示"保存时采用当前版本" | rackShared.ts |
| 3 | 服务端 autoGenerateCode 自动编码 | 后端无此能力,保存时前端生成;第 7 轮起时间戳+crypto 随机段+409 冲突重试(服务端序列生成仍是最终方案,见闭环 §11-C03) | rackShared.ts autoCode/submitWithAutoCode |
| 4 | u-layout 返回 used/devices | 实际返回 `positions[{…,device}]/free[]`,api 层统一映射（顺带修复屏3 U 位 Tab 一直显示 0 的缺陷） | resource/api.ts fetchULayout |
| 5 | 供电冗余路发 REDUNDANT | 后端枚举 `STAND_BY`,UI 文案仍为"冗余路" | PduManager.vue |
| 6 | socket status 表单可写 | 后端收权(INTENTIONAL C10)：状态由连接事实维护 | 后端已登记 |
| 7 | 设备导入/编辑可改 lifecycleStatus | 后端 PUT 设备强制保持原状态(service/device.go:396),导入更新行与编辑页同口径不携带;状态流转只能走上架/下架 | deviceImport.ts |

**诚实口径**：容量利用率缺字段显示"无法计算"+原因（不按 0 估算）；设备重量与 PDU 插座占用数据模型未维护,对应指标卡固定"无法计算"——这是数据模型边界,不是缺陷。

**已知未完成项**（v2 有、本版未做或待裁定;2026-09-13 第 7 轮闭环后更新）：
- 大屏机柜右键菜单（编辑/删除已从选中卡与工具栏可达,右键菜单本身未做;P2）;
- 行操作呈现方式（省略号菜单 vs 直接文字按钮）与应用壳结构差异——两项待产品/UI 负责人裁定,见上方能力状态表;
- 大屏"查看设备详情/编辑设备信息"、设备批量导入/导出、设备主表三列——**第 7 轮已全部实现**,不再是未完成项（闭环对照见 §11）。

## 6. 文档索引（按评审主题）

**前端复刻线**
- `docs/frontend/FRONTEND-REBUILD-ADR.md` — 为什么重建、路线选择
- `docs/frontend/ALE-BEHAVIOR-MAP.md` — v2 行为地图
- `docs/frontend/ALE-VISUAL-TOKENS.md` — 视觉令牌（与 ale-theme.css 对应）
- `docs/frontend/legacy-ale-MANIFEST.md` — 旧产物 46 文件 SHA-256 清单
- `docs/review/README.md` — 新旧对照截图索引

**后端接管与加固线（时序）**
`TAKEOVER-20260910` → `R0-BASELINE` → `HARDENING-20260910` → `COMPAT-DECISIONS` → `DIFFERENTIAL-REPLAY-20260910` → `DIFF-REPLAY-FINAL-20260911(.json)` → `FIELD-DIFF-CLASSIFICATION-20260911` + `field-diff-decisions.yaml`(+`field-baseline.json`) → `INDEPENDENT-VERIFY-20260911` → `R7-PRODUCTION-CHECKLIST`

**历轮提交与外部裁定**
- `EXPERT-SUBMISSION-20260911 / -ROUND3-20260912 / -ROUND4-20260912 / -ROUND5-20260912`（每轮含证据链）
- `EXPERT-COVER-LETTER-20260911`、`SELF-REVIEW-FOR-EXPERT-20260910`、`CODE-REVIEW-GUIDE`
- 中文裁定存档：`第三轮完整复评与Phase1执行决策-20260911`、`第四轮完整复评与Phase1-D启动裁定-20260912`、`字段差异分类与ALE前端Phase0复评-20260911`

**契约与数据**
- `docs/openapi.yaml`（74 操作;含 2026-09-12 如实化修订标注）
- `docs/field-diff-decisions.yaml` + `field-baseline.json`（394 差异五族台账,ratchet 门）

## 7. 第 6 轮 PR 索引（前端自主化主体,均 CI 全绿后合并）

| PR | 内容 | main |
|---|---|---|
| #48 | P1-D 收口：五态种子×新旧对照×三权限矩阵×像素基准 | fa4910b |
| #49 | ale-theme 主题移植（EP 级联顺序确定性） | fd64c52 |
| #50 | 屏1 运行概览 + 屏2 资源层级 | 8ea15fa |
| #51 | 屏3 设备台账（三 Tab/37 字段表单） | 054ae2f |
| #52 | 屏4 机柜管理 + 屏5 机柜模板 | 0f3db66 |
| #53 | 屏6 机房大屏（暗色） | 5a2b79b |
| #54 | 屏6b PDU 管理 + 机柜图 Excel 导出/导入 + 容量分析 | 4f48dda |
| #55 | 屏6b e2e 锁定（容量对话框+导出回导闭环） | 2ff888a |
| #56 | 评审总导航 + docs/review/ 新旧对照截图 | 2d47cea |
| #57 | 第 7 轮:评审意见闭环（2×P0+5×P1+P2-03,见 §11） | （见 CI） |

## 8. 质量门（每个 PR 必过 8 项）

quality（gofmt/vet/单测race/路由契约/证据自检/字段门禁）· integration（race+真PG）· supply-chain（govulncheck+SBOM）· frontend×2（gen:api 契约漂移门/lint/单测/build/Playwright e2e 真后端真旧源/负例/flag 注入检查）。
e2e 资产：五态种子新旧对照、生命周期筛选差分、三权限矩阵、**像素基准×5**（devices/hierarchy/racks/rack-templates/room-screen,大屏 mask 实时时钟）。

## 9. 评审建议动线

1. 读本文件 §3-§5 建立边界与差异认知（5 分钟）；
2. 翻 `docs/review/` 新旧对照截图（10 分钟）；
3. 开 `10.20.30.203:19502` 实操六屏与浮层,随时与 `:19501` 旧 UI 同屏对照（30 分钟）；
4. 抽查新 UI 源码关键文件（§5 表格第 3 列,每屏 1-2 个文件）与 `openapi.yaml`、`ALE-BEHAVIOR-MAP.md` 对读；
5. 需要历史依据时按 §6 索引下钻；
6. 输出裁定（分歧点建议注明"契约差异/如实化/缺陷"三分类,与 §5 总表口径对齐）。

---

## 11. 第六轮评审意见闭环（2026-09-13,PR #57）

评审裁定（《第六轮新旧UI复刻质量与主入口接替评审-20260913.md》）：复刻质量约 80/100;19502 可继续评审与内部试用;**19500 主入口接替 NO-GO**,完成 2×P0、5×P1 后可重新申请"有条件 GO、先灰度后全量"。以下为逐条闭环对照（验收标准均按评审原文）：

| 评审项 | 闭环实现 | 自动化证据 |
|---|---|---|
| UI-P0-01 设备批量导入死按钮 | `DeviceImportDialog.vue` + `deviceImport.ts`：真 xlsx 模板（v2 st 35 字段同序+别名+说明行）/解析/逐行校验（类型存在性、编码 80 长度、Excel 内三标识重复、IP 格式、状态枚举中文、数值非负、跨行同设备冲突）/三级匹配（编码→序列号/资产号联合识别,交叉冲突拒绝,v2 Oe 口径）/更新空白保持原值/预览表/逐行提交/失败行回执/结果 xlsx 导出 | e2e:模板下载文件校验+导出文件回导预览(UPDATE 匹配);单测 13 例 |
| UI-P0-02 导出设备信息死按钮 | `deviceExport.ts`：43 列(v2 Hl 集合含类型/分类中文/位置三段/上架状态),遵循当前筛选口径,分页并发拉全量,样式复刻(1F4E78 表头) | e2e:download 后 Node 侧 XLSX 读文件校验表头 43 列+数据行=筛选口径 1 台 |
| UI-P1-01 设备表缺三列 | 恢复 设备类型/资产编号/管理 IP 三列(列序对齐 v2,编码列 fixed left),缺省值统一"—"占位;类型名后端 Preload("Type") 直显+类型表回退 | e2e:表头三列断言 |
| UI-P1-03 大屏设备详情/编辑缺失 | `ScreenDeviceDrawer.vue`(720px 抽屉"设备详细信息",五分组+下架按钮)+`ScreenDeviceEditDialog.vue`(980px"编辑设备信息",复用 37 字段表单,positioned 约束),菜单文案逐字对齐 v2;保存后原地更新设备块不整屏重载 | e2e:右键菜单四项文案断言+抽屉分组断言+编辑对话框打开 |
| UI-P1-04 autoCode 并发碰撞 | 时间戳 base36 之上叠加 crypto 7 位随机段(同毫秒碰撞域≈780 亿)+`submitWithAutoCode` 409 冲突重生成重试一次;服务端序列生成登记为最终生产方案(§5 总表 #3) | 单测:2 万条无碰撞/冲突判定/重试语义(自动重试,手工不重试,非冲突不重试) |
| UI-P1-05 评审导航事实冲突 | 本文件修正:运行概览来源表述(§5 表,旧 UI 01-dashboard 截图对应)、设备/机柜导入分开如实登记(机柜 CSV/设备 xlsx)、新增六屏能力状态表(IMPLEMENTED/PARTIAL/NOT_IMPLEMENTED/ACCEPTED_DIFFERENCE)、契约总表补 #7 | 本文件 §5 |
| UI-P1-02 应用壳结构差异 | 登记 `VISUAL_DEVIATION_PENDING`(能力状态表),待 UI 负责人二选一:对齐旧结构或批准为自主 UI 升级——**需用户拍板,未擅自改** | —(决策项) |
| UI-P2-03 大屏 chunk 501.90 kB | 浮层组件 defineAsyncComponent + xlsx 动态 import:RoomScreenView chunk 501.90 kB → **36.56 kB**,xlsx 独立 429.53 kB 按需 chunk | 本地 vite build 产物清单 |

**评审 §1.1 其余最低条件的对应状态**：#4 六屏人工交互验收（评审批次 B,需现场执行）;#6 切换演练（评审批次 C）——两项不在本轮代码闭环范围,材料已就绪。

**顺带修复（本轮 e2e 真跑抓到）**：大屏切 DC/机房后选中机柜不自动重选,工具栏"编辑/详情"永久禁用(`ensureSelection` 未挂到 onDcChange/onRoomChange);右键菜单位置在视口边缘溢出不可点(`clampMenuPos` 夹紧);菜单外点不关闭(document click 监听)。

**质量门**：单测 71/71（新增 23）;vue-tsc/eslint/prettier/生产构建通过;e2e 双用例真后端(10.20.30.203 栈)全绿。

---
生成：2026-09-13。本文档随评审轮次滚动更新；差异新增一律先入 §5 总表再改代码。
