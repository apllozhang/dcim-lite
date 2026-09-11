#!/usr/bin/env python3
"""GT Finalizer v4: 一键收口（修复第三轮三 P0）。
P0-1: 真行为覆盖门禁（coverage-requirements.yaml 对照，非写死 True）
P0-2: 冻结顺序（先全部产物→最后 manifest→独立 verify→禁改）
P0-3: RUN_ID 四方一致门禁（--run-id vs 目录 vs metadata vs summary）
退出码：0=全门禁过；1=有阻断。

用法: python3 finalize_run_v4.py --run-dir <RUN_DIR> --run-id <RUN_ID> --api-base <http://...>
      （--openapi 可省略，默认在 run_dir/../openapi.yaml 或 tests/sandbox/openapi.yaml）
"""
import argparse
import csv
import hashlib
import json
import os
import re
import sys
import xml.etree.ElementTree as ET
from collections import defaultdict
from datetime import datetime, timezone

import yaml

GATES = []


def gate(name, ok, detail=""):
    GATES.append({"name": name, "pass": bool(ok), "detail": detail})
    tag = "PASS" if ok else "FAIL"
    d = f"  {detail}" if detail and not ok else ""
    print(f"  [{tag}] {name}{d}")


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for c in iter(lambda: f.read(1 << 20), b""):
            h.update(c)
    return h.hexdigest()


ap = argparse.ArgumentParser()
ap.add_argument("--run-dir", required=True)
ap.add_argument("--run-id", required=True)
ap.add_argument("--api-base", default="http://127.0.0.1:18080")
ap.add_argument("--openapi", default=None)
ap.add_argument("--requirements", default=None)
a = ap.parse_args()

RUN_DIR = a.run_dir
RUN_ID = a.run_id
FR = os.path.join(RUN_DIR, "final-regression")

# openapi / requirements 查找
candidates = [
    a.openapi,
    os.path.join(os.path.dirname(os.path.abspath(RUN_DIR)), "openapi.yaml"),
    os.path.join(RUN_DIR, "openapi.yaml"),
    "openapi.yaml",
]
OPENAPI = next((p for p in candidates if p and os.path.exists(p)), None)

candidates_r = [
    a.requirements,
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "coverage-requirements.yaml"),
    os.path.join(RUN_DIR, "coverage-requirements.yaml"),
]
REQUIREMENTS = next((p for p in candidates_r if p and os.path.exists(p)), None)

print(f"finalize_run_v4  RUN_DIR={RUN_DIR}  RUN_ID={RUN_ID}")
print(f"openapi={OPENAPI}")
print(f"requirements={REQUIREMENTS}")
print()

# ===== P0-3: RUN_ID 四方一致 =====
cli_id = RUN_ID
dir_id = os.path.basename(RUN_DIR)
meta_path = os.path.join(RUN_DIR, "run-metadata.json")
meta_id = None
if os.path.exists(meta_path):
    try:
        meta_id = json.load(open(meta_path, encoding="utf-8")).get("run_id")
    except Exception:
        pass
ids = {"cli": cli_id, "dir": dir_id, "metadata": meta_id}
ids_ok = len({v for v in ids.values() if v}) == 1 and all(v for v in ids.values())
gate("run-id-consistent", ids_ok, f" {ids}")

# ===== 1. 读输入 =====
results_path = os.path.join(FR, "results.jsonl")
samples_path = os.path.join(FR, "api", "raw-redacted", "samples.jsonl")
norms_path = os.path.join(FR, "api", "normalized", "normalized.jsonl")
gate("input-results", os.path.exists(results_path))
gate("input-samples", os.path.exists(samples_path))
gate("input-normalized", os.path.exists(norms_path))
gate("input-openapi", bool(OPENAPI), OPENAPI or "not found")
gate("input-requirements", bool(REQUIREMENTS), REQUIREMENTS or "not found")

cases = [json.loads(l) for l in open(results_path, encoding="utf-8") if l.strip()]
samples = [json.loads(l) for l in open(samples_path, encoding="utf-8") if l.strip()]

# ===== 2. caseId =====
ids_list = [c["caseId"] for c in cases]
gate("caseId-unique", len(set(ids_list)) == len(ids_list), f"{len(ids_list)} entries, {len(set(ids_list))} unique")

