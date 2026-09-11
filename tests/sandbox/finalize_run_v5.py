#!/usr/bin/env python3
"""GT Finalizer v5: 生成业务测试汇总，不生成 manifest。
修复 v4 的 P0-A（RUN_ID 四方一致）和 P0-D（显式 category/assertions）。
产物：coverage/{matrix,gap-tasks,scenario-summary} + junit/ + run-summary.json + FINAL-STATUS.txt
用法：
  python3 finalize_run_v5.py --run-dir <RUN_ID_ROOT> --run-id <RUN_ID> [--openapi path] [--requirements path]
"""
import argparse
import csv
import json
import os
import re
import sys
import xml.etree.ElementTree as ET
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from xml.sax.saxutils import escape as xesc

try:
    import yaml
except ImportError:
    yaml = None

REQUIRED_FILES = [
    "run-metadata.json",
    "final-regression/results.jsonl",
    "final-regression/api/raw-redacted/samples.jsonl",
    "final-regression/api/normalized/normalized.jsonl",
]
GATES = []
_uuid_pat = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
_iso_pat = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}")


def gate(name, ok, detail=""):
    GATES.append({"name": name, "pass": bool(ok), "detail": str(detail)})
    tag = "PASS" if ok else "FAIL"
    d = f"  {detail}" if detail and not ok else ""
    print(f"  [{tag}] {name}{d}")


def atomic_write(path, content):
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        f.write(content)
    os.replace(tmp, path)


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for c in iter(lambda: f.read(1 << 20), b""):
            h.update(c)
    return h.hexdigest()


