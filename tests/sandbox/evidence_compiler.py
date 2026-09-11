#!/usr/bin/env python3
"""Evidence Compiler v9: 从 results + samples + openapi + requirements 编译证据组和覆盖矩阵。
核心原则：只接受可验证的显式证据，拒绝弱证据。
用法：作为模块导入或独立运行。
"""
import csv
import json
import os
import re
import sys
from collections import defaultdict
from pathlib import Path

try:
    import yaml
except ImportError:
    yaml = None

UP = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
WEAK_PATTERNS = re.compile(r"tested_in_|confirmed_in_|_in_S\d+|evidence_only", re.I)
ALLOWED_CATEGORIES = {"happy_path", "auth_boundary", "validation", "conflict",
                      "stale_version", "invariant", "pagination"}


def compile_evidence(results, samples, openapi_ops, requirements, auth_matrix=None,
                     state_machine=None, run_id=""):
    """编译证据组和覆盖矩阵。返回 (evidence_groups, coverage_rows, errors, stats)。

    results: list of dict（results.jsonl 行）
    samples: list of dict（samples.jsonl 行）
    openapi_ops: list of (method, path)
    requirements: dict {(method, path): set(categories)}
    auth_matrix: dict（可选）
    state_machine: dict（可选）
    run_id: str（用于跨 RUN_ID 检测）
    """
    errors = []
    results_by_id = {c["caseId"]: c for c in results}
    samples_by_id = {s["caseId"]: s for s in samples}
    ops_set = set(openapi_ops)

    # sample → operation 映射
    sample_ops = {}
    for s in samples:
        op = _match_op(s.get("method", ""), s.get("path", ""), openapi_ops)
        if op:
            sample_ops[s["caseId"]] = op

    # 收集 evidence groups（来自显式声明的结果）
    evidence_groups = []
    for c in results:
        if not c.get("coverageContribution"):
            continue
        eg = _compile_single(c, results_by_id, samples_by_id, sample_ops, ops_set,
                             requirements, run_id, errors)
        if eg:
            evidence_groups.append(eg)

    # 覆盖矩阵
    coverage_rows = []
    reached = 0
    behavior_complete = 0
    for method, path in openapi_ops:
        key = (method, path)
        req_cats = requirements.get(key, set())
        # 收集该操作的所有合格证据
        passed_cats = set()
        eg_for_op = [eg for eg in evidence_groups if eg["operation"] == key]
        reached_samples = any(sample_ops.get(cid) == key for cid in samples_by_id)
        for eg in eg_for_op:
            passed_cats.add(eg["category"])
        missing = req_cats - passed_cats
        is_reached = reached_samples or bool(eg_for_op)
        is_complete = is_reached and not missing
        if is_reached:
            reached += 1
        if is_complete:
            behavior_complete += 1
        coverage_rows.append({
            "method": method, "path": path,
            "operation_reached": is_reached,
            "evidence_groups": len(eg_for_op),
            "explicit_passed_categories": ";".join(sorted(passed_cats)),
            "required_categories": ";".join(sorted(req_cats)),
            "missing_categories": ";".join(sorted(missing)),
            "behavior_complete": is_complete,
        })

    stats = {
        "total_results": len(results),
        "total_samples": len(samples),
        "evidence_groups": len(evidence_groups),
        "operations_reached": reached,
        "operations_total": len(openapi_ops),
        "behavior_complete": behavior_complete,
        "errors": len(errors),
    }
    return evidence_groups, coverage_rows, errors, stats


def _match_op(method, path, ops):
    if not method or not path:
        return None
    p = path.split("?")[0]
    for m, tmpl in ops:
        if m != method:
            continue
        ts = tmpl.strip("/").split("/")
        ps = p.strip("/").split("/")
        if len(ts) != len(ps):
            continue
        if all((t == s) if not (t.startswith("{") and t.endswith("}")) else UP.match(s)
               for t, s in zip(ts, ps)):
            return (m, tmpl)
    return None


