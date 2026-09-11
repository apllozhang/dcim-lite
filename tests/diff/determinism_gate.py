#!/usr/bin/env python3
"""P1-C②/③ 确定性门禁(第四轮 P1-R5 升级):同一候选镜像、两套全新数据库各回放一遍
golden 套件后,对 run1 vs run2 的比对报告做裁决。

用法:
  python3 determinism_gate.py --report <run1-vs-run2 diff-report.json> --ledger <field-diff-decisions.yaml> [--out gate.json]

规则:
  1. 真实断裂(breaks)必须为 0 —— 候选行为对自身必须可复现;
  2. 级联缺失(missing)必须为 0 —— 同一套件两轮不应有用例缺席;
  3. 字段差异仅允许出现在台账 observationStatus=ACCEPTED_NOISE 的条目上,
     且必须通过其 noiseRule 对左右实际值的执行(与 field_gate 共用规则引擎);
  4. SYMMETRIC_SWAP(并发对称胜负互换)视为等价,放行。

依赖: PyYAML(仓库 CI 步骤显式安装)。
"""
import argparse
import hashlib
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from field_gate import check_rule  # noqa: E402  规则引擎单一实现

try:
    import yaml
except ImportError:  # pragma: no cover
    sys.exit("PyYAML is required: python3 -m pip install pyyaml")


def norm(p):
    return re.sub(r"\[\d+\]", "[*]", p)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--report", required=True)
    ap.add_argument("--ledger", required=True)
    ap.add_argument("--out", default=None)
    args = ap.parse_args()

    doc = yaml.safe_load(open(args.ledger, encoding="utf-8"))
    allowed = {}
    for e in doc.get("entries") or []:
        if e.get("observationStatus") == "ACCEPTED_NOISE" and e.get("noiseRule") in (
                "EXACT", "ABSENT_OR_NULL", "UUID_V4", "TIMESTAMP_TOLERANCE",
                "ORDER_INSENSITIVE_BY_KEY", "FIXTURE_PREFIX", "SYMMETRIC_SWAP", "FIXTURE_STATE"):
            allowed[(e["caseId"], e["fieldPath"])] = e["noiseRule"]

    rep = json.load(open(args.report, encoding="utf-8"))
    errs = []
    breaks = rep.get("breaks") or []
    missing = rep.get("missing") or []
    for x in breaks:
        errs.append(f"real break between two fresh-DB runs: {x['caseId']} | {x['why']}")
    if missing:
        errs.append(f"missing cases between runs: {missing[:10]}")

    for fb in rep.get("field_breaks") or []:
        for dif in fb.get("diffs") or []:
            k = (fb["caseId"], norm(dif["field"]))
            rule = allowed.get(k)
            if rule is None:
                errs.append(f"unregistered nondeterminism (not ACCEPTED_NOISE in ledger): {k[0]} :: {k[1]}")
                continue
            why = check_rule(rule, k[1], dif.get("baseline"), dif.get("candidate"))
            if why:
                errs.append(f"noise rule violated between runs at {k[0]} :: {k[1]} [{rule}]: {why}")

    if errs:
        for m in errs:
            print("  FAIL:", m)
        sys.exit(f"determinism gate: FAIL ({len(errs)} issue(s))")

    summary = {
        "verdict": "PASS",
        "real_breaks": len(breaks),
        "missing": len(missing),
        "field_break_cases": len(rep.get("field_breaks") or []),
        "ledger_sha256": hashlib.sha256(open(args.ledger, "rb").read()).hexdigest(),
        "commit": os.environ.get("GITHUB_SHA", ""),
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    if args.out:
        with open(args.out, "w", encoding="utf-8", newline="\n") as f:
            json.dump(summary, f, ensure_ascii=False, indent=2)
            f.write("\n")
    print("determinism gate: PASS")


if __name__ == "__main__":
    main()
