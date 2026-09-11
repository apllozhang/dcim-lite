#!/usr/bin/env python3
"""GT Core v3: 状态模型、HTTP客户端（含header/requestId采集）、规范化器、结果写入。
按第三轮指南 §5/§7/§8/§14 重构，消除假绿四漏洞。

差分回放扩展（2026-09-10）：目标系统若要求登录验证码，可用环境变量 GT_CAPTCHA 开启
自动附加（auto|on 开启，off 完全原生）。差异本身由 diff-probe 用例显式记录，不靠本开关掩盖。
"""
import base64
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone

# ============ 状态模型（§5） ============
# PASS/FAIL/ERROR/BLOCKED/SKIPPED/INFO/SETUP
FORMAL_STATUSES = {"PASS", "FAIL", "ERROR", "BLOCKED", "SKIPPED"}
AUX_STATUSES = {"INFO", "SETUP"}


def classify_result(expect, transport_error, parse_error, actual_status, actual_code):
    """指南 §5.3 判断逻辑。"""
    if transport_error:
        return "ERROR"
    if parse_error:
        return "ERROR"
    if expect is None:
        return "INFO"
    exp_status, exp_code = expect
    if isinstance(exp_status, (list, tuple, set)):
        st_ok = actual_status in exp_status
    else:
        st_ok = actual_status == exp_status
    code_ok = exp_code is None or actual_code == exp_code
    return "PASS" if (st_ok and code_ok) else "FAIL"


# ============ caseId 唯一性（§7） ============
_SEEN_IDS = set()


def register_case(case_id):
    if case_id in _SEEN_IDS:
        raise ValueError(f"duplicate caseId: {case_id}")
    _SEEN_IDS.add(case_id)


# ============ HTTP 客户端（§8） ============
JWT_RE = re.compile(r"^eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*$")
UUID_RE_STR = r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
UUID_RE = re.compile(UUID_RE_STR)
ISO_RE = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})")

CASES = []    # 正式+辅助（status 字段区分）
SAMPLES = []  # raw-redacted（含 headers）
NORMS = []    # normalized
UUID_MAP = {} # uuid -> 稳定名
TOKEN = {"v": ""}
API = {"v": "http://127.0.0.1:18080"}
RUN_ID = {"v": ""}
SENSITIVE_HDRS = {"authorization", "cookie", "set-cookie", "proxy-authorization"}

# ============ 验证码兼容层（差分回放用，不改变 off 时的原生行为）============
CAPTCHA_MODE = {"v": os.environ.get("GT_CAPTCHA", "off").strip().lower()}  # off|auto|on
CAPTCHA_USED = {"v": False}


def _svg_digits(svg_text):
    return "".join(re.findall(r">(\d)</text>", svg_text))


def fetch_captcha():
    """取验证码并解析出数字（复用前端同款明文解析），返回 (captchaId, code)。
    不注册 case、不写样本——内部兼容用。"""
    req = urllib.request.Request(API["v"] + "/api/v1/auth/captcha", method="GET")
    req.add_header("X-Golden-Run-Id", RUN_ID["v"])
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            payload = json.loads(r.read().decode("utf-8", "replace"))
    except Exception:
        return None, None
    d = (payload or {}).get("data") or {}
    img = d.get("image") or ""
    code = None
    if img.startswith("data:image/svg+xml;base64,"):
        try:
            svg = base64.b64decode(img.split(",", 1)[1]).decode("utf-8", "replace")
            code = _svg_digits(svg)
        except Exception:
            code = None
    return d.get("id"), code


def redact_obj(o):
    """递归脱敏：password/JWT/authorization。"""
    if isinstance(o, dict):
        return {k: ("<REDACTED_PASSWORD>" if k.lower() in ("password", "bindpassword") and isinstance(v, str) and v
                    else "Bearer <REDACTED_JWT>" if k.lower() in SENSITIVE_HDRS
                    else "<REDACTED_JWT>" if isinstance(v, str) and JWT_RE.match(v)
                    else redact_obj(v))
                for k, v in o.items()}
    if isinstance(o, list):
        return [redact_obj(x) for x in o]
    return o


def redact_headers(h):
    return {k: ("<REDACTED>" if k.lower() in SENSITIVE_HDRS else v) for k, v in (h or {}).items()}


def normalize_obj(o):
    """规范化：UUID→稳定映射、ISO时间→<TS>、requestId→<REQ_ID>。"""
    if isinstance(o, dict):
        return {k: normalize_obj(v) for k, v in o.items()}
    if isinstance(o, list):
        return [normalize_obj(x) for x in o]
    if isinstance(o, str):
        s = o
        for u, n in UUID_MAP.items():
            s = s.replace(u, f"<ID:{n}>")
        s = ISO_RE.sub("<TS>", s)
        s = re.sub(r"req_[0-9a-f-]{36}", "<REQ_ID>", s)
        if JWT_RE.match(s):
            s = "<JWT>"
        return s
    return o


