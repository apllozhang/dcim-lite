# 新旧 UI 对照截图（2026-09-13）

同一套 203 dev 后端数据（72 数据中心 / 69 机房 / 90 机柜；`MTX-*`/`DUAL-*`/`DBG-*` 为 e2e 种子），两侧各登录 admin 后顺序截取，视口 1600×900（列表页 fullPage）。

| # | 页面 | 旧 UI（v2 复刻基准，:19501） | 新 UI（自主前端，:19502） |
|---|------|------|------|
| 01 | 运行概览 | old-ui/01-dashboard.png | new-ui/01-dashboard.png |
| 02 | 资源层级 | old-ui/02-hierarchy.png | new-ui/02-hierarchy.png |
| 03 | 设备台账 | old-ui/03-devices.png | new-ui/03-devices.png |
| 04 | 机柜管理 | old-ui/04-racks.png | new-ui/04-racks.png |
| 05 | 机柜模板 | old-ui/05-rack-templates.png | new-ui/05-rack-templates.png |
| 06 | 机房大屏（暗色） | old-ui/06-room-screen.png | new-ui/06-room-screen.png |
| 07 | 机柜详情与容量分析（新 UI 特有入口） | — | new-ui/07-capacity-dialog.png |
| 08 | 机柜图导入对话框（新 UI 特有入口） | — | new-ui/08-import-dialog.png |

说明：

- 旧 UI 即 `frontend/ale`（v2 编译产物，19500 主入口同款换肤版）；新 UI 即 `frontend/app` 自主源码。
- 07/08 为 v2 也存在但需交互才可见的浮层，按新 UI 现状截取；对应规格取自 v2 编译产物（见 `../EXPERT-REVIEW-GUIDE-20260913.md` §5）。
- 截图为运行时快照，数据会随后续测试变化；以本目录提交时点为准。
