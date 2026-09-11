#!/usr/bin/env bash
# 部署后冒烟(P0-R3/P0-R1):不通过则视为部署失败,立即按 README 回滚。
# 用法: ./smoke.sh [base-url]   默认 http://127.0.0.1:19500
set -euo pipefail

BASE="${1:-http://127.0.0.1:19500}"
fail() { echo "SMOKE FAIL: $1" >&2; exit 1; }

code() { curl -s -o /dev/null -w '%{http_code}' "$1"; }

# 1) 新前端可访问
[ "$(code "$BASE/login")" = "200" ] || fail "$BASE/login 非 200"
# 2) 后端反代可用(验证码接口)
[ "$(code "$BASE/api/v1/auth/captcha")" = "200" ] || fail "captcha 非 200(backend 反代异常)"
# 3) 健康就绪
[ "$(code "$BASE/health/ready")" = "200" ] || fail "/health/ready 非 200"
# 4) 旧 bundle 页面真实可加载(标题断言,而非仅 URL)
legacy_html="$(curl -s "$BASE/legacy/")"
echo "$legacy_html" | grep -q "机柜管理工具" || fail "/legacy/ 未返回旧 bundle 主文档"
# 5) 旧 bundle 静态资源可加载(取 index.html 里第一个 /assets/ 引用验证 200)
asset="$(echo "$legacy_html" | grep -o '/assets/[^"]*\.js' | head -n1 || true)"
[ -n "$asset" ] || fail "旧 index.html 未发现静态资源引用"
[ "$(code "$BASE$asset")" = "200" ] || fail "旧静态资源 $asset 非 200"

echo "SMOKE PASS ($BASE)"
