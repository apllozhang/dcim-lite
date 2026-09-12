#!/usr/bin/env bash
# 部署后冒烟(P0-R3/P0-R1/P1-D):不通过则视为部署失败,立即按 README 回滚。
# 用法: ./smoke.sh [new-ui-base] [legacy-base]
#   new-ui-base 默认 http://127.0.0.1:19500
#   legacy-base 默认 http://127.0.0.1:19501(旧 ALE 独立回退源)
set -euo pipefail

BASE="${1:-http://127.0.0.1:19500}"
LEGACY="${2:-http://127.0.0.1:19501}"
fail() { echo "SMOKE FAIL: $1" >&2; exit 1; }

code() { curl -s -o /dev/null -w '%{http_code}' "$1"; }

# 1) 新前端可访问
[ "$(code "$BASE/login")" = "200" ] || fail "$BASE/login 非 200"
# 2) 后端反代可用(验证码接口)
[ "$(code "$BASE/api/v1/auth/captcha")" = "200" ] || fail "captcha 非 200(backend 反代异常)"
# 3) 健康就绪
[ "$(code "$BASE/health/ready")" = "200" ] || fail "/health/ready 非 200"
# 4) 旧 UI 独立源:HTML5 路径经 fallback 返回主文档(标题断言,而非仅 URL)
legacy_html="$(curl -s "$LEGACY/devices")"
echo "$legacy_html" | grep -q "机柜管理工具" || fail "$LEGACY/devices 未返回旧 bundle 主文档"
# 5) 旧 bundle 静态资源可加载(取 index.html 里第一个 /assets/ 引用验证 200)
asset="$(echo "$legacy_html" | grep -o '/assets/[^"]*\.js' | head -n1 || true)"
[ -n "$asset" ] || fail "旧 index.html 未发现静态资源引用"
[ "$(code "$LEGACY$asset")" = "200" ] || fail "旧静态资源 $asset 非 200"
# 6) /metrics 不得经公网前端暴露(P1-R6)
[ "$(code "$BASE/metrics")" != "200" ] || fail "/metrics 经公网前端可达,违反收紧策略"

echo "SMOKE PASS ($BASE + $LEGACY)"
