#!/usr/bin/env python3
"""P1-C②/③ 确定性门禁:同一候选镜像、两套全新数据库各回放一遍 golden 套件后,
对 run1 vs run2 的比对报告做裁决。

用法:
  python3 determinism_gate.py --report <run1-vs-run2 diff-report.json> --ledger <field-diff-decisions.yaml> [--out gate.json]

规则:
  1. 真实断裂(breaks)必须为 0 —— 候选行为对自身必须可复现;
  2. 级联缺失(missing)必须为 0 —— 同一套件两轮不应有用例缺席;
  3. 字段差异仅允许出现在台账登记为 DATA_FIXTURE_NOISE 的条目上(并发竞态次生状态),
     其余任何字段差异 = 未登记的不确定性 → 失败;
  4. SYMMETRIC_SWAP(并发对称胜负互换)视为等价,放行。

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


def norm(p):
    return re.sub(r"\[\d+\]", "[*]", p)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--report", required=True)
    ap.add_argument("--ledger", required=True)
    ap.add_argument("--out", default=None)
    args = ap.parse_args()

    doc = yaml.safe_load(open(args.ledger, encoding="utf-8"))
    noise_keys = {(e["caseId"], e["fieldPath"]) for e in doc.get("entries") or []
                  if e.get("category") == "DATA_FIXTURE_NOISE"}

    rep = json.load(open(args.report, encoding="utf-8"))
    errs = []
    breaks = rep.get("breaks") or []
    missing = rep.get("missing") or []
    for x in breaks:
        errs.append(f"real break between two fresh-DB runs: {x['caseId']} | {x['why']}")
    if missing:
        errs.append(f"missing cases between runs: {missing[:10]}")

    unknown_nd = []
    for fb in rep.get("field_breaks") or []:
        for dif in fb.get("diffs") or []:
            k = (fb["caseId"], norm(dif["field"]))
            if k not in noise_keys:
                unknown_nd.append(k)
    for cid, p in sorted(set(unknown_nd)):
        errs.append(f"unregistered nondeterminism (not DATA_FIXTURE_NOISE in ledger): {cid} :: {p}")

    if errs:
        for m in errs:
            print("  FAIL:", m)
        sys.exit(f"determinism gate: FAIL ({len(errs)} issue(s))")

    summary = {
        "verdict": "PASS",
        "real_breaks": len(breaks),
        "missing": len(missing),
        "field_break_cases": len(rep.get("field_breaks") or []),
        "noise_field_breaks": len(rep.get("field_breaks") or []),
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
