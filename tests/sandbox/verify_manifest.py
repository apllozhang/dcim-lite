#!/usr/bin/env python3
"""GT Manifest Verifier v5 (strict mode).
只读复算 manifest，不修改被验证目录。
检查：路径安全、重复项、缺失项、哈希不符、自引用、必需产物纳入、意外文件。
用法: python3 verify_manifest.py <RUN_DIR> [--strict]
退出: 0=全过；1=有错。
"""
import hashlib
import os
import sys
from pathlib import Path

REQUIRED_IN_MANIFEST = [
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

ALLOWED_EXTRA = {"manifest-verification.txt", "cleanup-plan.txt", "cleanup-result.txt"}
SELF_NAME = "manifest.sha256"


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for c in iter(lambda: f.read(1 << 20), b""):
            h.update(c)
    return h.hexdigest()


def main():
    run_dir = Path(sys.argv[1]).resolve()
    strict = "--strict" in sys.argv
    manifest_path = run_dir / SELF_NAME

    if not manifest_path.exists():
        print(f"FATAL: {manifest_path} not found")
        sys.exit(1)

    errors = []
    seen = set()
    n_entries = 0

    for line in open(manifest_path, encoding="utf-8"):
        parts = line.strip().split(None, 1)
        if len(parts) != 2:
            errors.append({"path": "<malformed-line>", "error": "malformed"})
            continue
        expected, rel = parts[0], parts[1]
        n_entries += 1

        # 路径安全
        if rel.startswith("/") or rel.startswith("\\") or ".." in Path(rel).parts:
            errors.append({"path": rel, "error": "path_traversal"})
            continue
        # 自引用（比较完整相对路径，非 basename）
        if Path(rel).as_posix() == SELF_NAME:
            errors.append({"path": rel, "error": "self_reference"})
            continue
        # 重复
        if rel in seen:
            errors.append({"path": rel, "error": "duplicate"})
            continue
        seen.add(rel)

        fp = run_dir / rel
        if not fp.exists():
            errors.append({"path": rel, "error": "missing"})
            continue
        if sha256(str(fp)) != expected:
            errors.append({"path": rel, "error": "mismatch"})

    # 嵌套证据根检测
    for p in seen:
        if p.startswith("run_output/") or p.startswith("output/"):
            errors.append({"path": p, "error": "nested_evidence_root"})
            break

    # 必需产物纳入检查
    for req in REQUIRED_IN_MANIFEST:
        if req not in seen:
            errors.append({"path": req, "error": "required_not_in_manifest"})

    # 严格模式：意外文件
    if strict:
        manifest_files = {Path(rel) for rel in seen}
        actual_files = set()
        for root, dirs, files in os.walk(run_dir):
            dirs[:] = [d for d in dirs if d not in ("node_modules", ".git", "__pycache__")]
            for fn in files:
                fp = Path(root) / fn
                rel = fp.relative_to(run_dir)
                if rel.name in ALLOWED_EXTRA or rel.name == SELF_NAME:
                    continue
                actual_files.add(rel)
        unexpected = actual_files - manifest_files
        if unexpected:
            for u in sorted(unexpected):
                errors.append({"path": str(u), "error": "unexpected_file_not_in_manifest"})

    # 输出
    for e in errors:
        print(f"[{e['error']}] {e['path']}")
    verified = n_entries - len([e for e in errors if e["error"] in ("mismatch", "missing")])
    print(f"\nentries={n_entries} verified={verified}/{n_entries} errors={len(errors)}")
    print(f"RESULT: {'PASS' if not errors else 'FAIL'}")
    sys.exit(0 if not errors else 1)


if __name__ == "__main__":
    main()