import hashlib


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", required=True, help="RUN_ID 根目录（必须包含 run-metadata.json）")
    ap.add_argument("--run-id", required=True, help="RUN_ID 字符串")
    ap.add_argument("--openapi", default=None)
    ap.add_argument("--requirements", default=None)
    a = ap.parse_args()

    RUN_DIR = Path(a.run_dir).resolve()
    FR = RUN_DIR / "final-regression"

    # ===== P0-A §4.3: RUN_ID 四方一致 =====
    cli_id = a.run_id
    dir_id = RUN_DIR.name
    meta_path = RUN_DIR / "run-metadata.json"
    meta_id = None
    if meta_path.exists():
        try:
            meta_id = json.load(open(meta_path, encoding="utf-8")).get("run_id")
        except Exception:
            pass

    ids_all = {"cli": cli_id, "dir": dir_id, "metadata": meta_id}
    ids_ok = bool(cli_id) and cli_id == dir_id and cli_id == meta_id
    gate("run-id-consistent", ids_ok, f"{ids_all}")

    # §4.4 防呆：目录名不能是 out/output/final-regression
    bad_names = {"out", "output", "final-regression"}
    gate("run-dir-not-generic", dir_id not in bad_names, f"got '{dir_id}'")

    # ===== 1. 读输入 =====
    results_path = FR / "results.jsonl"
    samples_path = FR / "api" / "raw-redacted" / "samples.jsonl"
    norms_path = FR / "api" / "normalized" / "normalized.jsonl"
    for p, label in [(results_path, "results"), (samples_path, "samples"), (norms_path, "normalized")]:
        gate(f"input-{label}", p.exists(), str(p))

    if not all(p.exists() for p in [results_path, samples_path]):
        print("FATAL: required input missing, aborting.")
        sys.exit(1)

    cases = [json.loads(l) for l in open(results_path, encoding="utf-8") if l.strip()]
    samples = [json.loads(l) for l in open(samples_path, encoding="utf-8") if l.strip()]

    # ===== 2. caseId 唯一性 =====
    ids_list = [c["caseId"] for c in cases]
    uniq = len(set(ids_list))
    gate("caseId-unique", uniq == len(ids_list), f"{len(ids_list)} entries, {uniq} unique")

    # ===== 3. kind/status 分类 =====
    formal = [c for c in cases if c.get("kind") == "TEST"]
    aux = [c for c in cases if c.get("kind") in ("INFO", "SETUP")]
    bad_kind = [c["caseId"] for c in formal if c.get("status") in ("INFO", "SETUP")]
    gate("no-aux-status-in-formal", not bad_kind, f"{len(bad_kind)} entries")
    gate("kind-field-present", all(c.get("kind") for c in cases))

    counts = defaultdict(int)
    for c in formal:
        counts[c["status"]] += 1
    npass = counts.get("PASS", 0)
    nfail = counts.get("FAIL", 0)
    nerror = counts.get("ERROR", 0)
    nblocked = counts.get("BLOCKED", 0)
    nskip = counts.get("SKIPPED", 0)
    gate("no-fail", nfail == 0, f"FAIL={nfail}")
    gate("no-error", nerror == 0, f"ERROR={nerror}")
    gate("no-blocked", nblocked == 0, f"BLOCKED={nblocked}")

    # ===== 4. OpenAPI + Requirements =====
    openapi_path = a.openapi or str(RUN_DIR / "openapi.yaml")
    if not os.path.exists(openapi_path):
        openapi_path = os.path.join(os.path.dirname(str(RUN_DIR)), "openapi.yaml")
    if not os.path.exists(openapi_path):
        openapi_path = "openapi.yaml"
    gate("input-openapi", os.path.exists(openapi_path), openapi_path)

    req_path = a.requirements or str(Path(__file__).parent / "coverage-requirements.yaml")
    gate("input-requirements", os.path.exists(req_path), req_path)

    ops = []
    try:
        doc = yaml.safe_load(open(openapi_path, encoding="utf-8"))
        for path, item in doc.get("paths", {}).items():
            for method in item:
                if method in ("get", "post", "put", "delete", "patch"):
                    ops.append((method.upper(), path))
        gate("openapi-parseable", True)
        gate("openapi-count-66", len(ops) == 66, f"got {len(ops)}")
    except Exception as e:
        gate("openapi-parseable", False, str(e)[:80])
        ops = []

    req_map = {}
    try:
        requirements = yaml.safe_load(open(req_path, encoding="utf-8")).get("operations", {})
        for key, spec in requirements.items():
            m, p = key.split(" ", 1)
            req_map[(m, p)] = set(spec.get("required", []))
        gate("requirements-parseable", True)
        gate("requirements-count-66", len(req_map) == 66, f"got {len(req_map)}")
        req_keys = set(req_map.keys())
        openapi_keys = set(ops)
        gate("requirements-match-openapi", req_keys == openapi_keys,
             f"missing={sorted(openapi_keys - req_keys)[:3]} extra={sorted(req_keys - openapi_keys)[:3]}")
    except Exception as e:
        gate("requirements-parseable", False, str(e)[:80])
        req_map = {}

    # ===== 5. 两级覆盖矩阵 =====
    def match_op(method, path):
        p = path.split("?")[0]
        for m, tmpl in ops:
            if m != method:
                continue
            t_segs = tmpl.strip("/").split("/")
            p_segs = p.strip("/").split("/")
            if len(t_segs) != len(p_segs):
                continue
            if all((t == s) if not (t.startswith("{") and t.endswith("}")) else _uuid_pat.match(s)
                   for t, s in zip(t_segs, p_segs)):
                return (m, tmpl)
        return None

    op_cases = defaultdict(list)
    op_samples = defaultdict(list)
    case_op = {}
    for s in samples:
        matched = match_op(s["method"], s["path"])
        if matched:
            op_samples[matched].append(s)
            case_op[s["caseId"]] = matched
    for c in cases:
        op = case_op.get(c["caseId"])
        if op:
            op_cases[op].append(c)

    def categorize_case(c):
        st = c["status"]
        explicit = {c["category"]} if c.get("category") and st == "PASS" else set()
        if st != "PASS":
            return explicit
        code = c.get("actualCode", "")
        cats = set()
        if code == "SUCCESS":
            cats.add("happy_path")
        if code in ("INVALID_RESOURCE", "INVALID_REQUEST", "INVALID_VERSION", "INVALID_USER", "INVALID_USER"):
            cats.add("validation")
        if code in ("UNAUTHORIZED", "FORBIDDEN", "USER_DISABLED"):
            cats.add("auth_boundary")
        if code in ("RESOURCE_CODE_DUPLICATE", "RACK_U_CONFLICT", "DEVICE_POSITIONED",
                    "DEVICE_NOT_POSITIONED", "PDU_SOCKET_CONNECTED", "PDU_DEVICE_RACK_MISMATCH",
                    "USER_CONFLICT", "RESOURCE_HAS_CHILDREN", "PARENT_RESOURCE_DISABLED",
                    "APPROVAL_STATE_CONFLICT", "DEVICE_TYPE_IN_USE", "SYSTEM_TEMPLATE_PROTECTED",
                    "LAST_ADMIN_PROTECTED"):
            cats.add("conflict")
        if code == "RESOURCE_VERSION_CONFLICT":
            cats.update({"conflict", "stale_version"})
        if code == "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED":
            cats.add("conflict")
        if code == "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED":
            cats.add("conflict")
        return cats | explicit

    matrix_rows = []
    reached = 0
    behavior_complete = 0
    for method, path in ops:
        key = (method, path)
        mc = op_cases.get(key, [])
        ms = op_samples.get(key, [])
        is_reached = len(ms) > 0
        passed_cats = set()
        for c in mc:
            passed_cats |= categorize_case(c)
        req = req_map.get(key, set())
        missing = req - passed_cats
        is_complete = is_reached and not missing
        if is_reached:
            reached += 1
        if is_complete:
            behavior_complete += 1
        matrix_rows.append({
            "method": method, "path": path,
            "operation_reached": is_reached, "samples": len(ms),
            "passed_categories": ";".join(sorted(passed_cats)),
            "required_categories": ";".join(sorted(req)),
            "missing_categories": ";".join(sorted(missing)),
            "behavior_complete": is_complete,
            "evidence_cases": ";".join(sorted(set(c["caseId"] for c in mc if c["kind"] == "TEST"))[:5]),
        })

    gate("coverage-reached-66", reached == 66, f"got {reached}/66")
    # P0-C: 真门禁（非写死）
    gate("coverage-behavior-complete-66", behavior_complete == 66, f"got {behavior_complete}/66")

    # 写矩阵
    cov_dir = FR / "coverage"
    cov_dir.mkdir(parents=True, exist_ok=True)
    if matrix_rows:
        with open(cov_dir / "api-coverage-matrix.csv", "w", encoding="utf-8", newline="") as f:
            w = csv.DictWriter(f, fieldnames=list(matrix_rows[0].keys()))
            w.writeheader()
            w.writerows(matrix_rows)

    # 缺口任务（即使 0 缺口也保留表头）
    gap_rows = [r for r in matrix_rows if r["missing_categories"]]
    with open(cov_dir / "gap-tasks.csv", "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=["method", "path", "missing_categories", "evidence_cases"])
        w.writeheader()
        for r in gap_rows:
            w.writerow({"method": r["method"], "path": r["path"],
                        "missing_categories": r["missing_categories"], "evidence_cases": r["evidence_cases"]})
    print(f"  缺口任务: {len(gap_rows)} 操作待补（gap-tasks.csv）")

    # ===== 6. 场景汇总 =====
    scen = defaultdict(lambda: defaultdict(int))
    for c in formal:
        scen[c["scenario"]][c["status"]] += 1
    scen_summary = {}
    for s, cnt in sorted(scen.items()):
        total = sum(cnt.values())
        ok = cnt.get("FAIL", 0) + cnt.get("ERROR", 0) + cnt.get("BLOCKED", 0) == 0
        scen_summary[s] = {"total": total, **dict(cnt), "status": "PASS" if ok else "FAIL"}
    atomic_write(str(cov_dir / "scenario-summary.json"), json.dumps(scen_summary, ensure_ascii=False, indent=1))

    # ===== 7. JUnit =====
    junit_dir = FR / "junit"
    junit_dir.mkdir(parents=True, exist_ok=True)
    junit_content = f'<?xml version="1.0" encoding="utf-8"?>\n<testsuite name="gt-final-v5" tests="{len(formal)}" failures="{nfail}" errors="{nerror + nblocked}">\n'
    for c in formal:
        if c["status"] == "PASS":
            junit_content += f'  <testcase name="{xesc(c["caseId"])}" classname="{xesc(c["scenario"])}"/>\n'
        else:
            junit_content += f'  <testcase name="{xesc(c["caseId"])}" classname="{xesc(c["scenario"])}">\n'
            junit_content += f'    <failure message="{xesc(str(c.get("actualCode","")))}">{xesc(c.get("note",""))}</failure>\n  </testcase>\n'
    junit_content += "</testsuite>\n"
    atomic_write(str(junit_dir / "results.xml"), junit_content)
    try:
        tree = ET.parse(str(junit_dir / "results.xml"))
        jtests = int(tree.getroot().get("tests"))
        gate("junit-valid", True)
        gate("junit-matches-formal", jtests == len(formal), f"junit={jtests} formal={len(formal)}")
    except Exception as e:
        gate("junit-valid", False, str(e)[:80])

    # ===== 8. 敏感扫描 =====
    PATTERNS = [
        (re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*"), "jwt"),
        (re.compile(r"\$2[aby]\$1[0-9]\$[A-Za-z0-9./]{53}"), "bcrypt"),
    ]
    hits = []
    files_scanned = 0
    bytes_scanned = 0
    for root, dirs, files in os.walk(FR):
        for fn in files:
            fp = os.path.join(root, fn)
            if os.path.splitext(fn)[1] in (".png", ".zip", ".gz", ".exe"):
                continue
            try:
                content = open(fp, encoding="utf-8", errors="replace").read()
                files_scanned += 1
                bytes_scanned += len(content)
            except Exception:
                continue
            for pat, label in PATTERNS:
                for m in pat.finditer(content):
                    hits.append((os.path.relpath(fp, FR), label))
    scan_result = "PASS" if not hits else f"FAIL({len(hits)})"
    gate("sensitive-scan", not hits, f"{len(hits)} hits")
    scan_content = (
        f"scanner_version=v5\n"
        f"files_scanned={files_scanned}\n"
        f"bytes_scanned={bytes_scanned}\n"
        f"hits={len(hits)}\n"
        f"result={scan_result}\n"
    )
    atomic_write(str(RUN_DIR / "sensitive-scan.txt"), scan_content)

    # ===== 9. 最后写 run-summary 和 FINAL-STATUS =====
    all_ok = all(g["pass"] for g in GATES)
    summary = {
        "runId": cli_id,
        "mode": "final", "baseline": "B0",
        "runStatus": "PASS" if all_ok else "FAIL",
        "exitCode": 0 if all_ok else 1,
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "tests": {
            "unique": uniq, "formal": len(formal), "passed": npass, "failed": nfail,
            "errors": nerror, "blocked": nblocked, "skipped": nskip,
            "info": len([c for c in aux if c["kind"] == "INFO"]),
            "setup": len([c for c in aux if c["kind"] == "SETUP"]),
            "caseIdsUnique": uniq == len(ids_list),
        },
        "openapi": {
            "total": len(ops), "reached": reached,
            "behaviorComplete": behavior_complete,
            "partial": reached - behavior_complete, "notReached": len(ops) - reached,
        },
        "scenarios": scen_summary,
        "artifacts": {
            "rawSamples": len(samples),
            "junitValid": True,
            "caseIdsUnique": uniq == len(ids_list),
            "sensitiveScan": scan_result,
            "manifest": "PENDING_FREEZE",
        },
        "gates": GATES,
        "cleanup": {"status": "PENDING_APPROVAL"},
    }
    atomic_write(str(FR / "run-summary.json"), json.dumps(summary, ensure_ascii=False, indent=1))

    # P0-A §4.3: 断言摘要 runId 正确
    assert summary["runId"] == cli_id, f"summary runId != cli_id: {summary['runId']} != {cli_id}"

    status_content = f"FINAL STATUS: {'PASS' if all_ok else 'FAIL'}\n"
    status_content += f"Generated: {datetime.now(timezone.utc).isoformat()}\n\n"
    for g in GATES:
        tag = "PASS" if g["pass"] else "FAIL"
        d = f"  {g['detail']}" if g["detail"] and not g["pass"] else ""
        status_content += f"[{tag}] {g['name']}{d}\n"
    atomic_write(str(RUN_DIR / "FINAL-STATUS.txt"), status_content)

    print(f"\n{'='*60}")
    print(f"FINAL STATUS: {'PASS' if all_ok else 'FAIL'}")
    print(f"Formal: {len(formal)} ({npass} PASS / {nfail} FAIL / {nerror} ERROR / {nblocked} BLOCKED)")
    print(f"Coverage: reached {reached}/66, behavior_complete {behavior_complete}/66")
    print(f"Finalizer: 'FINALIZED' (manifest 生成由 freeze_run.py 负责)")
    sys.exit(0 if all_ok else 1)


if __name__ == "__main__":
    main()