# ALE 前端回归（实验性诊断脚本 / 手动探针）

> **定位声明**：本目录是**实验性诊断脚本**，用于人工排查与一次性回归，**不是**稳定可重复的 E2E 测试门禁，**不接入 CI**，不可由 CI/生产人员当作安全可重复执行的测试来运行。原因：
>
> 1. **有副作用**：会真实上架/移位设备、切换审批策略，且无 try/finally 清理，中途失败会留下脏数据；
> 2. **环境硬编码**：部署地址、API 地址、ADMIN_PASSWORD 路径均写死在脚本内，移植必须手改；
> 3. **fixture 不隔离**：直接取存量列表第一项作为操作对象（空环境会 IndexError，非空环境会改动任意真实设备）。
>
> 后续计划（重构为 pytest + Playwright 隔离套件后再标记为 E2E 并接 CI）：环境变量化、自建 fixture 数据（带唯一 run ID）、try/finally 恢复策略、失败时非零退出并留存截图。

针对 `frontend/ale` 编译产物（厂商前端 bundle + 我们的增强脚本）的浏览器级回归。
**目的**：验证后端改动没有打断用户在 ALE 界面上的真实操作路径；与 `tests/sandbox`（API 层差分回放）互补。

## 三类验证不可互替

| 类型 | 例 | 说明 |
|---|---|---|
| UI E2E | 上架流程 | 用户真实点击路径（登录 → 菜单 → 弹窗 → 提交） |
| API 兼容验证 | 移位流程 | 复用 ALE 前端的请求形状直打 API，验证的是后端兼容性，**不能算 UI 回归**（ALE 界面无移位入口） |
| DOM 探针 | probe_*.py | 探索旧编译前端的内部结构，产物是知识不是断言 |

## 环境

- 在部署主机上执行（前端需可从该机访问），使用独立 venv：`python3 -m venv ~/ale-venv && ~/ale-venv/bin/pip install playwright`
- Chromium 需 `--no-sandbox`
- 依赖环境变量文件中的 `ADMIN_PASSWORD`（脚本内硬编码路径 `/home/alec/cabinet-rebuild-deploy/.env`，移植时改这里）

## 脚本

| 文件 | 作用 |
|---|---|
| `ale_regress_flows.py` | 三条主流程：**上架（UI 全链路）**、**移位**、**审批** |
| `probe_device_ops.py` | 设备行「更多操作」菜单项探测（增强脚本折叠后的入口） |
| `probe_assign_modal.py` | 「设备上架」弹窗结构探测（容器、字段、控件类型） |

运行：`~/ale-venv/bin/python3 ale_regress_flows.py`

## 已知的交互要点（踩过的坑，复用请先读）

1. **「设备上架」弹窗不是 Element Plus 的 `el-dialog`/`el-drawer`**，而是自绘模态框。不要按容器类名定位；用「可见的提交按钮」回溯弹窗根：
   `submit.evaluate_handle("el => { let n = el; while (n && !(n.querySelector && n.querySelector(\"input[placeholder='Select']\"))) n = n.parentElement; return n; }")`
2. **「目标机柜」是级联选择器（cascader），不是下拉框**。展开必须在 `.el-cascader` wrapper 上派发 `mousedown/mouseup/click`（点内部 input 无效）。
3. **级联是多列结构**：每轮只操作**最后一列**的首个节点，四步走完 数据中心 → 机房 → 机柜；若每轮都点第一列，会反复点同一个数据中心（这是最常见的坑）。
4. 页面存在多个 `el-overlay` 残留与隐藏的同名抽屉，**一切定位以"可见性"为准**，不要用全局第一个匹配。
5. 判断成功不要只看提示气泡，**以 API 复核为准**（`lifecycleStatus` 与 `/history` 条数）。

## 覆盖现状（截至 2026-09-10）

| 流程 | 状态 |
|---|---|
| 上架（UI） | 曾完整通过（弹窗 → 级联选柜 → 提交 → lifecycle=RUNNING、履历写入）；**复跑出现过弹窗未打开的不稳定**，需定位时序 |
| 移位 | **UI 无入口**（设备行操作菜单恒为 编辑/上架/履历/删除）。已用 ALE 同构载荷 + 空闲 U 位验证接口：200 且履历增加 |
| 审批 | 后台入口已探明（点「系统管理」进入 `/admin`，页面含「批准」按钮）；**批准闭环未验证**（受上架不稳定影响，未生成待批单） |