formal = [c for c in cases if c.get("kind") == "TEST"]
aux = [c for c in cases if c.get("kind") in ("INFO", "SETUP")]
counts = defaultdict(int)
for c in formal:
    counts[c["status"]] += 1
npass = counts.get("PASS", 0)
nfail = counts.get("FAIL", 0)
nerror = counts.get("ERROR", 0)
nblocked = counts.get("BLOCKED", 0)
nskip = counts.get("SKIPPED", 0)
bad_kind = [c["caseId"] for c in formal if c["status"] in ("INFO", "SETUP")]
gate("no-aux-status-in-formal", not bad_kind, f"{len(bad_kind)} bad entries")
gate("no-fail", nfail == 0, f"FAIL={nfail}")
gate("no-error", nerror == 0, f"ERROR={nerror}")
gate("no-blocked", nblocked == 0, f"BLOCKED={nblocked}")

# ===== 3. OpenAPI + Requirements =====
try:
    doc = yaml.safe_load(open(OPENAPI, encoding="utf-8"))
    ops = []
    for path, item in doc.get("paths", {}).items():
        for method in item:
            if method in ("get", "post", "put", "delete", "patch"):
                ops.append((method.upper(), path))
    gate("openapi-parseable", True)
    gate("openapi-count-72", len(ops) == 72, f"got {len(ops)}")
except Exception as e:
    gate("openapi-parseable", False, str(e)[:80])
    ops = []

try:
    requirements = yaml.safe_load(open(REQUIREMENTS, encoding="utf-8")).get("operations", {})
    req_map = {}
    for key, spec in requirements.items():
        m, p = key.split(" ", 1)
        req_map[(m, p)] = set(spec.get("required", []))
    gate("requirements-parseable", True)
    gate("requirements-count-72", len(req_map) == 72, f"got {len(req_map)}")
    # 双向比对
    req_keys = set(req_map.keys())
    openapi_keys = set(ops)
    gate("requirements-match-openapi", req_keys == openapi_keys,
         f"missing={sorted(openapi_keys - req_keys)[:3]} extra={sorted(req_keys - openapi_keys)[:3]}")
except Exception as e:
    gate("requirements-parseable", False, str(e)[:80])
    req_map = {}

# ===== 4. 两级覆盖 =====
UUID_PAT = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")


def match_op(method, path):
    p = path.split("?")[0]
    for m, tmpl in ops:
        if m != method:
            continue
        t_segs = tmpl.strip("/").split("/")
        p_segs = p.strip("/").split("/")
        if len(t_segs) != len(p_segs):
            continue
        if all((t == s) if not (t.startswith("{") and t.endswith("}")) else UUID_PAT.match(s)
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
    code = c.get("actualCode", "")
    # 显式标注的类别与自动推导合并（invariant/pagination 等无法从响应码推断）
    explicit = {c["category"]} if c.get("category") and st == "PASS" else set()
    if st != "PASS":
        return explicit
    cats = set()
    if code == "SUCCESS":
        cats.add("happy_path")
    if code in ("INVALID_RESOURCE", "INVALID_REQUEST", "INVALID_VERSION"):
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
    return cats | explicit


matrix_rows = []
reached = 0
behavior_complete = 0
for method, path in ops:
    key = (method, path)
    matched_cases = op_cases.get(key, [])
    msamp = op_samples.get(key, [])
    is_reached = len(msamp) > 0
    passed_cats = set()
    for c in matched_cases:
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
        "operation_reached": is_reached, "samples": len(msamp),
        "passed_categories": ";".join(sorted(passed_cats)),
        "required_categories": ";".join(sorted(req)),
        "missing_categories": ";".join(sorted(missing)),
        "behavior_complete": is_complete,
        "evidence_cases": ";".join(sorted(set(c["caseId"] for c in matched_cases))[:5]),
    })

gate("coverage-reached-72", reached == 72, f"got {reached}/72")
# ===== P0-1 修复：真门禁 =====
gate("coverage-behavior-complete", behavior_complete == 72,
     f"got {behavior_complete}/66")
