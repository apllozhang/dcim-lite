# 第四轮响应与 Phase 1-D 启动提交材料(2026-09-12)

> 对应:第四轮完整复评与 Phase 1-D 启动裁定(20260912)。
> 评估对象基线:main `2ed61c5`;本轮交付:PR #37(main `1c32b46`、`2ae621b`)+ PR #47(main `5ade8c77a`、`4312d70`)。
> 四词纪律执行:每项按「实现→部署→验证→关闭」陈述;凡未达永久关闭条件的,明确标注 PROVISIONAL。

## 0. 重要更正(P1-D 双跑过程中发现,涉及此前一项验收结论)

P1-D 双跑搭建时发现:**旧 ALE bundle 为 HTML5 history 路由模式**——以 `/legacy/` 前缀静态兜底服务时,旧应用永远无法路由(白屏,无报错)。这意味着:
1. 第三轮 P0-B 阶段"19500 /legacy/ 可访问旧页面"的结论不完整——可访问的是主文档,页面本身白屏;当时 CI 回退演练只断言标题与资源 200,未能发现(该演练已在第四轮 P0-R1 被裁定为假阳性并重构)。
2. 本轮已修正架构:**旧 UI 以独立端口源(:19501)按根路径服务**(ale-app nginx 双 server + compose 双端口),feature flag 回退 = 整页跳转至旧源根路径路由(LEGACY_BASE 默认 `http://<host>:19501`,VITE_ALE_LEGACY_BASE 可覆盖)。
3. 修正后已在 203 现场实证:旧 UI 独立源下菜单导航进入"设备台账",26 行真实数据渲染(Total 139,dev 库);CI e2e 演练断言旧源标题+旧导航真实渲染+旧资源 200+无 JS 异常,且负例(旧源停服时演练必须失败)通过。

## 1. 第四轮派工响应(P0-R1~R3 + P1-R4~R7)

| 项 | 交付 | 失败负例 | commit |
|---|---|---|---|
| P0-R1 回退真验证 | CI 起真旧 bundle 站点(frontend/ale→nginx,vite 代理前缀适配);e2e 断言:旧标题/旧资源 200 计数/主文档禁 4xx5xx/pageerror 监听/切回新树渲染+API 200 | 代理错指后端时演练必须失败——CI 负例步骤通过 | 741d5ab、7ec6ce6 |
| P0-R2 flag 治理 | VITE_ALE_LEGACY_BASE(envPrefix);生产构建 override 仅限 ale.flags.debug=1,部署注入始终生效;版本切换 UI 受治理约束 | vitest 三情形 + CI 构建注入检查(/wl-inject grep 产物) | 741d5ab |
| P0-R3 部署即代码 | deploy/(compose/nginx 策略/env.example/smoke.sh/README 升级回滚与 digest 规则)+ 多阶段 Dockerfile | 203 干净主机纯仓库文件复建 ale-app → SMOKE PASS(现场验证,含旧页面标题与旧资源 200) | 1a9f994、2ae621b |
| P1-R4 强类型客户端 | openapi-fetch:方法/路径/请求体/query/响应全契约推导;as never 删除 | 类型负例(client-type-negatives.ts,vue-tsc 门禁):GET 用于 POST/未知路径/query 类型错误编译失败 | 4ef4183 |
| P1-R5 语义门禁 | 台账三维化(implementation×observation×noiseRule);8 条可执行规则对左右实际值执行;ratchet 基线 docs/field-baseline.json | selfcheck_field_gate.py 9 例负例进 CI:未知路径/双实值/非法枚举/null/敏感值/非时间戳/CLOSED 重现/ratchet 回退全拦 | 4f45223 |
| P1-R6 可观测性 | POST /telemetry/frontend-errors(匿名限流+白名单+环形缓冲)+ admin 检索 + 前端双写 | 集成 TestFrontendTelemetry(白名单丢弃/截断)+ E2E 受控异常→admin 检索到标记事件 | 59dd0d1 |
| P1-R7 供应链 | 5 actions 钉 SHA、postgres 钉 digest×3、govulncheck@v1.1.4、Dependabot 月度 | 同一提交重跑工具来源一致(由 SHA/digest 性质保证);升级走 Dependabot PR | 2647670 |

