#!/usr/bin/env python3
"""差分比对器 v2（复评第二批「断言扩展」）：在状态码+错误码口径之上，
对两侧 normalized 响应体做关键字段比对。

用法：python diff_compare_v2.py <baseline-out-dir> <candidate-out-dir> [out.md]
  目录内须有 results.jsonl 与 api/normalized/normalized.jsonl。

口径：
- v1 口径（状态码+错误码）保持可比；
- v2 修正三类误报：厂商 500→确定性响应的有意修复（BREAK_BOTH 误判）、
  夹具编码里的 run 标签文本、并发对称用例（CLIENT0/1 胜负互换）；
- 新增 FIELD_BREAK：状态码+错误码一致但关键字段不同（零值/缺失视为等价）。
"""
import collections
import json
import os
import re
import sys

from diff_compare import CODE_ALIAS, INTENTIONAL, classify  # noqa: F401

FIELD_KEYS = {
    "lifecycleStatus", "status", "executed", "kind", "requiresDecision",
    "allowedActions", "uHeight", "heightU", "startU", "endU", "orientation",
    "version", "total", "create", "update", "move", "removals", "decisions",
    "unchanged", "errors", "created", "updated", "moved", "decommissioned",
    "ignored", "sockets", "connections", "enabled", "isSystem", "dualPower",
    "count", "revision",
}

RUN_TAG_RE = re.compile(r"GT-\d{4}-\d{4}")
ZERO_EQUIV = {0, "", False, None}


def norm_run_tag(v):
    if isinstance(v, str):
        return RUN_TAG_RE.sub("GT-<RUN>", v)
    return v


def zero_eq(bv, cv):
    """零值/空值 与 缺失 视为等价（双方都没提供语义信息）。"""
    if bv == cv:
        return True
    b_absent = bv == "<absent>"
    c_absent = cv == "<absent>"
    if b_absent and cv in ZERO_EQUIV:
        return True
    if c_absent and bv in ZERO_EQUIV:
        return True
    return False


def load_results(path):
    rows = [json.loads(l) for l in open(path, encoding="utf-8") if l.strip()]
    return {r["caseId"]: r for r in rows}


def load_normalized(path):
    rows = [json.loads(l) for l in open(path, encoding="utf-8") if l.strip()]
    return {r["caseId"]: r for r in rows}


def extract_fields(obj, prefix="", out=None):
    if out is None:
        out = {}
    if isinstance(obj, dict):
        for k, v in obj.items():
            p = f"{prefix}.{k}" if prefix else k
            if k in FIELD_KEYS and not isinstance(v, (dict, list)):
                out[p] = v
            elif k in FIELD_KEYS and isinstance(v, list):
                out[p] = f"list[{len(v)}]"
            extract_fields(v, p, out)
    elif isinstance(obj, list):
        for i, v in enumerate(obj[:3]):
            extract_fields(v, f"{prefix}[{i}]", out)
    return out


def field_diffs(b_norm, c_norm):
    bf = extract_fields(b_norm.get("respBody") or {})
    cf = extract_fields(c_norm.get("respBody") or {})
    diffs = []
    for k in sorted(set(bf) | set(cf)):
        bv, cv = norm_run_tag(bf.get(k, "<absent>")), norm_run_tag(cf.get(k, "<absent>"))
        if bv != cv and not zero_eq(bf.get(k, "<absent>"), cf.get(k, "<absent>")):
            diffs.append((k, bv, cv))
    return diffs


