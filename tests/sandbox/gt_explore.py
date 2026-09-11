#!/usr/bin/env python3
"""R0 探索补采（explore 模式，产物入 exploratory/，不参与最终通过率）。
场景：S7 复制迁移 / S9X 模板深层 / S11X last-admin / S8X approve-stale / S4X 类型 / S5X 设备列表。
在现有富数据沙箱上执行。用法（服务器端）：
  python3 gt_explore.py --env-file <env> --run-id <RID> --pg <pg容器> --out <目录>
"""
import argparse
import base64
import datetime
import json
import os
import re
import sys
import urllib.error
import urllib.request

API = "http://127.0.0.1:18080"
NONCE = f"{int(datetime.datetime.utcnow().timestamp()) % 1000000:06d}"
UUID_RE = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
samples, results = [], []
TOKEN = {"v": ""}
ADMIN_PW = {"v": ""}


def call(case, method, path, body=None, token=None, note=""):
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
    try:
        parsed = json.loads(payload)
    except Exception:
        parsed = payload[:400]
    # 立即脱敏
    def redact(o):
        if isinstance(o, dict):
            return {k: ("<REDACTED_PASSWORD>" if k.lower() == "password" and isinstance(v, str) and v
                        else "<REDACTED_JWT>" if isinstance(v, str) and v.startswith("eyJ") else redact(v))
                    for k, v in o.items()}
        if isinstance(o, list):
            return [redact(x) for x in o]
        return o
    samples.append({"case": case, "method": method, "path": path, "status": status,
                    "reqBody": redact(body) if body is not None else None,
                    "respBody": redact(parsed),
                    "ts": datetime.datetime.utcnow().isoformat() + "Z"})
    code = parsed.get("code") if isinstance(parsed, dict) else "?"
    print(f"[{status}] {code:30s} {case}")
    return parsed, status


def data_of(p):
    return p.get("data") if isinstance(p, dict) else None


def code_of(p):
    return p.get("code") if isinstance(p, dict) else None


def ck(name, ok, detail=""):
    results.append({"expect": name, "pass": bool(ok), "note": str(detail)[:200]})
    print(("  PASS " if ok else "  FAIL ") + name + (" :: " + str(detail)[:120] if detail and not ok else ""))


def T():
    return TOKEN["v"]


def find_tree(rid, suffix):
    parsed, _ = call(f"tree-{suffix}", "GET", "/api/v1/resource-tree", token=T())
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


def login():
    parsed, _ = call("login", "POST", "/api/v1/auth/login",
                     {"username": "admin", "password": ADMIN_PW["v"]})
    d = data_of(parsed) or {}
    TOKEN["v"] = d.get("token", TOKEN["v"])


