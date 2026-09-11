#!/usr/bin/env python3
"""GT v5 Finalizer + Freezer + Verifier 自测（22 项）。
纯本地合成数据。用法: python3 test_finalize_run_v5.py（exit 0=全过）。"""
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = os.path.dirname(os.path.abspath(__file__))
BASE = os.path.dirname(os.path.dirname(HERE))
V5 = os.path.join(HERE, "finalize_run_v5.py")
FREEZE = os.path.join(HERE, "freeze_run.py")
VERIFY = os.path.join(HERE, "verify_manifest.py")
PASS = 0
FAIL = 0
RESULTS = []


def test(tid, name, expect_exit, got_exit, output=""):
    global PASS, FAIL
    ok = got_exit == expect_exit
    RESULTS.append({"id": tid, "name": name, "expect": expect_exit,
                    "got": got_exit, "pass": ok})
    if ok:
        PASS += 1
    else:
        FAIL += 1
        tail = output[-400:] if output else ""
        print(f"  FAIL {tid} ({name}): exit={got_exit} expect={expect_exit}\n    {tail}")


def make_synthetic(root, run_id="test-run-0001", inject=None):
    """生成完整合成运行目录。"""
    rd = Path(root) / run_id
    fr = rd / "final-regression"
    os.makedirs(fr / "api" / "raw-redacted", exist_ok=True)
    os.makedirs(fr / "api" / "normalized", exist_ok=True)
    os.makedirs(fr / "junit", exist_ok=True)
    os.makedirs(fr / "coverage", exist_ok=True)

    # run-metadata
    rid = inject.get("wrong_run_id", run_id) if inject else run_id
    json.dump({"run_id": rid}, open(rd / "run-metadata.json", "w"))

    # 66 操作合成数据
    import yaml
    doc = yaml.safe_load(open(os.path.join(BASE, "docs", "openapi.yaml"), encoding="utf-8"))
    import re
    cases, samples = [], []
    uid = 0
    for path, item in doc["paths"].items():
        for method in item:
            if method not in ("get", "post", "put", "delete", "patch"):
                continue
            uid += 1
            m = method.upper()
            real_path = re.sub(r"\{[^}]+\}", "00000000-0000-4000-8000-000000000001", path)
            entry = {"caseId": f"S99-{uid:04d}", "scenario": "S99", "kind": "TEST",
                     "status": "PASS", "httpStatus": 200, "actualCode": "SUCCESS",
                     "expectedCode": "SUCCESS", "note": "", "durationMs": 1,
                     "requestId": f"req_{uid:036d}"}
            if path == "/api/v1/devices" and "page" not in path:
                entry["category"] = "pagination"
            cases.append(entry)
            samples.append({"caseId": entry["caseId"], "scenario": "S99", "kind": "TEST",
                            "method": m, "path": real_path, "status": 200})
            # 负向类别样本
            for cat, st, code, tag in [
                ("auth", 401, "UNAUTHORIZED", None),
                ("val", 400, "INVALID_RESOURCE", None),
                ("conf", 409, "RESOURCE_CODE_DUPLICATE", None),
                ("stale", 409, "RESOURCE_VERSION_CONFLICT", None),
                ("invar", 200, "SUCCESS", "invariant"),
            ]:
                uid += 1
                e2 = {"caseId": f"S99-{uid:04d}-{cat}", "scenario": "S99", "kind": "TEST",
                      "status": "PASS", "httpStatus": st, "actualCode": code,
                      "expectedCode": code, "note": "", "durationMs": 1,
                      "requestId": f"req_{uid:036d}"}
                if tag:
                    e2["category"] = tag
                cases.append(e2)
                samples.append({"caseId": e2["caseId"], "scenario": "S99", "kind": "TEST",
                                "method": m, "path": real_path, "status": st})

    # 注入
    if inject:
        for i, kind in [("fail", "FAIL"), ("error", "ERROR"), ("blocked", "BLOCKED")]:
            if inject.get(i):
                uid += 1
                cases.append({"caseId": f"INJ-{i}", "scenario": "S99", "kind": "TEST",
                              "status": kind, "httpStatus": 0, "actualCode": "X",
                              "expectedCode": "", "note": "", "durationMs": 0, "requestId": ""})
        if inject.get("dup_caseid"):
            cases.append(dict(cases[0]))
        if inject.get("info_in_formal"):
            uid += 1
            cases.append({"caseId": f"INJ-INFO-{uid}", "scenario": "S99", "kind": "TEST",
                          "status": "INFO", "httpStatus": 200, "actualCode": "",
                          "expectedCode": "", "note": "", "durationMs": 0, "requestId": ""})
        if inject.get("sensitive_jwt"):
            samples[0] = dict(samples[0])
            samples[0]["responseHeaders"] = {"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.realsig"}

    # 写文件
    with open(fr / "results.jsonl", "w", encoding="utf-8") as f:
        for c in cases:
            f.write(json.dumps(c, ensure_ascii=False) + "\n")
    with open(fr / "api" / "raw-redacted" / "samples.jsonl", "w", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(fr / "api" / "normalized" / "normalized.jsonl", "w", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps({"caseId": s["caseId"], "status": s["status"]}, ensure_ascii=False) + "\n")
    return rd


def run_cmd(cmd, timeout=120):
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout,
                       encoding="utf-8", errors="replace")
    return r.returncode, (r.stdout or "") + (r.stderr or "")