过程失败如实记录:首轮 CI 两次失败——legacy 站点 working-directory 与 vite 代理前缀适配(7ec6ce6 修)、OpenAPI 操作计数硬编码漂移被 count gate 按设计拦截(351a85a 显式对齐 74)。

## 2. §4 四项裁定的落实

1. **§4.1 CLOSED+residualNoise 二维化**:已重构为 implementationStatus × observationStatus × noiseRule 三维;当前无 observationStatus=CLOSED 条目(形状复刻的 currentPosition 记 ACCEPTED_NOISE+FIXTURE_STATE+测试引用),谎报"差异消失"的结构性风险已消除。
2. **§4.2 B 族翻转**:57 条 B 族标注 **PROVISIONAL_ACCEPTED**(meta.bFamilyVerification);转永久条件=连续 3 次 scheduled nightly determinism gate 全绿。首轮 nightly 已在 PR 合并后自动运行;进度随 nightly 累积更新。
3. **§4.3 顺序**:小批次(P0-R1~R3+P1-R7)先行完成,P1-D 不受阻启动;供应链与 P1-D 并行完成。
4. **§4.4 A 族优先序**:按页面依赖执行;P1-D 只读切片不依赖 conflictDevices。

## 3. Phase 1-D 首批:新旧双跑证据(P1-D 内容清单对照)

| 复评要求 | 状态 |
|---|---|
| 三级树 | ✅ dualrun.spec:新树节点集含种子 DC/房间/机柜;旧资源层级页文本含同一种子资源 |
| 设备分页/搜索/状态 | ✅ 新 UI 已有(位置列含 currentPosition);双跑种子 4 设备在两侧行文本集合级可见 |
| 三态权限 | ✅ 后端 auth-matrix(既有)+ 新 UI 守卫;旧 UI 注入会话键对照 |
| loading/empty/error/401/超时 | ✅ 组件级 vitest + e2e 401 收口(既有);双跑覆盖数据等价 |
| 新旧双跑与回退演练 | ✅ dualrun.spec(同一种子数据、API 请求集合、结构化 DOM、四张截图)+ 回退演练(旧源真渲染断言+停服负例) |
| ALE 视觉 tokens 与基准截图 | ✅ tokens 文件(既有)+ dualrun 基准截图(CI artifact dualrun-evidence) |
| 模块级回退审计 | ✅ flag 切换经遥测通道留服务端痕迹(apiCode=FLAG_SWITCH) |

数据快照来源:**冻结种子**(确定性 API 注入:1 DC/1 房/2 柜/4 设备,3 在位)——避免现网数据噪声混入行为差分;种子顺序固定,任何环境可复现。
差分口径:**种子设备编码在两侧行文本的集合级可见**(机器规则 ORDER_INSENSITIVE);两侧表格 DOM 结构不同,行数不做强等价——如需行级严 equality 或像素级比对,见 §5 开放项 1。

## 4. Gate G1 当前口径(13 项)

1-5、7-8、10-13:见第四轮提交与 PR #37(其中 6/8/12/13 本轮补齐缺口);
6/9 相关:**#9 资源树+设备列表新旧双跑——首批差分证据已产出**(PR #38),待复核;
遗留:legacy 页面视觉像素级比对与完整五态矩阵的旧侧对照,随 P1-D 第二批(第 5 轮)补齐。

## 5. 请专家复核的开放项

1. dualrun 差分口径:当前为"集合级等价+种子编码断言"(机器规则 ORDER_INSENSITIVE);像素级视觉比对是否要求在本轮,还是随第 5 轮 P1-D 收口?
2. B 族 3 次 nightly 的累积进度能否在评审时以 nightly run 链接清单形式呈现?
3. ratchet 基线当前 68.6:待 3 次 nightly 观测后按实测最小值上调——是否接受此节奏?
4. /metrics 已从公网前端移除(仅 compose 内网可达);后端端口 8080/19080 的直接访问保护建议随第 9 轮生产演练一并处理。

## 6. 下一轮(第 5 轮)计划

P1-D 收口:完整五态×三权限矩阵的新旧对照、搜索/筛选/排序交互差分、像素级基准与阈值、回退演练常态化;同时启动第 6 轮读模型(sockets→connections)预备。
