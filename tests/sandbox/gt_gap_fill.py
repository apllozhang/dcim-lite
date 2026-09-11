#!/usr/bin/env python3
"""第四轮阶段 B：行为覆盖补齐。在 gt_final_v3 场景后调用 gt_gap_fill(rid)。
覆盖 auth_boundary（66 端点批量 401）+ validation（空/超长/枚举错）+ pagination + invariant。
用法：在 gt_final_v3.py 中 from gt_gap_fill import gt_gap_fill 后调用。
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gt_core import call, info_call, setup_call, data_of, T, CASES

FAKE_UUID = "00000000-0000-0000-0000-000000000001"


def _find_tree(rid, suffix):
    parsed, _ = info_call(f"INFO-S14-LOOKUP-{suffix}", "S14",
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


def _get_type_id():
    parsed, _ = info_call("INFO-S14-TYPE", "S14", "GET", "/api/v1/device-types", token=T())
    for t in data_of(parsed)["items"]:
        if t["code"] == "SERVER":
            return t["id"]
    return None


def gt_gap_fill(rid):
    P = f"GT-{rid}"
    dc = _find_tree(rid, "DC01")
    room = _find_tree(rid, "RM01")
    rack = _find_tree(rid, "RK01")
    rack2 = _find_tree(rid, "RK02")
    tid = _get_type_id()
    dc_id = dc["id"] if dc else FAKE_UUID
    room_id = room["id"] if room else FAKE_UUID
    rack_id = rack["id"] if rack else FAKE_UUID
    rack2_id = rack2["id"] if rack2 else FAKE_UUID

    # ===== auth_boundary 批量（无 token → 401） =====
    endpoints = [
        ("GET", "/api/v1/resource-tree"),
        ("POST", "/api/v1/data-centers"),
        ("PUT", f"/api/v1/data-centers/{dc_id}"),
        ("DELETE", f"/api/v1/data-centers/{dc_id}?version=1"),
        ("POST", f"/api/v1/data-centers/{dc_id}/rooms"),
        ("POST", f"/api/v1/data-centers/{dc_id}/copy"),
        ("PUT", f"/api/v1/rooms/{room_id}"),
        ("DELETE", f"/api/v1/rooms/{room_id}?version=1"),
        ("POST", f"/api/v1/rooms/{room_id}/racks"),
        ("POST", f"/api/v1/rooms/{room_id}/copy"),
        ("POST", f"/api/v1/rooms/{room_id}/move"),
        ("PUT", f"/api/v1/racks/{rack_id}"),
        ("DELETE", f"/api/v1/racks/{rack_id}?version=1"),
        ("POST", f"/api/v1/racks/{rack_id}/copy"),
        ("POST", f"/api/v1/racks/{rack_id}/move"),
        ("GET", f"/api/v1/racks/{rack_id}/u-layout"),
        ("GET", f"/api/v1/racks/{rack2_id}/pdus"),
        ("POST", f"/api/v1/racks/{rack2_id}/pdus"),
        ("GET", f"/api/v1/racks/{rack2_id}/pdu-connections"),
        ("GET", "/api/v1/device-types"),
        ("POST", "/api/v1/device-types"),
        ("PUT", f"/api/v1/device-types/{FAKE_UUID}"),
        ("DELETE", f"/api/v1/device-types/{FAKE_UUID}?version=1"),
        ("GET", "/api/v1/devices?page=1&pageSize=1"),
        ("POST", "/api/v1/devices"),
        ("GET", f"/api/v1/devices/{FAKE_UUID}"),
        ("PUT", f"/api/v1/devices/{FAKE_UUID}"),
        ("DELETE", f"/api/v1/devices/{FAKE_UUID}?version=1"),
        ("POST", f"/api/v1/devices/{FAKE_UUID}/assign"),
        ("POST", f"/api/v1/devices/{FAKE_UUID}/move"),
        ("POST", f"/api/v1/devices/{FAKE_UUID}/decommission"),
        ("GET", f"/api/v1/devices/{FAKE_UUID}/history"),
        ("GET", "/api/v1/rack-templates"),
        ("POST", "/api/v1/rack-templates"),
        ("PUT", f"/api/v1/rack-templates/{FAKE_UUID}"),
        ("DELETE", f"/api/v1/rack-templates/{FAKE_UUID}?version=1"),
        ("POST", f"/api/v1/rack-templates/{FAKE_UUID}/versions"),
        ("GET", "/api/v1/admin/users"),
        ("POST", "/api/v1/admin/users"),
        ("PUT", f"/api/v1/admin/users/{FAKE_UUID}"),
        ("DELETE", f"/api/v1/admin/users/{FAKE_UUID}?version=1"),
        ("POST", f"/api/v1/admin/users/{FAKE_UUID}/reset-password"),
        ("GET", "/api/v1/admin/roles"),
        ("GET", "/api/v1/admin/approvals"),
        ("POST", f"/api/v1/admin/approvals/{FAKE_UUID}/approve"),
        ("POST", f"/api/v1/admin/approvals/{FAKE_UUID}/reject"),
        ("GET", "/api/v1/admin/approval-policy"),
        ("PUT", "/api/v1/admin/approval-policy"),
        ("GET", "/api/v1/admin/ldap"),
        ("PUT", "/api/v1/admin/ldap"),
        ("POST", "/api/v1/admin/ldap/test"),
        ("POST", f"/api/v1/rooms/{room_id}/rack-diagram-import/validate"),
        ("POST", f"/api/v1/rooms/{room_id}/rack-diagram-import/commit"),
        ("PUT", f"/api/v1/pdus/{FAKE_UUID}"),
        ("DELETE", f"/api/v1/pdus/{FAKE_UUID}?version=1"),
        ("GET", f"/api/v1/pdus/{FAKE_UUID}/sockets"),
        ("POST", f"/api/v1/pdus/{FAKE_UUID}/sockets"),
        ("PUT", f"/api/v1/pdu-sockets/{FAKE_UUID}"),
        ("DELETE", f"/api/v1/pdu-sockets/{FAKE_UUID}?version=1"),
        ("POST", f"/api/v1/pdu-sockets/{FAKE_UUID}/connection"),
    ]
    n_auth = 0
    for i, (m, p) in enumerate(endpoints, 1):
        short = p.rsplit("/", 1)[-1][:14].replace("?", "")
        call(f"S14-AUTH-{i:03d}-{m[:4]}-{short}", "S14", m, p, token=None,
             expect=(401, "UNAUTHORIZED"))
        n_auth += 1
    # PDU 连接删除单独
    call(f"S14-AUTH-{len(endpoints)+1:03d}-DELETE-connection", "S14", "DELETE",
         f"/api/v1/pdu-connections/{FAKE_UUID}", token=None,
         expect=(401, "UNAUTHORIZED"))
    n_auth += 1
    print(f"  auth_boundary: {n_auth} 用例已补")

    # ===== validation =====
    val_cases = [
        ("S14-VAL-DTYPE-EMPTY", "POST", "/api/v1/device-types", {"code": "", "name": "空"}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-DTYPE-LONG", "POST", "/api/v1/device-types", {"code": "X"*51, "name": "超长"}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-DTYPE-CAT", "POST", "/api/v1/device-types", {"code": P+"-BC", "name": "坏类", "category": "INVALID"}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-DEV-NOTYPE", "POST", "/api/v1/devices", {"code": P+"-NT", "name": "无类型"}, 404, "RESOURCE_NOT_FOUND"),
        ("S14-VAL-DEV-EMPTYNAME", "POST", "/api/v1/devices", {"typeId": tid, "code": P+"-EN", "name": ""}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-DC-EMPTY", "POST", "/api/v1/data-centers", {"code": "", "name": ""}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-DC-LONGNAME", "POST", "/api/v1/data-centers", {"code": P+"-LN", "name": "长"*151}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-ROOM-EMPTY", "POST", f"/api/v1/data-centers/{dc_id}/rooms", {"code": "", "name": ""}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-RACK-EMPTY", "POST", f"/api/v1/rooms/{room_id}/racks", {"code": "", "name": ""}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-USER-SHORTPW", "POST", "/api/v1/admin/users",
         {"username": P.lower()+"-sp", "displayName": "短", "password": "1", "roles": ["user"]}, 400, "INVALID_USER"),
        ("S14-VAL-USER-EMPTYNAME", "POST", "/api/v1/admin/users",
         {"username": "", "displayName": "空", "password": "Valid!123", "roles": ["user"]}, 400, "INVALID_USER"),
        ("S14-VAL-TPL-EMPTY", "POST", "/api/v1/rack-templates", {"code": "", "name": ""}, 400, "INVALID_RESOURCE"),
        ("S14-VAL-PDU-EMPTY", "POST", f"/api/v1/racks/{rack_id}/pdus", {"code": "", "name": ""}, 400, "INVALID_RESOURCE"),
    ]
    for cid, m, p, b, exp_st, exp_code in val_cases:
        call(cid, "S14", m, p, body=b, token=T(), expect=(exp_st, exp_code))
    call("S14-VAL-SOCKET-BADSTD", "S14", "POST", f"/api/v1/pdus/{FAKE_UUID}/sockets",
         {"socketNo": 99, "standard": "XX", "amperageA": 10}, token=T(),
         expect=([400, 404, 409], None), note="FAKE PDU→可能404")
    print(f"  validation: {len(val_cases)+1} 用例已补")

    # ===== conflict =====
    if room:
        call("S14-ROOM-DELETE-HASCHILDREN", "S14", "DELETE",
             f"/api/v1/rooms/{room['id']}?version={room['version']}",
             token=T(), expect=(409, "RESOURCE_HAS_CHILDREN"))
    if rack and room:
        call("S14-RACK-MOVE-SAMEROOM", "S14", "POST", f"/api/v1/racks/{rack['id']}/move",
             {"code": P+"-RKSM", "name": "同房迁移", "autoGenerateCode": False,
              "targetRoomId": room["id"], "version": rack["version"]},
             token=T(), expect=(400, "INVALID_RESOURCE"), note="同机房迁移实录")
    print("  conflict: 2 用例已补")

    # ===== pagination =====
    for cid, q in [
        ("S14-PAGE-1", "page=1&pageSize=1"),
        ("S14-PAGE-2", "page=2&pageSize=1"),
        ("S14-PAGE-FILTER-TYPE", f"page=1&pageSize=5&typeId={tid}"),
        ("S14-PAGE-FILTER-LIFE", "page=1&pageSize=5&lifecycleStatus=WAITING_RACK"),
        ("S14-PAGE-FILTER-SEARCH", "page=1&pageSize=5&search=GT-"),
        ("S14-PAGE-FILTER-COMBO", f"page=1&pageSize=5&search=GT-&typeId={tid}&lifecycleStatus=WAITING_RACK"),
    ]:
        call(f"S14-{cid}", "S14", "GET", f"/api/v1/devices?{q}", token=T(), expect=(200, "SUCCESS"))
    print("  pagination: 6 用例已补")

    # ===== invariant（写后读验证字段）=====
    parsed, _ = setup_call("SETUP-S14-INV-DEV", "S14", "POST", "/api/v1/devices",
                           {"typeId": tid, "code": P+"-INV", "name": "不变量验证",
                            "heightU": 3, "managementIp": "10.0.0.1", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    inv = data_of(parsed)
    if inv:
        parsed, _ = call("S14-INV-DEV-READBACK", "S14", "GET", f"/api/v1/devices/{inv['id']}",
                         token=T(), expect=(200, "SUCCESS"))
        d = data_of(parsed) or {}
        ok = d.get("heightU") == 3 and d.get("managementIp") == "10.0.0.1"
        CASES.append({"caseId": "S14-INV-DEV-VERIFY", "scenario": "S14", "kind": "TEST",
                      "status": "PASS" if ok else "FAIL", "httpStatus": 200,
                      "actualCode": f"h={d.get('heightU')} ip={d.get('managementIp')}",
                      "expectedCode": "h=3 ip=10.0.0.1", "note": "写后读不变量",
                      "durationMs": 0, "requestId": ""})
    print("  invariant: 1 用例已补")

    # ===== room/rack DELETE 有子级 conflict =====
    parsed, _ = setup_call("SETUP-S14-RK9", "S14", "POST", f"/api/v1/rooms/{room_id}/racks",
                           {"code": P+"-RK9", "name": "临时删除用", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    rk9 = data_of(parsed)
    if rk9:
        parsed, _ = call("S14-RACK9-UPDATE", "S14", "PUT",
                         f"/api/v1/racks/{rk9['id']}?version={rk9['version']}",
                         {"code": rk9["code"], "name": rk9["name"]+"改", "version": rk9["version"]},
                         token=T(), expect=(200, "SUCCESS"))
        rk9b = data_of(parsed) or rk9
        call("S14-RACK9-DELETE", "S14", "DELETE",
             f"/api/v1/racks/{rk9['id']}?version={rk9b['version']}",
             token=T(), expect=(200, "SUCCESS"))
    parsed, _ = setup_call("SETUP-S14-RM9", "S14", "POST", f"/api/v1/data-centers/{dc_id}/rooms",
                           {"code": P+"-RM9", "name": "临时删除用", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    rm9 = data_of(parsed)
    if rm9:
        parsed, _ = call("S14-ROOM9-UPDATE", "S14", "PUT",
                         f"/api/v1/rooms/{rm9['id']}?version={rm9['version']}",
                         {"code": rm9["code"], "name": rm9["name"]+"改", "version": rm9["version"]},
                         token=T(), expect=(200, "SUCCESS"))
        rm9b = data_of(parsed) or rm9
        call("S14-ROOM9-DELETE", "S14", "DELETE",
             f"/api/v1/rooms/{rm9['id']}?version={rm9b['version']}",
             token=T(), expect=(200, "SUCCESS"))
    print("  room/rack update+delete: 4 用例已补")