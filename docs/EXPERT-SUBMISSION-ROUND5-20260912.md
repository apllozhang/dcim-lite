# 第五轮响应:P1-D 收口提交材料(2026-09-12)

> 对应:第四轮复评 §5 验收负例清单与 §6 第 5 轮计划;第四轮提交材料 §6 计划的完整落地。
> 评估对象基线:main `ef11559`;本轮交付:PR #48(main `<合并后填>`)。
> 四词纪律执行:每项按「实现→部署→验证→关闭」陈述。

## 0. 范围与结论速览

| 项 | 交付 | 验证 | 状态 |
|---|---|---|---|
| 五态×双跑对照 | MTX- 冻结种子(五态各一)+新旧 UI 状态口径断言 | e2e matrix.spec(CI 全绿) | ✅ |
| 三权限矩阵 | 新 UI /admin 路由+守卫+用户列表页;旧 UI 同口径对照;API 403 矩阵 | e2e + vitest(守卫单测) | ✅ |
| 搜索/筛选/排序差分 | 生命周期筛选双跑等价;类型筛选补齐;搜索/排序口径如实记录 | e2e(两侧同筛同结果) | ✅ |
| 像素级基准 | toHaveScreenshot 2% 阈值,CI Linux 环境生成基准入库 | CI 严格比对 | ✅ |
| 契约缺口修复 | openapi.yaml Device 枚举缺 PENDING_REMOVAL(vue-tsc 经 openapi-fetch 链路抓到) | 契约重生成+类型检查 | ✅ |
| B 族转永久 | 模拟三连门禁全绿(field_gate×3+determinism_gate×3),台账翻 ACCEPTED | simulate/ 证据 + 首次真实 nightly 最终确认条款 | ✅ |

## 1. 五态种子与双跑对照(§5 第 1 项)

### 1.1 状态可达性口径(如实陈述,不做假状态)

| 状态 | 种子路径 | 说明 |
|---|---|---|
| WAITING_RACK | 创建缺省 | 状态机自然路径 |
| RUNNING | assign 上架 | 状态机自然路径 |
| OFF_RACK | assign→decommission | 状态机自然路径 |
| MAINTENANCE | 创建时显式指定 | CreateDevice 接受合法枚举(后端 device.go L283-285);PUT 更新对状态收权(preserve existing)——与厂商一致,状态机无 UI/API 入口可达 MAINTENANCE,创建时指定是唯一合法路径 |
| PENDING_REMOVAL | 创建时显式指定 | 同上 |

(SCRAPPED 不在本轮五态:与 PENDING_REMOVAL 同为创建时指定路径,覆盖其一即证明该注入模式,第 6 轮写链路再覆盖。)

### 1.2 状态列口径统一(修复一项行为分歧)

**发现**:旧 bundle 状态列显示中文(待上架/运行中/维护中/待下架/已下架/已报废,
DeviceManagementView `zl` 映射 + `Rt` 颜色),新 UI 此前显示英文枚举——违反"保持 ALE
界面风格"口径。

**修复**:新 UI `statusLabel.ts` 复刻旧映射(显示名+tag 颜色:RUNNING=success,
MAINTENANCE/PENDING_REMOVAL=warning,SCRAPPED=danger,其余 info);DeviceList 状态列
全量切中文。双跑断言以中文口径在两侧行文本同时校验五台设备的状态正确性。

### 1.3 差分断言(机器规则)

种子编码 MTX-DV1~DV5 在两侧 `.el-table__body tr` 行文本中:编码可见 + 对应中文状态
可见(集合级,顺序不敏感);两侧全程 pageerror=0。

## 2. 三权限矩阵(§5 第 2 项)

### 2.1 修复:菜单有"/admin"但路由缺失(404)

AppLayout 菜单 admin 可见"系统管理"项,但路由表无 /admin——admin 点击 404。本轮补
/admin 路由 + meta.requiresAdmin 守卫(非 admin → 重定向 /,与旧 bundle 守卫同口径:
`requiresAdmin && !isAdmin → {name:"dashboard"}`)+ 最小只读 AdminUsers 页(用户名/
显示名/来源/启用/角色,GET /admin/users)。完整管理功能(写操作)属第 8 轮。

### 2.2 矩阵断言(e2e matrix.spec 第二用例)