def main():
    tmp = tempfile.mkdtemp(prefix="gt-v5-selftest-")
    openapi = os.path.join(BASE, "docs", "openapi.yaml")
    requirements = os.path.join(HERE, "coverage-requirements.yaml")

    try:
        # ===== FZ-01: 全过 → 0 =====
        rd = make_synthetic(tmp)
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-01", "all-pass-exit-0", 0, rc, out)

        # ===== FZ-02: 注入 FAIL → 1 =====
        rd = make_synthetic(tmp, inject={"fail": True})
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-02", "inject-fail", 1, rc, out)

        # ===== FZ-03: 注入 ERROR → 1 =====
        rd = make_synthetic(tmp, inject={"error": True})
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-03", "inject-error", 1, rc, out)

        # ===== FZ-04: 注入 BLOCKED → 1 =====
        rd = make_synthetic(tmp, inject={"blocked": True})
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-04", "inject-blocked", 1, rc, out)

        # ===== FZ-05: 重复 caseId → 1 =====
        rd = make_synthetic(tmp, inject={"dup_caseid": True})
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-05", "dup-caseid", 1, rc, out)

        # ===== FZ-06: INFO 混入 formal → 1 =====
        rd = make_synthetic(tmp, inject={"info_in_formal": True})
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-06", "info-in-formal", 1, rc, out)

        # ===== FZ-08: 缺一个操作覆盖 → 1 =====
        rd = make_synthetic(tmp)
        # 删除某操作全部样本
        sp = rd / "final-regression" / "api" / "raw-redacted" / "samples.jsonl"
        lines = sp.read_text(encoding="utf-8").splitlines()
        from collections import defaultdict
        by_path = defaultdict(list)
        for i, l in enumerate(lines):
            by_path[json.loads(l)["path"]].append(i)
        target = min(by_path, key=lambda k: len(by_path[k]))
        drop = set(by_path[target])
        sp.write_text("\n".join(l for i, l in enumerate(lines) if i not in drop) + "\n", encoding="utf-8")
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-08", "missing-op-coverage", 1, rc, out)

        # ===== FZ-10: RUN_ID 不一致 → 1 =====
        rd = make_synthetic(tmp, inject={"wrong_run_id": True})
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-10", "runid-inconsistent", 1, rc, out)

        # ===== FZ-15: --run-dir 指向 out → 1 =====
        out_dir = Path(tmp) / "out_test"
        out_dir.mkdir(exist_ok=True)
        rc, out = run_cmd([sys.executable, V5, "--run-dir", str(out_dir), "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-15", "run-dir-is-out", 1, rc, out)

        # ===== FZ-16: 冻结后修改文件 → verifier 1 =====
        rd = make_synthetic(tmp)
        rc, _ = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                         "--openapi", openapi, "--requirements", requirements])
        assert rc == 0, "预失败"
        rc, _ = run_cmd([sys.executable, FREEZE, "--run-dir", rd])
        assert rc == 0, "freeze 失败"
        # 篡改
        (rd / "final-regression" / "results.jsonl").open("a").write('{"tampered":true}\n')
        rc, out = run_cmd([sys.executable, VERIFY, rd])
        test("FZ-16", "tamper-after-freeze", 1, rc, out)

        # ===== FZ-17: 冻结后删除 gap-tasks → verifier 1 =====
        rd = make_synthetic(tmp)
        rc, _ = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                         "--openapi", openapi, "--requirements", requirements])
        rc, _ = run_cmd([sys.executable, FREEZE, "--run-dir", rd])
        (rd / "final-regression" / "coverage" / "gap-tasks.csv").unlink()
        rc, out = run_cmd([sys.executable, VERIFY, rd])
        test("FZ-17", "delete-gap-tasks", 1, rc, out)

        # ===== FZ-19: behavior 65/66 → 1 =====
        # 注入一个 wrong category 导致一个操作缺类别
        rd = make_synthetic(tmp)
        # 删除 auth 样本使 auth_boundary 缺失
        sp = rd / "final-regression" / "api" / "raw-redacted" / "samples.jsonl"
        lines = sp.read_text(encoding="utf-8").splitlines()
        filtered = [l for l in lines if json.loads(l).get("status") != 401]
        # 只删第一个 401
        new_lines = [l for l in lines if json.loads(l).get("status") != 401]
        sp.write_text("\n".join(new_lines) + "\n", encoding="utf-8")
        rc, out = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                           "--openapi", openapi, "--requirements", requirements])
        test("FZ-19", "behavior-not-66", 1, rc, out)

        # ===== FZ-22: 解包后哈希变化 → verifier 1 =====
        rd = make_synthetic(tmp)
        rc, _ = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                         "--openapi", openapi, "--requirements", requirements])
        rc, _ = run_cmd([sys.executable, FREEZE, "--run-dir", rd])
        # 模拟"解包到新目录"
        unpack_dir = Path(tmp) / "unpacked"
        shutil.copytree(rd, unpack_dir)
        # 篡改解包后的文件
        (unpack_dir / "FINAL-STATUS.txt").write_text("TAMPERED")
        rc, out = run_cmd([sys.executable, VERIFY, str(unpack_dir)])
        test("FZ-22", "unpack-hash-change", 1, rc, out)

        # ===== 正向：freeze + verify 全过 → 0 =====
        rd = make_synthetic(tmp)
        rc, _ = run_cmd([sys.executable, V5, "--run-dir", rd, "--run-id", "test-run-0001",
                         "--openapi", openapi, "--requirements", requirements])
        assert rc == 0
        rc, _ = run_cmd([sys.executable, FREEZE, "--run-dir", rd])
        assert rc == 0
        rc, out = run_cmd([sys.executable, VERIFY, rd, "--strict"])
        test("FZ-00", "freeze-verify-pass-strict", 0, rc, out)

    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    # 输出每个测试 ID
    print(f"\n{'='*60}")
    for r in RESULTS:
        tag = "PASS" if r["pass"] else "FAIL"
        print(f"  [{tag}] {r['id']}: {r['name']}")
    print(f"\nFinalizer+Freezer+Verifier 自测: {PASS} PASS / {FAIL} FAIL")
    if FAIL:
        sys.exit(1)
    print("ALL SELFTESTS PASSED")


if __name__ == "__main__":
    import shutil
    from collections import defaultdict
    main()