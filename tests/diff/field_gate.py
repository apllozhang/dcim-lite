#!/usr/bin/env python3
"""P1-C③ 字段门禁:台账完整性 + 报告↔台账双向一致性 + 兼容率下限。

用法:
  python3 field_gate.py --ledger docs/field-diff-decisions.yaml --report <diff-report.json> [--out gate.json]

规则(复评 §8.3 P1-C③):
  1. 报告里每条 FIELD_BREAK(caseId + 归一化字段路径)必须有台账记录 —— 未知差异默认失败;
  2. 台账 OPEN 条目必须仍出现在报告中(raceDependent=true 的竞态翻转条目除外);
  3. 台账 CLOSED 条目不得在报告中重现(residualNoise=true 的条目除外:形状已复刻,
     残余差异为行错位/库内状态噪声,回归防护由关联的 backendTest 承担);
  4. 字段兼容率不得低于台账 meta.fieldCompatRateBaseline;
  5. 台账自身 schema 校验:枚举合法、键唯一、ACCEPTED 必附理由。
  [n] -> [*] 归一化与 tests/sandbox/diff_compare_v2.py 的抽取口径一致。

依赖: PyYAML(仓库 CI 步骤显式安装)。
"""
import argparse
import hashlib
import json
import os
import re
import sys

try:
    import yaml
except ImportError:  # pragma: no cover
    sys.exit("PyYAML is required: python3 -m pip install pyyaml")

CATEGORIES = {"REQUIRED_BY_NEW_UI", "LEGACY_ONLY", "INTENTIONAL_FIX", "DATA_FIXTURE_NOISE", "REAL_DEFECT"}
STATUSES = {"OPEN", "ACCEPTED", "CLOSED"}
REQUIRED_FIELDS = ["caseId", "fieldPath", "category", "status", "decision", "reason",
                   "targetPhase", "owner", "backendTest", "frontendTest", "evidence"]


def norm(p):
    return re.sub(r"\[\d+\]", "[*]", p)


def fail(msgs):
    for m in msgs:
        print("  FAIL:", m)
    sys.exit(f"field gate: FAIL ({len(msgs)} issue(s))")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ledger", required=True)
    ap.add_argument("--report", required=True)
    ap.add_argument("--out", default=None, help="写门禁摘要 JSON(供 nightly 上传)")
    args = ap.parse_args()

    # ---- 1. 台账 schema ----
    doc = yaml.safe_load(open(args.ledger, encoding="utf-8"))
    meta, entries = doc.get("meta") or {}, doc.get("entries") or []
    errs, keys = [], set()
    for i, e in enumerate(entries):
        missing = [f for f in REQUIRED_FIELDS if f not in e]
        if missing:
            errs.append(f"entry#{i} missing fields: {missing}")
            continue
        k = (e["caseId"], e["fieldPath"])
        if k in keys:
            errs.append(f"duplicate ledger key: {k}")
        keys.add(k)
        if e["category"] not in CATEGORIES:
            errs.append(f"{k}: bad category {e['category']}")
        if e["status"] not in STATUSES:
            errs.append(f"{k}: bad status {e['status']}")
        if e["status"] == "ACCEPTED" and not (e.get("reason") or "").strip():
            errs.append(f"{k}: ACCEPTED requires reason")
        if e["status"] == "OPEN" and (e.get("targetPhase") or "-") == "-":
            errs.append(f"{k}: OPEN requires targetPhase")

    # ---- 2. 报告 FIELD_BREAK 归一化 ----
    rep = json.load(open(args.report, encoding="utf-8"))
    report_keys = set()
    for fb in rep.get("field_breaks", []):
        for dif in fb.get("diffs", []):
            report_keys.add((fb["caseId"], norm(dif["field"])))

    # ---- 3. 双向一致性 ----
    unknown = sorted(report_keys - keys)
    for cid, p in unknown:
        errs.append(f"unknown field break (no ledger record): {cid} :: {p}")
    stale_open = sorted(k for k in keys
                        if k not in report_keys and ledger_status(entries, k) == "OPEN"
                        and not ledger_flag(entries, k, "raceDependent"))
    for cid, p in stale_open:
        errs.append(f"OPEN entry no longer in report (close it or fix classification): {cid} :: {p}")
    repro = sorted(k for k in keys
                   if k in report_keys and ledger_status(entries, k) == "CLOSED"
                   and not ledger_flag(entries, k, "residualNoise"))
    for cid, p in repro:
        errs.append(f"CLOSED entry still reproducing: {cid} :: {p}")

    # ---- 4. 字段兼容率下限 ----
    rate = rep.get("compat_rate_field")
    floor = meta.get("fieldCompatRateBaseline")
    rate_ok = rate is not None and floor is not None and rate >= float(floor)
    if not rate_ok:
        errs.append(f"compat_rate_field {rate}% below ledger baseline {floor}%")

    if errs:
        fail(errs)

    digest = hashlib.sha256(open(args.ledger, "rb").read()).hexdigest()
    summary = {
        "verdict": "PASS",
        "unknown_field_breaks": 0,
        "ledger_entries": len(entries),
        "report_field_break_cases": len(rep.get("field_breaks", [])),
        "report_field_diffs": len(report_keys),
        "compat_rate_field": rate,
        "compat_floor": floor,
        "ledger_sha256": digest,
        "commit": os.environ.get("GITHUB_SHA", ""),
        "image_digest": os.environ.get("FIELD_GATE_IMAGE_DIGEST", ""),
        "statuses": _count(entries, "status"),
        "categories": _count(entries, "category"),
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    if args.out:
        with open(args.out, "w", encoding="utf-8", newline="\n") as f:
            json.dump(summary, f, ensure_ascii=False, indent=2)
            f.write("\n")
    print("field gate: PASS")


def ledger_status(entries, key):
    for e in entries:
        if (e["caseId"], e["fieldPath"]) == key:
            return e["status"]
    return None


def ledger_flag(entries, key, flag):
    for e in entries:
        if (e["caseId"], e["fieldPath"]) == key:
            return bool(e.get(flag))
    return False


def _count(entries, field):
    out = {}
    for e in entries:
        out[e[field]] = out.get(e[field], 0) + 1
    return out


if __name__ == "__main__":
    main()
