#!/usr/bin/env python3
"""GT v4 Finalizer 负向/正向自测（指南 §7，14 组）。
纯本地合成数据，不连接网络/数据库。用法: python3 test_finalize_run.py（exit 0=全过）。"""
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
BASE = os.path.dirname(os.path.dirname(HERE))  # 项目根（tests/sandbox → tests → root）
PASS = 0
FAIL = 0

# 66 操作的最小合成模板
SAMPLE_OPS = [
    ("POST", "/api/v1/auth/login", 200, "SUCCESS"),
    ("GET", "/api/v1/auth/me", 200, "SUCCESS"),
    ("POST", "/api/v1/auth/logout", 200, "SUCCESS"),
    ("GET", "/health/live", 200, "SUCCESS"),
    ("GET", "/health/ready", 200, "SUCCESS"),
    ("GET", "/api/v1/resource-tree", 200, "SUCCESS"),
]


def make_run_dir(root, op_list=None, inject=None, run_id="test-run-0001"):
    """生成合成运行目录。inject: dict 注入参数。"""
    rd = os.path.join(root, run_id)
    fr = os.path.join(rd, "final-regression")
    os.makedirs(os.path.join(fr, "api", "raw-redacted"), exist_ok=True)
    os.makedirs(os.path.join(fr, "api", "normalized"), exist_ok=True)
    os.makedirs(os.path.join(fr, "junit"), exist_ok=True)
    os.makedirs(os.path.join(fr, "coverage"), exist_ok=True)

    # run-metadata
    json.dump({"run_id": run_id}, open(os.path.join(rd, "run-metadata.json"), "w"))

    # 用完整 66 操作生成合成数据
    import yaml
    doc = yaml.safe_load(open(os.path.join(BASE, "docs", "openapi.yaml"), encoding="utf-8"))
    cases, samples = [], []
    uid = 0
    import re
    for path, item in doc["paths"].items():
        for method in item:
            if method not in ("get", "post", "put", "delete", "patch"):
                continue
            uid += 1
            m = method.upper()
            # 把 {id} 替换成固定 UUID（finalize 的 match_op 需要 UUID 才能匹配）
            real_path = re.sub(r"\{[^}]+\}", "00000000-0000-4000-8000-000000000001", path)
            entry = {"caseId": f"S99-OP-{uid:03d}", "scenario": "S99", "kind": "TEST",
                     "status": "PASS", "httpStatus": 200, "actualCode": "SUCCESS",
                     "expectedCode": "SUCCESS", "note": "", "durationMs": 1,
                     "requestId": f"req_{uid:036d}"}
            # 列表接口：该 happy_path 样本同时证明 pagination（带 page/pageSize 查询）
            if path == "/api/v1/devices":
                entry["category"] = "pagination"
            cases.append(entry)
            samples.append({"caseId": f"S99-OP-{uid:03d}", "scenario": "S99", "kind": "TEST",
                            "method": m, "path": real_path, "status": 200})

    # 覆盖类别样本：对需 auth_boundary/validation/conflict/stale 的操作补负向样本
    # 简化：为每操作追加 5 类合成 case
    for i, c in enumerate(list(cases)):
        op_key = (samples[i]["method"], samples[i]["path"])
        for cat, st, code, _cat_tag in [("auth", 401, "UNAUTHORIZED", None),
                              ("val", 400, "INVALID_RESOURCE", None),
                              ("conf", 409, "RESOURCE_CODE_DUPLICATE", None),
                              ("stale", 409, "RESOURCE_VERSION_CONFLICT", None),
                              ("invar", 200, "SUCCESS", "invariant")]:
            uid += 1
            entry = {"caseId": f"S99-OP-{uid:03d}-{cat}", "scenario": "S99", "kind": "TEST",
                     "status": "PASS", "httpStatus": st, "actualCode": code,
                     "expectedCode": code, "note": "", "durationMs": 1,
                     "requestId": f"req_{uid:036d}"}
            if cat == "invar":
                entry["category"] = "invariant"
            cases.append(entry)
            samples.append({"caseId": cases[-1]["caseId"], "scenario": "S99", "kind": "TEST",
                            "method": op_key[0], "path": op_key[1], "status": st})

    # 注入
    if inject:
        if inject.get("fail"):
            cases.append({"caseId": "INJECT-FAIL", "scenario": "S99", "kind": "TEST",
                          "status": "FAIL", "httpStatus": 500, "actualCode": "E",
                          "expectedCode": "S", "note": "", "durationMs": 1, "requestId": ""})
        if inject.get("error"):
            cases.append({"caseId": "INJECT-ERROR", "scenario": "S99", "kind": "TEST",
                          "status": "ERROR", "httpStatus": 0, "actualCode": "T",
                          "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
        if inject.get("blocked"):
            cases.append({"caseId": "INJECT-BLOCKED", "scenario": "S99", "kind": "TEST",
                          "status": "BLOCKED", "httpStatus": 0, "actualCode": "",
                          "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
        if inject.get("dup_caseid"):
            cases.append(dict(cases[0]))  # 复制第一条（重复 caseId）
        if inject.get("info_in_formal"):
            cases.append({"caseId": "INJECT-INFO", "scenario": "S99", "kind": "TEST",
                          "status": "INFO", "httpStatus": 200, "actualCode": "",
                          "expectedCode": "", "note": "", "durationMs": 1, "requestId": ""})
        if inject.get("wrong_run_id"):
            json.dump({"run_id": "wrong-id"}, open(os.path.join(rd, "run-metadata.json"), "w"))
        if inject.get("sensitive"):
            cases[0] = dict(cases[0])
            samples[0] = dict(samples[0])
            samples[0]["responseBody"] = {"token": "eyJfake.eyJfake.signature"}

    with open(os.path.join(fr, "results.jsonl"), "w", encoding="utf-8") as f:
        for c in cases:
            f.write(json.dumps(c, ensure_ascii=False) + "\n")
    with open(os.path.join(fr, "api", "raw-redacted", "samples.jsonl"), "w", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(fr, "api", "normalized", "normalized.jsonl"), "w", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps({"caseId": s["caseId"], "kind": s["kind"], "method": s["method"],
                                "status": s["status"], "code": "SUCCESS" if s["status"] == 200 else "?"},
                               ensure_ascii=False) + "\n")
    return rd


def run_finalizer(run_dir, run_id, extra=None):
    """在合成目录上运行 finalize_run_v4。返回 (exit_code, stdout)。"""
    cmd = [sys.executable, os.path.join(HERE, "finalize_run_v4.py"),
           "--run-dir", run_dir, "--run-id", run_id,
           "--openapi", os.path.join(BASE, "docs", "openapi.yaml"),
           "--requirements", os.path.join(HERE, "coverage-requirements.yaml")]
    if extra:
        cmd += extra
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=120,
                       encoding="utf-8", errors="replace")
    return r.returncode, (r.stdout or "") + (r.stderr or "")


def test(name, expect_exit, got_exit, output=""):
    global PASS, FAIL
    ok = got_exit == expect_exit
    if ok:
        PASS += 1
    else:
        FAIL += 1
        tail = output[-300:] if output else ""
        print(f"  FAIL {name}: exit={got_exit} expect={expect_exit}\n    {tail}")


def main():
    tmp = tempfile.mkdtemp(prefix="gt-v4-selftest-")
    openapi_backup = None
    try:
        # FZ-01: 全部门禁满足 → 0
        rd = make_run_dir(tmp)
        rc, out = run_finalizer(rd, "test-run-0001")
        if rc != 0:
            print("=== FZ-01 DEBUG ===")
            print(out[-1500:])
        test("FZ-01-all-pass", 0, rc, out)

        # FZ-02: 注入 FAIL → 1
        rd = make_run_dir(tmp, inject={"fail": True})
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-02-inject-fail", 1, rc, out)

        # FZ-03: 注入 ERROR → 1
        rd = make_run_dir(tmp, inject={"error": True})
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-03-inject-error", 1, rc, out)

        # FZ-04: 注入 BLOCKED → 1
        rd = make_run_dir(tmp, inject={"blocked": True})
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-04-inject-blocked", 1, rc, out)

        # FZ-05: 重复 caseId → 1
        rd = make_run_dir(tmp, inject={"dup_caseid": True})
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-05-dup-caseid", 1, rc, out)

        # FZ-06: INFO 混入正式（kind=TEST 但 status=INFO → 状态分类异常）→ 应非 0
        rd = make_run_dir(tmp, inject={"info_in_formal": True})
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-06-info-in-formal", 1, rc, out)

        # FZ-08: OpenAPI 只触达 65/66 → 1（删掉一个样本）
        rd = make_run_dir(tmp)
        fr = os.path.join(rd, "final-regression")
        samples_path = os.path.join(fr, "api", "raw-redacted", "samples.jsonl")
        lines = open(samples_path, encoding="utf-8").readlines()
        # 删除某操作的【全部】样本（找唯一 path 最少样本数的操作）
        from collections import defaultdict
        by_path = defaultdict(list)
        for i, l in enumerate(lines):
            sp = json.loads(l)["path"]
            by_path[sp].append(i)
        # 找样本数最少的 path，全部删除
        target = min(by_path, key=lambda k: len(by_path[k]))
        drop = set(by_path[target])
        open(samples_path, "w", encoding="utf-8").writelines(
            l for i, l in enumerate(lines) if i not in drop)
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-08-missing-op-coverage", 1, rc, out)
        shutil.rmtree(rd, ignore_errors=True)

        # FZ-10: RUN_ID 不一致 → 1
        rd = make_run_dir(tmp, inject={"wrong_run_id": True})
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-10-runid-inconsistent", 1, rc, out)

        # FZ-11: 篡改 manifest 覆盖文件 → verifier 1
        rd = make_run_dir(tmp)
        rc, _ = run_finalizer(rd, "test-run-0001")
        assert rc == 0, "预失败：合成基线应先过"
        # 篡改
        fr = os.path.join(rd, "final-regression")
        with open(os.path.join(fr, "results.jsonl"), "a", encoding="utf-8") as f:
            f.write('{"caseId":"TAMPERED"}\n')
        vm = os.path.join(HERE, "verify_manifest.py")
        rc2 = subprocess.run([sys.executable, vm, rd], capture_output=True, timeout=60).returncode
        test("FZ-11-tamper-manifest-verify", 1, rc2)

        # FZ-12: 删除一个必需产物 → verifier 1
        os.remove(os.path.join(fr, "results.jsonl"))
        rc2 = subprocess.run([sys.executable, vm, rd], capture_output=True, timeout=60).returncode
        test("FZ-12-delete-required", 1, rc2)

        # FZ-14: 敏感信息注入 → finalizer 1
        rd = make_run_dir(tmp, inject={"sensitive": True})
        # 手动注入 JWT 到 samples
        fr = os.path.join(rd, "final-regression")
        sp = os.path.join(fr, "api", "raw-redacted", "samples.jsonl")
        lines = open(sp, encoding="utf-8").readlines()
        s0 = json.loads(lines[0])
        s0["responseHeaders"] = {"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.real_sig"}
        lines[0] = json.dumps(s0, ensure_ascii=False) + "\n"
        open(sp, "w", encoding="utf-8").writelines(lines)
        rc, out = run_finalizer(rd, "test-run-0001")
        test("FZ-14-sensitive-hit", 1, rc, out)

    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print(f"\nFinalizer 自测: {PASS} PASS / {FAIL} FAIL")
    if FAIL:
        sys.exit(1)
    print("ALL FINALIZER SELFTESTS PASSED")


if __name__ == "__main__":
    main()