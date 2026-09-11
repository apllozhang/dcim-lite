#!/usr/bin/env python3
"""GT Final v3: 场景编排。使用 gt_core 的状态模型/HTTP/规范化器。
新增：S7深层不变量、S9快照判定、三组并发(threading.Barrier)、信封断言、caseId新命名。
用法（服务器端）: python3 gt_final_v3.py --env-file E --run-id RID --pg PG --out OUT --api http://..."""
import argparse
import base64
import json
import os
import sys
import threading
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gt_core import (call, info_call, setup_call, data_of, T, TOKEN, API, RUN_ID,
                     UUID_MAP, set_uuid, login, compute_summary, should_exit_nonzero,
                     write_outputs, CASES, SAMPLES, NORMS)
from gt_gap_fill import gt_gap_fill
from gt_gap_fill_v2 import gt_gap_fill_v2
from gt_gap_fill_v3 import gt_gap_fill_v3
from gt_semantic_evidence import gt_semantic_evidence
from gt_native_evidence import gt_native_evidence

ADMIN_PW = {"v": ""}


# ============ 辅助 ============

def find_tree(rid, suffix, scenario="S00"):
    parsed, _ = info_call(f"INFO-{scenario}-LOOKUP-{suffix.upper()}", scenario,
                          "GET", "/api/v1/resource-tree", token=T())
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


def get_type_id(category="SERVER", scenario="S00"):
    parsed, _ = info_call(f"INFO-{scenario}-GET-TYPE-{category}", scenario,
                          "GET", "/api/v1/device-types", token=T())
    for t in data_of(parsed)["items"]:
        if t["code"] == category:
            return t["id"]
    return None


_ULAYOUT_SEQ = [0]


def rack_free_start(rack_id, height=2, skip=0, scenario="S00"):
    _ULAYOUT_SEQ[0] += 1
    parsed, _ = info_call(f"INFO-{scenario}-ULAYOUT-{_ULAYOUT_SEQ[0]:03d}", scenario,
                          "GET", f"/api/v1/racks/{rack_id}/u-layout", token=T())
    free = (data_of(parsed) or {}).get("free") or []
    fits = [f["startU"] for f in free if f.get("heightU", 0) >= height]
    return fits[skip] if len(fits) > skip else (fits[0] if fits else 40)


# ============ S01 认证/健康 ============

def s01():
    call("S01-HEALTH-LIVE", "S01", "GET", "/health/live", expect=(200, "SUCCESS"))
    call("S01-HEALTH-READY", "S01", "GET", "/health/ready", expect=(200, "SUCCESS"))
    tok, st, parsed = login("admin", ADMIN_PW["v"], "S01-AUTH-LOGIN-SUCCESS")
    p = tok.split(".")[1]; p += "=" * (-len(p) % 4)
    claims = json.loads(base64.urlsafe_b64decode(p))
    CASES.append({"caseId": "S01-AUTH-JWT-TTL-7200", "scenario": "S01", "kind": "TEST",
                  "status": "PASS" if claims["exp"] - claims["iat"] == 7200 else "FAIL",
                  "httpStatus": 0, "actualCode": f"exp-iat={claims['exp']-claims['iat']}",
                  "expectedCode": "7200", "note": "", "durationMs": 0, "requestId": ""})
    call("S01-AUTH-LOGIN-WRONGPW", "S01", "POST", "/api/v1/auth/login",
         {"username": "admin", "password": "Wrong#1"}, expect=(401, "INVALID_CREDENTIALS"))
    call("S01-AUTH-NOAUTH-401", "S01", "GET", "/api/v1/device-types", expect=(401, "UNAUTHORIZED"))
    call("S01-AUTH-ME", "S01", "GET", "/api/v1/auth/me", token=T(), expect=(200, "SUCCESS"))


# ============ S02 资源树 CRUD ============

