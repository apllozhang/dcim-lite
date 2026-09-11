#!/usr/bin/env python3
"""GT Freezer: 检查产物齐全并生成 manifest（P0-B §5.3）。
在 finalizer 之后运行。manifest 生成后运行目录冻结，禁止修改。
用法: python3 freeze_run.py --run-dir <RUN_ID_ROOT>
退出: 0=冻结成功并验证通过；1=有错误。
"""
import argparse
import hashlib
import json
import os
import sys
from pathlib import Path

REQUIRED = [
    "run-metadata.json",
    "FINAL-STATUS.txt",
    "sensitive-scan.txt",
    "final-regression/results.jsonl",
    "final-regression/run-summary.json",
    "final-regression/api/raw-redacted/samples.jsonl",
    "final-regression/api/normalized/normalized.jsonl",
    "final-regression/coverage/api-coverage-matrix.csv",
    "final-regression/coverage/gap-tasks.csv",
    "final-regression/coverage/scenario-summary.json",
    "final-regression/junit/results.xml",
]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for c in iter(lambda: f.read(1 << 20), b""):
            h.update(c)
    return h.hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", required=True)
    a = ap.parse_args()
    run_dir = Path(a.run_dir).resolve()
    manifest_path = run_dir / "manifest.sha256"

    # 1. 检查必需产物
    missing = [r for r in REQUIRED if not (run_dir / r).exists()]
    if missing:
        print(f"FATAL: {len(missing)} required artifacts missing:")
        for m in missing:
            print(f"  MISSING: {m}")
        sys.exit(1)
    print(f"Required artifacts: {len(REQUIRED)}/{len(REQUIRED)} present")

    # 2. 检查 run-metadata 里的 RUN_ID 与目录一致
    meta = json.load(open(run_dir / "run-metadata.json", encoding="utf-8"))
    dir_id = run_dir.name
    if meta.get("run_id") != dir_id:
        print(f"FATAL: run-metadata.run_id='{meta.get('run_id')}' != directory='{dir_id}'")
        sys.exit(1)
    print(f"RUN_ID consistency: OK ({dir_id})")

    # 3. 计算清单（不含 manifest 自身，防自引用）
    entries = []
    for root, dirs, files in os.walk(run_dir):
        dirs[:] = [d for d in dirs if d not in ("node_modules", ".git", "__pycache__")]
        for fn in sorted(files):
            fp = os.path.join(root, fn)
            rel = os.path.relpath(fp, run_dir).replace("\\", "/")
            if rel in ("manifest.sha256", "manifest-verification.txt"):
                continue
            entries.append((rel, sha256(fp)))
    entries.sort()

    # 4. 原子写 manifest
    content = "".join(f"{h}  {rel}\n" for rel, h in entries)
    tmp = str(manifest_path) + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        f.write(content)
    os.replace(tmp, str(manifest_path))
    print(f"manifest.sha256 written: {len(entries)} files (atomic)")

    # 5. 立即反向验证（重新读磁盘）
    verify_ok = True
    for expected, rel in [(h, r) for r, h in entries]:
        fp = run_dir / rel
        if not fp.exists() or sha256(str(fp)) != expected:
            print(f"VERIFY FAIL: {rel}")
            verify_ok = False
    if not verify_ok:
        sys.exit(1)
    print(f"Immediate verification: {len(entries)}/{len(entries)} PASS")

    # 6. 写验证结果（在 manifest 覆盖范围外）
    ver_result = run_dir / "manifest-verification.txt"
    ver_result.write_text(
        f"manifest_entries={len(entries)}\n"
        f"verified={len(entries)}\n"
        f"mismatch=0\n"
        f"result=PASS\n"
        f"frozen_at={datetime_now_iso()}\n",
        encoding="utf-8")
    print("manifest-verification.txt written (outside manifest-covered set)")
    print("\nFREEZE COMPLETE - run directory is now read-only for evidence integrity")


def datetime_now_iso():
    from datetime import datetime, timezone
    return datetime.now(timezone.utc).isoformat()


if __name__ == "__main__":
    main()