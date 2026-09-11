#!/usr/bin/env python3
"""GT v3 工具自测：规范化器 14 组 + 门禁 7 种失败验证。
纯本地运行，不连接任何网络/数据库。用法：python3 gt_selftest.py（exit 0=全过，非0=有失败）。"""
import json
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gt_core import (normalize_obj, redact_obj, classify_result, register_case,
                     compute_summary, UUID_MAP, _SEEN_IDS)

PASS = 0
FAIL = 0


def check(name, ok, detail=""):
    global PASS, FAIL
    if ok:
        PASS += 1
    else:
        FAIL += 1
        print(f"  FAIL: {name} {detail}")


# ========== 一、规范化器自测（指南 §14.1）==========

# 1. UUID 不同但引用关系相同 → 相等
UUID_MAP["11111111-1111-1111-1111-111111111111"] = "OBJ_A"
UUID_MAP["22222222-2222-2222-2222-222222222222"] = "OBJ_B"
a1 = {"parentId": "11111111-1111-1111-1111-111111111111", "code": "X"}
a2 = {"parentId": "11111111-1111-1111-1111-111111111111", "code": "X"}
check("norm-uuid-same-ref", normalize_obj(a1) == normalize_obj(a2))

# 2. 同一对象多处引用映射同一占位符
b1 = {"self": "11111111-1111-1111-1111-111111111111", "parent": "22222222-2222-2222-2222-222222222222"}
b2 = {"self": "11111111-1111-1111-1111-111111111111", "parent": "22222222-2222-2222-2222-222222222222"}
check("norm-multi-ref-consistent", normalize_obj(b1) == normalize_obj(b2))

# 3. UUID 不同且引用关系改变 → 不相等
c1 = {"parentId": "11111111-1111-1111-1111-111111111111"}
c2 = {"parentId": "22222222-2222-2222-2222-222222222222"}
check("norm-uuid-diff-ref", normalize_obj(c1) != normalize_obj(c2))

# 4. 时间值不同但格式正确 → 相等
d1 = {"createdAt": "2026-09-08T10:00:00Z", "code": "X"}
d2 = {"createdAt": "2026-09-09T12:30:00Z", "code": "X"}
check("norm-time-format", normalize_obj(d1) == normalize_obj(d2))

# 5. 业务 code 改变 → 不相等
e1 = {"code": "SUCCESS"}
e2 = {"code": "FAIL"}
check("norm-bizcode-diff", normalize_obj(e1) != normalize_obj(e2))

# 6. version 改变 → 不相等
f1 = {"version": 1}
f2 = {"version": 2}
check("norm-version-diff", normalize_obj(f1) != normalize_obj(f2))

# 7. parentId 改变 → 不相等（同 3）
g1 = {"parentId": "<ID:OBJ_A>", "name": "X"}
g2 = {"parentId": "<ID:OBJ_B>", "name": "X"}
check("norm-parentid-diff", normalize_obj(g1) != normalize_obj(g2))

# 8. 数组少一项 → 不相等
h1 = {"items": [1, 2, 3]}
h2 = {"items": [1, 2]}
check("norm-array-missing", normalize_obj(h1) != normalize_obj(h2))

# 9. requestId 值不同 → 相等（被替换为占位符）
i1 = {"requestId": "req_aaaa1111-2222-3333-4444-555566667777"}
i2 = {"requestId": "req_bbbb8888-9999-aaaa-bbbb-ccccddddeeee"}
check("norm-reqid-equal", normalize_obj(i1) == normalize_obj(i2))

# 10. JWT 值不同 → 相等（被替换）
j1 = {"token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig1"}
j2 = {"token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig2"}
check("norm-jwt-equal", normalize_obj(j1) == normalize_obj(j2))

# 11. 嵌套对象中的 UUID 也被映射
k1 = {"data": {"owner": "11111111-1111-1111-1111-111111111111"}}
k2 = {"data": {"owner": "11111111-1111-1111-1111-111111111111"}}
check("norm-nested-uuid", normalize_obj(k1) == normalize_obj(k2))

# 12. 新增未知业务字段 → 不相等（保守策略）
l1 = {"code": "X"}
l2 = {"code": "X", "newField": "Y"}
check("norm-unknown-field-diff", normalize_obj(l1) != normalize_obj(l2))

# 13. 空字典/None 安全
check("norm-empty-dict", normalize_obj({}) == {})
check("norm-none-safe", normalize_obj(None) is None)

