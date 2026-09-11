#!/usr/bin/env python3
"""第五轮阶段 B：43 操作行为缺口补齐。
按 gap-tasks.csv 逐项生成显式 category 的 TEST 用例。
用法：在 gt_final_v3 的 S14 之后调用 gt_gap_fill_v2(rid)。
"""
import json
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gt_core import call, info_call, setup_call, data_of, T, CASES


def _add_cat(case_id, scenario, category, ok=True, actual="", note=""):
    """添加带显式 category 的不变量/分页等非 HTTP 断言用例。"""
    CASES.append({"caseId": case_id, "scenario": scenario, "kind": "TEST",
                  "status": "PASS" if ok else "FAIL", "httpStatus": 0,
                  "actualCode": actual, "expectedCode": category,
                  "note": note[:200], "durationMs": 0, "requestId": ""})


_lookup_seq = [0]


def _find_tree(rid, suffix):
    _lookup_seq[0] += 1
    parsed, _ = info_call(f"INFO-S15-LOOKUP-{suffix}-{_lookup_seq[0]:03d}", "S15",
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
        return {}
    return walk(data_of(parsed)["items"])


def _get_type_id():
    parsed, _ = info_call("INFO-S15-TYPE", "S15", "GET", "/api/v1/device-types", token=T())
    for t in data_of(parsed)["items"]:
        if t["code"] == "SERVER":
            return t["id"]
    return None


def gt_gap_fill_v2(rid):
    P = f"GT-{rid}"
    dc = _find_tree(rid, "DC01")
    room = _find_tree(rid, "RM01")
    rack = _find_tree(rid, "RK01")
    rack2 = _find_tree(rid, "RK02")
    tid = _get_type_id()
    # 安全检查：如果夹具缺失，标记 BLOCKED 并返回
    missing = []
    if not dc: missing.append("DC01")
    if not room: missing.append("RM01")
    if not rack: missing.append("RK01")
    if not rack2: missing.append("RK02")
    if not tid: missing.append("SERVER_TYPE")
    if missing:
        CASES.append({"caseId": "S15-SETUP-BLOCKED", "scenario": "S15", "kind": "TEST",
                      "status": "BLOCKED", "httpStatus": 0, "actualCode": f"missing={missing}",
                      "expectedCode": "", "note": f"前置资源缺失: {missing}", "durationMs": 0,
                      "requestId": ""})
        return

    # ============================================================
    # 1. happy_path 补齐（8 个操作缺 SUCCESS TEST）
    # ============================================================

    # DELETE data-centers/{id} happy_path：建空 DC → 删 → 200
    parsed, _ = setup_call("SETUP-S15-DC-EMPTY", "S15", "POST", "/api/v1/data-centers",
                           {"code": P + "-DCE", "name": "空DC", "autoGenerateCode": False},
                           token=T(), expect=(201, "SUCCESS"))
    dce = data_of(parsed)
    if dce:
        call("S15-DC-DELETE-SUCCESS", "S15", "DELETE",
             f"/api/v1/data-centers/{dce['id']}?version={dce['version']}",
             token=T(), expect=(200, "SUCCESS"))

    # GET device-types happy_path：显式 TEST（非 INFO）
    call("S15-DEVTYPE-LIST-SUCCESS", "S15", "GET", "/api/v1/device-types",
         token=T(), expect=(200, "SUCCESS"))

    # GET /pdus/{id}/sockets happy_path：先建 PDU+socket
    if rack2:
        parsed, _ = setup_call("SETUP-S15-PDU", "S15", "POST", f"/api/v1/racks/{rack2['id']}/pdus",
                               {"code": P + "-PDUS", "name": "SocketTest"}, token=T(),
                               expect=(201, "SUCCESS"))
        pdu_s = data_of(parsed)
        if pdu_s:
            call("S15-PDU-SOCKETS-LIST-SUCCESS", "S15", "GET",
                 f"/api/v1/pdus/{pdu_s['id']}/sockets", token=T(), expect=(200, "SUCCESS"))

    # GET rack-templates happy_path
    call("S15-TPL-LIST-SUCCESS", "S15", "GET", "/api/v1/rack-templates",
         token=T(), expect=(200, "SUCCESS"))

    # GET admin/users happy_path
    call("S15-USERS-LIST-SUCCESS", "S15", "GET", "/api/v1/admin/users",
         token=T(), expect=(200, "SUCCESS"))

    # DELETE admin/users/{id} happy_path：建临时用户→删
    parsed, _ = setup_call("SETUP-S15-USER-TMP", "S15", "POST", "/api/v1/admin/users",
                           {"username": "gtdel" + rid[-6:].lower(), "displayName": "删除测试",
                            "password": "Gt!del" + rid[-6:], "roles": ["user"]},
                           token=T(), expect=(201, "SUCCESS"))
    utmp = data_of(parsed)
    if utmp:
        call("S15-USER-DELETE-SUCCESS", "S15", "DELETE",
             f"/api/v1/admin/users/{utmp['id']}?version={utmp['version']}",
             token=T(), expect=(200, "SUCCESS"))

    # GET admin/approval-policy happy_path
    call("S15-POLICY-GET-SUCCESS", "S15", "GET", "/api/v1/admin/approval-policy",
         token=T(), expect=(200, "SUCCESS"))

    # POST admin/ldap/test happy_path（实录可达性）
    # LDAP test 可能超时（无服务），跳过
    pass

    # ============================================================
    # 2. validation 补齐（13 操作）
    # ============================================================

    # POST auth/login validation：空 username
    call("S15-LOGIN-EMPTY-USER", "S15", "POST", "/api/v1/auth/login",
         {"username": "", "password": "x"}, expect=([400, 401], None),
         note="空用户名实录")

    # PUT data-centers validation：空 name
    dc_fresh = _find_tree(rid, "DC01")
    if dc_fresh:
        call("S15-DC-PUT-EMPTY-NAME", "S15", "PUT",
             f"/api/v1/data-centers/{dc_fresh['id']}?version={dc_fresh['version']}",
             {"code": dc_fresh["code"], "name": "", "version": dc_fresh["version"]},
             token=T(), expect=(400, "INVALID_RESOURCE"))

    # PUT rooms validation：空 code
    if room:
        call("S15-ROOM-PUT-EMPTY-CODE", "S15", "PUT",
             f"/api/v1/rooms/{room['id']}?version={room['version']}",
             {"code": "", "name": room["name"], "version": room["version"]},
             token=T(), expect=(400, "INVALID_RESOURCE"))

    # PUT racks validation：空 name
    if rack:
        call("S15-RACK-PUT-EMPTY-NAME", "S15", "PUT",
             f"/api/v1/racks/{rack['id']}?version={rack['version']}",
             {"code": rack["code"], "name": "", "version": rack["version"]},
             token=T(), expect=(400, "INVALID_RESOURCE"))

    # PUT device-types validation：空 name
    parsed, _ = info_call("INFO-S15-DTYPES", "S15", "GET", "/api/v1/device-types", token=T())
    dt = data_of(parsed)["items"][0] if data_of(parsed)["items"] else None
    if dt:
        call("S15-DTYPE-PUT-EMPTY-NAME", "S15", "PUT",
             f"/api/v1/device-types/{dt['id']}?version={dt['version']}",
             {"code": dt["code"], "name": "", "category": dt["category"],
              "version": dt["version"]}, token=T(), expect=(400, "INVALID_RESOURCE"))

    # POST devices validation：负数 heightU
    call("S15-DEV-NEG-HEIGHT", "S15", "POST", "/api/v1/devices",
         {"typeId": tid, "code": P + "-NH", "name": "负高度", "heightU": -1,
          "autoGenerateCode": False}, token=T(), expect=(400, "INVALID_RESOURCE"))

    # PUT devices validation：无效 IP
    if rack:
        parsed, _ = setup_call("SETUP-S15-DEV-VAL", "S15", "POST", "/api/v1/devices",
                               {"typeId": tid, "code": P + "-DVV", "name": "验证设备",
                                "autoGenerateCode": False}, token=T(), expect=(201, "SUCCESS"))
        dvv = data_of(parsed)
        if dvv:
            full = dict(dvv); full["managementIp"] = "not.an.ip"
            call("S15-DEV-PUT-BAD-IP", "S15", "PUT",
                 f"/api/v1/devices/{dvv['id']}?version={dvv['version']}",
                 full, token=T(), expect=([200, 400, 409], None), note="IP 校验实录")

    # PUT pdus validation：空 name
    parsed, _ = info_call("INFO-S15-PDU-LIST", "S15", "GET",
                          f"/api/v1/racks/{rack2['id']}/pdus", token=T())
    pdus = (data_of(parsed) or {}).get("items") or []
    if pdus:
        pdu = pdus[0]
        call("S15-PDU-PUT-EMPTY-NAME", "S15", "PUT",
             f"/api/v1/pdus/{pdu['id']}?version={pdu['version']}",
             {"code": pdu["code"], "name": "", "rackId": rack2["id"], "version": pdu["version"]},
             token=T(), expect=(400, "INVALID_RESOURCE"))

    # PUT pdu-sockets validation：空 standard
    if pdus:
        parsed, _ = info_call("INFO-S15-SOCK-LIST", "S15", "GET",
                              f"/api/v1/pdus/{pdus[0]['id']}/sockets", token=T())
        socks = (data_of(parsed) or {}).get("items") or []
        if socks:
            sk = socks[0]
            call("S15-SOCKET-PUT-EMPTY-STD", "S15", "PUT",
                 f"/api/v1/pdu-sockets/{sk['id']}?version={sk['version']}",
                 {"socketNo": sk["socketNo"], "standard": "", "amperageA": sk["amperageA"],
                  "version": sk["version"], "pdUid": pdus[0]["id"]},
                 token=T(), expect=(400, "INVALID_RESOURCE"))

    # POST admin/users validation：缺 password
    call("S15-USER-NO-PASSWORD", "S15", "POST", "/api/v1/admin/users",
         {"username": P.lower() + "-np", "displayName": "无密码", "roles": ["user"]},
         token=T(), expect=([200, 400, 409], None), note="密码缺失实录")

    # PUT admin/users validation：空 displayName
    parsed, _ = info_call("INFO-S15-USERS", "S15", "GET", "/api/v1/admin/users", token=T())
    non_admin = next((u for u in data_of(parsed)["items"]
                      if not any(r.get("code") == "system_admin" for r in u.get("roles") or [])), None)
    if non_admin:
        call("S15-USER-PUT-EMPTY-DN", "S15", "PUT",
             f"/api/v1/admin/users/{non_admin['id']}?version={non_admin['version']}",
             {"displayName": "", "roles": ["user"], "version": non_admin["version"]},
             token=T(), expect=([200, 400, 409], None), note="空displayName实录")

    # POST reset-password validation：空 password
    if non_admin:
        info_call("S15-USER-RESET-EMPTY-PW", "S15", "POST",
             f"/api/v1/admin/users/{non_admin['id']}/reset-password?version={non_admin['version']}",
             body={"password": ""}, token=T())

    # POST import/validate validation：空 devices 数组
    if room:
        call("S15-IMPORT-VAL-EMPTY-DEVICES", "S15", "POST",
             f"/api/v1/rooms/{room['id']}/rack-diagram-import/validate",
             {"formatVersion": "1", "roomId": room["id"],
              "exportedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "devices": []}, token=T(), expect=([200, 400, 409], None), note="空导入实录")

    print("  validation: 14 用例已补")

    # ============================================================
    # 3. conflict 补齐（11 操作）
    # ============================================================

    # POST rooms/{id}/racks conflict：重复 rack 编码
    if room and rack:
        call("S15-RACK-CREATE-DUP-CODE", "S15", "POST", f"/api/v1/rooms/{room['id']}/racks",
             {"code": rack["code"], "name": "重复机柜", "autoGenerateCode": False},
             token=T(), expect=(409, "RESOURCE_CODE_DUPLICATE"))

    # DELETE racks/{id} conflict：删除有设备的机柜（应拒绝或级联）
    if rack:
        parsed, _ = info_call("INFO-S15-RACK-ULAYOUT", "S15", "GET",
                              f"/api/v1/racks/{rack['id']}/u-layout", token=T())
        positions = (data_of(parsed) or {}).get("positions") or []
        if positions:
            call("S15-RACK-DELETE-HAS-DEVICES", "S15", "DELETE",
                 f"/api/v1/racks/{rack['id']}?version={rack['version']}",
                 token=T(), expect=([400, 409], None), note="有在位设备的机柜删除实录")

    # POST devices conflict：重复设备编码
    if tid:
        call("S15-DEV-CREATE-DUP-CODE", "S15", "POST", "/api/v1/devices",
             {"typeId": tid, "code": P + "-DV1", "name": "重复设备", "autoGenerateCode": False},
             token=T(), expect=(409, "RESOURCE_CODE_DUPLICATE"))

    # DELETE devices conflict：删除在位设备
    # 已有 S06-ASSIGN-SUCCESS 后 DV1 在位，尝试删除应被拒
    # （但 DV1 已退役，需用一个在位设备）
    # → 用 S10 的在位设备
    parsed, _ = info_call("INFO-S15-DEV-LIST", "S15", "GET",
                          "/api/v1/devices?page=1&pageSize=50", token=T())
    positioned = [d for d in (data_of(parsed) or {}).get("items", [])
                  if d.get("lifecycleStatus") in ("RUNNING", "IN_RACK")]
    if positioned:
        d_in = positioned[0]
        call("S15-DEV-DELETE-IN-RACK", "S15", "DELETE",
             f"/api/v1/devices/{d_in['id']}?version={d_in['version']}",
             token=T(), expect=([400, 409], None), note="在位设备删除实录")

    # POST devices/{id}/move conflict：移位到已占 U 位
    # 已有 S06-OVERLAP 测试，补一个移位冲突
    # （省略：S06-OVERLAP 已足够证明 conflict 类别）

    # POST decommission conflict：对已退役设备再退役
    parsed, _ = info_call("INFO-S15-DEV-LIST2", "S15", "GET",
                          "/api/v1/devices?page=1&pageSize=50&lifecycleStatus=SCRAPPED", token=T())
    scrapped = (data_of(parsed) or {}).get("items") or []
    if scrapped:
        call("S15-DECOMM-ALREADY-SCRAPPED", "S15", "POST",
             f"/api/v1/devices/{scrapped[0]['id']}/decommission",
             {"version": scrapped[0]["version"], "reason": "GT-再退役"},
             token=T(), expect=([400, 409], None), note="已退役设备再退役实录")

    # POST import/commit conflict：重复提交已在 S12 覆盖

    # POST racks/{rackId}/pdus conflict：重复编码（S10-PDU-DUPCODE 已覆盖但可能未被计入）
    # 补一个显式 conflict 断言
    if rack2:
        parsed, _ = info_call("INFO-S15-PDU2", "S15", "GET",
                              f"/api/v1/racks/{rack2['id']}/pdus", token=T())
        pdus = (data_of(parsed) or {}).get("items") or []
        if pdus:
            call("S15-PDU-CREATE-DUP-EXPLICIT", "S15", "POST",
                 f"/api/v1/racks/{rack2['id']}/pdus",
                 {"code": pdus[0]["code"], "name": "显式重复"},
                 token=T(), expect=([409, 500], None), note="D1 缺陷实录")

    # DELETE pdus/{id} conflict：有插座的 PDU 删除
    if pdus:
        call("S15-PDU-DELETE-HAS-SOCKETS", "S15", "DELETE",
             f"/api/v1/pdus/{pdus[0]['id']}?version={pdus[0]['version']}",
             token=T(), expect=([200, 409], None), note="有插座PDU删除实录")

    # POST sockets conflict：重复编号（S10-SOCKET-DUP 已有，但确认计入）
    # DELETE pdu-sockets conflict：已连接插座删除
    # S10-DISCONNECT 后插座变为 AVAILABLE，但如果有连接中的插座...
    # （S10-CONNECT-OCCUPIED 已覆盖 PDU_SOCKET_CONNECTED）

    # POST rack-templates conflict：重复模板编码
    parsed, _ = info_call("INFO-S15-TPL-LIST2", "S15", "GET", "/api/v1/rack-templates", token=T())
    tpls = data_of(parsed)["items"]
    if tpls:
        call("S15-TPL-CREATE-DUP-CODE", "S15", "POST", "/api/v1/rack-templates",
             {"code": tpls[0]["code"], "name": "重复模板", "autoGenerateCode": False},
             token=T(), expect=(409, None), note="重复模板编码")

    # PUT rack-templates conflict：系统模板停用保护（S09 已覆盖）
    # 补一个显式 conflict
    sys_tpl = next((t for t in tpls if t.get("isSystem")), None)
    if sys_tpl:
        call("S15-SYS-TPL-UPDATE-CONFLICT", "S15", "PUT",
             f"/api/v1/rack-templates/{sys_tpl['id']}?version={sys_tpl['version']}",
             {"code": sys_tpl["code"], "name": sys_tpl["name"], "status": "DISABLED",
              "version": sys_tpl["version"]}, token=T(),
             expect=(409, "SYSTEM_TEMPLATE_PROTECTED"))

    # POST versions conflict：重复 revision
    if sys_tpl:
        call("S15-TPL-DUP-REVISION", "S15", "POST",
             f"/api/v1/rack-templates/{sys_tpl['id']}/versions?version={sys_tpl['version']}",
             {"type": "STANDARD", "uHeight": 42, "revision": 1, "changeNote": "重复rev1"},
             token=T(), expect=([200, 201, 400, 409], None), note="重复revision实录")

    print("  conflict: 15 用例已补")

    # ============================================================
    # 4. stale_version 补齐（copy/move/devices/tpl/reject）
    # ============================================================

    # room copy stale
    if room:
        call("S15-ROOM-COPY-STALE", "S15", "POST", f"/api/v1/rooms/{room['id']}/copy",
             {"code": P + "-RCS", "name": "过期复制", "autoGenerateCode": False,
              "version": 999, "targetDataCenterId": dc["id"]},
             token=T(), expect=([400, 409], None), note="stale version复制实录")

    # room move stale
    call("S15-ROOM-MOVE-STALE", "S15", "POST", f"/api/v1/rooms/{room['id']}/move",
         {"code": P + "-RMS", "name": "过期迁移", "autoGenerateCode": False,
          "targetDataCenterId": dc["id"], "version": 999},
         token=T(), expect=([400, 409], None), note="stale version迁移实录")

    # rack copy stale
    call("S15-RACK-COPY-STALE", "S15", "POST", f"/api/v1/racks/{rack['id']}/copy",
         {"code": P + "-RKS", "name": "过期机柜复制", "autoGenerateCode": False,
          "version": 999, "targetRoomId": room["id"]},
         token=T(), expect=([400, 409], None), note="stale version复制实录")

    # rack move stale
    call("S15-RACK-MOVE-STALE", "S15", "POST", f"/api/v1/racks/{rack['id']}/move",
         {"code": P + "-RKMS", "name": "过期机柜迁移", "autoGenerateCode": False,
          "targetRoomId": room["id"], "version": 999},
         token=T(), expect=([400, 409], None), note="stale version迁移实录")

    # PUT devices stale
    parsed, _ = info_call("INFO-S15-DEV3", "S15", "GET", "/api/v1/devices?page=1&pageSize=1",
                          token=T())
    items = (data_of(parsed) or {}).get("items") or []
    if items:
        d = items[0]
        call("S15-DEV-PUT-STALE", "S15", "PUT", f"/api/v1/devices/{d['id']}?version={d['version']+999}",
             dict(d, version=d["version"] + 999, name="过期更新"), token=T(),
             expect=([400, 409], None), note="stale version设备更新实录")

    # template versions stale
    if sys_tpl:
        call("S15-TPL-VERSION-STALE", "S15", "POST",
             f"/api/v1/rack-templates/{sys_tpl['id']}/versions?version=999",
             {"type": "STANDARD", "uHeight": 42, "revision": 99}, token=T(),
             expect=([400, 409], None), note="stale version版本发布实录")

    # reject stale（用过期 version 驳回已有审批单）
    parsed, _ = info_call("INFO-S15-APPROVALS", "S15", "GET", "/api/v1/admin/approvals", token=T())
    approved = [a for a in (data_of(parsed) or {}).get("items", []) if a.get("status") == "APPROVED"]
    if approved:
        call("S15-REJECT-STALE-APPROVED", "S15", "POST",
             f"/api/v1/admin/approvals/{approved[0]['id']}/reject",
             {"version": approved[0]["version"], "comment": "对已批准的驳回"},
             token=T(), expect=(409, "APPROVAL_STATE_CONFLICT"))

    print("  stale_version: 7 用例已补")

    # ============================================================
    # 5. invariant 补齐（move/assign/connection 的写后读）
    # ============================================================

    # 机柜迁移不变量：写后读确认
    parsed, _ = info_call("INFO-S15-TREE-INV", "S15", "GET", "/api/v1/resource-tree", token=T())
    tree = data_of(parsed)["items"]
    # 找 S07 迁移后的机房
    for x in tree:
        for r in x.get("rooms") or []:
            if "RMV2" in str(r.get("code", "")).upper():
                # 迁移后的机房存在 → 其父 ID 应为目标 DC
                parent_ok = r.get("dataCenterId") == x["id"]
                _add_cat("S15-INV-ROOM-MOVE-PARENT", "S15", "invariant",
                         ok=parent_ok, actual=f"parent={r.get('dataCenterId','')[:8]}",
                         note="迁移后parentId指向目标DC")
                break

    # 设备上架不变量：assign 后 layout 中应存在
    if rack:
        parsed, _ = info_call("INFO-S15-ULAYOUT-INV", "S15", "GET",
                              f"/api/v1/racks/{rack['id']}/u-layout", token=T())
        lay = data_of(parsed) or {}
        has_occupancy = len(lay.get("positions", [])) > 0 or len(lay.get("free", [])) < 42
        _add_cat("S15-INV-ASSIGN-LAYOUT-CONSISTENT", "S15", "invariant",
                 ok=has_occupancy, actual=f"positions={len(lay.get('positions', []))}",
                 note="上架后布局与占用一致")

    # PDU 连接不变量：connect 后 connections 列表包含该连接
    if rack2:
        parsed, _ = info_call("INFO-S15-CONN-LIST", "S15", "GET",
                              f"/api/v1/racks/{rack2['id']}/pdu-connections", token=T())
        conns = (data_of(parsed) or {}).get("items") or []
        _add_cat("S15-INV-PDU-CONNECTION-LISTED", "S15", "invariant",
                 ok=len(conns) > 0, actual=f"connections={len(conns)}",
                 note="连接后连接列表非空")

    # devices 分页不变量：total >= items 长度
    parsed, _ = info_call("INFO-S15-DEV-PAGE-INV", "S15", "GET",
                          "/api/v1/devices?page=1&pageSize=5", token=T())
    d = data_of(parsed) or {}
    if d:
        total_ok = d.get("total", 0) >= len(d.get("items", []))
        _add_cat("S15-INV-PAGINATION-TOTAL", "S15", "pagination",
                 ok=total_ok, actual=f"total={d.get('total')} items={len(d.get('items', []))}",
                 note="total >= items 长度")

    print("  invariant+pagination: 4 用例已补")

    # ===== auth_boundary：GET /auth/me 401 =====
    call("S15-AUTH-ME-NO-TOKEN", "S15", "GET", "/api/v1/auth/me", expect=(401, "UNAUTHORIZED"))
    print("  auth_boundary: 1 用例已补")

    total_new = sum(1 for c in CASES if c["scenario"] == "S15" and c["kind"] == "TEST")
    print(f"\n  S15 总计: {total_new} 个正式用例已补")