def _compile_single(case, results_by_id, samples_by_id, sample_ops, ops_set,
                    requirements, run_id, errors):
    """将一条 coverageContribution=true 的结果编译为证据组。"""
    cid = case.get("caseId", "")
    op_str = case.get("operation", "")
    category = case.get("category", "")
    status = case.get("status", "")
    kind = case.get("kind", "")

    def err(code, detail):
        errors.append({"caseId": cid, "code": code, "detail": str(detail)[:200]})

    # 禁止规则检查
    # 弱证据：httpStatus=0 且无 primaryCaseId
    if case.get("httpStatus", 0) == 0 and not case.get("primaryCaseId"):
        err("EC-01", "httpStatus=0 and no primaryCaseId")
        return None

    # BLOCKED/INFO 不能贡献覆盖
    if status in ("BLOCKED", "INFO", "SETUP"):
        err("EC-09", f"status={status} cannot contribute coverage")
        return None

    # 操作验证
    if not op_str:
        err("EC-02", "missing explicit operation")
        return None
    parts = op_str.split(" ", 1)
    if len(parts) != 2:
        err("EC-02", f"invalid operation format: {op_str}")
        return None
    op_method, op_path = parts[0].upper(), parts[1]
    op_key = (op_method, op_path)
    if op_key not in ops_set:
        err("EC-02", f"operation not in OpenAPI: {op_key}")
        return None

    # 类别验证
    if not category:
        err("EC-08", "missing category")
        return None
    if category not in ALLOWED_CATEGORIES:
        err("EC-08", f"invalid category: {category}")
        return None
    req_cats = requirements.get(op_key, set())
    if category not in req_cats:
        err("EC-08", f"category={category} not in required={req_cats}")
        return None

    # 主请求验证
    primary = case.get("primaryCaseId", cid)
    if primary not in results_by_id:
        err("EC-03", f"primaryCaseId={primary} not in results")
        return None
    if primary not in samples_by_id:
        err("EC-03", f"primaryCaseId={primary} not in samples")
        return None

    # 主请求操作一致性
    primary_op = sample_ops.get(primary)
    if primary_op and primary_op != op_key:
        err("EC-02", f"primary sample maps to {primary_op}, declared {op_key}")
        return None

    # supporting caseIds
    supporting = case.get("supportingCaseIds", [])
    for sc in supporting:
        if sc not in results_by_id and sc not in samples_by_id:
            err("EC-04", f"supportingCaseId={sc} not found")

    # assertions 验证
    assertions = case.get("assertions", [])
    if not assertions:
        # 弱证据：无 assertions
        err("EC-04", "assertions empty")
        return None
    for a in assertions:
        if not isinstance(a, dict):
            err("EC-05", f"assertion not dict: {a}")
            return None
        if "actual" not in a or "expected" not in a:
            # 弱证据：文字替代
            actual_code = str(case.get("actualCode", ""))
            if WEAK_PATTERNS.search(actual_code):
                err("EC-07", f"weak evidence: actualCode={actual_code}")
                return None
            err("EC-05", f"assertion missing actual/expected: {a.get('name','')}")
            return None
        # passed 可复算性
        if a.get("passed") and str(a["actual"]) != str(a["expected"]):
            err("EC-06", f"passed=true but actual={a['actual']} != expected={a['expected']}")
            return None

    # 主请求失败却贡献 happy_path/invariant
    primary_result = results_by_id.get(primary, {})
    if primary_result.get("httpStatus", 0) >= 400 and category in ("happy_path", "invariant"):
        err("EC-09", f"primary request failed ({primary_result.get('httpStatus')}) but category={category}")
        return None

    return {
        "evidenceGroupId": case.get("evidenceGroupId", f"EG-{cid}"),
        "operation": op_key,
        "category": category,
        "primaryCaseId": primary,
        "supportingCaseIds": supporting,
        "assertions": assertions,
        "status": "VALID",
    }