def call(case_id, scenario, method, path, body=None, token=None, expect=None,
         note="", kind="TEST", category=None, operation=None, assertions=None):
    """统一 HTTP 调用。kind: TEST|INFO|SETUP。
    expect=(status, code) 或 (status, None) 或 None（INFO/SETUP）。
    自动采集 responseHeaders/X-Request-Id 并做信封一致性断言。
    """
    register_case(case_id)
    # 目标要求验证码时自动附加（原生行为由 GT_CAPTCHA=off 保证）
    if (CAPTCHA_MODE["v"] in ("auto", "on") and method == "POST"
            and path == "/api/v1/auth/login" and isinstance(body, dict)
            and "captchaId" not in body):
        cid, cap_code = fetch_captcha()
        if cid and cap_code:
            body = dict(body)
            body["captchaId"] = cid
            body["captcha"] = cap_code
            CAPTCHA_USED["v"] = True
    t0 = time.time()
    url = API["v"] + path
    req = urllib.request.Request(url, method=method)
    req.add_header("Content-Type", "application/json")
    req.add_header("X-Golden-Run-Id", RUN_ID["v"])
    req_headers_sent = {"Content-Type": "application/json", "X-Golden-Run-Id": RUN_ID["v"]}
    if token:
        req.add_header("Authorization", "Bearer " + token)
        req_headers_sent["Authorization"] = "<REDACTED>"
    data = json.dumps(body).encode() if body is not None else None

    status, payload, resp_headers, transport_err = -1, "", {}, False
    try:
        with urllib.request.urlopen(req, data=data, timeout=30) as r:
            status = r.status
            payload = r.read().decode("utf-8", "replace")
            resp_headers = {k: v for k, v in r.headers.items()}
    except urllib.error.HTTPError as e:
        status = e.code
        payload = e.read().decode("utf-8", "replace")
        resp_headers = {k: v for k, v in e.headers.items()}
    except Exception as e:
        payload = str(e)
        transport_err = True

    dur = int((time.time() - t0) * 1000)
    parse_err = False
    try:
        parsed = json.loads(payload) if payload else {}
    except Exception:
        parsed = {"code": "NON_JSON", "raw": payload[:200]}
        parse_err = True

    code = parsed.get("code", "?") if isinstance(parsed, dict) else "NON_JSON"
    rid_hdr = resp_headers.get("X-Request-Id", resp_headers.get("x-request-id", ""))
    rid_body = parsed.get("requestId", "") if isinstance(parsed, dict) else ""

    # 信封一致性（仅对 JSON 响应且非 NON_JSON）
    envelope_ok = True
    envelope_note = ""
    if not parse_err and isinstance(parsed, dict) and status >= 200:
        env_keys = set(parsed.keys())
        has_envelope = "code" in env_keys and "message" in env_keys
        ct = resp_headers.get("Content-Type", resp_headers.get("content-type", ""))
        ct_ok = "json" in ct.lower() if ct else False
        rid_match = (rid_hdr == rid_body) if (rid_hdr and rid_body) else True
        if has_envelope and not ct_ok:
            envelope_ok = False
            envelope_note = f"Content-Type missing json: {ct}"
        if has_envelope and rid_hdr and rid_body and not rid_match:
            envelope_ok = False
            envelope_note = f"requestId mismatch: hdr={rid_hdr[:20]} body={rid_body[:20]}"

    # 状态判定
    if kind in ("INFO", "SETUP") and expect is None:
        status_val = kind
    else:
        status_val = classify_result(expect, transport_err, parse_err, status, code)
        # 信封异常降级为 FAIL
        if status_val == "PASS" and not envelope_ok:
            status_val = "FAIL"
            envelope_note = envelope_note or "envelope check failed"

    CASES.append({
        "caseId": case_id, "scenario": scenario, "kind": kind, "status": status_val,
        "httpStatus": status, "actualCode": code,
        "expectedCode": str(expect[1]) if expect and expect[1] else (
            ("/".join(str(x) for x in expect[0]) if expect and isinstance(expect[0], (list, tuple, set))
             else str(expect[0]) if expect else "")),
        "note": (note + ("; " + envelope_note if envelope_note else ""))[:300],
        "durationMs": dur, "requestId": rid_hdr,
        "category": category,
        "operation": operation,
        "assertions": assertions,
        "coverageContribution": bool(category and kind == "TEST"),
    })

    SAMPLES.append({
        "caseId": case_id, "scenario": scenario, "kind": kind, "method": method,
        "path": path, "pathNormalized": normalize_obj(path) if isinstance(path, str) else path,
        "requestHeaders": redact_headers(req_headers_sent),
        "requestBody": redact_obj(body) if body is not None else None,
        "httpStatus": status, "responseHeaders": redact_headers(resp_headers),
        "responseBody": redact_obj(parsed),
        "durationMs": dur, "capturedAt": datetime.now(timezone.utc).isoformat(),
    })
    NORMS.append({
        "caseId": case_id, "kind": kind, "method": method, "status": status_val,
        "code": code, "respBody": normalize_obj(parsed),
        "respHeaders": normalize_obj(redact_headers(resp_headers)),
    })
    return parsed, status