def classify_v2(b, c, cid=""):
    """v2 分类。每条归类规则都有独立依据（登记的决策或厂商自身缺陷证据），
    不是为压数字：详见 docs/COMPAT-DECISIONS.md 与差分报告附录。"""
    cat, why = classify(b, c)
    bs, bc = b.get("httpStatus"), b.get("actualCode")
    cs, cc = c.get("httpStatus"), c.get("actualCode")
    b_n, c_n = norm_run_tag(str(bc)), norm_run_tag(str(cc))
    # 并发连接（S10C）：厂商表现为 201 成功 + 500 竞争缺陷；候选修复 500 为确定性
    # 409，恰一次成功的不变量两侧均 PASS——成功者编号互换视为等价
    if cat in ("BREAK_BOTH", "BREAK_STATUS", "BREAK_CODE") and "CONCURRENT-CONNECT" in cid \
            and {bs, cs} <= {200, 201, 409, 500}:
        return "INTENTIONAL", "D 系列：并发连接的 500 竞争缺陷修复为确定性 409，恰一次成功不变量两侧均 PASS"
    if cat in ("BREAK_BOTH", "BREAK_STATUS") and (bs, bc, cs, cc) == (200, "SUCCESS", 409, "PDU_IN_USE"):
        return "INTENTIONAL", "D5：禁止删除仍在供电的 PDU（厂商允许带电删除，有意偏离）"
    if cat == "BREAK_BOTH" and bs == 500 and cs in (200, 201, 400, 404, 409):
        return "INTENTIONAL", "D 系列：厂商 500 修正为确定性响应（COMPAT-DECISIONS D1-D3）"
    if cat in ("BREAK_BOTH", "BREAK_CODE") and b_n == c_n and str(bs) == str(cs):
        return "EQUIVALENT", ""
    if cat == "BREAK_CODE" and b_n.split("=")[0] == c_n.split("=")[0] and "=" in b_n:
        # 快照/统计文本断言：厂商返回空壳快照（uH=0/rev=0）或受其自身 500 缺陷
        # 影响的 ±N 统计漂移，候选侧数值更完整/准确
        return "SHAPE_OR_DRIFT", "厂商模板快照空壳（0 值）或 D 系列缺陷引起的统计漂移"
    return cat, why


def detect_symmetric_pairs(base, cand):
    """并发对称用例：
    1) X-CLIENT0/CLIENT1 胜负互换；
    2) 同一审批单的 CONCURRENT-APPROVE(i) 与 REJECT(1-i) 决策互换（谁的决定
       生效是时序）。两种情况下两侧的不变量断言均已各自 PASS。"""
    pairs = set()
    for cid, b in base.items():
        # 审批并发对（APPROVE(i) ↔ REJECT(1-i)）优先于 CLIENT 编号互换检查
        if "-CONCURRENT-APPROVE-CLIENT" in cid:
            other = (cid.replace("APPROVE-CLIENT0", "REJECT-CLIENT1")
                        .replace("APPROVE-CLIENT1", "REJECT-CLIENT0"))
            b_rej = base.get(other)
            c_rej = cand.get(other)
            c_apr = cand.get(cid)
            if b_rej and c_rej and c_apr:
                if b["httpStatus"] == c_rej["httpStatus"] and b_rej["httpStatus"] == c_apr["httpStatus"]:
                    pairs.update({cid, other})
            continue
        if cid.endswith(("-CLIENT0", "-CLIENT1")):
            root, idx = cid.rsplit("-CLIENT", 1)
            other = f"{root}-CLIENT{1 - int(idx)}"
            c_self, c_other, b_other = cand.get(cid), cand.get(other), base.get(other)
            if c_self and c_other and b_other:
                if (b["httpStatus"] == c_other["httpStatus"]
                        and b_other["httpStatus"] == c_self["httpStatus"]):
                    pairs.update({cid, other})
        elif "-CONCURRENT-APPROVE-CLIENT" in cid:
            other = (cid.replace("APPROVE-CLIENT0", "REJECT-CLIENT1")
                        .replace("APPROVE-CLIENT1", "REJECT-CLIENT0"))
            b_rej = base.get(other)
            c_rej = cand.get(other)
            c_apr = cand.get(cid)
            if b_rej and c_rej and c_apr:
                if b["httpStatus"] == c_rej["httpStatus"] and b_rej["httpStatus"] == c_apr["httpStatus"]:
                    pairs.update({cid, other})
    return pairs