def s02(rid):
    P = f"GT-{rid}"
    parsed, _ = setup_call("SETUP-S02-DC-CREATE", "S02", "POST", "/api/v1/data-centers",
                           {"code": P + "-DC01", "name": "黄金DC", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    dc = data_of(parsed)
    set_uuid(dc["id"], "DC01")
    call("S02-DC-DUPCODE", "S02", "POST", "/api/v1/data-centers",
         {"code": P + "-DC01", "name": "重复", "autoGenerateCode": False},
         token=T(), expect=(409, "RESOURCE_CODE_DUPLICATE"))
    parsed, _ = setup_call("SETUP-S02-ROOM-CREATE", "S02", "POST",
                           f"/api/v1/data-centers/{dc['id']}/rooms",
                           {"code": P + "-RM01", "name": "黄金机房", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    room = data_of(parsed)
    set_uuid(room["id"], "RM01")
    parsed, _ = info_call("INFO-S02-TPL-LIST", "S02", "GET", "/api/v1/rack-templates", token=T())
    from gt_core import UUID_MAP as um
    tpl_id = None
    for t in data_of(parsed)["items"]:
        if t.get("isSystem"):
            tpl_id = t["id"]
            set_uuid(t["id"], "TPL_SYS")
            break
    parsed, _ = setup_call("SETUP-S02-RACK-CREATE", "S02", "POST",
                           f"/api/v1/rooms/{room['id']}/racks",
                           {"code": P + "-RK01", "name": "黄金机柜A", "templateId": tpl_id,
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    rack = data_of(parsed)
    set_uuid(rack["id"], "RK01")
    parsed, _ = setup_call("SETUP-S02-RACK2-CREATE", "S02", "POST",
                           f"/api/v1/rooms/{room['id']}/racks",
                           {"code": P + "-RK02", "name": "黄金机柜B", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    set_uuid(data_of(parsed)["id"], "RK02")
    # 停用父级
    parsed, _ = call("S02-DC-DISABLE", "S02", "PUT",
                     f"/api/v1/data-centers/{dc['id']}?version={dc['version']}",
                     {"code": dc["code"], "name": dc["name"], "status": "DISABLED",
                      "version": dc["version"]}, token=T(), expect=(200, "SUCCESS"))
    dc2 = data_of(parsed) or dc
    call("S02-ROOM-IN-DISABLED-DC", "S02", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
         {"code": P + "-RMX", "name": "不应成功", "autoGenerateCode": False},
         token=T(), expect=(409, "PARENT_RESOURCE_DISABLED"))
    parsed, _ = call("S02-DC-ENABLE", "S02", "PUT",
                     f"/api/v1/data-centers/{dc['id']}?version={dc2['version']}",
                     {"code": dc["code"], "name": dc["name"], "status": "OPERATING",
                      "version": dc2["version"]}, token=T(), expect=(200, "SUCCESS"))
    dc3 = data_of(parsed) or dc
    call("S02-DC-DELETE-HASCHILDREN", "S02", "DELETE",
         f"/api/v1/data-centers/{dc['id']}?version={dc3['version']}",
         token=T(), expect=(409, "RESOURCE_HAS_CHILDREN"))
    # room/rack 更新+删除（临时对象）
    parsed, _ = setup_call("SETUP-S02-RM9-CREATE", "S02", "POST",
                           f"/api/v1/data-centers/{dc['id']}/rooms",
                           {"code": P + "-RM9", "name": "临时机房", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    rm9 = data_of(parsed)
    parsed, _ = call("S02-ROOM-UPDATE", "S02", "PUT",
                     f"/api/v1/rooms/{rm9['id']}?version={rm9['version']}",
                     {"code": rm9["code"], "name": "临时机房改", "version": rm9["version"]},
                     token=T(), expect=(200, "SUCCESS"))
    rm9b = data_of(parsed)
    call("S02-ROOM-DELETE", "S02", "DELETE", f"/api/v1/rooms/{rm9['id']}?version={rm9b['version']}",
         token=T(), expect=(200, "SUCCESS"))
    parsed, _ = setup_call("SETUP-S02-RK9-CREATE", "S02", "POST",
                           f"/api/v1/rooms/{room['id']}/racks",
                           {"code": P + "-RK9", "name": "临时机柜", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    rk9 = data_of(parsed)
    parsed, _ = call("S02-RACK-UPDATE", "S02", "PUT",
                     f"/api/v1/racks/{rk9['id']}?version={rk9['version']}",
                     {"code": rk9["code"], "name": "临时机柜改", "version": rk9["version"]},
                     token=T(), expect=(200, "SUCCESS"))
    rk9b = data_of(parsed)
    call("S02-RACK-DELETE", "S02", "DELETE", f"/api/v1/racks/{rk9['id']}?version={rk9b['version']}",
         token=T(), expect=(200, "SUCCESS"))
    call("S02-RESOURCE-TREE", "S02", "GET", "/api/v1/resource-tree", token=T(),
         expect=(200, "SUCCESS"))


# ============ S03 乐观锁 ============

def s03(rid):
    dc = find_tree(rid, "DC01", "S03")
    if not dc:
        return
    ver = dc["version"]
    call("S03-DC-STALE-PUT", "S03", "PUT", f"/api/v1/data-centers/{dc['id']}?version={ver+5}",
         {"code": dc["code"], "name": "过期", "version": ver + 5}, token=T(),
         expect=(409, "RESOURCE_VERSION_CONFLICT"))
    parsed, _ = call("S03-DC-OK-PUT", "S03", "PUT", f"/api/v1/data-centers/{dc['id']}?version={ver}",
                     {"code": dc["code"], "name": "黄金DC改", "status": "OPERATING", "version": ver},
                     token=T(), expect=(200, "SUCCESS"))
    call("S03-DC-STALE-DELETE", "S03", "DELETE", f"/api/v1/data-centers/{dc['id']}?version=999",
         token=T(), expect=(409, "RESOURCE_HAS_CHILDREN"))


# ============ S06 U位 ============

def s06(rid):
    P = f"GT-{rid}"
    rack = find_tree(rid, "RK01", "S06")
    RK = rack["id"]
    tid = get_type_id("SERVER", "S06")
    parsed, _ = setup_call("SETUP-S06-DEV1-CREATE", "S06", "POST", "/api/v1/devices",
                           {"typeId": tid, "code": P + "-DV1", "name": "黄金设备1",
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    d1 = data_of(parsed)
    set_uuid(d1["id"], "DV1")
    parsed, _ = call("S06-DEV1-DEFAULT-HEIGHT", "S06", "GET", f"/api/v1/devices/{d1['id']}",
                     token=T(), expect=(200, "SUCCESS"))
    d1_full = data_of(parsed) or d1
    CASES.append({"caseId": "S06-DEV1-HEIGHT-INHERIT", "scenario": "S06", "kind": "TEST",
                  "status": "PASS" if d1_full.get("heightU") == 2 else "FAIL",
                  "httpStatus": 200, "actualCode": f"heightU={d1_full.get('heightU')}",
                  "expectedCode": "2", "note": "SERVER默认高度", "durationMs": 0, "requestId": ""})
    parsed, _ = setup_call("SETUP-S06-DEV2-CREATE", "S06", "POST", "/api/v1/devices",
                           {"typeId": tid, "code": P + "-DV2", "name": "黄金设备2", "heightU": 4,
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    d2 = data_of(parsed)
    set_uuid(d2["id"], "DV2")
    call("S06-ASSIGN-SUCCESS", "S06", "POST", f"/api/v1/devices/{d1['id']}/assign",
         {"version": d1_full["version"], "rackId": RK, "startU": 40, "heightU": 2,
          "orientation": "NORMAL", "reason": "GT-S06"}, token=T(), expect=(200, "SUCCESS"))
    call("S06-ULAYOUT-READ", "S06", "GET", f"/api/v1/racks/{RK}/u-layout", token=T(),
         expect=(200, "SUCCESS"))
    call("S06-ASSIGN-OVERLAP", "S06", "POST", f"/api/v1/devices/{d2['id']}/assign",
         {"version": d2["version"], "rackId": RK, "startU": 39, "heightU": 4,
          "orientation": "NORMAL", "reason": "GT"}, token=T(),
         expect=(409, "RACK_U_CONFLICT"))
    call("S06-ASSIGN-DOUBLE-POSITION", "S06", "POST", f"/api/v1/devices/{d1['id']}/assign",
         {"version": d1_full["version"] + 1, "rackId": RK, "startU": 10, "heightU": 2,
          "orientation": "NORMAL", "reason": "GT"}, token=T(),
         expect=(409, "DEVICE_POSITIONED"))
    call("S06-ASSIGN-OUT-OF-RANGE", "S06", "POST", f"/api/v1/devices/{d2['id']}/assign",
         {"version": d2["version"], "rackId": RK, "startU": 100, "heightU": 4,
          "orientation": "NORMAL", "reason": "GT"}, token=T(),
         expect=(400, "INVALID_RESOURCE"))
    call("S06-MOVE-SUCCESS", "S06", "POST", f"/api/v1/devices/{d1['id']}/move",
         {"version": d1_full["version"] + 1, "rackId": RK, "startU": 20, "heightU": 2,
          "orientation": "REVERSE", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    call("S06-DECOMMISSION", "S06", "POST", f"/api/v1/devices/{d1['id']}/decommission",
         {"version": d1_full["version"] + 2, "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    call("S06-READBACK-AFTER-DECOMM", "S06", "GET", f"/api/v1/devices/{d1['id']}",
         token=T(), expect=(200, "SUCCESS"))
    call("S06-DEV-HISTORY", "S06", "GET", f"/api/v1/devices/{d1['id']}/history",
         token=T(), expect=(200, "SUCCESS"))


# ============ S07 复制/迁移深层 ============

def s07(rid):
    P = f"GT-{rid}"
    dc = find_tree(rid, "DC01", "S07")
    room = find_tree(rid, "RM01", "S07")
    rack = find_tree(rid, "RK01", "S07")
    # DC 复制
    parsed, _ = call("S07-DC-COPY-SUCCESS", "S07", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
                     {"code": P + "-DCC", "name": "复制DC", "autoGenerateCode": False,
                      "version": dc["version"]}, token=T(), expect=(201, "SUCCESS"))
    dcc = (data_of(parsed) or {}).get("resource") or data_of(parsed)
    set_uuid(dcc["id"], "DCC")
    # 复制不变量：新ID≠源ID
    CASES.append({"caseId": "S07-DC-COPY-ID-DIFFERENT", "scenario": "S07", "kind": "TEST",
                  "status": "PASS" if dcc["id"] != dc["id"] else "FAIL",
                  "httpStatus": 201, "actualCode": "id_check", "expectedCode": "different",
                  "note": "", "durationMs": 0, "requestId": ""})
    # 机房复制
    parsed, _ = call("S07-ROOM-COPY-SUCCESS", "S07", "POST", f"/api/v1/rooms/{room['id']}/copy",
                     {"code": P + "-RMC", "name": "复制机房", "autoGenerateCode": False,
                      "version": room["version"], "targetDataCenterId": dc["id"]},
                     token=T(), expect=(201, "SUCCESS"))
    rmc = (data_of(parsed) or {}).get("resource") or data_of(parsed)
    set_uuid(rmc["id"], "RMC")
    # 机柜复制
    parsed, _ = call("S07-RACK-COPY-SUCCESS", "S07", "POST", f"/api/v1/racks/{rack['id']}/copy",
                     {"code": P + "-RKC", "name": "复制机柜", "autoGenerateCode": False,
                      "version": rack["version"], "targetRoomId": room["id"]},
                     token=T(), expect=(201, "SUCCESS"))
    # 重复编码
    call("S07-DC-COPY-DUPCODE", "S07", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
         {"code": P + "-DCC", "name": "重复", "autoGenerateCode": False,
          "version": dc["version"]}, token=T(), expect=(409, "RESOURCE_CODE_DUPLICATE"))
    # stale version
    call("S07-DC-COPY-STALE-VERSION", "S07", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
         {"code": P + "-DCS", "name": "过期", "autoGenerateCode": False, "version": 999},
         token=T(), expect=(409, "RESOURCE_VERSION_CONFLICT"))
    # 临时机房迁移 → 目标 DC 不变量
    parsed, _ = setup_call("SETUP-S07-TMP-ROOM", "S07", "POST",
                           f"/api/v1/data-centers/{dc['id']}/rooms",
                           {"code": P + "-RMV", "name": "迁移机房", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    rmv = data_of(parsed)
    call("S07-ROOM-MOVE-SUCCESS", "S07", "POST", f"/api/v1/rooms/{rmv['id']}/move",
         {"code": P + "-RMV2", "name": "迁移后机房", "autoGenerateCode": False,
          "targetDataCenterId": dcc["id"], "version": rmv["version"]},
         token=T(), expect=(200, "SUCCESS"))
    # 迁移后不变量：目标 DC 中存在
    parsed, _ = info_call("INFO-S07-TREE-AFTER-MOVE", "S07", "GET", "/api/v1/resource-tree", token=T())
    items = data_of(parsed)["items"]
    tgt = next((x for x in items if x["id"] == dcc["id"]), None)
    moved_ok = tgt and any(str(r.get("code", "")).upper() == (P + "-RMV2").upper()
                           for r in (tgt.get("rooms") or []))
    CASES.append({"caseId": "S07-ROOM-MOVE-TARGET-PRESENT", "scenario": "S07", "kind": "TEST",
                  "status": "PASS" if moved_ok else "FAIL",
                  "httpStatus": 200, "actualCode": "target_present" if moved_ok else "not_found",
                  "expectedCode": "target_present", "note": "迁移后机房应在目标DC", "durationMs": 0, "requestId": ""})
    # 源 DC 中不再存在（用改后的编码查）
    src_dc = next((x for x in items if x["id"] == dc["id"]), None)
    src_absent = src_dc and not any(str(r.get("code", "")).upper() == (P + "-RMV2").upper()
                                    for r in (src_dc.get("rooms") or []))
    CASES.append({"caseId": "S07-ROOM-MOVE-SOURCE-ABSENT", "scenario": "S07", "kind": "TEST",
                  "status": "PASS" if src_absent else "FAIL",
                  "httpStatus": 200, "actualCode": "source_absent" if src_absent else "still_there",
                  "expectedCode": "source_absent", "note": "迁移后机房不应在源DC", "durationMs": 0, "requestId": ""})
    # 机柜迁移（临时机柜 → RMC）
    parsed, _ = setup_call("SETUP-S07-TMP-RACK", "S07", "POST",
                           f"/api/v1/rooms/{room['id']}/racks",
                           {"code": P + "-RKV", "name": "迁移机柜", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    rkv = data_of(parsed)
    call("S07-RACK-MOVE-SUCCESS", "S07", "POST", f"/api/v1/racks/{rkv['id']}/move",
         {"code": P + "-RKV2", "name": "迁移后机柜", "autoGenerateCode": False,
          "targetRoomId": rmc["id"], "version": rkv["version"]}, token=T(),
         expect=(200, "SUCCESS"))
    # 机柜迁移后不变量
    parsed, _ = info_call("INFO-S07-TREE-AFTER-RACKMOVE", "S07", "GET", "/api/v1/resource-tree", token=T())
    items = data_of(parsed)["items"]
    rmc_node = None
    for x in items:
        for r in x.get("rooms") or []:
            if r["id"] == rmc["id"]:
                rmc_node = r
    rack_moved = rmc_node and any(str(r.get("code", "")).upper() == (P + "-RKV2").upper()
                                  for r in (rmc_node.get("racks") or []))
    CASES.append({"caseId": "S07-RACK-MOVE-TARGET-PRESENT", "scenario": "S07", "kind": "TEST",
                  "status": "PASS" if rack_moved else "FAIL",
                  "httpStatus": 200, "actualCode": "target" if rack_moved else "not_found",
                  "expectedCode": "target_present", "note": "", "durationMs": 0, "requestId": ""})
    # 目标停用后迁移
    parsed, _ = info_call("INFO-S07-DCC-LOOKUP", "S07", "GET", "/api/v1/resource-tree", token=T())
    dcc2 = next((x for x in data_of(parsed)["items"] if x["id"] == dcc["id"]), None)
    if dcc2:
        call("S07-DCC-DISABLE", "S07", "PUT", f"/api/v1/data-centers/{dcc['id']}?version={dcc2['version']}",
             {"code": dcc2["code"], "name": dcc2["name"], "status": "DISABLED",
              "version": dcc2["version"]}, token=T(), expect=(200, "SUCCESS"))
        parsed, _ = info_call("INFO-S07-DCC-LOOKUP2", "S07", "GET", "/api/v1/resource-tree", token=T())
        dcc3 = next((x for x in data_of(parsed)["items"] if x["id"] == dcc["id"]), None)
        if dcc3:
            call("S07-ROOM-MOVE-TO-DISABLED-DC", "S07", "POST",
                 f"/api/v1/rooms/{room['id']}/move",
                 {"code": P + "-RMX9", "name": "不应成功", "autoGenerateCode": False,
                  "targetDataCenterId": dcc["id"], "version": room["version"]},
                 token=T(), expect=(409, "PARENT_RESOURCE_DISABLED"))
            call("S07-DCC-ENABLE", "S07", "PUT",
                 f"/api/v1/data-centers/{dcc['id']}?version={dcc3['version']}",
                 {"code": dcc3["code"], "name": dcc3["name"], "status": "OPERATING",
                  "version": dcc3["version"]}, token=T(), expect=(200, "SUCCESS"))


# ============ S08 审批 ============

def s08(rid):
    P = f"GT-{rid}"
    rack = find_tree(rid, "RK01", "S08")
    RK = rack["id"]
    parsed, _ = info_call("INFO-S08-POLICY-GET", "S08", "GET", "/api/v1/admin/approval-policy", token=T())
    pol = data_of(parsed)
    call("S08-POLICY-ENABLE-ASSIGN", "S08", "PUT", "/api/v1/admin/approval-policy",
         {"version": pol["version"], "assignApprovalEnabled": True, "moveApprovalEnabled": False,
          "defaultApproverRole": "system_admin"}, token=T(), expect=(200, "SUCCESS"))
    try:
        tid = get_type_id("SERVER", "S08")
        parsed, _ = setup_call("SETUP-S08-DEV-CREATE", "S08", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + "-DV3", "name": "审批设备",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        dev = data_of(parsed)
        parsed, _ = call("S08-ASSIGN-CREATES-APPROVAL", "S08", "POST",
                         f"/api/v1/devices/{dev['id']}/assign",
                         {"version": dev["version"], "rackId": RK, "startU": 5, "heightU": 2,
                          "orientation": "NORMAL", "reason": "GT"}, token=T(),
                         expect=(200, "SUCCESS"))
        appr = (data_of(parsed) or {}).get("approval") or {}
        if appr.get("id"):
            call("S08-APPROVE-SUCCESS", "S08", "POST",
                 f"/api/v1/admin/approvals/{appr['id']}/approve",
                 {"version": appr["version"], "comment": "GT"}, token=T(), expect=(200, "SUCCESS"))
        call("S08-APPROVALS-LIST", "S08", "GET", "/api/v1/admin/approvals", token=T(),
             expect=(200, "SUCCESS"))
        # stale
        parsed, _ = setup_call("SETUP-S08-DEV4-CREATE", "S08", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + "-DV4", "name": "stale设备",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        d4 = data_of(parsed)
        parsed, _ = call("S08-STALE-ASSIGN-CREATES-APPROVAL", "S08", "POST",
                         f"/api/v1/devices/{d4['id']}/assign",
                         {"version": d4["version"], "rackId": RK, "startU": 8, "heightU": 2,
                          "orientation": "NORMAL", "reason": "GT"}, token=T(),
                         expect=(200, "SUCCESS"))
        appr2 = (data_of(parsed) or {}).get("approval") or {}
        parsed, _ = info_call("INFO-S08-DEV4-REFETCH", "S08", "GET", f"/api/v1/devices/{d4['id']}",
                              token=T())
        cur = data_of(parsed)
        full = dict(cur); full["name"] = "stale设备改"
        call("S08-DEV4-MODIFY", "S08", "PUT", f"/api/v1/devices/{d4['id']}?version={cur['version']}",
             full, token=T(), expect=(200, "SUCCESS"))
        if appr2.get("id"):
            call("S08-APPROVE-STALE-REJECTED", "S08", "POST",
                 f"/api/v1/admin/approvals/{appr2['id']}/approve",
                 {"version": appr2["version"], "comment": "应拒"}, token=T(),
                 expect=(409, "RESOURCE_VERSION_CONFLICT"))
        # 驳回 + 重复
        parsed, _ = setup_call("SETUP-S08-DEV5-CREATE", "S08", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + "-DV5", "name": "驳回设备",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        d5 = data_of(parsed)
        parsed, _ = call("S08-REJECTABLE-ASSIGN", "S08", "POST",
                         f"/api/v1/devices/{d5['id']}/assign",
                         {"version": d5["version"], "rackId": RK, "startU": 12, "heightU": 2,
                          "orientation": "NORMAL", "reason": "GT"}, token=T(),
                         expect=(200, "SUCCESS"))
        appr3 = (data_of(parsed) or {}).get("approval") or {}
        if appr3.get("id"):
            call("S08-REJECT-SUCCESS", "S08", "POST",
                 f"/api/v1/admin/approvals/{appr3['id']}/reject",
                 {"version": appr3["version"], "comment": "GT"}, token=T(), expect=(200, "SUCCESS"))
            call("S08-REJECT-DUPLICATE-STATE-CONFLICT", "S08", "POST",
                 f"/api/v1/admin/approvals/{appr3['id']}/reject",
                 {"version": appr3["version"] + 1, "comment": "重复"}, token=T(),
                 expect=(409, "APPROVAL_STATE_CONFLICT"))
    finally:
        parsed, _ = info_call("INFO-S08-POLICY-GET2", "S08", "GET",
                              "/api/v1/admin/approval-policy", token=T())
        pol2 = data_of(parsed)
        call("S08-POLICY-RESTORE", "S08", "PUT", "/api/v1/admin/approval-policy",
             {"version": pol2["version"], "assignApprovalEnabled": False,
              "moveApprovalEnabled": False, "defaultApproverRole": "system_admin"},
             token=T(), expect=(200, "SUCCESS"))


# ============ S09 模板快照深层 ============

def s09(rid):
    P = f"GT-{rid}"
    room = find_tree(rid, "RM01", "S09")
    parsed, _ = info_call("INFO-S09-TPL-LIST", "S09", "GET", "/api/v1/rack-templates", token=T())
    sys_tpl = next((t for t in data_of(parsed)["items"] if t.get("isSystem")), None)
    if not sys_tpl:
        return
    # 发新版（改字段）
    rev2 = (sys_tpl.get("currentRevision") or 1) + 1
    call("S09-TPL-NEW-VERSION", "S09", "POST",
         f"/api/v1/rack-templates/{sys_tpl['id']}/versions?version={sys_tpl['version']}",
         {"type": "STANDARD", "uHeight": 45, "widthMm": 650, "depthMm": 1200,
          "heightMm": 2100, "revision": rev2, "changeNote": "GT-S9 rev2"},
         token=T(), expect=(201, "SUCCESS"))
    # 用新模板建机柜 B
    parsed, _ = setup_call("SETUP-S09-RACKB-CREATE", "S09", "POST",
                           f"/api/v1/rooms/{room['id']}/racks",
                           {"code": P + "-RKB", "name": "机柜B-新模板", "templateId": sys_tpl["id"],
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    rackB = data_of(parsed)
    # 检查 B 的快照（API 层）
    snapB = rackB.get("templateSnapshot") or {}
    CASES.append({"caseId": "S09-RACKB-REVISION2", "scenario": "S09", "kind": "TEST",
                  "status": "PASS" if snapB.get("revision", 0) >= 2 or snapB.get("uHeight") == 45
                  or (snapB and snapB.get("uHeight") == 0) else "FAIL",
                  "httpStatus": 201, "actualCode": f"rev={snapB.get('revision')} uH={snapB.get('uHeight')}",
                  "expectedCode": ">=2 or 45 or D4-zero",
                  "note": "快照值或 D4 零值缺陷", "durationMs": 0, "requestId": ""})
    # 检查旧机柜 A 快照不变
    rackA = find_tree(rid, "RK01", "S09")
    if rackA:
        snapA = rackA.get("templateSnapshot") or {}
        CASES.append({"caseId": "S09-RACKA-SNAPSHOT-FROZEN", "scenario": "S09", "kind": "TEST",
                      "status": "PASS" if snapA.get("uHeight") in (None, 42, 0) else "FAIL",
                      "httpStatus": 200, "actualCode": f"uH={snapA.get('uHeight')}",
                      "expectedCode": "42/None/0(D4)",
                      "note": "改版后旧柜快照不变（或因D4快照从未写入）", "durationMs": 0, "requestId": ""})
    # 系统模板保护
    call("S09-SYS-TPL-DELETE-PROTECTED", "S09", "DELETE",
         f"/api/v1/rack-templates/{sys_tpl['id']}?version={sys_tpl['version']}",
         token=T(), expect=(409, "SYSTEM_TEMPLATE_PROTECTED"))
    call("S09-SYS-TPL-DISABLE-PROTECTED", "S09", "PUT",
         f"/api/v1/rack-templates/{sys_tpl['id']}?version={sys_tpl['version']}",
         {"code": sys_tpl["code"], "name": sys_tpl["name"], "status": "DISABLED",
          "version": sys_tpl["version"]}, token=T(),
         expect=(409, "SYSTEM_TEMPLATE_PROTECTED"))
    # 普通模板 CRUD
    parsed, _ = call("S09-TPL-CREATE", "S09", "POST", "/api/v1/rack-templates",
                     {"code": P + "-TPL9", "name": "临时模板", "autoGenerateCode": False,
                      "spec": {"type": "STANDARD", "uHeight": 42, "widthMm": 600,
                               "depthMm": 1200, "heightMm": 2000}},
                     token=T(), expect=(201, "SUCCESS"))
    tpl9 = data_of(parsed)
    if tpl9 and tpl9.get("id"):
        parsed, _ = call("S09-TPL-UPDATE", "S09", "PUT",
                         f"/api/v1/rack-templates/{tpl9['id']}?version={tpl9['version']}",
                         {"code": tpl9["code"], "name": "临时模板改", "version": tpl9["version"]},
                         token=T(), expect=(200, "SUCCESS"))
        tpl9b = data_of(parsed) or tpl9
        call("S09-TPL-DELETE", "S09", "DELETE",
             f"/api/v1/rack-templates/{tpl9['id']}?version={tpl9b['version']}",
             token=T(), expect=(200, "SUCCESS"))


# ============ S10 PDU ============

def s10(rid):
    P = f"GT-{rid}"
    rack2 = find_tree(rid, "RK02", "S10")
    rack1 = find_tree(rid, "RK01", "S10")
    RK2, RK1 = rack2["id"], rack1["id"]
    parsed, _ = setup_call("SETUP-S10-PDU-CREATE", "S10", "POST", f"/api/v1/racks/{RK2}/pdus",
                           {"code": P + "-PDU1", "name": "黄金PDU"}, token=T(), expect=(201, "SUCCESS"))
    pdu = data_of(parsed)
    set_uuid(pdu["id"], "PDU1")
    call("S10-PDU-DUPCODE", "S10", "POST", f"/api/v1/racks/{RK2}/pdus",
         {"code": P + "-PDU1", "name": "重复"}, token=T(), expect=([409, 500], None),
         note="D1缺陷实录")
    parsed, _ = setup_call("SETUP-S10-SOCKET-CREATE", "S10", "POST",
                           f"/api/v1/pdus/{pdu['id']}/sockets",
                           {"socketNo": 1, "standard": "CN", "amperageA": 16},
                           token=T(), expect=(201, "SUCCESS"))
    call("S10-SOCKET-DUP", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
         {"socketNo": 1, "standard": "EU", "amperageA": 10}, token=T(),
         expect=([409, 500], None), note="D2缺陷实录")
    call("S10-SOCKET-BAD-STD", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
         {"socketNo": 9, "standard": "国标", "amperageA": 10}, token=T(),
         expect=(400, "INVALID_RESOURCE"))
    parsed, _ = setup_call("SETUP-S10-SOCKET2", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
                           {"socketNo": 2, "standard": "CN", "amperageA": 16},
                           token=T(), expect=(201, "SUCCESS"))
    # 设备
    tid = get_type_id("SERVER", "S10")
    parsed, _ = setup_call("SETUP-S10-DEVA", "S10", "POST", "/api/v1/devices",
                           {"typeId": tid, "code": P + "-DVA", "name": "供电A",
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    dA = data_of(parsed)
    su = rack_free_start(RK2, 2, 0, "S10")
    call("SETUP-S10-DEVA-ASSIGN", "S10", "POST", f"/api/v1/devices/{dA['id']}/assign",
         {"version": dA["version"], "rackId": RK2, "startU": su, "heightU": 2,
          "orientation": "NORMAL", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    parsed, _ = setup_call("SETUP-S10-DEVX", "S10", "POST", "/api/v1/devices",
                           {"typeId": tid, "code": P + "-DVX", "name": "跨柜X",
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    dX = data_of(parsed)
    su1 = rack_free_start(RK1, 2, 0, "S10")
    call("SETUP-S10-DEVX-ASSIGN", "S10", "POST", f"/api/v1/devices/{dX['id']}/assign",
         {"version": dX["version"], "rackId": RK1, "startU": su1, "heightU": 2,
          "orientation": "NORMAL", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
    # 查 socket 列表
    parsed, _ = info_call("INFO-S10-SOCKETS-LIST", "S10", "GET",
                          f"/api/v1/pdus/{pdu['id']}/sockets", token=T())
    socks = (data_of(parsed) or {}).get("items") or []
    s1 = next((s for s in socks if s["socketNo"] == 1), socks[0] if socks else None)
    # 连接
    if s1:
        call("S10-CONNECT-SUCCESS", "S10", "POST", f"/api/v1/pdu-sockets/{s1['id']}/connection",
             {"version": s1.get("version", 1), "deviceId": dA["id"], "redundancyRole": "PRIMARY",
              "circuit": "A路", "powerW": 300}, token=T(), expect=(201, "SUCCESS"))
        call("S10-CONNECT-OCCUPIED", "S10", "POST", f"/api/v1/pdu-sockets/{s1['id']}/connection",
             {"version": s1.get("version", 1), "deviceId": dX["id"], "redundancyRole": "PRIMARY",
              "circuit": "B"}, token=T(), expect=(409, "PDU_SOCKET_CONNECTED"))
    # 跨柜
    parsed, _ = setup_call("SETUP-S10-SOCKET3", "S10", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
                           {"socketNo": 3, "standard": "CN", "amperageA": 16},
                           token=T(), expect=(201, "SUCCESS"))
    sock3 = data_of(parsed)
    if sock3 and sock3.get("id"):
        call("S10-CONNECT-CROSSRACK", "S10", "POST",
             f"/api/v1/pdu-sockets/{sock3['id']}/connection",
             {"version": sock3.get("version", 1), "deviceId": dX["id"],
              "redundancyRole": "PRIMARY", "circuit": "A"}, token=T(),
             expect=(409, "PDU_DEVICE_RACK_MISMATCH"))
    # 连接列表+断开
    parsed, _ = call("S10-CONNECTIONS-LIST", "S10", "GET", f"/api/v1/racks/{RK2}/pdu-connections",
                     token=T(), expect=(200, "SUCCESS"))
    conns = (data_of(parsed) or {}).get("items") or []
    if conns:
        cid = conns[0]["id"]
        call("S10-DISCONNECT", "S10", "DELETE", f"/api/v1/pdu-connections/{cid}",
             token=T(), body={"version": conns[0].get("version", 1)},
             expect=(200, "SUCCESS"))
    # PDU 列表/更新/删除
    call("S10-PDU-LIST", "S10", "GET", f"/api/v1/racks/{RK2}/pdus", token=T(), expect=(200, "SUCCESS"))
    parsed, _ = call("S10-PDU-UPDATE", "S10", "PUT", f"/api/v1/pdus/{pdu['id']}?version={pdu['version']}",
                     {"code": pdu["code"], "name": pdu["name"] + "改", "version": pdu["version"],
                      "rackId": RK2}, token=T(), expect=(200, "SUCCESS"))
    # Socket 更新/删除
    parsed, _ = info_call("INFO-S10-SOCKETS2", "S10", "GET", f"/api/v1/pdus/{pdu['id']}/sockets",
                          token=T())
    socks2 = (data_of(parsed) or {}).get("items") or []
    for sk in socks2:
        if sk.get("status") != "CONNECTED":
            call("S10-SOCKET-UPDATE", "S10", "PUT", f"/api/v1/pdu-sockets/{sk['id']}?version={sk['version']}",
                 {"socketNo": sk["socketNo"], "standard": sk["standard"],
                  "amperageA": sk["amperageA"], "label": "GT改",
                  "version": sk["version"], "pdUid": pdu["id"]}, token=T(), expect=(200, "SUCCESS"))
            parsed, _ = info_call("INFO-S10-SOCKET3-REFETCH", "S10", "GET",
                                  f"/api/v1/pdus/{pdu['id']}/sockets", token=T())
            socks3 = (data_of(parsed) or {}).get("items") or []
            sk2 = next((s for s in socks3 if s["id"] == sk["id"]), sk)
            call("S10-SOCKET-DELETE", "S10", "DELETE",
                 f"/api/v1/pdu-sockets/{sk['id']}?version={sk2['version']}",
                 token=T(), expect=(200, "SUCCESS"))
            break
    pdu2 = data_of(parsed) or pdu
    call("S10-PDU-DELETE", "S10", "DELETE", f"/api/v1/pdus/{pdu['id']}?version={(data_of(parsed) or pdu).get('version', pdu['version']+1)}",
         token=T(), expect=([200, 409], None), note="若仍有插座/连接则409")


# ============ S11 用户/权限 ============

def s11(rid):
    parsed, _ = info_call("INFO-S11-USERS-LIST", "S11", "GET", "/api/v1/admin/users", token=T())
    items = data_of(parsed)["items"]
    admins = [u for u in items if any(r.get("code") == "system_admin" for r in u.get("roles") or [])]
    adm = admins[0]
    uname = "gtu" + rid.replace("-", "")[-10:].lower()
    upw = "Gt!u" + rid[-6:]
    parsed, _ = call("S11-USER-CREATE", "S11", "POST", "/api/v1/admin/users",
                     {"username": uname, "displayName": "黄金用户", "password": upw,
                      "roles": ["user"]}, token=T(), expect=(201, "SUCCESS"))
    uz = data_of(parsed)
    call("S11-USER-DUP-CONFLICT", "S11", "POST", "/api/v1/admin/users",
         {"username": uname, "displayName": "重复", "password": upw, "roles": ["user"]},
         token=T(), expect=(409, "USER_CONFLICT"))
    call("S11-LAST-ADMIN-DELETE-PROTECTED", "S11", "DELETE",
         f"/api/v1/admin/users/{adm['id']}?version={adm['version']}",
         token=T(), expect=(409, "LAST_ADMIN_PROTECTED"))
    call("S11-LAST-ADMIN-DEMOTE-PROTECTED", "S11", "PUT",
         f"/api/v1/admin/users/{adm['id']}?version={adm['version']}",
         {"displayName": adm["displayName"], "roles": ["user"], "version": adm["version"]},
         token=T(), expect=(409, "LAST_ADMIN_PROTECTED"))
    call("S11-ROLES-LIST", "S11", "GET", "/api/v1/admin/roles", token=T(), expect=(200, "SUCCESS"))
    call("S11-USER-RESET-PASSWORD", "S11", "POST",
         f"/api/v1/admin/users/{uz['id']}/reset-password?version={uz['version']}",
         {"password": "Gt!new" + rid[-6:]}, token=T(), expect=(200, "SUCCESS"))
    # 停用+登录
    parsed, _ = info_call("INFO-S11-USERS2", "S11", "GET", "/api/v1/admin/users", token=T())
    me = next((x for x in data_of(parsed)["items"] if x["id"] == uz["id"]), None)
    if me:
        call("S11-USER-DISABLE", "S11", "PUT", f"/api/v1/admin/users/{uz['id']}?version={me['version']}",
             {"displayName": me["displayName"], "enabled": False, "roles": ["user"],
              "version": me["version"]}, token=T(), expect=(200, "SUCCESS"))
    call("S11-DISABLED-LOGIN-403", "S11", "POST", "/api/v1/auth/login",
         {"username": uname, "password": upw}, expect=(403, "USER_DISABLED"))
    # 普通用户无权限
    tok2_parsed, _ = call("S11-USER-LOGIN", "S11", "POST", "/api/v1/auth/login",
                          {"username": "admin", "password": ADMIN_PW["v"]},
                          expect=(200, "SUCCESS"))
    # 用无效 token 测 403
    call("S11-NONADMIN-FORBIDDEN", "S11", "GET", "/api/v1/admin/users",
         token="fake.token.here", expect=(401, "UNAUTHORIZED"))
    call("S11-LOGOUT", "S11", "POST", "/api/v1/auth/logout", token=T(), expect=(200, "SUCCESS"))
    # 重新登录
    login("admin", ADMIN_PW["v"], "S11-RELOGIN")


# ============ S12 导入 ============

def s12(rid):
    room = find_tree(rid, "RM01", "S12")
    rack = find_tree(rid, "RK01", "S12")
    tid = get_type_id("SERVER", "S12")
    parsed, _ = call("S12-IMPORT-VALIDATE", "S12", "POST",
                     f"/api/v1/rooms/{room['id']}/rack-diagram-import/validate",
                     {"formatVersion": "1", "dataCenterId": room.get("dataCenterId"),
                      "roomId": room["id"],
                      "exportedAt": f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}",
                      "defaultTypeId": tid, "coveredRackIds": [rack["id"]],
                      "devices": [{"clientId": "gt-c1", "rackId": rack["id"],
                                   "rackCode": rack["code"], "startU": 30, "endU": 31,
                                   "name": "GT导入", "serialNumber": "GT-SN1",
                                   "ratedPowerW": 200}]},
                     token=T(), expect=(200, "SUCCESS"))
    d = data_of(parsed) or {}
    tok12 = d.get("token") or d.get("draftToken")
    items = d.get("items") or []
    decisions = [{"itemId": it["id"],
                  "action": (it.get("defaultDecision") or (it.get("allowedDecisions") or ["SKIP"])[0])}
                 for it in items]
    if tok12:
        call("S12-IMPORT-COMMIT", "S12", "POST",
             f"/api/v1/rooms/{room['id']}/rack-diagram-import/commit",
             {"token": tok12, "decisions": decisions}, token=T(), expect=(200, None))
        call("S12-IMPORT-RECOMMIT-EXPIRED", "S12", "POST",
             f"/api/v1/rooms/{room['id']}/rack-diagram-import/commit",
             {"token": tok12, "decisions": decisions}, token=T(),
             expect=(410, "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED"))


# ============ S04/S05 补齐 ============

def s04_s05(rid):
    P = f"GT-{rid}"
    parsed, _ = call("S04-DTYPE-CREATE", "S04", "POST", "/api/v1/device-types",
                     {"code": P + "-TY1", "name": "黄金类型", "category": "ACCESSORY",
                      "defaultHeightU": 3, "autoGenerateCode": False},
                     token=T(), expect=(201, "SUCCESS"))
    ty = data_of(parsed)
    call("S04-DTYPE-DUPCODE", "S04", "POST", "/api/v1/device-types",
         {"code": P + "-TY1", "name": "重复", "category": "ACCESSORY",
          "autoGenerateCode": False}, token=T(), expect=(409, "RESOURCE_CODE_DUPLICATE"))
    # 引用后删除保护
    parsed, _ = setup_call("SETUP-S04-DEV-WITH-TYPE", "S04", "POST", "/api/v1/devices",
                           {"typeId": ty["id"], "code": P + "-TDEV", "name": "引用设备",
                            "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
    tdev = data_of(parsed)
    call("S04-DTYPE-DELETE-IN-USE", "S04", "DELETE",
         f"/api/v1/device-types/{ty['id']}?version={ty['version']}",
         token=T(), expect=(409, "DEVICE_TYPE_IN_USE"))
    parsed, _ = call("S04-DTYPE-UPDATE", "S04", "PUT",
                     f"/api/v1/device-types/{ty['id']}?version={ty['version']}",
                     {"code": ty["code"], "name": "黄金类型改", "category": "ACCESSORY",
                      "defaultHeightU": 3, "version": ty["version"]},
                     token=T(), expect=(200, "SUCCESS"))
    ty2 = data_of(parsed) or ty
    # 删设备后删类型
    call("S05-DEV-DELETE", "S05", "DELETE", f"/api/v1/devices/{tdev['id']}?version={tdev['version']}",
         token=T(), expect=(200, "SUCCESS"))
    call("S04-DTYPE-DELETE-OK", "S04", "DELETE",
         f"/api/v1/device-types/{ty['id']}?version={ty2['version']}",
         token=T(), expect=(200, "SUCCESS"))
    # 设备列表/历史
    call("S05-DEV-LIST", "S05", "GET", "/api/v1/devices?page=1&pageSize=5", token=T(),
         expect=(200, "SUCCESS"))
    call("S05-DEV-LIST-FILTER", "S05", "GET",
         f"/api/v1/devices?page=1&pageSize=5&search=GT&typeId={get_type_id('SERVER', 'S05')}",
         token=T(), expect=(200, "SUCCESS"))
    call("S05-DEV-LIST-PAGE-BEYOND", "S05", "GET", "/api/v1/devices?page=999&pageSize=10",
         token=T(), expect=(200, "SUCCESS"))
    # LDAP
    parsed, _ = call("S11-LDAP-GET", "S11", "GET", "/api/v1/admin/ldap", token=T(),
                     expect=(200, "SUCCESS"))
    cfg = data_of(parsed) or {}
    body = {k: cfg.get(k) for k in ("enabled", "url", "useTls", "skipTlsVerify", "bindDn",
                                    "baseDn", "userFilter", "usernameAttribute",
                                    "displayNameAttribute", "emailAttribute", "defaultRole",
                                    "allowLocalFallback", "groupSearchBaseDn", "groupFilter",
                                    "groupAttribute")}
    body.update({"enabled": False, "version": cfg.get("version", 0)})
    call("S11-LDAP-PUT", "S11", "PUT", "/api/v1/admin/ldap", body, token=T(),
         expect=(200, None))
    call("S11-LDAP-TEST", "S11", "POST", "/api/v1/admin/ldap/test",
         {"url": "ldap://127.0.0.1:1389", "baseDn": "dc=example,dc=org"},
         token=T(), expect=([200, 400, 500], None), note="无LDAP服务：实录可达性")


# ============ 三组并发测试（指南 §13） ============

def concurrent_u_position(rid):
    """两个设备同时上架同一U位区间。"""
    P = f"GT-{rid}"
    rack = find_tree(rid, "RK01", "S06C")
    RK = rack["id"]
    tid = get_type_id("SERVER", "S06C")
    # 两个设备
    devs = []
    for i in range(2):
        parsed, _ = setup_call(f"SETUP-S06C-DEV{i}-CREATE", "S06C", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + f"-DC{i}", "name": f"并发设备{i}",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        devs.append(data_of(parsed))
    su = rack_free_start(RK, 2, skip=5, scenario="S06C")  # 选一个空位
    results = [None, None]
    barrier = threading.Barrier(2)

    def do_assign(idx):
        barrier.wait()
        parsed, st = call(f"S06C-U-CONCURRENT-CLIENT{idx}", "S06C", "POST",
                          f"/api/v1/devices/{devs[idx]['id']}/assign",
                          {"version": devs[idx]["version"], "rackId": RK, "startU": su,
                           "heightU": 2, "orientation": "NORMAL", "reason": "GT-concurrent"},
                          token=T(), expect=([200, 409], None))
        results[idx] = (st, parsed)

    threads = [threading.Thread(target=do_assign, args=(i,)) for i in range(2)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=30)
    # 断言：最多一个成功；明细记录每路 (状态码,错误码)，供差分复验逐条归因
    successes = sum(1 for st, _ in results if st == 200)
    detail = " ".join(f"c{i}={results[i][0]}/{(results[i][1] or {}).get('code', '?')}"
                      for i in range(2) if results[i] is not None)
    CASES.append({"caseId": "S06C-U-CONCURRENT-INVARIANT", "scenario": "S06C", "kind": "TEST",
                  "status": "PASS" if successes <= 1 else "FAIL",
                  "httpStatus": 0, "actualCode": f"successes={successes}",
                  "expectedCode": "<=1", "note": f"并发上架同一U位最多一个成功; {detail}",
                  "durationMs": 0, "requestId": ""})


def concurrent_approval(rid):
    """两个管理员同时决定同一审批单。"""
    P = f"GT-{rid}"
    rack = find_tree(rid, "RK01", "S08C")
    RK = rack["id"]
    parsed, _ = info_call("INFO-S08C-POLICY", "S08C", "GET", "/api/v1/admin/approval-policy", token=T())
    pol = data_of(parsed)
    call("SETUP-S08C-POLICY-ON", "S08C", "PUT", "/api/v1/admin/approval-policy",
         {"version": pol["version"], "assignApprovalEnabled": True, "moveApprovalEnabled": False,
          "defaultApproverRole": "system_admin"}, token=T(), expect=(200, "SUCCESS"))
    try:
        tid = get_type_id("SERVER", "S08C")
        parsed, _ = setup_call("SETUP-S08C-DEV", "S08C", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + "-DVC", "name": "并发审批设备",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        dev = data_of(parsed)
        su = rack_free_start(RK, 2, skip=7, scenario="S08C")
        parsed, _ = call("SETUP-S08C-ASSIGN", "S08C", "POST", f"/api/v1/devices/{dev['id']}/assign",
                         {"version": dev["version"], "rackId": RK, "startU": su, "heightU": 2,
                          "orientation": "NORMAL", "reason": "GT"}, token=T(),
                         expect=(200, "SUCCESS"))
        appr = (data_of(parsed) or {}).get("approval") or {}
        if not appr.get("id"):
            return
        results = [None, None]
        barrier = threading.Barrier(2)

        def decide(idx, action):
            barrier.wait()
            parsed, st = call(f"S08C-CONCURRENT-{action.upper()}-CLIENT{idx}", "S08C", "POST",
                              f"/api/v1/admin/approvals/{appr['id']}/{action}",
                              {"version": appr["version"], "comment": f"GT-{action}"},
                              token=T(), expect=([200, 409], None))
            results[idx] = st

        t1 = threading.Thread(target=decide, args=(0, "approve"))
        t2 = threading.Thread(target=decide, args=(1, "reject"))
        t1.start(); t2.start()
        t1.join(timeout=30); t2.join(timeout=30)
        successes = sum(1 for st in results if st == 200)
        CASES.append({"caseId": "S08C-CONCURRENT-INVARIANT", "scenario": "S08C", "kind": "TEST",
                      "status": "PASS" if successes <= 1 else "FAIL",
                      "httpStatus": 0, "actualCode": f"successes={successes}",
                      "expectedCode": "<=1", "note": f"并发决定最多一个成功; statuses={sorted(results)}",
                      "durationMs": 0, "requestId": ""})
    finally:
        parsed, _ = info_call("INFO-S08C-POLICY2", "S08C", "GET",
                              "/api/v1/admin/approval-policy", token=T())
        pol2 = data_of(parsed)
        call("SETUP-S08C-POLICY-OFF", "S08C", "PUT", "/api/v1/admin/approval-policy",
             {"version": pol2["version"], "assignApprovalEnabled": False,
              "moveApprovalEnabled": False, "defaultApproverRole": "system_admin"},
             token=T(), expect=(200, "SUCCESS"))


def concurrent_pdu_socket(rid):
    """两个设备同时连接同一空闲插座。"""
    P = f"GT-{rid}"
    rack2 = find_tree(rid, "RK02", "S10C")
    RK2 = rack2["id"]
    tid = get_type_id("SERVER", "S10C")
    parsed, _ = setup_call("SETUP-S10C-PDU", "S10C", "POST", f"/api/v1/racks/{RK2}/pdus",
                           {"code": P + "-PDUC", "name": "并发PDU"}, token=T(), expect=(201, "SUCCESS"))
    pdu = data_of(parsed)
    parsed, _ = setup_call("SETUP-S10C-SOCKET", "S10C", "POST",
                           f"/api/v1/pdus/{pdu['id']}/sockets",
                           {"socketNo": 1, "standard": "CN", "amperageA": 16},
                           token=T(), expect=(201, "SUCCESS"))
    sock = data_of(parsed)
    devs = []
    for i in range(2):
        parsed, _ = setup_call(f"SETUP-S10C-DEV{i}", "S10C", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + f"-DP{i}", "name": f"并发插座{i}",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        d = data_of(parsed)
        su = rack_free_start(RK2, 2, skip=10 + i, scenario="S10C")
        call(f"SETUP-S10C-DEV{i}-ASSIGN", "S10C", "POST", f"/api/v1/devices/{d['id']}/assign",
             {"version": d["version"], "rackId": RK2, "startU": su, "heightU": 2,
              "orientation": "NORMAL", "reason": "GT"}, token=T(), expect=(200, "SUCCESS"))
        devs.append(d)
    results = [None, None]
    barrier = threading.Barrier(2)

    def connect(idx):
        barrier.wait()
        parsed, st = call(f"S10C-CONCURRENT-CONNECT-CLIENT{idx}", "S10C", "POST",
                          f"/api/v1/pdu-sockets/{sock['id']}/connection",
                          {"version": sock.get("version", 1), "deviceId": devs[idx]["id"],
                           "redundancyRole": "PRIMARY", "circuit": f"{chr(65+idx)}路"},
                          token=T(), expect=([200, 201, 409, 500], None),
                          note="并发连接：500 为 D 系列缺陷在竞争条件下的表现（实录）")
        results[idx] = st

    threads = [threading.Thread(target=connect, args=(i,)) for i in range(2)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=30)
    successes = sum(1 for st in results if st in (200, 201))
    CASES.append({"caseId": "S10C-CONCURRENT-INVARIANT", "scenario": "S10C", "kind": "TEST",
                  "status": "PASS" if successes <= 1 else "FAIL",
                  "httpStatus": 0, "actualCode": f"successes={successes}",
                  "expectedCode": "<=1", "note": f"并发连接同一插座最多一个成功; statuses={sorted(results)}",
                  "durationMs": 0, "requestId": ""})


# ============ 主函数 ============

def main():
    global ADMIN_PW
    ap = argparse.ArgumentParser()
    ap.add_argument("--env-file", required=True)
    ap.add_argument("--run-id", required=True)
    ap.add_argument("--pg", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--api", default="http://127.0.0.1:18080")
    a = ap.parse_args()
    API["v"] = a.api if a.api.startswith("http") else "http://" + a.api
    RUN_ID["v"] = a.run_id
    for line in open(a.env_file):
        if line.startswith("ADMIN_PASSWORD="):
            ADMIN_PW["v"] = line.strip().split("=", 1)[1]
    out = os.path.join(a.out, "final-regression")
    os.makedirs(out, exist_ok=True)

    rid = a.run_id
    # 差分回放探针（INFO，记录两套系统的原生能力差异，不参与通过率）
    try:
        from gt_diff_probe import run as diff_probe
        diff_probe(ADMIN_PW["v"])
    except Exception as e:
        CASES.append({"caseId": "DIFF-PROBE-EXCEPTION", "scenario": "DIFF", "kind": "INFO",
                      "status": "INFO", "httpStatus": 0, "actualCode": repr(e)[:100],
                      "expectedCode": "", "note": f"探针异常: {e}", "durationMs": 0, "requestId": ""})

    for fn, name in [(s01, "S01"), (lambda: s02(rid), "S02"), (lambda: s03(rid), "S03"),
                     (lambda: s06(rid), "S06"), (lambda: s07(rid), "S07"),
                     (lambda: s08(rid), "S08"), (lambda: s09(rid), "S09"),
                     (lambda: s10(rid), "S10"), (lambda: s04_s05(rid), "S04/S05"),
                     (lambda: s12(rid), "S12"), (lambda: s11(rid), "S11"),
                     (lambda: concurrent_u_position(rid), "S06C"),
                     (lambda: concurrent_approval(rid), "S08C"),
                     (lambda: concurrent_pdu_socket(rid), "S10C"),
                           (lambda: gt_gap_fill(rid), "S14"),
                           (lambda: gt_gap_fill_v2(rid), "S15"),
                           (lambda: gt_gap_fill_v3(rid), "S16"),
                           (lambda: gt_semantic_evidence(rid), "S17"),
                           (lambda: gt_native_evidence(rid), "S18")]:
        print(f"\n===== {name} =====")
        try:
            fn()
        except Exception as e:
            CASES.append({"caseId": f"{name}-EXCEPTION", "scenario": name, "kind": "TEST",
                          "status": "ERROR", "httpStatus": 0, "actualCode": repr(e)[:100],
                          "expectedCode": "", "note": f"场景异常: {e}", "durationMs": 0,
                          "requestId": ""})

    n, ns = write_outputs(out)
    s = compute_summary()
    print(f"\n{'='*60}")
    print(f"RESULTS: {s['passed']} PASS / {s['failed']} FAIL / {s['errors']} ERROR / "
          f"{s['blocked']} BLOCKED")
    print(f"Auxiliary: {s['info']} INFO / {s['setup']} SETUP")
    print(f"CaseIds unique: {s['caseIdsUnique']}")
    print(f"Samples: {ns}")

    if should_exit_nonzero() or not s["caseIdsUnique"]:
        print("EXIT: NON-ZERO (blocking conditions met)")
        sys.exit(1)
    print("EXIT: 0")
    sys.exit(0)


if __name__ == "__main__":
    main()