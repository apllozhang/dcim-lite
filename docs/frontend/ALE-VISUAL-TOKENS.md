# ALE 视觉令牌规范(Phase 0 冻结版,2026-09-11)

> 基准:ALE WebUI 设计规范 v4.1 + 用户指定参考站(10.20.30.103:8899)`/assets/css/tokens.css`
> 2026-09-07 实测值。实现落点:`frontend/app/src/design-system/tokens.css`,
> Element Plus 主题变量经该文件覆盖(`:root` + `--el-*` 映射),业务页面不得散布硬编码色值。

## 1. 色彩

| 令牌 | 值 | 用途 |
|---|---|---|
| --ale-primary | #6b489d | 主紫:主按钮、选中态、品牌强调 |
| --ale-primary-deep | #4f3478 | 深紫:侧边栏底、页头、hover 加深 |
| --ale-primary-light | #7e5cb4 | 浅紫:链接 hover、次级强调 |
| --ale-purple-100 | #f1ecf7 | 紫底 100:选中行底、标签底 |
| --ale-ink-900 | #1a1a1a | 一级文字 |
| --ale-ink-700 | #4b4d50 | 二级文字 |
| --ale-ink-500 | #75787b | 辅助文字、占位 |
| --ale-surface | #ffffff | 卡片/面板底 |
| --ale-surface-soft | #f7f7f5 | 页面底、次级面板 |
| --ale-line | #d9d9d6 | 分隔线、边框 |
| --ale-line-soft | #e9e8e4 | 次级分隔 |
| 成功 | #4f9153 | 成功态(绿色,与换肤批次一致) |
| 信息 | #6b489d(紫)/EP info 紫调 | 信息态(语义色最终版:绿=成功、紫=信息) |
| 警告 / 危险 | EP 默认 amber/red | 沿用 EP,经 --el-* 映射统一灰阶 |

对比度:文字/背景组合满足 WCAG 2.2 AA(规范 §WCAG 章);ink-900/700/500 仅用于浅底;
深底(侧边栏/页头)文字一律 #ffffff 或 purple-100。

## 2. 字体

```css
--ale-font-family: "Trebuchet MS", "Noto Sans SC", "Microsoft YaHei",
  -apple-system, "Segoe UI", sans-serif;
```
字号阶:12(辅助)/ 13(表格紧凑)/ 14(正文默认)/ 16(小标题)/ 20(页标题)。
数字列(U 位、功率、版本)使用 `font-variant-numeric: tabular-nums`。

## 3. 几何

| 令牌 | 值 |
|---|---|
| --ale-radius | 6px(卡片 8px) |
| --ale-space-1..4 | 4 / 8 / 16 / 24px |
| --ale-header-h | 56px(页头) |
| --ale-sidebar-w | 220px(可折叠至 64px) |
| 阴影 | 0 1px 2px rgba(26,26,26,.08)(浮层加倍) |

## 4. Element Plus 映射(摘要)

```css
:root {
  --el-color-primary: var(--ale-primary);
  --el-color-primary-dark-2: var(--ale-primary-deep);
  --el-color-primary-light-3: var(--ale-primary-light);
  --el-color-primary-light-9: var(--ale-purple-100);
  --el-text-color-primary: var(--ale-ink-900);
  --el-text-color-regular: var(--ale-ink-700);
  --el-text-color-secondary: var(--ale-ink-500);
  --el-border-color: var(--ale-line);
  --el-border-color-lighter: var(--ale-line-soft);
  --el-fill-color-light: var(--ale-surface-soft);
  --el-font-family: var(--ale-font-family);
  --el-border-radius-base: var(--ale-radius);
}
```

## 5. Logo 与品牌资产

- 官方 logo 库:`F:\AIwork\Marketing Resources\ALE Brand\ALE-logos\`(嵌套 zip,取 PNG 彩色/反白两版);
- 浅底用彩色版、深底(侧边栏/登录页左幅)用反白版;禁止拉伸、禁止使用 JPG 白底版;
- 版权标注 "Do not redistribute commercially"(商务委托途径,用户已确认);页脚法律声明按规范附录 D.4,上线前过法务。

## 6. 验收

- 任一页面截图与旧 ALE 同屏对比:品牌色、字体、密度一致(信息架构允许优化);
- `tokens.css` 为唯一样式来源;lint 规则禁止业务代码出现 `#hex`(design-system 目录除外)。