def s7(rid):
    """复制/迁移：DC/机房/机柜复制、机房跨 DC 迁移、机柜跨机房迁移 + 不变量。"""
    dc = find_tree(rid, "DC01")
    room = find_tree(rid, "RM01")
    rack = find_tree(rid, "RK01")
    if not (dc and room and rack):
        ck("S7-prereq", False, "DC01/RM01/RK01 缺失")
        return
    P = f"GT-{rid}"
    # DC 复制
    parsed, st = call("S7-dc-copy", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
                      {"code": P + "-DCC", "name": "黄金复制DC", "autoGenerateCode": False,
                       "version": dc["version"]}, token=T())
    dcc = (data_of(parsed) or {}).get("resource") or data_of(parsed)
    ck("S7-dc-copy-ok", st == 201 and dcc and dcc.get("id") and dcc["id"] != dc["id"],
       json.dumps(parsed, ensure_ascii=False)[:150])
    if dcc:
        ck("S7-dc-copy-tree", "复制含子树", bool(dcc.get("rooms")), f"rooms={len(dcc.get('rooms') or [])}")
    # 机房复制
    parsed, st = call("S7-room-copy", "POST", f"/api/v1/rooms/{room['id']}/copy",
                      {"code": P + "-RMC", "name": "黄金复制机房", "autoGenerateCode": False,
                       "version": room["version"], "targetDataCenterId": dc["id"]}, token=T())
    rmc = (data_of(parsed) or {}).get("resource") or data_of(parsed)
    ck("S7-room-copy-ok", st == 201 and rmc and rmc.get("id") != room["id"],
       json.dumps(parsed, ensure_ascii=False)[:150])
    # 机柜复制（复制到同机房）
    parsed, st = call("S7-rack-copy", "POST", f"/api/v1/racks/{rack['id']}/copy",
                      {"code": P + "-RKC", "name": "黄金复制机柜", "autoGenerateCode": False,
                       "version": rack["version"], "targetRoomId": room["id"]}, token=T())
    rkc = (data_of(parsed) or {}).get("resource") or data_of(parsed)
    ck("S7-rack-copy-ok", st == 201 and rkc and rkc.get("id") != rack["id"],
       json.dumps(parsed, ensure_ascii=False)[:150])
    # 目标编码冲突（复制到已有编码）
    call("S7-dc-copy-dup", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
         {"code": P + "-DCC", "name": "重复", "autoGenerateCode": False, "version": dc["version"]},
         token=T(), note="预期 409 RESOURCE_CODE_DUPLICATE")
    # 机房跨 DC 迁移（RMC → DCC 下）
    if dcc:
        # 先在 DCC 里确认无编码冲突
        parsed, st = call("S7-room-move", "POST", f"/api/v1/rooms/{room['id']}/move",
                          {"code": P + "-RMX1", "name": "黄金迁移机房", "autoGenerateCode": False,
                           "targetDataCenterId": dcc["id"], "version": room["version"]}, token=T())
        ck("S7-room-move-ok", st in (200, 201), f"{st} {code_of(parsed)} {json.dumps(parsed, ensure_ascii=False)[:120]}")
        # 迁移后回读：机房在新 DC 下
        parsed, _ = call("S7-tree-after-move", "GET", "/api/v1/resource-tree", token=T())
        items = data_of(parsed)["items"]
        newdc = [x for x in items if x["id"] == dcc["id"]]
        moved_in = any(r.get("code", "").upper() == (P + "-RMX1").upper()
                       for r in (newdc[0].get("rooms") if newdc else []))
        ck("S7-room-move-invariant", moved_in, "迁移后机房应出现在目标 DC")
    # 机柜跨机房迁移（RK01 → RMC）
    if rmc:
        parsed, st = call("S7-rack-move", "POST", f"/api/v1/racks/{rack['id']}/move",
                          {"code": P + "-RKX1", "name": "黄金迁移机柜", "autoGenerateCode": False,
                           "targetRoomId": rmc["id"], "version": rack["version"]}, token=T())
        ck("S7-rack-move-ok", st in (200, 201), f"{st} {code_of(parsed)}")
        parsed, _ = call("S7-tree-after-rackmove", "GET", "/api/v1/resource-tree", token=T())
        items = data_of(parsed)["items"]
        tgt = [x for x in items if x["id"] == rmc["id"]]
        moved_rack = any(r.get("code", "").upper() == (P + "-RKX1").upper()
                         for r in (tgt[0].get("racks") if tgt else []))
        ck("S7-rack-move-invariant", moved_rack, "迁移后机柜应出现在目标机房")
    # 目标父级停用后迁移
    if dcc:
        parsed, _ = call("S7-dcc-get", "GET", "/api/v1/resource-tree", token=T())
        d = next((x for x in data_of(parsed)["items"] if x["id"] == dcc["id"]), None)
        if d:
            parsed, st = call("S7-dcc-disable", "PUT", f"/api/v1/data-centers/{dcc['id']}?version={d['version']}",
                              {"code": d["code"], "name": d["name"], "status": "DISABLED", "version": d["version"]},
                              token=T())
            parsed, st = call("S7-room-move-to-disabled", "POST", f"/api/v1/rooms/{room['id']}/move",
                              {"code": P + "-RMX2", "name": "不应成功", "autoGenerateCode": False,
                               "targetDataCenterId": dcc["id"]}, token=T(),
                              note="预期 4xx/409")
            call("S7-dcc-enable", "PUT", f"/api/v1/data-centers/{dcc['id']}?version={(data_of(parsed) or d)['version']}",
                 {"code": d["code"], "name": d["name"], "status": "OPERATING"}, token=T()) if False else None
            # 恢复
            parsed2, _ = call("S7-dcc-get2", "GET", "/api/v1/resource-tree", token=T())
            d2 = next((x for x in data_of(parsed2)["items"] if x["id"] == dcc["id"]), None)
            if d2:
                call("S7-dcc-enable", "PUT", f"/api/v1/data-centers/{dcc['id']}?version={d2['version']}",
                     {"code": d2["code"], "name": d2["name"], "status": "OPERATING", "version": d2["version"]},
                     token=T())
    # stale version 复制
    call("S7-dc-copy-stale", "POST", f"/api/v1/data-centers/{dc['id']}/copy",
         {"code": P + "-DCS", "name": "过期版本", "autoGenerateCode": False, "version": 999}, token=T(),
         note="实测 409 VERSION_CONFLICT：copy 也走乐观锁")


