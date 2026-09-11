#!/usr/bin/env python3
"""差分比对器：把基线（厂商原版）与候选（重建版）的回放结果做逐用例比对、
分类与量化，产出 Markdown + JSON 报告。

用法：python diff_compare.py <baseline-results.jsonl> <candidate-results.jsonl> [out.md]
"""
import collections
import json
import os
import sys

# 厂商码 -> 重建码 的语义等价映射（命名差异，语义相同）
CODE_ALIAS = {
    "RESOURCE_CODE_DUPLICATE": "DUPLICATE_CODE",
    "RESOURCE_VERSION_CONFLICT": "RESOURCE_VERSION",
    "RESOURCE_HAS_CHILDREN": "HAS_CHILDREN",
    "PARENT_RESOURCE_DISABLED": "PARENT_DISABLED",
    "LAST_ADMIN_PROTECTED": "LAST_ADMIN",
    "RACK_U_CONFLICT": "U_SLOT_CONFLICT",
    "PDU_SOCKET_CONNECTED": "SOCKET_CONNECTED",
    "APPROVAL_STATE_CONFLICT": "APPROVAL_STATE",
    "RESOURCE_NOT_FOUND": "NOT_FOUND",
    "SYSTEM_TEMPLATE_PROTECTED": "SYSTEM_TEMPLATE_PROTECTED",
    "DEVICE_TYPE_IN_USE": "DEVICE_TYPE_IN_USE",
    "DEVICE_POSITIONED": "DEVICE_POSITIONED",
    "USER_CONFLICT": "USER_CONFLICT",
    "PDU_DEVICE_RACK_MISMATCH": "PDU_DEVICE_RACK_MISMATCH",
    "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED": "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED",
    "INVALID_RESOURCE": "INVALID_RESOURCE",
    "INVALID_REQUEST": "INVALID_RESOURCE",
    "UNAUTHORIZED": "UNAUTHORIZED",
    "SUCCESS": "SUCCESS",
}

# 已登记的有意修复（docs/COMPAT-DECISIONS.md）——差异可接受
INTENTIONAL = {
    # 厂商 500 -> 重建 409（D1-D3 有意修复）
    ("500", "409"): "D1-D3：重复编码/插座等由 500 修正为 409",
    # 重建要求登录验证码（新增能力，非语义破坏）
    ("SUCCESS", "CAPTCHA_REQUIRED"): "S7：登录新增强制验证码",
}


def load(path):
    rows = [json.loads(l) for l in open(path, encoding="utf-8") if l.strip()]
    out = {}
    for r in rows:
        cid = r["caseId"]
        # 同名 caseId 保留最后一条（重复注册时后写的为准）
        out[cid] = r
    return out, rows


def classify(b, c):
    bs, bc = b.get("httpStatus"), b.get("actualCode")
    cs, cc = c.get("httpStatus"), c.get("actualCode")
    if bs == cs and bc == cc:
        return "EQUIVALENT", ""
    if bc == cc and bs != cs:
        if (str(bs), str(cs)) in INTENTIONAL:
            return "INTENTIONAL", INTENTIONAL[(str(bs), str(cs))]
        if bs == 201 and cs == 200:
            return "BREAK_STATUS", "厂商创建返回 201，重建返回 200"
        return "BREAK_STATUS", f"状态码 {bs} -> {cs}"
    if bs == cs and bc != cc:
        if CODE_ALIAS.get(bc) == cc:
            return "CODE_ALIAS", f"{bc} -> {cc}（同义命名）"
        if (str(bc), str(cc)) in INTENTIONAL:
            return "INTENTIONAL", INTENTIONAL[(str(bc), str(cc))]
        if str(bc).isdigit() or str(cc).isdigit() or "_" in str(cc):
            # 非错误码类断言（如 invariant/pagination 文本）
            return "ASSERTION_TEXT", f"{bc} -> {cc}"
        return "BREAK_CODE", f"{bc} -> {cc}"
    return "BREAK_BOTH", f"({bs},{bc}) -> ({cs},{cc})"