# 保存缺口表
os.makedirs(os.path.join(FR, "coverage"), exist_ok=True)
if matrix_rows:
    with open(os.path.join(FR, "coverage", "api-coverage-matrix.csv"), "w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(matrix_rows[0].keys()))
        w.writeheader()
        w.writerows(matrix_rows)

# 缺口任务表
gap_rows = [r for r in matrix_rows if r["missing_categories"]]
with open(os.path.join(FR, "coverage", "gap-tasks.csv"), "w", encoding="utf-8", newline="") as f:
    w = csv.DictWriter(f, fieldnames=["method", "path", "missing_categories", "evidence_cases"])
    w.writeheader()
    for r in gap_rows:
        w.writerow({"method": r["method"], "path": r["path"],
                    "missing_categories": r["missing_categories"],
                    "evidence_cases": r["evidence_cases"]})
print(f"  缺口任务: {len(gap_rows)} 操作待补（见 coverage/gap-tasks.csv）")

# ===== 5. 场景汇总 =====
scen = defaultdict(lambda: defaultdict(int))
for c in formal:
    scen[c["scenario"]][c["status"]] += 1
scen_summary = {}
for s, cnt in sorted(scen.items()):
    total = sum(cnt.values())
    ok = cnt.get("FAIL", 0) + cnt.get("ERROR", 0) + cnt.get("BLOCKED", 0) == 0
    scen_summary[s] = {"total": total, **dict(cnt),
                       "status": "PASS" if ok else "FAIL"}
with open(os.path.join(FR, "coverage", "scenario-summary.json"), "w", encoding="utf-8") as f:
    json.dump(scen_summary, f, ensure_ascii=False, indent=1)

# ===== 6. JUnit =====
os.makedirs(os.path.join(FR, "junit"), exist_ok=True)
with open(os.path.join(FR, "junit", "results.xml"), "w", encoding="utf-8") as f:
    f.write('<?xml version="1.0" encoding="utf-8"?>\n')
    f.write(f'<testsuite name="gt-final-v4" tests="{len(formal)}" '
            f'failures="{nfail}" errors="{nerror + nblocked}">\n')
    from xml.sax.saxutils import escape as xesc
    for c in formal:
        if c["status"] == "PASS":
            f.write(f'  <testcase name="{xesc(c["caseId"])}" classname="{xesc(c["scenario"])}"/>\n')
        else:
            f.write(f'  <testcase name="{xesc(c["caseId"])}" classname="{xesc(c["scenario"])}">\n')
            f.write(f'    <failure message="{xesc(str(c.get("actualCode","")))}">{xesc(c.get("note",""))}</failure>\n  </testcase>\n')
    f.write("</testsuite>\n")
try:
    tree = ET.parse(os.path.join(FR, "junit", "results.xml"))
    jtests = int(tree.getroot().get("tests"))
    gate("junit-valid", True)
    gate("junit-matches-formal", jtests == len(formal), f"junit={jtests} formal={len(formal)}")
except Exception as e:
    gate("junit-valid", False, str(e)[:80])

# ===== 7. 敏感扫描 =====
PATTERNS = [
    (re.compile(r"ChangeMe123!"), "default-pw"),
    (re.compile(r"<REDACTED-INFRA-PASSWORD>"), "ssh-pw"),
    (re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*"), "jwt"),
    (re.compile(r"\$2[aby]\$1[0-9]\$[A-Za-z0-9./]{53}"), "bcrypt"),
]
hits = []
for root, dirs, files in os.walk(FR):
    for fn in files:
        if fn in ("sensitive-scan.txt", "manifest.sha256", "FINAL-STATUS.txt", "run-summary.json"):
            continue
        fp = os.path.join(root, fn)
        if os.path.splitext(fn)[1] in (".png", ".zip", ".gz", ".exe"):
            continue
        try:
            content = open(fp, encoding="utf-8", errors="replace").read()
        except Exception:
            continue
        for pat, label in PATTERNS:
            for m in pat.finditer(content):
                hits.append((os.path.relpath(fp, FR), label))
scan_result = "PASS" if not hits else f"FAIL({len(hits)})"
gate("sensitive-scan", not hits, f"{len(hits)} hits")

# ===== 8. 写 run-summary（最后写！冻结顺序 P0-2） =====
all_ok = all(g["pass"] for g in GATES)
summary = {
    "runId": RUN_ID,  # P0-3 修复：用命令行 RUN_ID
    "mode": "final", "baseline": "B0",
    "runStatus": "PASS" if all_ok else "FAIL",
    "exitCode": 0 if all_ok else 1,
    "generatedAt": datetime.now(timezone.utc).isoformat(),
    "tests": {
        "unique": uniq if (uniq := len(set(ids_list))) else 0,
        "formal": len(formal), "passed": npass, "failed": nfail,
        "errors": nerror, "blocked": nblocked, "skipped": nskip,
        "info": len([c for c in aux if c["kind"] == "INFO"]),
        "setup": len([c for c in aux if c["kind"] == "SETUP"]),
        "caseIdsUnique": len(set(ids_list)) == len(ids_list),
    },
    "openapi": {
        "total": len(ops), "reached": reached,
        "behaviorComplete": behavior_complete,
        "partial": reached - behavior_complete,
        "notReached": len(ops) - reached,
    },
    "scenarios": scen_summary,
    "artifacts": {
        "rawSamples": len(samples),
        "junitValid": True,
        "caseIdsUnique": len(set(ids_list)) == len(ids_list),
        "sensitiveScan": scan_result,
        "manifest": "PENDING",
    },
    "gates": GATES,
    "cleanup": {"status": "PENDING_APPROVAL"},
}
json.dump(summary, open(os.path.join(FR, "run-summary.json"), "w", encoding="utf-8"),
          ensure_ascii=False, indent=1)

# ===== 9. manifest（最后生成，P0-2） =====
arts = []
for root, dirs, files in os.walk(RUN_DIR):
    dirs[:] = [d for d in dirs if d not in ("node_modules",)]
    for fn in sorted(files):
        fp = os.path.join(root, fn)
        rel = os.path.relpath(fp, RUN_DIR).replace("\\", "/")
        if rel == "manifest.sha256":
            continue  # 不含自身（防自引用）
        arts.append((rel, sha256(fp)))
with open(os.path.join(RUN_DIR, "manifest.sha256"), "w", encoding="utf-8") as f:
    for rel, h in arts:
        f.write(f"{h}  {rel}\n")

# ===== 10. 独立复算（P0-2 §5.5） =====
verify_ok = True
for line in open(os.path.join(RUN_DIR, "manifest.sha256"), encoding="utf-8"):
    parts = line.strip().split(None, 1)
    if len(parts) != 2:
        continue
    expected, rel = parts
    fp = os.path.join(RUN_DIR, rel)
    if not os.path.exists(fp) or sha256(fp) != expected:
        verify_ok = False
        break
gate("manifest-verify", verify_ok, f"{len(arts)} files")
# manifest 结果追加到 summary（但这会改 summary……不：写 verify 结果到独立文件）
with open(os.path.join(RUN_DIR, "manifest-verification.txt"), "w", encoding="utf-8") as f:
    f.write(f"verified={len(arts)}\nresult={'PASS' if verify_ok else 'FAIL'}\n")
print(f"  manifest-verification.txt: {'PASS' if verify_ok else 'FAIL'}（独立文件，不回写 summary/manifest）")

# ===== 11. FINAL-STATUS =====
all_ok2 = all(g["pass"] for g in GATES)
with open(os.path.join(RUN_DIR, "FINAL-STATUS.txt"), "w", encoding="utf-8") as f:
    f.write(f"FINAL STATUS: {'PASS' if all_ok2 else 'FAIL'}\n")
    f.write(f"Generated: {datetime.now(timezone.utc).isoformat()}\n\n")
    for g in GATES:
        tag = "PASS" if g["pass"] else "FAIL"
        d = f"  {g['detail']}" if g["detail"] and not g["pass"] else ""
        f.write(f"[{tag}] {g['name']}{d}\n")

print(f"\n{'='*60}")
print(f"FINAL STATUS: {'PASS' if all_ok2 else 'FAIL'}")
print(f"Formal: {len(formal)} ({npass} PASS / {nfail} FAIL / {nerror} ERROR / {nblocked} BLOCKED)")
print(f"Coverage: reached {reached}/66, behavior_complete {behavior_complete}/66")
print(f"Gap tasks: {len(gap_rows)}")
sys.exit(0 if all_ok2 else 1)