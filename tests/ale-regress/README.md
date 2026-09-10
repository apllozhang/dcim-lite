# ALE 前端回归

针对 `frontend/ale` 编译产物（厂商前端 bundle + 我们的增强脚本）的浏览器级回归。
与 `tests/sandbox`（API 层差分回放）互补。

## 组成

| 路径 | 定位 | 说明 |
|---|---|---|
| `conftest.py` + `test_ale_flows.py` | **pytest 套件（可信回归）** | 隔离、幂等、非零退出；失败留截图。可做 CI 门禁（前提：执行环境有 ALE 前端 + Chromium） |
| `probe_device_ops.py` / `probe_assign_modal.py` | 实验性 DOM 探针 | 探索旧编译前端内部结构的一次性脚本，产物是知识不是断言；有硬编码、无清理，勿当测试跑 |

## 三类验证不可互替

| 类型 | 例 | 说明 |
|---|---|---|
| UI E2E | 登录、上架、审批批准 | 用户真实点击路径 |
| API 兼容验证 | 移位用例 | 复用 ALE 前端的请求形状直打 API，验证的是后端兼容性。**ALE 界面无移位入口**（已实测：设备行操作菜单恒为 编辑/上架/履历/删除） |
| DOM 探针 | probe_*.py | 探索编译产物内部结构 |

## pytest 套件运行方式

在部署主机执行（前端需可从该机访问；Chromium 需 `--no-sandbox`）：

```bash
python3 -m venv ~/ale-venv
~/ale-venv/bin/pip install playwright pytest
~/ale-venv/bin/playwright install chromium

export ALE_BASE_URL=http://127.0.0.1:19173   # 前端地址
export ALE_API_URL=http://127.0.0.1:19080    # 后端地址
export E2E_USERNAME=admin
export E2E_PASSWORD=...                       # 从部署 .env 读取，勿入库
export E2E_ALLOW_MUTATION=true                # 变异门控：用例会写业务数据

~/ale-venv/bin/python -m pytest test_ale_flows.py -v
# 失败截图在 E2E_ARTIFACT_DIR（默认 ./ale-regress-artifacts/）
```

全部环境变量缺省即失败（无硬编码地址/口令路径）；未显式 `E2E_ALLOW_MUTATION=true`
时套件拒绝执行。任一用例失败进程返回非零，可直接接 CI。

## 隔离与清理

- 每个用例经 `env` fixture **自建**唯一 run ID 的数据中心/机房/机柜/设备（API 创建），
  UI 级联选择只点名称含本 run ID 的节点，绝不操作存量数据；
- 审批策略先读原值，teardown 无条件恢复（try/finally 语义）；
- teardown 按依赖逆序清理自建对象（设备下架删除 → 机柜 → 机房 → 数据中心），
  清理失败打印 `[WARN]` 不掩盖用例结果。

## 已知的交互要点（踩过的坑，复用请先读）

1. **「设备上架」弹窗不是 Element Plus 的 `el-dialog`/`el-drawer`**，而是自绘模态框。
   定位用「可见的提交按钮」回溯弹窗根（`conftest.py` 之外的 `pick_own_rack` 有实现）。
   弹窗偶发不打开（历史时序问题）：`open_assign_modal` 显式 `wait_for_selector` + 3 次重试。
2. **「目标机柜」是级联选择器（cascader），不是下拉框**。展开必须在 `.el-cascader`
   wrapper 上派发 `mousedown/mouseup/click`（点内部 input 无效）。
3. **级联是多列结构**：每轮只操作**最后一列**；若每轮都点第一列，会反复点同一个数据中心。
4. 页面存在多个 `el-overlay` 残留与隐藏的同名抽屉，**一切定位以"可见性"为准**。
5. 判断成功不要只看提示气泡，**以 API 复核为准**（`lifecycleStatus` 与 `/history` 条数）。

## 覆盖现状（截至 2026-09-11）

| 流程 | 类型 | 状态 |
|---|---|---|
| 登录 | UI E2E | ✅ pytest 套件 |
| 上架 | UI E2E | ✅ pytest 套件（自建数据 + 弹窗重试；旧版偶发不打开问题已用显式等待+重试缓解） |
| 审批批准闭环 | UI E2E | ✅ pytest 套件（/admin 后台点批准 → API 复核 RUNNING） |
| 移位 | API 兼容 | ✅ pytest 套件（UI 无入口，同构载荷 + 空闲 U 位） |