def main():
    bp, cp = sys.argv[1], sys.argv[2]
    outp = sys.argv[3] if len(sys.argv) > 3 else "diff-report.md"
    base, base_rows = load(bp)
    cand, cand_rows = load(cp)

    cats = collections.Counter()
    breaks = []
    aliases = []
    missing = []
    extra = []

    for cid, b in base.items():
        if b["kind"] not in ("TEST", "SETUP"):
            continue
        c = cand.get(cid)
        if c is None:
            missing.append(cid)
            cats["MISSING"] += 1
            continue
        cat, why = classify(b, c)
        cats[cat] += 1
        if cat in ("BREAK_STATUS", "BREAK_CODE", "BREAK_BOTH"):
            breaks.append((cid, why, b, c))
        elif cat == "CODE_ALIAS":
            aliases.append((cid, why))

    for cid in cand:
        if cid not in base:
            extra.append(cid)

    considered = sum(v for k, v in cats.items() if k != "MISSING")
    ok = cats["EQUIVALENT"] + cats["CODE_ALIAS"] + cats["INTENTIONAL"]
    total_expected = considered + cats["MISSING"]

    L = []
    A = L.append
    A("# 差分回放报告：厂商原版 vs 重建版\n")
    A(f"- 基线用例（TEST/SETUP）：{total_expected}")
    A(f"- 候选实际可比：{considered}；未产生（级联缺失）：{cats['MISSING']}")
    A(f"- 逐用例一致：{cats['EQUIVALENT']}")
    A(f"- 语义等价（仅命名差异）：{cats['CODE_ALIAS']}")
    A(f"- 已登记有意修复：{cats['INTENTIONAL']}")
    A(f"- 断言文本差异（非错误码）：{cats['ASSERTION_TEXT']}")
    A(f"- **真实断裂：{len(breaks)}**\n")
    if considered:
        A(f"**可比用例兼容率 = (一致 + 命名等价 + 有意修复) / 可比 = "
          f"{ok}/{considered} = {100.0*ok/considered:.1f}%**\n")

    A("## 断裂清单（按类型聚合）\n")
    by_why = collections.Counter(w for _, w, _, _ in breaks)
    A("| 次数 | 差异 | 典型用例 |")
    A("|---|---|---|")
    for why, n in by_why.most_common():
        sample = next(cid for cid, w, _, _ in breaks if w == why)
        A(f"| {n} | {why} | `{sample}` |")

    A("\n## 断裂明细\n")
    A("| 用例 | 基线 | 候选 | 说明 |")
    A("|---|---|---|---|")
    for cid, why, b, c in breaks[:60]:
        A(f"| `{cid}` | {b['httpStatus']}/{b['actualCode']} | {c['httpStatus']}/{c['actualCode']} | {why} |")

    A(f"\n## 语义等价（命名差异，{len(aliases)} 项）\n")
    pairs = collections.Counter(w for _, w in aliases)
    for w, n in pairs.most_common():
        A(f"- {w} × {n}")

    A(f"\n## 级联缺失（{len(missing)} 项）\n")
    A("这些用例在候选侧完全没有产生——通常是夹具（SETUP）失败导致场景中断：\n")
    pre = collections.Counter(cid.split("-")[0] for cid in missing)
    for k, v in pre.most_common():
        A(f"- {k}: {v}")
    A(f"\n示例：{', '.join(missing[:20])}")

    if extra:
        A(f"\n## 候选独有（{len(extra)} 项）\n")
        for cid in extra[:20]:
            A(f"- `{cid}`")

    text = "\n".join(L) + "\n"
    open(outp, "w", encoding="utf-8").write(text)
    print(text)
    json.dump({
        "categories": dict(cats),
        "considered": considered,
        "compat_rate": round(100.0 * ok / considered, 1) if considered else None,
        "breaks": [{"caseId": cid, "why": w,
                    "baseline": f"{b['httpStatus']}/{b['actualCode']}",
                    "candidate": f"{c['httpStatus']}/{c['actualCode']}"} for cid, w, b, c in breaks],
        "missing": missing,
        "alias_pairs": dict(pairs),
    }, open(outp.replace(".md", ".json"), "w"), ensure_ascii=False, indent=1)
    print(f"\nwritten: {outp}")


if __name__ == "__main__":
    main()