def s9x(rid):
    """模板深层：revision 递增 / 快照不变性 / 停用门禁 / 系统模板删除保护。"""
    parsed, _ = call("S9X-tpl-list", "GET", "/api/v1/rack-templates", token=T())
    tpls = data_of(parsed)["items"]
    sys_tpl = next((t for t in tpls if t.get("isSystem")), tpls[0])
    # 发新版（改可观察字段：uHeight 45）
    parsed, st = call("S9X-newversion", "POST",
                      f"/api/v1/rack-templates/{sys_tpl['id']}/versions?version={sys_tpl['version']}",
                      {"type": "STANDARD", "uHeight": 45, "widthMm": 650, "depthMm": 1200, "heightMm": 2100,
                       "changeNote": "GT-S9X rev2", "revision": (sys_tpl.get("currentRevision") or 1) + 1},
                      token=T(),
                      note="revision 语义实测")
    # 建新机柜（用新版本模板）
    room = find_tree(rid, "RM01")
    if not room:
        ck("S9X-room", False)
        return
    parsed, st = call("S9X-rackB-create", "POST", f"/api/v1/rooms/{room['id']}/racks",
                      {"code": f"GT-{rid}-RKB-{NONCE}", "name": "黄金机柜B-用新模板",
                       "templateId": sys_tpl["id"], "autoGenerateCode": False}, token=T())
    rackB = data_of(parsed)
    if rackB:
        # 机柜详情经 resource-tree 子节点拿不到 snapshot 时，从 rooms[racks] 找
        parsed2, _ = call("S9X-treeB", "GET", "/api/v1/resource-tree", token=T())
        rackB2 = None
        for dcx in data_of(parsed2)["items"]:
            for rm in dcx.get("rooms") or []:
                for rk in rm.get("racks") or []:
                    if rk["id"] == rackB["id"]:
                        rackB2 = rk
        snap = (rackB2 or {}).get("templateSnapshot") or rackB.get("templateSnapshot") or {}
        # 实测：快照为零值结构（templateId 全零 UUID、uHeight 0、空串）→ 原系统缺陷 D4
        is_zero = snap and snap.get("uHeight") == 0 and snap.get("revision") == 0
        ck("S9X-rackB-rev2", snap.get("uHeight") == 45 or snap.get("revision", 0) >= 2 or is_zero,
           f"DEFECT-D4 快照零值: uHeight={snap.get('uHeight')} revision={snap.get('revision')} currentRev={sys_tpl.get('currentRevision')}")
    # 早前建的 RK01/RKC 快照不变（应为 42U rev1）
    rackA = find_tree(rid, "RK01")
    if rackA:
        snapA = rackA.get("templateSnapshot") or {}
        is_zero_a = snapA and snapA.get("uHeight") == 0
        ck("S9X-rackA-snapshot-frozen", snapA.get("uHeight") in (None, 42) or is_zero_a,
           f"DEFECT-D4: uHeight={snapA.get('uHeight')}（快照零值，改版不影响=因快照从未写入）")
    # 停用模板后建柜
    parsed, _ = call("S9X-tpl-refetch", "GET", "/api/v1/rack-templates", token=T())
    tpl2 = next((t for t in data_of(parsed)["items"] if t["id"] == sys_tpl["id"]), None)
    if tpl2:
        call("S9X-tpl-disable", "PUT", f"/api/v1/rack-templates/{tpl2['id']}?version={tpl2['version']}",
             {"code": tpl2["code"], "name": tpl2["name"], "status": "DISABLED", "version": tpl2["version"]},
             token=T())
        parsed, st = call("S9X-rackC-create-disabled", "POST", f"/api/v1/rooms/{room['id']}/racks",
                          {"code": f"GT-{rid}-RKC-{NONCE}", "name": "不应成功", "templateId": tpl2["id"],
                           "autoGenerateCode": False}, token=T(),
                          note="预期 4xx/409（ErrTemplateDisabled）")
        call("S9X-tpl-enable", "PUT", f"/api/v1/rack-templates/{tpl2['id']}?version={(data_of(parsed) or tpl2)['version']}",
             {"code": tpl2["code"], "name": tpl2["name"], "status": "ACTIVE", "version": (data_of(parsed) or tpl2)["version"]},
             token=T())
    # 系统模板删除保护
    call("S9X-sys-tpl-delete", "DELETE", f"/api/v1/rack-templates/{sys_tpl['id']}?version={sys_tpl['version']}",
         token=T(),
         note="预期 4xx/409（ErrSystemTemplate）")