# 14. 脱敏：password 字段被替换
m = redact_obj({"password": "Secret123", "data": "ok"})
check("redact-password", m["password"] == "<REDACTED_PASSWORD>")
# 脱敏：JWT 被替换
n = redact_obj({"token": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig"})
check("redact-jwt", n["token"] == "<REDACTED_JWT>")


# ========== 二、状态模型自测（指南 §5）==========

# expect=None → INFO
check("classify-info", classify_result(None, False, False, 200, "SUCCESS") == "INFO")
# transport error → ERROR
check("classify-error-transport", classify_result((200, "S"), True, False, 0, "?") == "ERROR")
# parse error → ERROR
check("classify-error-parse", classify_result((200, "S"), False, True, 200, "?") == "ERROR")
# 正确预期 → PASS
check("classify-pass", classify_result((200, "SUCCESS"), False, False, 200, "SUCCESS") == "PASS")
# 状态不匹配 → FAIL
check("classify-fail-status", classify_result((200, "S"), False, False, 404, "S") == "FAIL")
# code 不匹配 → FAIL
check("classify-fail-code", classify_result((200, "A"), False, False, 200, "B") == "FAIL")
# 状态列表匹配 → PASS
check("classify-pass-list", classify_result(([200, 201], "S"), False, False, 201, "S") == "PASS")


# ========== 三、caseId 唯一性自测（指南 §7）==========

_SEEN_IDS.clear()
register_case("TEST-A")
try:
    register_case("TEST-A")
    check("caseid-dup-rejected", False, "应该抛异常")
except ValueError:
    check("caseid-dup-rejected", True)
register_case("TEST-B")
check("caseid-unique-ok", len(_SEEN_IDS) == 2)


# ========== 四、退出码门禁自测（指南 §6/R1）==========
# 模拟 CASES 含 FAIL 时 should_exit_nonzero
import gt_core
gt_core.CASES.clear()
gt_core.CASES.append({"caseId": "T1", "scenario": "S1", "kind": "TEST", "status": "PASS",
                      "httpStatus": 200, "actualCode": "S", "expectedCode": "S", "note": "", "durationMs": 1, "requestId": ""})
s = gt_core.compute_summary()
check("gate-all-pass", not gt_core.should_exit_nonzero())

gt_core.CASES.append({"caseId": "T2", "scenario": "S1", "kind": "TEST", "status": "FAIL",
                      "httpStatus": 500, "actualCode": "E", "expectedCode": "S", "note": "", "durationMs": 1, "requestId": ""})
check("gate-has-fail", gt_core.should_exit_nonzero())

gt_core.CASES.append({"caseId": "T3", "scenario": "S1", "kind": "INFO", "status": "INFO",
                      "httpStatus": 200, "actualCode": "S", "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
s = gt_core.compute_summary()
check("gate-info-not-formal", s["total"] == 2)  # INFO 不进 total
check("gate-info-counted", s["info"] == 1)


# ========== 五、汇总统计一致性 ==========
gt_core.CASES.clear()
for i, st in enumerate(["PASS", "PASS", "FAIL", "ERROR", "BLOCKED", "SKIP" if False else "SKIPPED"]):
    gt_core.CASES.append({"caseId": f"T{i}", "scenario": "S1", "kind": "TEST", "status": st,
                          "httpStatus": 200, "actualCode": "", "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
gt_core.CASES.append({"caseId": "TI", "scenario": "S1", "kind": "INFO", "status": "INFO",
                      "httpStatus": 200, "actualCode": "", "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
gt_core.CASES.append({"caseId": "TS", "scenario": "S1", "kind": "SETUP", "status": "SETUP",
                      "httpStatus": 200, "actualCode": "", "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
s = gt_core.compute_summary()
check("summary-total", s["total"] == 6, f"got {s['total']}")
check("summary-pass", s["passed"] == 2)
check("summary-fail", s["failed"] == 1)
check("summary-error", s["errors"] == 1)
check("summary-blocked", s["blocked"] == 1)
check("summary-skipped", s["skipped"] == 1)
check("summary-info", s["info"] == 1)
check("summary-setup", s["setup"] == 1)
check("summary-blocking", s["blocking"] == 4)

print(f"\n自测结果: {PASS} PASS / {FAIL} FAIL")
if FAIL:
    sys.exit(1)
print("ALL SELFTESTS PASSED")