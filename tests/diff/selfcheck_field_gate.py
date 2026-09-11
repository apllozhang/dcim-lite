#!/usr/bin/env python3
"""P1-R5 负向自测:语义门禁必须能拦住"已登记路径上的新语义漂移"。

每例构造(合成台账 + 合成报告)→ 运行 field_gate.main 断言失败/通过:
  1) 未知路径                          → FAIL
  2) ABSENT_OR_NULL 路径两侧均实值      → FAIL
  3) 枚举路径注入非法枚举               → FAIL
  4) 噪声路径注入 null                  → FAIL
  5) 噪声路径注入疑似敏感值             → FAIL
  6) TIMESTAMP 规则路径注入非时间戳     → FAIL
  7) CLOSED 条目重现                    → FAIL
  8) 兼容率低于 ratchet 基线            → FAIL
  9) 全部合规                          → PASS
运行: python3 tests/diff/selfcheck_field_gate.py   (CI quality job 调用,退出码非 0 即门禁失效)
"""
import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = os.path.join(HERE, "field_gate.py")

LEDGER = """
meta:
  fieldCompatRateBaseline: 70.0
categories: [REAL_DEFECT]
implementationStatuses: [IMPLEMENTED, PENDING]
observationStatuses: [CLOSED, ACCEPTED_NOISE, OBSERVED]
entries:
  - caseId: CASE-A
    fieldPath: data.items[*].device.version
    category: REAL_DEFECT
    implementationStatus: IMPLEMENTED
    observationStatus: ACCEPTED_NOISE
    noiseRule: "FIXTURE_STATE"
    decision: NOISE_VERIFIED
    reason: "数值随库内状态漂移"
    targetPhase: "x"
    owner: backend
    backendTest: "t1"
    frontendTest: "t2"
    evidence: "e"
  - caseId: CASE-B
    fieldPath: data.type.status
    category: INTENTIONAL_FIX
    implementationStatus: IMPLEMENTED
    observationStatus: ACCEPTED_NOISE
    noiseRule: "ABSENT_OR_NULL"
    decision: KEEP_CANDIDATE
    reason: "厂商空壳"
    targetPhase: "x"
    owner: backend
    backendTest: "t3"
    frontendTest: ""
    evidence: "e"
  - caseId: CASE-C
    fieldPath: data.rack.createdAt
    category: REAL_DEFECT
    implementationStatus: IMPLEMENTED
    observationStatus: ACCEPTED_NOISE
    noiseRule: "TIMESTAMP_TOLERANCE"
    decision: NOISE_VERIFIED
    reason: "时间戳归一化差异"
    targetPhase: "x"
    owner: backend
    backendTest: "t4"
    frontendTest: ""
    evidence: "e"
  - caseId: CASE-D
    fieldPath: data.items[*].currentPosition.endU
    category: REAL_DEFECT
    implementationStatus: IMPLEMENTED
    observationStatus: CLOSED
    noiseRule: "EXACT"
    decision: REPLICATED
    reason: "形状已复刻且不再出现"
    targetPhase: "x"
    owner: backend
    backendTest: "t5"
    frontendTest: "t6"
    evidence: "e"
"""


def run_case(ledger_text, report, expect_pass, label):
    tmp = tempfile.mkdtemp(prefix="fieldgate-neg-")
    led = os.path.join(tmp, "ledger.yaml")
    rep = os.path.join(tmp, "report.json")
    with open(led, "w", encoding="utf-8", newline="\n") as f:
        f.write(ledger_text)
    with open(rep, "w", encoding="utf-8", newline="\n") as f:
        json.dump(report, f)
    r = subprocess.run([sys.executable, GATE, "--ledger", led, "--report", rep],
                       capture_output=True, text=True)
    passed = (r.returncode == 0)
    ok = (passed == expect_pass)
    print(("PASS-CASE OK " if ok else "!! SELFTEST FAIL ") + label + f" (gate exit={r.returncode})")
    if not ok:
        print(r.stdout[-800:], r.stderr[-200:])
        sys.exit(1)


def report_for(entries, rate=71.0):
    return {
        "compat_rate_field": rate,
        "field_breaks": [{"caseId": c, "diffs": [{"field": f, "baseline": b, "candidate": cd} for f, b, cd in ds]}
                         for c, ds in entries],
    }


# 9) 全部合规基线:合法噪声形态
GOOD = report_for([
    ("CASE-A", [("data.items[*].device.version", 3, 4)]),
    ("CASE-B", [("data.type.status", "", "ACTIVE")]),
    ("CASE-C", [("data.rack.createdAt", "<TS>", "<TS>")]),
])
run_case(LEDGER, GOOD, True, "baseline 合规报告 → PASS")

# 1) 未知路径
run_case(LEDGER, report_for([("CASE-Z", [("data.new.field", 1, 2)])]), False, "未知路径默认失败")

# 2) ABSENT_OR_NULL 两侧均实值
run_case(LEDGER, report_for([("CASE-B", [("data.type.status", "ACTIVE", "ACTIVE")])]), False,
         "ABSENT_OR_NULL 两侧实值被拦")

# 3) 枚举注入非法值(lifecycleStatus/status 枚举绑定)
run_case(LEDGER, report_for([("CASE-B", [("data.device.lifecycleStatus", "RUNNING", "NOT_AN_ENUM")])]), False,
         "枚举字段非法值被拦")

# 4) null 注入
run_case(LEDGER, report_for([("CASE-A", [("data.items[*].device.version", 3, None)])]), False,
         "null 注入被拦")

# 5) 敏感值注入
run_case(LEDGER, report_for([("CASE-A", [("data.items[*].device.version", 3, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9SECRET")])]),
         False, "疑似敏感值注入被拦")

# 6) TIMESTAMP 路径注入非时间戳
run_case(LEDGER, report_for([("CASE-C", [("data.rack.createdAt", "<TS>", "not-a-timestamp")])]), False,
         "非时间戳值被拦")

# 7) CLOSED 条目重现
run_case(LEDGER, report_for([("CASE-D", [("data.items[*].currentPosition.endU", 5, 6)])]), False,
         "CLOSED 重现被拦")

# 8) 兼容率低于 ratchet
run_case(LEDGER, report_for([("CASE-A", [("data.items[*].device.version", 3, 4)])], rate=69.5), False,
         "兼容率低于 ratchet 基线被拦")

print("field gate selftest: ALL PASS")