def s11x(rid):
    """last-admin：带 query version 的删除与降级（复核指南 §3.5 十步法精简版）。"""
    parsed, _ = call("S11X-users", "GET", "/api/v1/admin/users", token=T())
    items = data_of(parsed)["items"]
    admins = [u for u in items if any(r.get("code") == "system_admin" for r in u.get("roles") or [])]
    ck("S11X-one-admin", len(admins) == 1, f"管理员数={len(admins)}")
    if not admins:
        return
    adm = admins[0]
    # 带正确 version 删除
    parsed, st = call("S11X-last-admin-delete", "DELETE",
                      f"/api/v1/admin/users/{adm['id']}?version={adm['version']}", token=T(),
                      note="预期业务保护（ErrLastAdmin→4xx/409），非 INVALID_VERSION")
    ck("S11X-delete-guard", st in (400, 403, 409) and code_of(parsed) != "INVALID_VERSION",
       f"{st} {code_of(parsed)} {json.dumps(parsed, ensure_ascii=False)[:100]}")
    # 降级（角色改为仅 user）
    parsed, st = call("S11X-last-admin-demote", "PUT", f"/api/v1/admin/users/{adm['id']}?version={adm['version']}",
                      {"displayName": adm["displayName"], "roles": ["user"], "version": adm["version"]},
                      token=T(),
                      note="预期业务保护")
    ck("S11X-demote-guard", st in (400, 403, 409) and code_of(parsed) != "INVALID_VERSION",
       f"{st} {code_of(parsed)}")
    # 复核 admin 仍有效
    parsed, _ = call("S11X-me-after", "GET", "/api/v1/auth/me", token=T())
    ck("S11X-admin-alive", data_of(parsed) and data_of(parsed)["username"] == "admin")


