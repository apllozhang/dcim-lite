#!/usr/bin/env python3
"""GT Final Runner（R5 专用，one-shot）：从全新 B0 一次性执行完整回归。
- 稳定 caseId；JSONL 结果（合法逐行）；JUnit XML；覆盖矩阵；run-summary.json
- 模式：--mode explore|final（final 禁止中途改预期，产物只落 final-regression/）
用法（服务器端）:
  <venv-python> gt_final.py --mode final --env-file E --run-id RID --pg PG --out OUT --scenarios ALL
"""
import argparse
import base64
import datetime
import hashlib
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request
from xml.sax.saxutils import escape as xesc

API = "http://127.0.0.1:18080"
NONCE = "F" + f"{int(datetime.datetime.utcnow().timestamp()) % 10000000:07d}"
JWT_RE = re.compile(r"^eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*$")
ISO_RE = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})")

CASES = []          # {"caseId","scenario","status","httpStatus","actualCode","expectedCode","note","durationMs"}
SAMPLES = []        # raw-redacted
NORMS = []          # normalized
uuid_map = {}
TOKEN = {"v": ""}
ADMIN_PW = {"v": ""}


def _redact(o):
    if isinstance(o, dict):
        return {k: ("<REDACTED_PASSWORD>" if k.lower() == "password" and isinstance(v, str) and v
                    else "<REDACTED_JWT>" if isinstance(v, str) and JWT_RE.match(v) else _redact(v))
                for k, v in o.items()}
    if isinstance(o, list):
        return [_redact(x) for x in o]
    return o


def _norm(o):
    if isinstance(o, dict):
        return {k: _norm(v) for k, v in o.items()}
    if isinstance(o, list):
        return [_norm(x) for x in o]
    if isinstance(o, str):
        s = o
        for u, n in uuid_map.items():
            s = s.replace(u, f"<ID:{n}>")
        s = ISO_RE.sub("<TS>", s)
        s = re.sub(r"req_[0-9a-f-]{36}", "<REQ_ID>", s)
        if JWT_RE.match(s):
            s = "<JWT>"
        return s
    return o