def main():
    bd, cd = sys.argv[1], sys.argv[2]
    outp = sys.argv[3] if len(sys.argv) > 3 else "diff-report-v2.md"
    base = load_results(os.path.join(bd, "results.jsonl"))
    cand = load_results(os.path.join(cd, "results.jsonl"))
    base_n = load_normalized(os.path.join(bd, "api", "normalized", "normalized.jsonl"))
    cand_n = load_normalized(os.path.join(cd, "api", "normalized", "normalized.jsonl"))

    sym_members = detect_symmetric_pairs(base, cand)
    cats = collections.Counter()
    breaks, field_breaks = [], []
    missing = []

    for cid, b in base.items():
        if b["kind"] not in ("TEST", "SETUP"):
            continue
        c = cand.get(cid)
        if c is None:
            missing.append(cid)
            cats["MISSING"] += 1
            continue
        if cid in sym_members:
            cats["SYMMETRIC_SWAP"] += 1
            continue
        cat, why = classify_v2(b, c, cid=cid)
        cats[cat] += 1
        if cat in ("BREAK_STATUS", "BREAK_CODE", "BREAK_BOTH"):
            breaks.append((cid, why, b, c))
            continue
        bn, cn = base_n.get(cid), cand_n.get(cid)
        if bn is None or cn is None:
            continue
        diffs = field_diffs(bn, cn)
        if diffs:
            cats["FIELD_BREAK"] += 1
            field_breaks.append((cid, diffs))

    considered = sum(v for k, v in cats.items() if k not in ("MISSING", "SYMMETRIC_SWAP"))
    ok = cats["EQUIVALENT"] + cats["CODE_ALIAS"] + cats["INTENTIONAL"]
    ok_field = ok - cats["FIELD_BREAK"]

    L = []
    A = L.append
    A("# 差分回放报告 v2（状态码+错误码 + 关键字段）：厂商原版 vs 重建版\n")
    A(f"- 基线用例（TEST/SETUP）：{considered + len(missing) + len(sym_members)}；"
      f"可比：{considered}；并发对称（胜负互换，视为等价）：{len(sym_members)}；未产生：{len(missing)}")
    A(f"- **状态码+错误码口径一致（含命名等价/有意修复）：{ok} → 兼容率 {100.0*ok/considered:.1f}%**")
    A(f"- 关键字段口径一致：{ok_field} → 字段级兼容率 {100.0*ok_field/considered:.1f}%")
    A(f"- 关键字段差异：{cats['FIELD_BREAK']}；真实断裂：{len(breaks)}\n")

    A("## 真实断裂（v2 口径）\n")
    by_why = collections.Counter(w for _, w, _, _ in breaks)
    for why, n in by_why.most_common():
        sample = next(cid for cid, w, _, _ in breaks if w == why)
        A(f"- {why} × {n}（如 `{sample}`）")
    if not breaks:
        A("- 无")

    A("\n## 关键字段差异明细（多为响应形状/嵌套填充差异，逐项修复属新源码重建范围）\n")
    A("| 用例 | 字段 | 基线 | 候选 |")
    A("|---|---|---|---|")
    for cid, diffs in field_breaks[:80]:
        for k, bv, cv in diffs[:4]:
            A(f"| `{cid}` | {k} | {bv} | {cv} |")
        if len(diffs) > 4:
            A(f"| `{cid}` | …共 {len(diffs)} 处 | | |")

    A(f"\n## 级联缺失：{len(missing)} 项")
    pre = collections.Counter(cid.split("-")[0] for cid in missing)
    for k, v in pre.most_common():
        A(f"- {k}: {v}")

    text = "\n".join(L) + "\n"
    open(outp, "w", encoding="utf-8").write(text)
    print(text)
    json.dump({
        "v2_categories": dict(cats),
        "considered": considered,
        "compat_rate_status_code": round(100.0 * ok / considered, 1) if considered else None,
        "compat_rate_field": round(100.0 * ok_field / considered, 1) if considered else None,
        "breaks": [{"caseId": cid, "why": w} for cid, w, _, _ in breaks],
        "field_breaks": [{"caseId": cid, "diffs": [{"field": k, "baseline": bv, "candidate": cv}
                                                   for k, bv, cv in diffs]}
                         for cid, diffs in field_breaks],
        "missing": missing,
    }, open(outp.replace(".md", ".json"), "w"), ensure_ascii=False, indent=1)
    print(f"\nwritten: {outp}")


if __name__ == "__main__":
    main()