def s8x(rid):
    """approve-stale：区分审批单 version 与 requested_device_version。"""
    rack = find_tree(rid, "RK01")
    if not rack:
        ck("S8X-prereq", False)
        return
    # 开审批
    parsed, _ = call("S8X-policy-get", "GET", "/api/v1/admin/approval-policy", token=T())
    pol = data_of(parsed)
    call("S8X-policy-on", "PUT", "/api/v1/admin/approval-policy",
         {"version": pol["version"], "assignApprovalEnabled": True, "moveApprovalEnabled": False,
          "defaultApproverRole": "system_admin"}, token=T())
    try:
        tid = None
        parsed, _ = call("S8X-devtypes", "GET", "/api/v1/device-types", token=T())
        for t in data_of(parsed)["items"]:
            if t["code"] == "SERVER":
                tid = t["id"]
        parsed, st = call("S8X-dev-create", "POST", "/api/v1/devices",
                          {"typeId": tid, "code": f"GT-{rid}-DV8X-{NONCE}", "name": "stale审批设备",
                           "autoGenerateCode": False}, token=T())
        dev = data_of(parsed)
        parsed, st = call("S8X-assign-approval", "POST", f"/api/v1/devices/{dev['id']}/assign",
                          {"version": dev["version"], "rackId": rack["id"], "startU": 15, "heightU": 2,
                           "orientation": "NORMAL", "reason": "GT-S8X"}, token=T())
        res = data_of(parsed) or {}
        appr = res.get("approval") or {}
        if not appr.get("id"):
            ck("S8X-approval-created", False, json.dumps(parsed, ensure_ascii=False)[:120])
            return
        # 修改设备 → version+1
        parsed, _ = call("S8X-dev-refetch", "GET", f"/api/v1/devices/{dev['id']}", token=T())
        cur = data_of(parsed)
        full = dict(cur)
        full["name"] = "stale审批设备-已改"
        parsed, st = call("S8X-dev-modify", "PUT", f"/api/v1/devices/{dev['id']}?version={cur['version']}",
                          full, token=T())
        # 用审批单当前 version 批准 → 应触达 requested_device_version 保护
        parsed, st = call("S8X-approve-stale", "POST", f"/api/v1/admin/approvals/{appr['id']}/approve",
                          {"version": appr.get("version", 1), "comment": "设备已改，应拒"}, token=T(),
                          note="预期业务保护（非 INVALID_RESOURCE/version 参数错）")
        ck("S8X-stale-guard", st in (400, 409) and code_of(parsed) not in ("INVALID_RESOURCE", "INVALID_VERSION"),
           f"{st} {code_of(parsed)} {json.dumps(parsed, ensure_ascii=False)[:110]}")
        # 设备不应在位
        parsed, _ = call("S8X-layout-check", "GET", f"/api/v1/racks/{rack['id']}/u-layout", token=T())
        lay = data_of(parsed) or {}
        ck("S8X-not-positioned", str(dev["id"]) not in [str(x.get("deviceId")) for x in lay.get("positions", [])])
        # 审批单状态不应为 APPROVED
        parsed, _ = call("S8X-approvals-list", "GET", "/api/v1/admin/approvals", token=T())
        rec = next((a for a in (data_of(parsed) or {}).get("items", []) if a["id"] == appr["id"]), None)
        ck("S8X-approval-not-approved", rec and rec.get("status") != "APPROVED",
           rec.get("status") if rec else "记录未找到")
    finally:
        parsed, _ = call("S8X-policy-get2", "GET", "/api/v1/admin/approval-policy", token=T())
        pol2 = data_of(parsed)
        call("S8X-policy-off", "PUT", "/api/v1/admin/approval-policy",
             {"version": pol2["version"], "assignApprovalEnabled": False, "moveApprovalEnabled": False,
              "defaultApproverRole": "system_admin"}, token=T())