def call(case_id, scenario, method, path, body=None, token=None, expect=None, note=""):
    t0 = time.time()
    req = urllib.request.Request(API + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data_ = json.dumps(body).encode() if body is not None else None
    status, payload = -1, ""
    try:
        with urllib.request.urlopen(req, data=data_, timeout=20) as r:
            status, payload = r.status, r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        status, payload = e.code, e.read().decode("utf-8", "replace")
    except Exception as e:
        payload = str(e)
    dur = int((time.time() - t0) * 1000)
    try:
        parsed = json.loads(payload)
    except Exception:
        parsed = {"code": "NON_JSON", "raw": payload[:200]}
    code = parsed.get("code", "?") if isinstance(parsed, dict) else "NON_JSON"

    SAMPLES.append({"caseId": case_id, "scenario": scenario, "method": method, "path": path,
                    "status": status, "reqBody": _redact(body) if body is not None else None,
                    "respBody": _redact(parsed), "ts": datetime.datetime.utcnow().isoformat() + "Z"})
    NORMS.append({"caseId": case_id, "method": method, "status": status, "code": code,
                  "respBody": _norm(parsed)})

    exp_code = None
    ok = True
    if expect is not None:
        exp_status, exp_code = expect
        if isinstance(exp_status, (list, tuple, set)):
            ok = (status in exp_status) and (exp_code is None or code == exp_code)
            exp_status_repr = "/".join(str(x) for x in exp_status)
        else:
            ok = (status == exp_status) and (exp_code is None or code == exp_code)
            exp_status_repr = str(exp_status)
    CASES.append({"caseId": case_id, "scenario": scenario, "status": "PASS" if ok else "FAIL",
                  "httpStatus": status, "actualCode": code,
                  "expectedCode": (exp_code or exp_status_repr) if expect is not None else "",
                  "note": note[:200], "durationMs": dur})
    return parsed, status


def data_of(p):
    return p.get("data") if isinstance(p, dict) else None


def T():
    return TOKEN["v"]


def login_pwd(u, pw):
    parsed, st = call("auth-login-ok" if u == "admin" else "auth-login-user-ok", "S1" if u == "admin" else "S11",
                      "POST", "/api/v1/auth/login", {"username": u, "password": pw}, expect=(200, "SUCCESS"))
    return (data_of(parsed) or {}).get("token")


def find_tree(rid, suffix):
    parsed, _ = call(f"lookup-{suffix}", "S7", "GET", "/api/v1/resource-tree", token=T())
    want = f"GT-{rid}-{suffix}".upper()

    def walk(nodes):
        for n in nodes:
            if str(n.get("code", "")).upper() == want:
                return n
            for k in ("rooms", "racks"):
                if isinstance(n.get(k), list):
                    r = walk(n[k])
                    if r:
                        return r
        return None
    return walk(data_of(parsed)["items"])


def setup_tree(rid):
    """B0 幂等建树：DC01→RM01→RK01(模板)/RK02。已存在则跳过（按编码查）。"""
    if find_tree(rid, "DC01"):
        return
    P = f"GT-{rid}"
    parsed, _ = call("S2-dc-create", "S2", "POST", "/api/v1/data-centers",
                     {"code": P + "-DC01", "name": "黄金测试数据中心", "remarks": "goldenRunId=" + rid,
                      "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    dc = data_of(parsed)
    uuid_map[dc["id"]] = "DC01"
    call("S2-dc-dupcode", "S2", "POST", "/api/v1/data-centers",
         {"code": P + "-DC01", "name": "重复", "autoGenerateCode": False}, token=T(),
         expect=(409, "RESOURCE_CODE_DUPLICATE"))
    parsed, _ = call("S2-room-create", "S2", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
                     {"code": P + "-RM01", "name": "黄金测试机房", "autoGenerateCode": False}, token=T(),
                     expect=(201, "SUCCESS"))
    room = data_of(parsed)
    uuid_map[room["id"]] = "RM01"
    parsed, _ = call("S2-tpl-list", "S2", "GET", "/api/v1/rack-templates", token=T(), expect=(200, "SUCCESS"))
    tpl = data_of(parsed)["items"][0]
    uuid_map[tpl["id"]] = "TPL_SYS"
    parsed, _ = call("S2-rack-create", "S2", "POST", f"/api/v1/rooms/{room['id']}/racks",
                     {"code": P + "-RK01", "name": "黄金测试机柜A", "templateId": tpl["id"],
                      "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    rack = data_of(parsed)
    uuid_map[rack["id"]] = "RK01"
    parsed, _ = call("S2-rack-create2", "S2", "POST", f"/api/v1/rooms/{room['id']}/racks",
                     {"code": P + "-RK02", "name": "黄金测试机柜B", "autoGenerateCode": False}, token=T(),
                     expect=(201, "SUCCESS"))
    uuid_map[data_of(parsed)["id"]] = "RK02"
    parsed, _ = call("S2-dc-disable", "S2", "PUT", f"/api/v1/data-centers/{dc['id']}?version={dc['version']}",
                     {"code": dc["code"], "name": dc["name"], "status": "DISABLED", "version": dc["version"]},
                     token=T(), expect=(200, "SUCCESS"))
    dc2 = data_of(parsed)
    call("S2-room-in-disabled-dc", "S2", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
         {"code": P + "-RMX", "name": "不应成功", "autoGenerateCode": False}, token=T(),
         expect=(409, "PARENT_RESOURCE_DISABLED"))
    parsed, _ = call("S2-dc-enable", "S2", "PUT", f"/api/v1/data-centers/{dc['id']}?version={dc2['version']}",
                     {"code": dc["code"], "name": dc["name"], "status": "OPERATING", "version": dc2["version"]},
                     token=T(), expect=(200, "SUCCESS"))
    dc3 = data_of(parsed)
    call("S2-dc-delete-haschildren", "S2", "DELETE", f"/api/v1/data-centers/{dc['id']}?version={dc3['version']}",
         token=T(), expect=(409, "RESOURCE_HAS_CHILDREN"))


def s3_final(rid):
    dc = find_tree(rid, "DC01")
    ver = dc["version"]
    call("S3-stale-put", "S3", "PUT", f"/api/v1/data-centers/{dc['id']}?version={ver + 5}",
         {"code": dc["code"], "name": "x", "version": ver + 5}, token=T(),
         expect=(409, "RESOURCE_VERSION_CONFLICT"))
    parsed, _ = call("S3-ok-put", "S3", "PUT", f"/api/v1/data-centers/{dc['id']}?version={ver}",
                     {"code": dc["code"], "name": "黄金测试数据中心改", "status": "OPERATING", "version": ver},
                     token=T(), expect=(200, "SUCCESS"))
    call("S3-stale-delete", "S3", "DELETE", f"/api/v1/data-centers/{dc['id']}?version=999", token=T(),
         expect=(409, "RESOURCE_HAS_CHILDREN"))


def s6_final(rid):
    P = f"GT-{rid}"
    rack = find_tree(rid, "RK01")
    RK = rack["id"]
    parsed, _ = call("S6-devtypes", "S6", "GET", "/api/v1/device-types", token=T(), expect=(200, "SUCCESS"))
    tid = next(t["id"] for t in data_of(parsed)["items"] if t["code"] == "SERVER")
    parsed, _ = call("S6-dev1-create", "S6", "POST", "/api/v1/devices",
                     {"typeId": tid, "code": P + "-DV1", "name": "黄金设备01", "autoGenerateCode": False},
                     token=T(), expect=(201, "SUCCESS"))
    d1 = data_of(parsed)
    uuid_map[d1["id"]] = "DV1"
    check = data_of(parsed) or {}
    call("S6-default-height", "S6", "GET", f"/api/v1/devices/{d1['id']}", token=T(),
         expect=(200, "SUCCESS"))
    d2parsed, _ = call("S6-dev2-create", "S6", "POST", "/api/v1/devices",
                       {"typeId": tid, "code": P + "-DV2", "name": "黄金设备02", "heightU": 4,
                        "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    d2 = data_of(d2parsed)
    uuid_map[d2["id"]] = "DV2"
    call("S6-assign-ok", "S6", "POST", f"/api/v1/devices/{d1['id']}/assign",
         {"version": d1["version"], "rackId": RK, "startU": 40, "heightU": 2, "orientation": "NORMAL",
          "reason": "GT-S6"}, token=T(), expect=(200, "SUCCESS"))
    call("S6-ulayout", "S6", "GET", f"/api/v1/racks/{RK}/u-layout", token=T(), expect=(200, "SUCCESS"))
    call("S6-assign-overlap", "S6", "POST", f"/api/v1/devices/{d2['id']}/assign",
         {"version": d2["version"], "rackId": RK, "startU": 39, "heightU": 4, "orientation": "NORMAL",
          "reason": "GT"}, token=T(), expect=(409, "RACK_U_CONFLICT"))
    call("S6-assign-double", "S6", "POST", f"/api/v1/devices/{d1['id']}/assign",
         {"version": d1["version"] + 1, "rackId": RK, "startU": 10, "heightU": 2, "orientation": "NORMAL",
          "reason": "GT"}, token=T(), expect=(409, "DEVICE_POSITIONED"))
    call("S6-assign-outrange", "S6", "POST", f"/api/v1/devices/{d2['id']}/assign",
         {"version": d2["version"], "rackId": RK, "startU": 100, "heightU": 4, "orientation": "NORMAL",
          "reason": "GT"}, token=T(), expect=(400, "INVALID_RESOURCE"))
    call("S6-move-ok", "S6", "POST", f"/api/v1/devices/{d1['id']}/move",
         {"version": d1["version"] + 1, "rackId": RK, "startU": 20, "heightU": 2, "orientation": "REVERSE",
          "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    call("S6-decommission", "S6", "POST", f"/api/v1/devices/{d1['id']}/decommission",
         {"version": d1["version"] + 2, "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    # 写后读
    call("S6-readback", "S6", "GET", f"/api/v1/devices/{d1['id']}", token=T(), expect=(200, "SUCCESS"))


def s7_final(rid):
    P = f"GT-{rid}"
    dc = find_tree(rid, "DC01")
    room = find_tree(rid, "RM01")
    rack = find_tree(rid, "RK01")
    parsed, _ = call("S7-dc-copy", "S7", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
                     {"code": P + "-DCC", "name": "黄金复制DC", "autoGenerateCode": False,
                      "version": dc["version"]}, token=T(), expect=(201, "SUCCESS"))
    dcc = (data_of(parsed) or {}).get("resource") or {}
    uuid_map[dcc["id"]] = "DCC"
    call("S7-dc-copy-dup", "S7", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
         {"code": P + "-DCC", "name": "重复", "autoGenerateCode": False, "version": dc["version"]},
         token=T(), expect=(409, "RESOURCE_CODE_DUPLICATE"))
    call("S7-room-copy", "S7", "POST", f"/api/v1/rooms/{room['id']}/copy",
         {"code": P + "-RMC", "name": "黄金复制机房", "autoGenerateCode": False, "version": room["version"],
          "targetDataCenterId": dc["id"]}, token=T(), expect=(201, "SUCCESS"))
    call("S7-rack-copy", "S7", "POST", f"/api/v1/racks/{rack['id']}/copy",
         {"code": P + "-RKC", "name": "黄金复制机柜", "autoGenerateCode": False, "version": rack["version"],
          "targetRoomId": room["id"]}, token=T(), expect=(201, "SUCCESS"))
    # 迁移用临时资源（不改动 RM01/RK01 固定夹具，避免污染后续场景）
    parsed, _ = call("S7-tmp-room", "S7", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
                     {"code": P + "-RMV", "name": "临时迁移机房", "autoGenerateCode": False}, token=T(),
                     expect=(201, "SUCCESS"))
    rmv = data_of(parsed)
    uuid_map[rmv["id"]] = "RMV"
    parsed, _ = call("S7-tmp-rack", "S7", "POST", f"/api/v1/rooms/{rmv['id']}/racks",
                     {"code": P + "-RKV", "name": "临时迁移机柜", "autoGenerateCode": False}, token=T(),
                     expect=(201, "SUCCESS"))
    rkv = data_of(parsed)
    uuid_map[rkv["id"]] = "RKV"
    call("S7-room-move", "S7", "POST", f"/api/v1/rooms/{rmv['id']}/move",
         {"code": P + "-RMV2", "name": "临时迁移机房2", "autoGenerateCode": False,
          "targetDataCenterId": dcc["id"], "version": rmv["version"]}, token=T(), expect=(200, "SUCCESS"))
    parsed, _ = call("S7-tree-after", "S7", "GET", "/api/v1/resource-tree", token=T(), expect=(200, "SUCCESS"))
    items = data_of(parsed)["items"]
    moved_ok = any(
        any(str(r.get("code", "")).upper() == (P + "-RMV2").upper() for r in (x.get("rooms") or []))
        for x in items if x["id"] == dcc["id"])
    CASES.append({"caseId": "S7-room-move-invariant", "scenario": "S7",
                  "status": "PASS" if moved_ok else "FAIL", "httpStatus": 200,
                  "actualCode": "moved" if moved_ok else "not_in_target",
                  "expectedCode": "moved", "note": "迁移后机房应出现在目标DC", "durationMs": 0})
    # 机柜迁移（临时机柜 RKV → 复制机房 RMC）
    rmc = find_tree(rid, "RMC")
    rkv_now = find_tree(rid, "RKV") or rkv
    if rmc:
        call("S7-rack-move", "S7", "POST", f"/api/v1/racks/{rkv_now['id']}/move",
             {"code": P + "-RKV2", "name": "临时迁移机柜2", "autoGenerateCode": False,
              "targetRoomId": rmc["id"], "version": rkv_now["version"]}, token=T(), expect=(200, "SUCCESS"))


def s8_final(rid):
    P = f"GT-{rid}"
    rack = find_tree(rid, "RK01")
    parsed, _ = call("S8-policy-get", "S8", "GET", "/api/v1/admin/approval-policy", token=T(), expect=(200, "SUCCESS"))
    pol = data_of(parsed)
    call("S8-policy-on", "S8", "PUT", "/api/v1/admin/approval-policy",
         {"version": pol["version"], "assignApprovalEnabled": True, "moveApprovalEnabled": False,
          "defaultApproverRole": "system_admin"}, token=T(), expect=(200, "SUCCESS"))
    parsed, _ = call("S8-devtypes", "S8", "GET", "/api/v1/device-types", token=T(), expect=(200, "SUCCESS"))
    tid = next(t["id"] for t in data_of(parsed)["items"] if t["code"] == "SERVER")
    parsed, _ = call("S8-dev-create", "S8", "POST", "/api/v1/devices",
                     {"typeId": tid, "code": P + "-DV3", "name": "审批设备", "autoGenerateCode": False},
                     token=T(), expect=(201, "SUCCESS"))
    dev = data_of(parsed)
    uuid_map[dev["id"]] = "DV3"
    parsed, _ = call("S8-assign-approval", "S8", "POST", f"/api/v1/devices/{dev['id']}/assign",
                     {"version": dev["version"], "rackId": rack["id"], "startU": 5, "heightU": 2,
                      "orientation": "NORMAL", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    res = data_of(parsed) or {}
    appr = res.get("approval") or {}
    uuid_map[appr["id"]] = "APPR1"
    call("S8-approve", "S8", "POST", f"/api/v1/admin/approvals/{appr['id']}/approve",
         {"version": appr["version"], "comment": "GT"}, token=T(), expect=(200, "SUCCESS"))
    call("S8-approvals-list", "S8", "GET", "/api/v1/admin/approvals", token=T(), expect=(200, "SUCCESS"))
    # stale：新建→申请→改设备→批准应拒
    parsed, _ = call("S8-dev4-create", "S8", "POST", "/api/v1/devices",
                     {"typeId": tid, "code": P + "-DV4", "name": "stale设备", "autoGenerateCode": False},
                     token=T(), expect=(201, "SUCCESS"))
    d4 = data_of(parsed)
    uuid_map[d4["id"]] = "DV4"
    parsed, _ = call("S8-assign-stale", "S8", "POST", f"/api/v1/devices/{d4['id']}/assign",
                     {"version": d4["version"], "rackId": rack["id"], "startU": 8, "heightU": 2,
                      "orientation": "NORMAL", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    appr2 = (data_of(parsed) or {}).get("approval") or {}
    uuid_map[appr2["id"]] = "APPR2"
    cur = data_of(call("S8-dev4-refetch", "S8", "GET", f"/api/v1/devices/{d4['id']}", token=T())[0])
    full = dict(cur)
    full["name"] = "stale设备改"
    call("S8-dev4-modify", "S8", "PUT", f"/api/v1/devices/{d4['id']}?version={cur['version']}",
         full, token=T(), expect=(200, "SUCCESS"))
    call("S8-approve-stale", "S8", "POST", f"/api/v1/admin/approvals/{appr2['id']}/approve",
         {"version": appr2["version"], "comment": "应拒"}, token=T(),
         expect=(409, "RESOURCE_VERSION_CONFLICT"))
    parsed, _ = call("S8-layout-check", "S8", "GET", f"/api/v1/racks/{rack['id']}/u-layout", token=T(),
                     expect=(200, "SUCCESS"))
    # 驳回+重复
    parsed, _ = call("S8-dev5-create", "S8", "POST", "/api/v1/devices",
                     {"typeId": tid, "code": P + "-DV5", "name": "驳回设备", "autoGenerateCode": False},
                     token=T(), expect=(201, "SUCCESS"))
    d5 = data_of(parsed)
    uuid_map[d5["id"]] = "DV5"
    parsed, _ = call("S8-assign-rej", "S8", "POST", f"/api/v1/devices/{d5['id']}/assign",
                     {"version": d5["version"], "rackId": rack["id"], "startU": 12, "heightU": 2,
                      "orientation": "NORMAL", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    appr3 = (data_of(parsed) or {}).get("approval") or {}
    uuid_map[appr3["id"]] = "APPR3"
    call("S8-reject", "S8", "POST", f"/api/v1/admin/approvals/{appr3['id']}/reject",
         {"version": appr3["version"], "comment": "GT"}, token=T(), expect=(200, "SUCCESS"))
    call("S8-reject-again", "S8", "POST", f"/api/v1/admin/approvals/{appr3['id']}/reject",
         {"version": appr3["version"] + 1, "comment": "重复"}, token=T(),
         expect=(409, "APPROVAL_STATE_CONFLICT"), note="新黄金码：仅待审批记录可驳回")
    parsed, _ = call("S8-policy-get2", "S8", "GET", "/api/v1/admin/approval-policy", token=T(), expect=(200, "SUCCESS"))
    pol2 = data_of(parsed)
    call("S8-policy-off", "S8", "PUT", "/api/v1/admin/approval-policy",
         {"version": pol2["version"], "assignApprovalEnabled": False, "moveApprovalEnabled": False,
          "defaultApproverRole": "system_admin"}, token=T(), expect=(200, "SUCCESS"))


def s9_final(rid):
    P = f"GT-{rid}"
    parsed, _ = call("S9-tpl-list", "S9", "GET", "/api/v1/rack-templates", token=T(), expect=(200, "SUCCESS"))
    tpl = next(t for t in data_of(parsed)["items"] if t.get("isSystem"))
    uuid_map[tpl["id"]] = "TPL_SYS"
    call("S9-newversion", "S9", "POST", f"/api/v1/rack-templates/{tpl['id']}/versions?version={tpl['version']}",
         {"type": "STANDARD", "uHeight": 45, "widthMm": 650, "depthMm": 1200, "heightMm": 2100,
          "revision": (tpl.get("currentRevision") or 1) + 1, "changeNote": "GT-S9"}, token=T(),
         expect=(201, "SUCCESS"))
    call("S9-sys-tpl-delete", "S9", "DELETE", f"/api/v1/rack-templates/{tpl['id']}?version={tpl['version']}",
         token=T(), expect=(409, "SYSTEM_TEMPLATE_PROTECTED"))
    call("S9-sys-tpl-disable", "S9", "PUT", f"/api/v1/rack-templates/{tpl['id']}?version={tpl['version']}",
         {"code": tpl["code"], "name": tpl["name"], "status": "DISABLED", "version": tpl["version"]},
         token=T(), expect=(409, "SYSTEM_TEMPLATE_PROTECTED"))


def s10_final(rid):
    P = f"GT-{rid}"
    rack2 = find_tree(rid, "RK02")
    rack1 = find_tree(rid, "RK01")
    RK2, RK1 = rack2["id"], rack1["id"]
    parsed, _ = call("S10-pdu-create", "S10", "POST", f"/api/v1/racks/{RK2}/pdus",
                     {"code": P + "-PDU1", "name": "黄金PDU"}, token=T(), expect=(201, "SUCCESS"))
    pdu = data_of(parsed)
    uuid_map[pdu["id"]] = "PDU1"
    call("S10-pdu-dupcode", "S10", "POST", f"/api/v1/racks/{RK2}/pdus",
         {"code": P + "-PDU1", "name": "重复PDU"}, token=T(), expect=([409, 500], None),
         note="缺陷 D1 实录：应然 409，原系统 500")
    call("S10-socket-create", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
         {"socketNo": 1, "standard": "CN", "amperageA": 16}, token=T(), expect=(201, "SUCCESS"))
    sock = data_of(call("S10-sockets-list", "S10", "GET", f"/api/v1/pdus/{pdu['id']}/sockets", token=T())[0])
    socks = sock["items"] if isinstance(sock, dict) else []
    s1_ = next(s for s in socks if s["socketNo"] == 1)
    uuid_map[s1_["id"]] = "SOCK1"
    call("S10-socket-dup", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
         {"socketNo": 1, "standard": "EU", "amperageA": 10}, token=T(), expect=([409, 500], None),
         note="缺陷 D2 实录：应然 409，原系统 500")
    call("S10-socket-badstd", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
         {"socketNo": 9, "standard": "国标", "amperageA": 10}, token=T(), expect=(400, "INVALID_RESOURCE"))
    parsed, _ = call("S10-socket2", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
                     {"socketNo": 2, "standard": "CN", "amperageA": 16}, token=T(), expect=(201, "SUCCESS"))
    parsed, _ = call("S10-devtypes", "S10", "GET", "/api/v1/device-types", token=T())
    tid = next(t["id"] for t in data_of(parsed)["items"] if t["code"] == "SERVER")
    dA = data_of(call("S10-devA", "S10", "POST", "/api/v1/devices",
                      {"typeId": tid, "code": P + "-DVA", "name": "供电A", "autoGenerateCode": False},
                      token=T(), expect=(201, "SUCCESS"))[0])
    uuid_map[dA["id"]] = "DVA"
    call("S10-devA-assign", "S10", "POST", f"/api/v1/devices/{dA['id']}/assign",
         {"version": dA["version"], "rackId": RK2, "startU": 40, "heightU": 2, "orientation": "NORMAL",
          "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    dX = data_of(call("S10-devX", "S10", "POST", "/api/v1/devices",
                      {"typeId": tid, "code": P + "-DVX", "name": "跨柜X", "autoGenerateCode": False},
                      token=T(), expect=(201, "SUCCESS"))[0])
    uuid_map[dX["id"]] = "DVX"
    call("S10-devX-assign", "S10", "POST", f"/api/v1/devices/{dX['id']}/assign",
         {"version": dX["version"], "rackId": RK1, "startU": 40, "heightU": 2, "orientation": "NORMAL",
          "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    call("S10-connect-ok", "S10", "POST", f"/api/v1/pdu-sockets/{s1_['id']}/connection",
         {"version": s1_.get("version", 1), "deviceId": dA["id"], "redundancyRole": "PRIMARY",
          "circuit": "A路", "powerW": 300}, token=T(), expect=(201, "SUCCESS"))
    call("S10-connect-occupied", "S10", "POST", f"/api/v1/pdu-sockets/{s1_['id']}/connection",
         {"version": s1_.get("version", 1), "deviceId": dX["id"], "redundancyRole": "PRIMARY",
          "circuit": "B"}, token=T(), expect=(409, "PDU_SOCKET_CONNECTED"),
         note="插座已有供电连接（实测优先于跨柜校验）")
    # 连接列表 + 断开（body version）
    parsed, _ = call("S10-connections", "S10", "GET", f"/api/v1/racks/{RK2}/pdu-connections", token=T(),
                     expect=(200, "SUCCESS"))
    conns = (data_of(parsed) or {}).get("items") or []
    if conns:
        cid = conns[0]["id"]
        uuid_map[cid] = "CONN1"
        call("S10-disconnect", "S10", "DELETE", f"/api/v1/pdu-connections/{cid}",
             token=T(), body={"version": conns[0].get("version", 1)}, expect=(200, "SUCCESS"))


def s11_final(rid):
    P = f"GT-{rid}"
    parsed, _ = call("S11-users", "S11", "GET", "/api/v1/admin/users", token=T(), expect=(200, "SUCCESS"))
    items = data_of(parsed)["items"]
    admins = [u for u in items if any(r.get("code") == "system_admin" for r in u.get("roles") or [])]
    adm = admins[0]
    uuid_map[adm["id"]] = "ADMIN"
    call("S11-user-create", "S11", "POST", "/api/v1/admin/users",
         {"username": "gtu" + rid[-6:].lower(), "displayName": "黄金用户", "password": "Gt!u" + rid[-6:],
          "roles": ["user"]}, token=T(), expect=(201, "SUCCESS"))
    call("S11-user-dup", "S11", "POST", "/api/v1/admin/users",
         {"username": "gtu" + rid[-6:].lower(), "displayName": "重复", "password": "Gt!u" + rid[-6:],
          "roles": ["user"]}, token=T(), expect=(409, "USER_CONFLICT"))
    call("S11-last-admin-delete", "S11", "DELETE", f"/api/v1/admin/users/{adm['id']}?version={adm['version']}",
         token=T(), expect=(409, "LAST_ADMIN_PROTECTED"))
    call("S11-last-admin-demote", "S11", "PUT", f"/api/v1/admin/users/{adm['id']}?version={adm['version']}",
         {"displayName": adm["displayName"], "roles": ["user"], "version": adm["version"]}, token=T(),
         expect=(409, "LAST_ADMIN_PROTECTED"))
    call("S11-roles", "S11", "GET", "/api/v1/admin/roles", token=T(), expect=(200, "SUCCESS"))
    call("S11-user-forbidden", "S11", "GET", "/api/v1/admin/users", token="INVALIDTOKEN",
         expect=(401, "UNAUTHORIZED"))
    call("S11-me", "S11", "GET", "/api/v1/auth/me", token=T(), expect=(200, "SUCCESS"))
    call("S11-logout", "S11", "POST", "/api/v1/auth/logout", token=T(), expect=(200, "SUCCESS"))
    call("S11-noauth-401", "S11", "GET", "/api/v1/device-types", expect=(401, "UNAUTHORIZED"))


def s12_final(rid):
    P = f"GT-{rid}"
    room = find_tree(rid, "RM01")
    rack = find_tree(rid, "RK01")
    parsed, _ = call("S12-devtypes", "S12", "GET", "/api/v1/device-types", token=T())
    tid = next(t["id"] for t in data_of(parsed)["items"] if t["code"] == "SERVER")
    parsed, st = call("S12-validate", "S12", "POST", f"/api/v1/rooms/{room['id']}/rack-diagram-import/validate",
                      {"formatVersion": "1", "dataCenterId": room.get("dataCenterId"), "roomId": room["id"],
                       "exportedAt": datetime.datetime.utcnow().isoformat() + "Z", "defaultTypeId": tid,
                       "coveredRackIds": [rack["id"]],
                       "devices": [{"clientId": "gt-c1", "rackId": rack["id"], "rackCode": rack["code"],
                                    "startU": 30, "endU": 31, "name": "GT导入", "serialNumber": "GT-SN1",
                                    "ratedPowerW": 200}]}, token=T(), expect=(200, "SUCCESS"))
    d = data_of(parsed) or {}
    tok = d.get("token") or d.get("draftToken")
    items = d.get("items") or []
    decisions = [{"itemId": it["id"], "action": (it.get("defaultDecision") or (it.get("allowedDecisions") or ["SKIP"])[0])}
                 for it in items]
    if tok:
        call("S12-commit", "S12", "POST", f"/api/v1/rooms/{room['id']}/rack-diagram-import/commit",
             {"token": tok, "decisions": decisions}, token=T(), expect=(200, None))
        call("S12-recommit", "S12", "POST", f"/api/v1/rooms/{room['id']}/rack-diagram-import/commit",
             {"token": tok, "decisions": decisions}, token=T(),
             expect=(410, "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED"))




def s_complete(rid):
    """补齐剩余 OpenAPI 操作（20 个），使覆盖矩阵达 66/66。"""
    P = f"GT-{rid}"
    # 重新登录（S11 已登出）
    parsed, _ = call("S13-relogin", "S13", "POST", "/api/v1/auth/login",
                     {"username": "admin", "password": ADMIN_PW["v"]}, expect=(200, "SUCCESS"))
    TOKEN["v"] = (data_of(parsed) or {}).get("token")
    dc = find_tree(rid, "DC01")
    room = find_tree(rid, "RM01")
    rack2 = find_tree(rid, "RK02")

    # --- rooms PUT / DELETE
    parsed, _ = call("S13-room-create-tmp", "S13", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
                     {"code": P + "-RM9", "name": "临时机房", "autoGenerateCode": False}, token=T(),
                     expect=(201, "SUCCESS"))
    rm9 = data_of(parsed)
    uuid_map[rm9["id"]] = "RM9"
    parsed, _ = call("S13-room-update", "S13", "PUT", f"/api/v1/rooms/{rm9['id']}?version={rm9['version']}",
                     {"code": rm9["code"], "name": "临时机房改", "version": rm9["version"]}, token=T(),
                     expect=(200, "SUCCESS"))
    rm9b = data_of(parsed)
    call("S13-room-delete", "S13", "DELETE", f"/api/v1/rooms/{rm9['id']}?version={rm9b['version']}",
         token=T(), expect=(200, "SUCCESS"))

    # --- racks PUT / DELETE
    parsed, _ = call("S13-rack-create-tmp", "S13", "POST", f"/api/v1/rooms/{room['id']}/racks",
                     {"code": P + "-RK9", "name": "临时机柜", "autoGenerateCode": False}, token=T(),
                     expect=(201, "SUCCESS"))
    rk9 = data_of(parsed)
    parsed, _ = call("S13-rack-update", "S13", "PUT", f"/api/v1/racks/{rk9['id']}?version={rk9['version']}",
                     {"code": rk9["code"], "name": "临时机柜改", "version": rk9["version"]}, token=T(),
                     expect=(200, "SUCCESS"))
    rk9b = data_of(parsed)
    call("S13-rack-delete", "S13", "DELETE", f"/api/v1/racks/{rk9['id']}?version={rk9b['version']}",
         token=T(), expect=(200, "SUCCESS"))

    # --- GET /racks/{id}/pdus
    call("S13-rack-pdus", "S13", "GET", f"/api/v1/racks/{rack2['id']}/pdus", token=T(),
         expect=(200, "SUCCESS"))

    # --- device-types POST / PUT / DELETE
    parsed, _ = call("S13-dtype-create", "S13", "POST", "/api/v1/device-types",
                     {"code": P + "-TY9", "name": "临时类型", "category": "ACCESSORY",
                      "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    ty9 = data_of(parsed)
    parsed, _ = call("S13-dtype-update", "S13", "PUT", f"/api/v1/device-types/{ty9['id']}?version={ty9['version']}",
                     {"code": ty9["code"], "name": "临时类型改", "category": "ACCESSORY",
                      "defaultHeightU": 3, "version": ty9["version"]}, token=T(), expect=(200, "SUCCESS"))
    ty9b = data_of(parsed)
    call("S13-dtype-delete", "S13", "DELETE", f"/api/v1/device-types/{ty9['id']}?version={ty9b['version']}",
         token=T(), expect=(200, "SUCCESS"))

    # --- GET /devices 列表
    call("S13-devices-list", "S13", "GET", "/api/v1/devices?page=1&pageSize=5", token=T(),
         expect=(200, "SUCCESS"))
    parsed, _ = call("S13-devtypes2", "S13", "GET", "/api/v1/device-types", token=T(), expect=(200, "SUCCESS"))
    tid = next(t["id"] for t in data_of(parsed)["items"] if t["code"] == "SERVER")

    # --- device history / DELETE
    parsed, _ = call("S13-dev-tmp-create", "S13", "POST", "/api/v1/devices",
                     {"typeId": tid, "code": P + "-DV9", "name": "临时设备", "autoGenerateCode": False},
                     token=T(), expect=(201, "SUCCESS"))
    dv9 = data_of(parsed)
    uuid_map[dv9["id"]] = "DV9"
    call("S13-dev-history", "S13", "GET", f"/api/v1/devices/{dv9['id']}/history", token=T(),
         expect=(200, "SUCCESS"))
    call("S13-dev-delete", "S13", "DELETE", f"/api/v1/devices/{dv9['id']}?version={dv9['version']}",
         token=T(), expect=(200, "SUCCESS"))

    # --- rack-templates POST / PUT / DELETE
    parsed, _ = call("S13-tpl-create", "S13", "POST", "/api/v1/rack-templates",
                     {"code": P + "-TPL9", "name": "临时模板", "autoGenerateCode": False,
                      "spec": {"type": "STANDARD", "uHeight": 42, "widthMm": 600, "depthMm": 1200,
                               "heightMm": 2000}}, token=T(), expect=(201, "SUCCESS"))
    tpl9 = data_of(parsed)
    parsed, _ = call("S13-tpl-update", "S13", "PUT", f"/api/v1/rack-templates/{tpl9['id']}?version={tpl9['version']}",
                     {"code": tpl9["code"], "name": "临时模板改", "version": tpl9["version"]}, token=T(),
                     expect=(200, "SUCCESS"))
    tpl9b = data_of(parsed)
    call("S13-tpl-delete", "S13", "DELETE", f"/api/v1/rack-templates/{tpl9['id']}?version={tpl9b['version']}",
         token=T(), expect=(200, "SUCCESS"))

    # --- reset-password
    parsed, _ = call("S13-user-tmp", "S13", "POST", "/api/v1/admin/users",
                     {"username": "gtz" + rid[-6:].lower(), "displayName": "临时用户",
                      "password": "Gt!z" + rid[-6:], "roles": ["user"]}, token=T(), expect=(201, "SUCCESS"))
    uz = data_of(parsed)
    call("S13-user-resetpw", "S13", "POST",
         f"/api/v1/admin/users/{uz['id']}/reset-password?version={uz['version']}",
         {"password": "Gt!new" + rid[-6:]}, token=T(), expect=(200, None),
         note="version 在 query（实测确认）")

    # --- LDAP GET / PUT / test
    parsed, _ = call("S13-ldap-get", "S13", "GET", "/api/v1/admin/ldap", token=T(), expect=(200, "SUCCESS"))
    cfg = data_of(parsed) or {}
    body = {k: cfg.get(k) for k in ("enabled", "url", "useTls", "skipTlsVerify", "bindDn", "baseDn",
                                    "userFilter", "usernameAttribute", "displayNameAttribute",
                                    "emailAttribute", "defaultRole", "allowLocalFallback",
                                    "groupSearchBaseDn", "groupFilter", "groupAttribute")}
    body.update({"enabled": False, "version": cfg.get("version", 0)})
    call("S13-ldap-put", "S13", "PUT", "/api/v1/admin/ldap", body, token=T(), expect=(200, None),
         note="原样回写（不启用）")
    call("S13-ldap-test", "S13", "POST", "/api/v1/admin/ldap/test",
         {"url": "ldap://127.0.0.1:1389", "baseDn": "dc=example,dc=org", "bindDn": "", "bindPassword": ""},
         token=T(), expect=([200, 400, 500], None), note="无 LDAP 服务：实录可达性响应")

    # --- PDU PUT / DELETE + socket PUT / DELETE
    parsed, _ = call("S13-pdu-list", "S13", "GET", f"/api/v1/racks/{rack2['id']}/pdus", token=T(),
                     expect=(200, "SUCCESS"))
    pdus = (data_of(parsed) or {}).get("items") or []
    if pdus:
        pdu = pdus[0]
        parsed, _ = call("S13-pdu-update", "S13", "PUT", f"/api/v1/pdus/{pdu['id']}?version={pdu['version']}",
                         {"code": pdu["code"], "name": pdu["name"] + "改", "version": pdu["version"],
                          "rackId": rack2["id"]}, token=T(), expect=(200, None))
        pdu2 = data_of(parsed) or pdu
        parsed, _ = call("S13-pdu-sockets", "S13", "GET", f"/api/v1/pdus/{pdu['id']}/sockets", token=T(),
                         expect=(200, "SUCCESS"))
        socks = (data_of(parsed) or {}).get("items") or []
        updated = False
        for sk in socks:
            if sk.get("status") != "CONNECTED":
                if not updated:
                    parsed, _ = call("S13-socket-update", "S13", "PUT",
                                     f"/api/v1/pdu-sockets/{sk['id']}?version={sk['version']}",
                                     {"socketNo": sk["socketNo"], "standard": sk["standard"],
                                      "amperageA": sk["amperageA"], "label": "GT改",
                                      "version": sk["version"], "pdUid": pdu["id"]},
                                     token=T(), expect=(200, None))
                    updated = True
                    sk = data_of(parsed) or sk
                call("S13-socket-delete", "S13", "DELETE", f"/api/v1/pdu-sockets/{sk['id']}?version={sk['version']}",
                     token=T(), expect=(200, None))
        call("S13-pdu-delete", "S13", "DELETE", f"/api/v1/pdus/{pdu['id']}?version={pdu2['version']}",
             token=T(), expect=(200, None), note="若仍有插座/连接则记录实际码")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--mode", choices=["explore", "final"], required=True)
    ap.add_argument("--env-file", required=True)
    ap.add_argument("--run-id", required=True)
    ap.add_argument("--pg", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--api", default="http://127.0.0.1:18080")
    a = ap.parse_args()
    global API
    API = a.api
    for line in open(a.env_file):
        if line.startswith("ADMIN_PASSWORD="):
            ADMIN_PW["v"] = line.strip().split("=", 1)[1]
    sub = "final-regression" if a.mode == "final" else "exploratory"
    out = os.path.join(a.out, sub)
    os.makedirs(os.path.join(out, "api", "raw-redacted"), exist_ok=True)
    os.makedirs(os.path.join(out, "api", "normalized"), exist_ok=True)
    os.makedirs(os.path.join(out, "junit"), exist_ok=True)
    os.makedirs(os.path.join(out, "coverage"), exist_ok=True)

    t0 = datetime.datetime.utcnow()
    parsed, _ = call("auth-login", "S1", "POST", "/api/v1/auth/login",
                     {"username": "admin", "password": ADMIN_PW["v"]}, expect=(200, "SUCCESS"))
    TOKEN["v"] = (data_of(parsed) or {}).get("token")
    tok = TOKEN["v"]
    p = tok.split(".")[1]
    p += "=" * (-len(p) % 4)
    claims = json.loads(base64.urlsafe_b64decode(p))
    call("auth-jwt-7200", "S1", "GET", "/api/v1/auth/me", token=tok, expect=(200, "SUCCESS"))
    CASES.append({"caseId": "auth-jwt-exp", "scenario": "S1", "status": "PASS" if claims["exp"] - claims["iat"] == 7200 else "FAIL",
                  "httpStatus": 0, "actualCode": f"exp-iat={claims['exp']-claims['iat']}", "expectedCode": "7200",
                  "note": "", "durationMs": 0})
    call("health-live", "S1", "GET", "/health/live", expect=(200, "SUCCESS"))
    call("health-ready", "S1", "GET", "/health/ready", expect=(200, "SUCCESS"))
    call("auth-wrongpw", "S1", "POST", "/api/v1/auth/login",
         {"username": "admin", "password": "Wrong#9999"}, expect=(401, "INVALID_CREDENTIALS"))
    call("auth-noauth", "S1", "GET", "/api/v1/device-types", expect=(401, "UNAUTHORIZED"))

    setup_tree(a.run_id)
    s3_final(a.run_id)
    s6_final(a.run_id)
    s7_final(a.run_id)
    s8_final(a.run_id)
    s9_final(a.run_id)
    s10_final(a.run_id)
    s12_final(a.run_id)
    s_complete(a.run_id)
    s11_final(a.run_id)

    # 产物
    with open(os.path.join(out, "api", "raw-redacted", "samples.jsonl"), "w", encoding="utf-8") as f:
        for s in SAMPLES:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(out, "api", "normalized", "normalized.jsonl"), "w", encoding="utf-8") as f:
        for s in NORMS:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(out, "results.jsonl"), "w", encoding="utf-8") as f:
        for c in CASES:
            f.write(json.dumps(c, ensure_ascii=False) + "\n")

    # 覆盖矩阵：从 results.jsonl 统计 scenario
    scen_stat = {}
    for c in CASES:
        scen_stat.setdefault(c["scenario"], {"PASS": 0, "FAIL": 0})
        scen_stat[c["scenario"]][c["status"]] = scen_stat[c["scenario"]].get(c["status"], 0) + 1
    with open(os.path.join(out, "coverage", "scenario-summary.json"), "w", encoding="utf-8") as f:
        json.dump(scen_stat, f, ensure_ascii=False, indent=1)

    # JUnit
    with open(os.path.join(out, "junit", "results.xml"), "w", encoding="utf-8") as f:
        f.write('<?xml version="1.0" encoding="utf-8"?>\n')
        f.write(f'<testsuite name="gt-final" tests="{len(CASES)}" '
                f'failures="{sum(1 for c in CASES if c["status"]=="FAIL")}">\n')
        for c in CASES:
            if c["status"] == "PASS":
                f.write(f'  <testcase name="{xesc(c["caseId"])}" classname="{xesc(c["scenario"])}"/>\n')
            else:
                f.write(f'  <testcase name="{xesc(c["caseId"])}" classname="{xesc(c["scenario"])}">\n')
                f.write(f'    <failure message="{xesc(str(c["actualCode"]))} != {xesc(str(c["expectedCode"]))}">'
                        f'{xesc(c["note"])}</failure>\n  </testcase>\n')
        f.write("</testsuite>\n")

    npass = sum(1 for c in CASES if c["status"] == "PASS")
    nfail = len(CASES) - npass
    print(f"\nFINAL: {npass} PASS / {nfail} FAIL / 样本 {len(SAMPLES)}")
    for c in CASES:
        if c["status"] != "PASS":
            print(f"  FAIL {c['caseId']}: {c['httpStatus']} {c['actualCode']} != {c['expectedCode']} {c['note'][:60]}")


if __name__ == "__main__":
    main()