def info_call(case_id, scenario, method, path, token=None, body=None, expect=None):
    """辅助查询（INFO），不计入通过率。"""
    return call(case_id, scenario, method, path, body=body, token=token, kind="INFO", expect=expect)


def setup_call(case_id, scenario, method, path, body=None, token=None, expect=None):
    """夹具创建（SETUP），失败阻断依赖场景。"""
    parsed, st = call(case_id, scenario, method, path, body=body, token=token,
                      expect=expect, kind="SETUP")
    if CASES[-1]["status"] not in ("SETUP", "PASS"):
        raise RuntimeError(f"SETUP failed: {case_id} -> {CASES[-1]['status']} {CASES[-1]['note']}")
    return parsed, st


def block_dependents(scenario, reason):
    """将指定场景的后续用例标记 BLOCKED。"""
    CASES.append({"caseId": f"{scenario}-BLOCKED", "scenario": scenario, "kind": "TEST",
                  "status": "BLOCKED", "httpStatus": 0, "actualCode": "", "expectedCode": "",
                  "note": reason[:300], "durationMs": 0, "requestId": ""})


def data_of(p):
    return p.get("data") if isinstance(p, dict) else None


def T():
    return TOKEN["v"]


def set_uuid(uuid, name):
    UUID_MAP[uuid] = name


def login(username, password, case_id="S01-AUTH-LOGIN-SUCCESS"):
    parsed, st = call(case_id, "S01", "POST", "/api/v1/auth/login",
                      {"username": username, "password": password},
                      expect=(200, "SUCCESS"))
    tok = (data_of(parsed) or {}).get("token")
    if tok:
        TOKEN["v"] = tok
    return tok, st, parsed


# ============ 统计（§5.4） ============

def compute_summary():
    formal = [c for c in CASES if c["status"] in FORMAL_STATUSES or
              (c["kind"] == "TEST" and c["status"] not in AUX_STATUSES)]
    # 其实 formal = kind==TEST 的所有条目
    formal = [c for c in CASES if c["kind"] == "TEST"]
    aux = [c for c in CASES if c["kind"] in ("INFO", "SETUP")]
    counts = {}
    for c in formal:
        counts[c["status"]] = counts.get(c["status"], 0) + 1
    total = len(formal)
    npass = counts.get("PASS", 0)
    nfail = counts.get("FAIL", 0) + counts.get("ERROR", 0) + counts.get("BLOCKED", 0) + counts.get("SKIPPED", 0)
    return {
        "total": total, "passed": npass, "failed": counts.get("FAIL", 0),
        "errors": counts.get("ERROR", 0), "blocked": counts.get("BLOCKED", 0),
        "skipped": counts.get("SKIPPED", 0),
        "info": len([c for c in aux if c["kind"] == "INFO"]),
        "setup": len([c for c in aux if c["kind"] == "SETUP"]),
        "caseIdsUnique": len(set(c["caseId"] for c in CASES)) == len(CASES),
        "blocking": nfail,
    }


def should_exit_nonzero():
    s = compute_summary()
    return s["failed"] > 0 or s["errors"] > 0 or s["blocked"] > 0


def write_outputs(out_dir):
    """写 JSONL + normalized。"""
    import os
    os.makedirs(os.path.join(out_dir, "api", "raw-redacted"), exist_ok=True)
    os.makedirs(os.path.join(out_dir, "api", "normalized"), exist_ok=True)
    with open(os.path.join(out_dir, "results.jsonl"), "w", encoding="utf-8") as f:
        for c in CASES:
            f.write(json.dumps(c, ensure_ascii=False) + "\n")
    with open(os.path.join(out_dir, "api", "raw-redacted", "samples.jsonl"), "w", encoding="utf-8") as f:
        for s in SAMPLES:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(out_dir, "api", "normalized", "normalized.jsonl"), "w", encoding="utf-8") as f:
        for s in NORMS:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    return len(CASES), len(SAMPLES)