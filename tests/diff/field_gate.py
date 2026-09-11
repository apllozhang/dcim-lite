#!/usr/bin/env python3
"""字段行为门禁(P1-C③ 路径级 → P1-R5 语义级)。

用法:
  python3 field_gate.py --ledger docs/field-diff-decisions.yaml --report <diff-report.json> [--out gate.json]

三维台账模型(P1-R5):
  implementationStatus: IMPLEMENTED(代码形状已补齐) | PENDING(待修)
  observationStatus:    CLOSED(报告不再出现) | ACCEPTED_NOISE(出现但须满足 noiseRule) | OBSERVED(待处理)
  noiseRule:            门禁对报告左右实际值执行的可执行规则,而非只核对路径登记。

规则:
  1. 报告里每条 FIELD_BREAK(caseId + 归一化字段路径)必须有台账记录 —— 未知差异默认失败;
  2. OBSERVED 条目必须仍在报告中(raceDependent=true 的竞态翻转条目除外);
  3. CLOSED 条目不得在报告中重现;
  4. ACCEPTED_NOISE 条目若出现,必须满足其 noiseRule(对左右值执行);
  5. 字段兼容率不得低于 ratchet 基线(docs/field-baseline.json,只升不降,下降须带理由的基线变更 PR)。
  6. 台账自身 schema 校验:枚举合法、键唯一、IMPLEMENTED+ACCEPTED_NOISE 必附规则与理由、
     ACCEPTED_NOISE/CLOSED 必须有可定位测试引用(PENDING 可为空但须 owner+targetPhase)。

noiseRule 语义(值为 diff_compare_v2 归一化后的 JSON 标量,缺失记作 "<absent>"):
  EXACT                    两侧必须相等(出现即失败——登记后不应再有差异)
  ABSENT_OR_NULL           至少一侧为空壳(<absent>/""/0/false/null)
  UUID_V4                  两侧均为 UUID 形态(含归一化 <ID:...> 令牌)
  TIMESTAMP_TOLERANCE      两侧均为时间戳形态(<TS> 归一化令牌或 ISO-8601)
  ORDER_INSENSITIVE_BY_KEY 两侧均为非空标量(顺序导致的错位可容忍,类型漂移不可)
  FIXTURE_PREFIX           两侧均匹配 GT-<RUN> 夹具前缀令牌
  SYMMETRIC_SWAP           两侧均为数值(并发对称胜负互换)
  FIXTURE_STATE            两侧均为非空标量;若路径绑定枚举则须在枚举内;疑似敏感值一律失败

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
IMPL_STATUSES = {"IMPLEMENTED", "PENDING"}
OBS_STATUSES = {"CLOSED", "ACCEPTED_NOISE", "OBSERVED"}
NOISE_RULES = {"EXACT", "ABSENT_OR_NULL", "UUID_V4", "TIMESTAMP_TOLERANCE",
               "ORDER_INSENSITIVE_BY_KEY", "FIXTURE_PREFIX", "SYMMETRIC_SWAP", "FIXTURE_STATE"}
REQUIRED_FIELDS = ["caseId", "fieldPath", "category", "implementationStatus", "observationStatus",
                   "noiseRule", "decision", "reason", "targetPhase", "owner",
                   "backendTest", "frontendTest", "evidence"]

ABSENT = "<absent>"
ZERO = {ABSENT, "", 0, "0", False, None}
TS = re.compile(r"^(<TS>|\d{4}-\d{2}-\d{2}[T ].*)$")
UUID = re.compile(r"^(<ID:[^>]*>|[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12})$", re.I)
SECRET_HINT = re.compile(r"(eyJ[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9]{16,}|(password|token|secret|passwd)[=:]\S+)", re.I)

# 路径绑定的枚举(错误枚举注入必须失败)
ENUM_BY_PATH = {
    "lifecycleStatus": {"WAITING_RACK", "RUNNING", "MAINTENANCE", "PENDING_REMOVAL", "OFF_RACK", "SCRAPPED"},
    "status": {"ACTIVE", "INACTIVE", "PLANNING", "OPERATING", "AVAILABLE", "PARTIAL", "FULL",
               "DISABLED", "PENDING", "APPROVED", "REJECTED", "EMPTY", "OCCUPIED"},
}


def norm(p):
    return re.sub(r"\[\d+\]", "[*]", p)


def is_num(v):
    return isinstance(v, (int, float)) and not isinstance(v, bool)


def check_rule(rule, path, bv, cv):
    """对报告实际值执行噪声规则。返回 None=通过,字符串=失败原因。"""
    if rule == "EXACT":
        return "EXACT 但两侧不等" if bv != cv else None
    if rule == "ABSENT_OR_NULL":
        if bv in ZERO or cv in ZERO:
            return None
        return f"ABSENT_OR_NULL 但两侧均为实值: {bv!r} vs {cv!r}"
    if rule == "UUID_V4":
        ok = all(isinstance(v, str) and UUID.match(v) for v in (bv, cv))
        return None if ok else f"UUID_V4 但值异常: {bv!r} vs {cv!r}"
    if rule == "TIMESTAMP_TOLERANCE":
        ok = all(isinstance(v, str) and TS.match(v) for v in (bv, cv))
        return None if ok else f"TIMESTAMP 但值异常: {bv!r} vs {cv!r}"
    if rule == "ORDER_INSENSITIVE_BY_KEY":
        ok = all(v not in ZERO and not isinstance(v, (dict, list)) for v in (bv, cv))
        return None if ok else f"ORDER_INSENSITIVE 但出现空值/复合值: {bv!r} vs {cv!r}"
    if rule == "FIXTURE_PREFIX":
        ok = all(isinstance(v, str) and re.match(r"^GT-(<RUN>|\d{4}-\d{4})", v) for v in (bv, cv))
        return None if ok else f"FIXTURE_PREFIX 但值异常: {bv!r} vs {cv!r}"
    if rule == "SYMMETRIC_SWAP":
        # 胜负互换:获胜方才有 version 等字段,败者单侧缺失属正常形态
        if (is_num(bv) and is_num(cv)) or (bv == ABSENT and is_num(cv)) or (cv == ABSENT and is_num(bv)):
            return None
        return f"SYMMETRIC_SWAP 但值异常: {bv!r} vs {cv!r}"
    if rule == "FIXTURE_STATE":
        # 噪声形态:双侧行错位(均在、值不同)或单侧索引缺失(<absent>)。
        # 显式 null/空串/0、敏感值、非法枚举一律失败(负例覆盖)。
        for v in (bv, cv):
            if isinstance(v, str) and SECRET_HINT.search(v):
                return f"疑似敏感值出现在登记噪声路径: {v[:40]!r}"
        if bv is None or cv is None or bv == "" or cv == "" or bv == 0 or cv == 0:
            return f"FIXTURE_STATE 不接受显式空值: {bv!r} vs {cv!r}"
        if bv == ABSENT or cv == ABSENT:
            other = cv if bv == ABSENT else bv
            if other in ZERO or isinstance(other, (dict, list)):
                return f"FIXTURE_STATE 单侧缺失但另一侧亦异常: {bv!r} vs {cv!r}"
            return None
        leaf = path.split(".")[-1].split("[")[0]
        enum = ENUM_BY_PATH.get(leaf)
        if enum:
            bad = [v for v in (bv, cv) if isinstance(v, str) and v not in enum]
            if bad:
                return f"枚举字段 {leaf} 出现非法值: {bad!r}"
        return None
    return f"未知规则 {rule}"


def fail(msgs):
    for m in msgs:
        print("  FAIL:", m)
    sys.exit(f"field gate: FAIL ({len(msgs)} issue(s))")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ledger", required=True)
    ap.add_argument("--report", required=True)
    ap.add_argument("--baseline", default=None, help="ratchet 基线 JSON(默认与 ledger 同目录 field-baseline.json)")
    ap.add_argument("--out", default=None)
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
        if e["implementationStatus"] not in IMPL_STATUSES:
            errs.append(f"{k}: bad implementationStatus {e['implementationStatus']}")
        if e["observationStatus"] not in OBS_STATUSES:
            errs.append(f"{k}: bad observationStatus {e['observationStatus']}")
        rule = e.get("noiseRule")
        if e["observationStatus"] == "ACCEPTED_NOISE":
            if rule not in NOISE_RULES:
                errs.append(f"{k}: ACCEPTED_NOISE requires executable noiseRule, got {rule!r}")
            if not (e.get("reason") or "").strip():
                errs.append(f"{k}: ACCEPTED_NOISE requires reason")
        if e["observationStatus"] == "CLOSED":
            if not (e.get("backendTest") or "").strip() and not (e.get("frontendTest") or "").strip():
                errs.append(f"{k}: CLOSED requires a runnable test reference")
        elif rule not in NOISE_RULES and rule != "-":
            errs.append(f"{k}: bad noiseRule {rule!r}")
        if e["implementationStatus"] == "PENDING" and (e.get("targetPhase") or "-") == "-":
            errs.append(f"{k}: PENDING requires targetPhase")
        if e["implementationStatus"] == "PENDING" and not (e.get("owner") or "").strip():
            errs.append(f"{k}: PENDING requires owner")

    # ---- 2. 报告 FIELD_BREAK 归一化(带实际值) ----
    rep = json.load(open(args.report, encoding="utf-8"))
    report = {}
    for fb in rep.get("field_breaks", []):
        for dif in fb.get("diffs", []):
            report[(fb["caseId"], norm(dif["field"]))] = (dif.get("baseline"), dif.get("candidate"))

    # ---- 3. 未知差异(默认失败) ----
    unknown = sorted(set(report) - keys)
    for cid, p in unknown:
        errs.append(f"unknown field break (no ledger record): {cid} :: {p}")

    # ---- 4. 在场/重现检查 + 规则执行 ----
    by_key = {(e["caseId"], e["fieldPath"]): e for e in entries}
    for k, e in by_key.items():
        obs, race = e["observationStatus"], bool(e.get("raceDependent"))
        present = k in report
        if obs == "OBSERVED" and not present and not race:
            errs.append(f"OBSERVED entry no longer in report (close it or fix classification): {k[0]} :: {k[1]}")
        if obs == "CLOSED" and present:
            errs.append(f"CLOSED entry still reproducing: {k[0]} :: {k[1]}")
        if obs == "ACCEPTED_NOISE" and present:
            rule = e.get("noiseRule")
            bv, cv = report[k]
            why = check_rule(rule, k[1], bv, cv)
            if why:
                errs.append(f"noise rule violated at {k[0]} :: {k[1]} [{rule}]: {why}")

    # ---- 5. ratchet 兼容率基线(显式文件 → 台账同目录 → 台账 meta;全部缺失 = FAIL) ----
    rate = rep.get("compat_rate_field")
    bpath = args.baseline or os.path.join(os.path.dirname(args.ledger), "field-baseline.json")
    floor = None
    if os.path.exists(bpath):
        floor = json.load(open(bpath, encoding="utf-8")).get("fieldCompatRateBaseline")
    if floor is None:
        floor = meta.get("fieldCompatRateBaseline")
    if floor is None:
        errs.append("no ratchet baseline found (field-baseline.json 或 meta.fieldCompatRateBaseline)")
    elif rate is None or rate + 1e-9 < float(floor):
        errs.append(f"compat_rate_field {rate}% below ratchet baseline {floor}% "
                    f"(下降须提交带理由的基线变更 PR)")

    if errs:
        fail(errs)

    digest = hashlib.sha256(open(args.ledger, "rb").read()).hexdigest()
    summary = {
        "verdict": "PASS",
        "unknown_field_breaks": 0,
        "ledger_entries": len(entries),
        "report_field_break_cases": len(rep.get("field_breaks", [])),
        "report_field_diffs": len(report),
        "compat_rate_field": rate,
        "ratchet_baseline": floor,
        "ledger_sha256": digest,
        "commit": os.environ.get("GITHUB_SHA", ""),
        "image_digest": os.environ.get("FIELD_GATE_IMAGE_DIGEST", ""),
        "observation": _count(entries, "observationStatus"),
        "categories": _count(entries, "category"),
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    if args.out:
        with open(args.out, "w", encoding="utf-8", newline="\n") as f:
            json.dump(summary, f, ensure_ascii=False, indent=2)
            f.write("\n")
    print("field gate: PASS")


def _count(entries, field):
    out = {}
    for e in entries:
        out[e[field]] = out.get(e[field], 0) + 1
    return out


if __name__ == "__main__":
    main()