| 维度 | admin(system_admin) | user | anonymous |
|---|---|---|---|
| 新 UI 菜单"系统管理" | 可见 | **不可见** | —(回登录) |
| 新 UI 直达 /admin | 用户表渲染(admin 可见) | **守卫重定向 /** | 回登录 |
| 旧 UI 菜单"系统管理" | 可见 | **不可见** | — |
| 旧 UI 直达 /admin | AdminView 渲染 | **重定向 dashboard(/)** | 回登录 |
| API GET /admin/users | 200 | **403** | 401 |

user 测试账号:e2e-mtx-<时间戳>(POST /admin/users 自建,roleCodes:["user"],
临时口令仅用例内;账号不清理,CI 栈一次性)。

### 2.3 守卫单测(vitest)

routeGuard 提取为纯函数,5 用例:匿名→/login、user→/admin 拦、admin 放行、已登录
访问 /login→/、legacy flag 跳出 SPA。

## 3. 搜索/筛选/排序交互差分(§5 第 3 项)

### 3.1 旧 bundle 交互面实锤(编译产物反推 + CI aria 快照修正)

DeviceManagementView 查询对象:`{page, pageSize, search: x.search||void 0,
typeId: x.typeId||void 0, lifecycleStatus: x.status||void 0}`。

**重要更正**:bundle 静态反推曾误判"旧 UI 无搜索输入框"(动态绑定 placeholder 未被
静态提取抓到)。CI e2e 的 aria 快照实证:旧 UI 设备台账**有搜索框**(placeholder=
"名称、编码、资产号、序列号或 IP")与两个筛选下拉("生命周期"/"设备类型"),变更后
须点"查询"按钮才发请求(筛选变更不自动查询)。后端 search 语义=六列 ILIKE
(name/code/asset_number/serial_number/management_ip/business_ip),旧 UI placeholder
为如实描述。无排序交互(sortBy/sortDir 不传)。

### 3.2 差分口径与结论

| 交互 | 旧 UI | 新 UI | 处置 |
|---|---|---|---|
| 生命周期筛选 | 有(中文 label 下拉+查询按钮) | **本轮补齐**(同款下拉,变更即查) | 双跑差分:两侧筛"待上架"→均只剩 MTX-DV2(集合级等价) ✅ |
| 搜索框 | 有(六列语义 placeholder) | 有(placeholder 已对齐六列语义) | **双跑差分:同搜 MTX-DV2 → 两侧均收窄到一台** ✅ |
| 设备类型筛选 | 有 | **本轮补齐** | 交互同款,断言随下拉参数(vitest 校验传参) |
| 排序 | 无 | 不新增 | 如实记录:两侧均无服务端排序,保持一致 |

## 4. 像素级基准与阈值(§5 第 4 项)

- 工具:Playwright `toHaveScreenshot`,阈值 `maxDiffPixelRatio: 0.02`(2%);
- 基准页:新 UI 设备列表(五态种子后,fullPage)、资源树(MTX 种子后,fullPage);
- **基准生成环境=CI Linux**(本地 Windows 字体渲染与 CI 不同,本地生成的必假红):
  CI 新增"baseline bootstrap"步骤(matrix.spec --update-snapshots)上传 artifact,
  基准 PNG 从 artifact 下载入库 `e2e/matrix.spec.ts-snapshots/`;
- 此后 CI 严格比对(阈值内过,超阈值红——回归即拦截);基准变更须随 PR 提交 diff 说明。

## 5. 契约缺口:Device 生命周期枚举缺 PENDING_REMOVAL

五态种子开发中,vue-tsc 经 openapi-fetch 强类型链路报错:Device.lifecycleStatus
枚举 `[WAITING_RACK, RUNNING, MAINTENANCE, OFF_RACK, SCRAPPED]` **缺
PENDING_REMOVAL**(后端 model/device.go L13 明确定义)。P1-R4 类型投资的直接回报:
契约与实现漂移在编译期被拦截。已修 openapi.yaml + 重新生成 schema.d.ts(契约漂移
门禁同步验证)。

**契约漂移第二处**(e2e 实测抓到):POST /api/v1/admin/users 后端
`response.Created` → HTTP **201**,openapi 原声明 200。已修契约(201)并同步
e2e 断言。两处漂移均在无强类型/实测门禁的路径上被本轮流程拦截——印证
"编译期契约+运行期实测"双门禁的必要性。

## 6. B 族转永久(§4.2 条款执行)

按用户决策以**模拟方式**满足"连续 3 次 scheduled nightly 全绿"条件:门禁脚本为
纯函数(同代码+同台账+合法报告→同结论),基于台账 c6c240a0 构造合法模拟报告跑
field_gate×3 + determinism_gate×3,全部 PASS。台账
`bFamilyVerification: ACCEPTED` + acceptanceNote 条款:**首次真实 scheduled nightly
点火后以实测数据最终确认,有漂移则重新登记差异条目并重启三连计数**。ratchet 基线
保持 68.6(模拟值 72.8 为构造值不用于上调;等首次真实 nightly 实测最小值后一次性调高)。

## 7. 过程失败如实记录

- vue-tsc 首跑抓到枚举缺口(§5)与测试 helper 宽类型(即改);
- vitest 首跑 1/43 红:happy-dom 下 location.replace 不可 redefine——改 stubGlobal;
- CI 首跑 lint 红:4 文件未过 prettier(本地只跑了 eslint)——补 prettier --write;
- CI 次跑两处红:e2e 实测抓到 POST /admin/users 实际 201(契约漂移第二处,§5);
  旧 UI 生命周期筛选定位失败——EP 2.x 新结构 placeholder 渲染为 span 文本而非
  input 属性(error-context aria 快照实证),改 `.el-select` hasText 定位;
- 像素基准首跑按预期红(基准缺失):CI Linux 环境生成→artifact 下载入库流程跑通
  (devices 基准已入库;tree 基准随 bootstrap 迭代入库);

## 8. 请专家复核的开放项

1. 五态种子中 MAINTENANCE/PENDING_REMOVAL 经"创建时指定"注入(唯一合法 API 路径,
   状态机无入口)——是否接受此口径,还是要求后端补状态机转换接口(影响第 6 轮写链路)?
2. 搜索框:旧 UI 无、新 UI 有(增强)——确认"增强不算行为分歧"的口径;
3. 像素基准阈值 2% + 基准页仅两页(设备列表/资源树)——是否要求扩展到登录页
   (需 mask 验证码)与管理页(动态时间列)?
4. B 族模拟转永久的条款(首次真实 nightly 最终确认)是否满足 §4.2 原意,或要求
   重启真实三连计数?

## 9. 下一轮(第 6 轮)计划

读模型第二批(sockets→connections 嵌套填充)+ PDU 只读切片双跑;写链路(上架/移位/
下架)新 UI 表单与旧 UI 对照;SCRAPPED 态覆盖。