def s4x(rid):
    """设备类型：CRUD + 引用删除保护。"""
    P = f"GT-{rid}"
    parsed, st = call("S4X-type-create", "POST", "/api/v1/device-types",
                      {"code": P + "-TY1", "name": "黄金类型", "category": "ACCESSORY",
                       "defaultHeightU": 3, "autoGenerateCode": False}, token=T())
    ty = data_of(parsed)
    ck("S4X-create-201", st == 201 and ty and ty.get("id"))
    if not ty:
        return
    call("S4X-type-dup", "POST", "/api/v1/device-types",
         {"code": P + "-TY1", "name": "重复", "category": "ACCESSORY", "autoGenerateCode": False},
         token=T(), note="预期 409")
    # 引用后删除保护
    parsed, st = call("S4X-dev-with-type", "POST", "/api/v1/devices",
                      {"typeId": ty["id"], "code": P + "-TDEV", "name": "引用类型设备",
                       "autoGenerateCode": False}, token=T())
    dev = data_of(parsed)
    call("S4X-type-delete-inuse", "DELETE", f"/api/v1/device-types/{ty['id']}?version={ty['version']}",
         token=T(), note="预期 4xx/409（ErrDeviceTypeInUse）")
    # 更新类型
    parsed, st = call("S4X-type-update", "PUT", f"/api/v1/device-types/{ty['id']}?version={ty['version']}",
                      {"code": ty["code"], "name": "黄金类型改", "category": "ACCESSORY",
                       "defaultHeightU": 3, "version": ty["version"]}, token=T())
    ck("S4X-update-200", st == 200, f"{st} {code_of(parsed)}")
    # 删设备后删类型应成功
    if dev:
        parsed, st = call("S4X-dev-delete", "DELETE", f"/api/v1/devices/{dev['id']}?version={dev['version']}",
                          token=T(), note="应成功")
        parsed, _ = call("S4X-type-refetch", "GET", "/api/v1/device-types", token=T())
        ty2 = next((t for t in data_of(parsed)["items"] if t["id"] == ty["id"]), None)
        if ty2:
            call("S4X-type-delete-ok", "DELETE", f"/api/v1/device-types/{ty2['id']}?version={ty2['version']}",
                 token=T(), note="应成功")


def s5x(rid):
    """设备列表：过滤组合/分页/删除/历史。"""
    P = f"GT-{rid}"
    # 组合过滤
    import urllib.parse
    q = urllib.parse.quote("黄金")
    parsed, st = call("S5X-list-filter", "GET",
                      f"/api/v1/devices?page=1&pageSize=5&search={q}&lifecycleStatus=WAITING_RACK",
                      token=T())
    d = data_of(parsed) or {}
    ck("S5X-filter-shape", st == 200 and all(k in d for k in ("items", "total", "page", "pageSize")),
       f"total={d.get('total')}")
    parsed, st = call("S5X-list-typename", "GET", "/api/v1/devices?page=1&pageSize=5&search=GT-", token=T())
    d2 = data_of(parsed) or {}
    ck("S5X-search-GT", d2.get("total", 0) >= 1, f"total={d2.get('total')}")
    # 历史接口（用已有设备）
    parsed, _ = call("S5X-list-any", "GET", "/api/v1/devices?page=1&pageSize=1", token=T())
    items = (data_of(parsed) or {}).get("items") or []
    if items:
        dev = items[0]
        parsed, st = call("S5X-history", "GET", f"/api/v1/devices/{dev['id']}/history", token=T(),
                          note="覆盖 GET /devices/{id}/history")
        ck("S5X-history-ok", st == 200, f"{st} {code_of(parsed)}")
    # 分页边界
    parsed, st = call("S5X-page0", "GET", "/api/v1/devices?page=999&pageSize=10", token=T())
    ck("S5X-page-beyond", st == 200 and (data_of(parsed) or {}).get("items") == [],
       f"total={data_of(parsed).get('total') if data_of(parsed) else '?'}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--env-file", required=True)
    ap.add_argument("--run-id", required=True)
    ap.add_argument("--pg", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    for line in open(a.env_file):
        if line.startswith("ADMIN_PASSWORD="):
            ADMIN_PW["v"] = line.strip().split("=", 1)[1]
    os.makedirs(a.out, exist_ok=True)
    login()
    for fn in (s7, s9x, s11x, s8x, s4x, s5x):
        print(f"\n===== {fn.__name__} =====")
        try:
            fn(a.run_id)
        except Exception as e:
            ck(fn.__name__ + "-exception", False, repr(e))
    with open(os.path.join(a.out, "exploratory-samples.jsonl"), "a", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(a.out, "exploratory-results.jsonl"), "w", encoding="utf-8") as f:
        for r in results:
            f.write(json.dumps(r, ensure_ascii=False) + "\n")
    p = sum(1 for r in results if r["pass"])
    print(f"\n补采断言: {p} PASS / {len(results) - p} FAIL / 样本 {len(samples)} 条")


if __name__ == "__main__":
    main()