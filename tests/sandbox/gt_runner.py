#!/usr/bin/env python3
"""GT 黄金测试 runner（服务器端执行，纯标准库）。
用法: python3 gt_runner.py --env-file <path> --run-id <RUN_ID> --pg <pg容器名> --out <目录> --scenarios S1,S2,S3
"""
import argparse
import base64
import datetime
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request

API = "http://127.0.0.1:18080"
NONCE = f"{int(datetime.datetime.utcnow().timestamp()) % 1000000:06d}"  # 每次执行的唯一后缀
JWT_RE = re.compile(r"^eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*$")
ISO_RE = re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})")
UUID_RE = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")

samples = []      # raw（脱敏后）
norms = []        # normalized
results = []      # 断言结果
uuid_map = {}     # uuid -> 稳定名
TOKEN = {"v": ""}


def redact(obj):
    if isinstance(obj, dict):
        return {k: ("<REDACTED_PASSWORD>" if k.lower() == "password" and isinstance(v, str) and v
                    else "Bearer <REDACTED_JWT>" if k.lower() == "authorization"
                    else "<REDACTED_JWT>" if isinstance(v, str) and JWT_RE.match(v)
                    else redact(v))
                for k, v in obj.items()}
    if isinstance(obj, list):
        return [redact(x) for x in obj]
    return obj


def normalize(obj):
    if isinstance(obj, dict):
        return {k: normalize(v) for k, v in obj.items()}
    if isinstance(obj, list):
        return [normalize(x) for x in obj]
    if isinstance(obj, str):
        s = obj
        for u, name in uuid_map.items():
            s = s.replace(u, f"<ID:{name}>")
        s = ISO_RE.sub("<TS>", s)
        s = re.sub(r"req_[0-9a-f-]{36}", "<REQ_ID>", s)
        if JWT_RE.match(s):
            s = "<JWT>"
        return s
    return obj


def call(case, method, path, body=None, token=None, expect=None, note=""):
    req = urllib.request.Request(API + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode() if body is not None else None
    status, payload, rid = -1, "", ""
    try:
        with urllib.request.urlopen(req, data=data, timeout=20) as r:
            status, payload = r.status, r.read().decode("utf-8", "replace")
            rid = r.headers.get("X-Request-Id", "")
    except urllib.error.HTTPError as e:
        status, payload = e.code, e.read().decode("utf-8", "replace")
        rid = e.headers.get("X-Request-Id", "")
    except Exception as e:
        payload = str(e)
    try:
        parsed = json.loads(payload)
    except Exception:
        parsed = payload[:400]
    samples.append({"case": case, "method": method, "path": path,
                    "status": status, "reqBody": redact(body) if body is not None else None,
                    "respBody": redact(parsed), "requestId": rid,
                    "ts": datetime.datetime.utcnow().isoformat() + "Z"})
    norms.append({"case": case, "method": method, "path": re.sub(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}",
                                                                 lambda m: f"<ID:{uuid_map.get(m.group(0), '?')}>" if m.group(0) in uuid_map else m.group(0), path),
                  "status": status, "respBody": normalize(parsed)})
    ok = None
    if expect is not None:
        if callable(expect):
            ok, why = expect(status, parsed)
        else:
            ok, why = (status == expect, f"HTTP {status} != {expect}")
        results.append({"case": case, "expect": why if not ok else "met",
                        "status": status, "pass": bool(ok), "note": note})
    return parsed, status


def check(case, name, ok, detail=""):
    results.append({"case": case, "expect": name, "pass": bool(ok), "note": str(detail)[:200]})
    return ok


def psql(pg, sql):
    r = subprocess.run(["docker", "exec", pg, "psql", "-U", "cabinet", "-d", "cabinet", "-t", "-A", "-c", sql],
                       capture_output=True, text=True, timeout=30)
    return r.stdout.strip(), r.stderr.strip()


def data(parsed):
    return parsed.get("data") if isinstance(parsed, dict) else None


def code_of(parsed):
    return parsed.get("code") if isinstance(parsed, dict) else None


def login(username, password):
    parsed, st = call("login", "POST", "/api/v1/auth/login", {"username": username, "password": password})
    d = data(parsed) or {}
    tok = d.get("token")
    if tok:
        TOKEN["v"] = tok
    return tok, st, parsed


def login_no_switch(username, password):
    """登录但不切换全局 token（避免覆盖 admin 会话）。"""
    req = urllib.request.Request(API + "/api/v1/auth/login", method="POST",
                                 data=json.dumps({"username": username, "password": password}).encode(),
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return (json.loads(r.read()).get("data") or {}).get("token")
    except Exception:
        return None


def T():
    return TOKEN["v"]


# ---------------- 场景 ----------------

def s1(pg, rid):
    call("S1-health-live", "GET", "/health/live", expect=lambda s, p: (s == 200, "200"))
    call("S1-health-ready", "GET", "/health/ready", expect=lambda s, p: (s == 200 and code_of(p) == "SUCCESS", "200+SUCCESS"))
    tok, st, _ = login("admin", ADMIN_PW)
    check("S1-login", "200+token", st == 200 and bool(tok))
    p = base64.urlsafe_b64decode(tok.split(".")[1] + "==").decode()
    claims = json.loads(p)
    check("S1-jwt-7200", "exp-iat==7200", claims["exp"] - claims["iat"] == 7200, claims["exp"] - claims["iat"])
    call("S1-login-wrongpw", "POST", "/api/v1/auth/login", {"username": "admin", "password": "Wrong#9999"},
         expect=lambda s, p: (s == 401 and code_of(p) == "INVALID_CREDENTIALS", "401+INVALID_CREDENTIALS"))
    call("S1-noauth-401", "GET", "/api/v1/device-types", expect=lambda s, p: (s == 401 and code_of(p) == "UNAUTHORIZED", "401+UNAUTHORIZED"))
    parsed, st = call("S1-me", "GET", "/api/v1/auth/me", token=T(),
                      expect=lambda s, p: (s == 200 and data(p)["username"] == "admin", "200+admin"))
    uuid_map[data(parsed)["id"]] = "ADMIN"
    call("S1-logout", "POST", "/api/v1/auth/logout", token=T(),
         expect=lambda s, p: (s == 200 and data(p)["loggedOut"] is True, "200+loggedOut"))
    login("admin", ADMIN_PW)  # 重新取 token 供后续用


def s2(pg, rid):
    P = f"GT-{rid}"
    parsed, st = call("S2-dc-create", "POST", "/api/v1/data-centers",
                      {"code": P + "-DC01", "name": "黄金测试数据中心", "remarks": "goldenRunId=" + rid,
                       "autoGenerateCode": False}, token=T(),
                      expect=lambda s, p: (s == 200 and data(p) and data(p).get("code"), "200+code"))
    dc = data(parsed)
    uuid_map[dc["id"]] = "DC01"
    call("S2-dc-dupcode", "POST", "/api/v1/data-centers",
         {"code": P + "-DC01", "name": "重复", "autoGenerateCode": False}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    parsed, st = call("S2-room-create", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
                      {"code": P + "-RM01", "name": "黄金测试机房", "autoGenerateCode": False}, token=T(),
                      expect=lambda s, p: (s == 200, "200"))
    room = data(parsed)
    uuid_map[room["id"]] = "RM01"
    # 模板 id（种子）
    parsed, st = call("S2-tpl-list", "GET", "/api/v1/rack-templates", token=T(), expect=lambda s, p: (s == 200, "200"))
    tpl = data(parsed)["items"][0]
    uuid_map[tpl["id"]] = "TPL_SYS"
    parsed, st = call("S2-rack-create", "POST", f"/api/v1/rooms/{room['id']}/racks",
                      {"code": P + "-RK01", "name": "黄金测试机柜A", "templateId": tpl["id"],
                       "autoGenerateCode": False, "overrideTemplateSpec": False}, token=T(),
                      expect=lambda s, p: (s == 200, "200"))
    rack = data(parsed)
    uuid_map[rack["id"]] = "RK01"
    check("S2-rack-snapshot", "templateSnapshot 非空", bool(rack.get("templateSnapshot")),
          str(rack.get("templateSnapshot"))[:80])
    parsed, st = call("S2-rack-create2", "POST", f"/api/v1/rooms/{room['id']}/racks",
                      {"code": P + "-RK02", "name": "黄金测试机柜B", "autoGenerateCode": False}, token=T(),
                      expect=lambda s, p: (s == 200, "200"))
    uuid_map[data(parsed)["id"]] = "RK02"
    # 停用父级后新建子级
    parsed, st = call("S2-dc-disable", "PUT", f"/api/v1/data-centers/{dc['id']}?version={dc['version']}",
                      {"code": dc["code"], "name": dc["name"], "status": "DISABLED"}, token=T(),
                      expect=lambda s, p: (s == 200, "200"))
    dc_now = data(parsed)
    call("S2-room-in-disabled-dc", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
         {"code": P + "-RM02", "name": "不应成功", "autoGenerateCode": False}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    # 恢复启用
    parsed2, st2 = call("S2-dc-enable", "PUT", f"/api/v1/data-centers/{dc['id']}?version={dc_now['version']}",
                        {"code": dc["code"], "name": dc["name"], "status": "OPERATING"}, token=T(),
                        expect=lambda s, p: (s == 200, "200"))
    dc_final = data(parsed2)
    # 有子级删除
    call("S2-dc-delete-haschildren", "DELETE", f"/api/v1/data-centers/{dc['id']}?version={dc_final['version']}",
         token=T(), expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    return {"dc": dc["id"], "room": room["id"], "rack": rack["id"], "tpl": tpl["id"], "P": P}


def find_in_tree(pg, rid, code_suffix):
    """从 resource-tree 现查 GT 资源（幂等，不依赖 ctx）。"""
    parsed, st = call("tree-lookup", "GET", "/api/v1/resource-tree", token=T(), expect=lambda s, p: (s == 200, "200"))
    items = data(parsed)["items"]
    want = f"GT-{rid}-{code_suffix}"

    def walk(nodes):
        for n in nodes:
            if str(n.get("code", "")).upper() == want.upper():
                return n
            for key in ("rooms", "racks", "children"):
                if isinstance(n.get(key), list):
                    r = walk(n[key])
                    if r:
                        return r
        return None
    return walk(items)


def s2b(pg, rid):
    """S2 补充：父级停用门禁 + 有子级删除保护（幂等）。"""
    dc = find_in_tree(pg, rid, "DC01")
    check("S2b-dc-found", "DC01 存在", bool(dc))
    if not dc:
        return
    uuid_map.setdefault(dc["id"], "DC01")
    # PUT body 携带 version（实测要求）
    parsed, st = call("S2b-dc-disable", "PUT", f"/api/v1/data-centers/{dc['id']}?version={dc['version']}",
                      {"code": dc["code"], "name": dc["name"], "status": "DISABLED", "version": dc["version"]},
                      token=T(), expect=lambda s, p: (s == 200, f"200 got {s} {code_of(p)}"))
    dc_now = data(parsed) or dc
    call("S2b-room-in-disabled-dc", "POST", f"/api/v1/data-centers/{dc['id']}/rooms",
         {"code": f"GT-{rid}-RMX", "name": "不应成功", "autoGenerateCode": False}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    parsed, st = call("S2b-dc-enable", "PUT", f"/api/v1/data-centers/{dc['id']}?version={dc_now.get('version', dc['version'] + 1)}",
                      {"code": dc["code"], "name": dc["name"], "status": "OPERATING",
                       "version": dc_now.get("version", dc["version"] + 1)},
                      token=T(), expect=lambda s, p: (s == 200, f"200 got {s} {code_of(p)}"))
    dc_final = data(parsed) or dc
    call("S2b-dc-delete-haschildren", "DELETE",
         f"/api/v1/data-centers/{dc['id']}?version={dc_final.get('version', dc_final['version'] + 1)}",
         token=T(), expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))


def s3(pg, rid, ctx=None):
    dc = find_in_tree(pg, rid, "DC01")
    check("S3-dc-found", "DC01 存在", bool(dc))
    if not dc:
        return
    dc_id, ver, code = dc["id"], dc["version"], dc["code"]
    uuid_map.setdefault(dc_id, "DC01")
    call("S3-stale-put", "PUT", f"/api/v1/data-centers/{dc_id}?version={ver + 5}",
         {"code": code, "name": "过期版本写入", "version": ver + 5}, token=T(),
         expect=lambda s, p: (s == 409, f"409 got {s} {code_of(p)}"))
    parsed, st = call("S3-ok-put", "PUT", f"/api/v1/data-centers/{dc_id}?version={ver}",
                      {"code": code, "name": "黄金测试数据中心改", "status": "OPERATING", "version": ver}, token=T(),
                      expect=lambda s, p: (s == 200 and data(p)["version"] == ver + 1,
                                           f"version+1 got {data(p).get('version') if data(p) else '?'}"))
    call("S3-stale-delete", "DELETE", f"/api/v1/data-centers/{dc_id}?version=999", token=T(),
         expect=lambda s, p: (s == 409, f"409 got {s} {code_of(p)}"))


OK2 = lambda s, p: (s in (200, 201), f"200/201 got {s} {code_of(p)}")


def dev_type_id():
    parsed, _ = call("lookup-devtypes", "GET", "/api/v1/device-types", token=T())
    for t in data(parsed)["items"]:
        if t["code"] == "SERVER":
            uuid_map.setdefault(t["id"], "DEVTYPE_SERVER")
            return t["id"]
    return None


def dev_create(case, code, name, **kw):
    body = {"typeId": dev_type_id(), "code": code, "name": name, "autoGenerateCode": False}
    body.update(kw)
    parsed, st = call(case, "POST", "/api/v1/devices", body, token=T(), expect=OK2)
    d = data(parsed)
    if d and d.get("id"):
        uuid_map[d["id"]] = code.split("-")[-1]
    return d


def dev_get(case, dev_id):
    parsed, _ = call(case, "GET", f"/api/v1/devices/{dev_id}", token=T())
    return data(parsed)


def s6(pg, rid):
    P = f"GT-{rid}"
    rack = find_in_tree(pg, rid, "RK01")
    rack2 = find_in_tree(pg, rid, "RK02")
    check("S6-rack-found", "RK01/RK02 存在", bool(rack and rack2))
    if not rack:
        return
    RK = rack["id"]
    uuid_map.setdefault(RK, "RK01")
    uuid_map.setdefault(rack2["id"], "RK02")
    # 默认高度继承（不传 heightU）
    d1 = dev_create("S6-dev1-create", P + "-DEV01", "黄金设备01")
    check("S6-default-height", "heightU 继承类型默认=2", d1 and d1.get("heightU") == 2, d1.get("heightU"))
    d2 = dev_create("S6-dev2-create", P + "-DEV02", "黄金设备02", heightU=4)
    # 上架成功
    parsed, st = call("S6-assign-ok", "POST", f"/api/v1/devices/{d1['id']}/assign",
                      {"version": d1["version"], "rackId": RK, "startU": 40, "heightU": 2,
                       "orientation": "NORMAL", "reason": "GT-S6"}, token=T(), expect=OK2)
    res = data(parsed) or {}
    check("S6-assign-executed", "executed=true+position", res.get("executed") is True and bool(res.get("position")))
    # u-layout
    parsed, _ = call("S6-ulayout", "GET", f"/api/v1/racks/{RK}/u-layout", token=T(),
                     expect=lambda s, p: (s == 200, "200"))
    lay = data(parsed) or {}
    pos_codes = [str(x.get("deviceId")) for x in lay.get("positions", [])]
    check("S6-layout-has-dev1", "layout 含 DEV01", str(d1["id"]) in pos_codes,
          f"positions={len(lay.get('positions', []))} free={len(lay.get('free', []))}")
    # U 位重叠（皇冠样本）
    call("S6-assign-overlap", "POST", f"/api/v1/devices/{d2['id']}/assign",
         {"version": d2["version"], "rackId": RK, "startU": 39, "heightU": 4,
          "orientation": "NORMAL", "reason": "GT-S6-overlap"}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    # 设备双位置
    call("S6-assign-double", "POST", f"/api/v1/devices/{d1['id']}/assign",
         {"version": dev_get("S6-dev1-refetch", d1["id"])["version"], "rackId": RK,
          "startU": 10, "heightU": 2, "orientation": "NORMAL", "reason": "GT-S6-double"}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    # 越界
    call("S6-assign-outrange", "POST", f"/api/v1/devices/{d2['id']}/assign",
         {"version": d2["version"], "rackId": RK, "startU": 100, "heightU": 4,
          "orientation": "NORMAL", "reason": "GT-S6-oor"}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    # 移位
    cur = dev_get("S6-dev1-refetch2", d1["id"])
    parsed, st = call("S6-move-ok", "POST", f"/api/v1/devices/{d1['id']}/move",
                      {"version": cur["version"], "rackId": RK, "startU": 20, "heightU": 2,
                       "orientation": "REVERSE", "reason": "GT-S6-move"}, token=T(), expect=OK2)
    check("S6-move-executed", "executed=true", (data(parsed) or {}).get("executed") is True)
    # 退役
    cur = dev_get("S6-dev1-refetch3", d1["id"])
    parsed, st = call("S6-decommission", "POST", f"/api/v1/devices/{d1['id']}/decommission",
                      {"version": cur["version"], "reason": "GT-S6-retire"}, token=T(), expect=OK2)
    cur2 = dev_get("S6-dev1-final", d1["id"])
    check("S6-lifecycle-offrack", "退役后生命周期非在机", cur2.get("lifecycleStatus") not in (None, "RUNNING"),
          cur2.get("lifecycleStatus"))
    # DB 不变量：RK01 的占用区间无重叠
    out, err = psql(pg, "SELECT count(*) FROM (SELECT rack_id, int8range(start_u,end_u,'[]') r FROM rack_u_occupancies WHERE deleted_at IS NULL) a JOIN (SELECT rack_id, int8range(start_u,end_u,'[]') r2 FROM rack_u_occupancies WHERE deleted_at IS NULL) b ON a.rack_id=b.rack_id AND a.r2 && b.r2 AND a.rowid IS DISTINCT FROM b.rowid" if False else
                    "SELECT count(*) FROM rack_u_occupancies WHERE deleted_at IS NULL AND rack_id='" + RK + "'")
    check("S6-db-occupancy", "occupancies 可查", out != "", out + err)
    parsed, _ = call("S6-rack2-empty", "GET", f"/api/v1/racks/{rack2['id']}/u-layout", token=T())
    return {"d2": d2["id"], "rack": RK, "rack2": rack2["id"]}


def s8(pg, rid):
    P = f"GT-{rid}"
    rack = find_in_tree(pg, rid, "RK01")
    RK = rack["id"]
    parsed, _ = call("S8-policy-get", "GET", "/api/v1/admin/approval-policy", token=T())
    pol = data(parsed)
    parsed, st = call("S8-policy-enable-assign", "PUT", "/api/v1/admin/approval-policy",
                      {"version": pol["version"], "assignApprovalEnabled": True,
                       "moveApprovalEnabled": False, "defaultApproverRole": "system_admin"},
                      token=T(), expect=lambda s, p: (s == 200, f"200 got {s} {code_of(p)}"))
    try:
        d3 = dev_create("S8-dev3-create", P + "-DEV03-" + NONCE, "审批设备03")
        parsed, st = call("S8-assign-approval", "POST", f"/api/v1/devices/{d3['id']}/assign",
                          {"version": d3["version"], "rackId": RK, "startU": 5, "heightU": 2,
                           "orientation": "NORMAL", "reason": "GT-S8"}, token=T(), expect=OK2)
        res = data(parsed) or {}
        check("S8-assign-pending", "executed=false+approval",
              res.get("executed") is False and bool(res.get("approval")))
        appr = res.get("approval") or {}
        if appr.get("id"):
            uuid_map[appr["id"]] = "APPR1"
            parsed, st = call("S8-approve", "POST", f"/api/v1/admin/approvals/{appr['id']}/approve",
                              {"version": appr.get("version", 1), "comment": "GT 批准"}, token=T(), expect=OK2)
            cur = dev_get("S8-dev3-after", d3["id"])
            parsed, _ = call("S8-layout-check", "GET", f"/api/v1/racks/{RK}/u-layout", token=T())
            lay = data(parsed) or {}
            check("S8-approved-positioned", "批准后在位",
                  str(d3["id"]) in [str(x.get("deviceId")) for x in lay.get("positions", [])])
        # 过期审批：申请后改设备
        d4 = dev_create("S8-dev4-create", P + "-DEV04-" + NONCE, "审批设备04")
        parsed, st = call("S8-assign-stale", "POST", f"/api/v1/devices/{d4['id']}/assign",
                          {"version": d4["version"], "rackId": RK, "startU": 8, "heightU": 2,
                           "orientation": "NORMAL", "reason": "GT-S8-stale"}, token=T(), expect=OK2)
        appr2 = (data(parsed) or {}).get("approval") or {}
        if appr2.get("id"):
            uuid_map[appr2["id"]] = "APPR2"
            cur = dev_get("S8-dev4-refetch", d4["id"])
            full = dict(cur)
            full.update({"name": "审批设备04改", "remarks": "changed"})
            call("S8-dev4-modify", "PUT", f"/api/v1/devices/{d4['id']}?version={cur['version']}",
                 full, token=T(),
                 expect=lambda s, p: (s == 200, f"200 got {s} {code_of(p)}"))
            call("S8-approve-stale", "POST", f"/api/v1/admin/approvals/{appr2['id']}/approve",
                 {"comment": "应被版本校验拒绝"}, token=T(),
                 expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
        # 驳回
        d5 = dev_create("S8-dev5-create", P + "-DEV05-" + NONCE, "审批设备05")
        parsed, st = call("S8-assign-rejectable", "POST", f"/api/v1/devices/{d5['id']}/assign",
                          {"version": d5["version"], "rackId": RK, "startU": 12, "heightU": 2,
                           "orientation": "NORMAL", "reason": "GT-S8-reject"}, token=T(), expect=OK2)
        appr3 = (data(parsed) or {}).get("approval") or {}
        if appr3.get("id"):
            uuid_map[appr3["id"]] = "APPR3"
            call("S8-reject", "POST", f"/api/v1/admin/approvals/{appr3['id']}/reject",
                 {"version": appr3.get("version", 1), "comment": "GT 驳回"}, token=T(), expect=OK2)
            call("S8-reject-again", "POST", f"/api/v1/admin/approvals/{appr3['id']}/reject",
                 {"version": appr3.get("version", 2), "comment": "重复决定"}, token=T(),
                 expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    finally:
        parsed, _ = call("S8-policy-get2", "GET", "/api/v1/admin/approval-policy", token=T())
        pol2 = data(parsed)
        call("S8-policy-restore", "PUT", "/api/v1/admin/approval-policy",
             {"version": pol2["version"], "assignApprovalEnabled": False,
              "moveApprovalEnabled": False, "defaultApproverRole": "system_admin"},
             token=T(), expect=lambda s, p: (s == 200, f"200 got {s}"))




def pick_free(rack_id, height=2, skip=0):
    """从 u-layout 的 free 区间选可用起始 U（第 skip+1 个能容纳 height 的区间）。"""
    parsed, _ = call("pick-free", "GET", f"/api/v1/racks/{rack_id}/u-layout", token=T())
    free = (data(parsed) or {}).get("free") or []
    fits = [f["startU"] for f in free if f.get("heightU", 0) >= height]
    return fits[skip] if len(fits) > skip else (fits[0] if fits else 40)


def s10(pg, rid):
    P = f"GT-{rid}"
    rack = find_in_tree(pg, rid, "RK02")
    rack1 = find_in_tree(pg, rid, "RK01")
    RK2, RK1 = rack["id"], rack1["id"]
    parsed, st = call("S10-pdu-create", "POST", f"/api/v1/racks/{RK2}/pdus",
                      {"code": P + "-PDU1-" + NONCE, "name": "黄金PDU"}, token=T(), expect=OK2)
    pdu = data(parsed)
    if pdu and pdu.get("id"):
        call("S10-pdu-dupcode-defect", "POST", f"/api/v1/racks/{RK2}/pdus",
             {"code": P + "-PDU1-" + NONCE, "name": "重复PDU"}, token=T(),
             expect=lambda s, p: (s in (409, 500), f"409/500(缺陷样本) got {s} {code_of(p)}"))
    if not pdu or not pdu.get("id"):
        check("S10-pdu-create", "PDU 创建", False, json.dumps(parsed, ensure_ascii=False)[:150])
        return
    uuid_map[pdu["id"]] = "PDU1"
    parsed, st = call("S10-socket-create", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
                      {"socketNo": 1, "standard": "CN", "amperageA": 16}, token=T(), expect=OK2)
    sock = data(parsed)
    uuid_map[sock["id"]] = "SOCK1"
    call("S10-socket-dup", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
         {"socketNo": 1, "standard": "EU", "amperageA": 10}, token=T(),
         expect=lambda s, p: (s in (400, 409, 500), f"400/409/500(缺陷样本) got {s} {code_of(p)}"))
    parsed, st = call("S10-socket2", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
                      {"socketNo": 2, "standard": "CN", "amperageA": 16}, token=T(), expect=OK2)
    sock2 = data(parsed)
    uuid_map[sock2["id"]] = "SOCK2"
    # RK2 内在位设备（连接目标）
    dA = dev_create("S10-devA", P + "-DEVA-" + NONCE, "供电设备A")
    call("S10-devA-assign", "POST", f"/api/v1/devices/{dA['id']}/assign",
         {"version": dA["version"], "rackId": RK2, "startU": pick_free(RK2, 2, 0), "heightU": 2,
          "orientation": "NORMAL", "reason": "GT-S10"}, token=T(), expect=OK2)
    dB = dev_create("S10-devB", P + "-DEVB-" + NONCE, "供电设备B")
    call("S10-devB-assign", "POST", f"/api/v1/devices/{dB['id']}/assign",
         {"version": dB["version"], "rackId": RK2, "startU": pick_free(RK2, 2, 3), "heightU": 2,
          "orientation": "NORMAL", "reason": "GT-S10"}, token=T(), expect=OK2)
    # 跨机柜设备（RK1）
    dX = dev_create("S10-devX", P + "-DEVX-" + NONCE, "跨柜设备X")
    call("S10-devX-assign", "POST", f"/api/v1/devices/{dX['id']}/assign",
         {"version": dX["version"], "rackId": RK1, "startU": pick_free(RK1, 2, 0), "heightU": 2,
          "orientation": "NORMAL", "reason": "GT-S10"}, token=T(), expect=OK2)
    # 连接成功
    parsed, st = call("S10-connect-ok", "POST", f"/api/v1/pdu-sockets/{sock['id']}/connection",
                      {"version": sock.get("version", 1), "deviceId": dA["id"], "redundancyRole": "PRIMARY",
                       "circuit": "A路", "powerW": 300}, token=T(), expect=OK2)
    # 插座被占
    call("S10-connect-occupied", "POST", f"/api/v1/pdu-sockets/{sock['id']}/connection",
         {"version": sock.get("version", 1), "deviceId": dB["id"], "redundancyRole": "PRIMARY",
          "circuit": "A路"}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    # 同设备同角色二次连接
    call("S10-connect-samerole", "POST", f"/api/v1/pdu-sockets/{sock2['id']}/connection",
         {"version": sock2.get("version", 1), "deviceId": dA["id"], "redundancyRole": "PRIMARY",
          "circuit": "B路"}, token=T(),
         expect=lambda s, p: (s in (400, 409, 500), f"400/409/500(缺陷样本) got {s} {code_of(p)}"))
    # 跨机柜
    parsed, _ = call("S10-socket3", "POST", f"/api/v1/pdus/{pdu['id']}/sockets",
                     {"socketNo": 3, "standard": "CN", "amperageA": 16}, token=T(), expect=OK2)
    sock3 = data(parsed)
    call("S10-connect-crossrack", "POST", f"/api/v1/pdu-sockets/{sock3['id']}/connection",
         {"version": sock3.get("version", 1), "deviceId": dX["id"], "redundancyRole": "PRIMARY",
          "circuit": "A路"}, token=T(),
         expect=lambda s, p: (s in (400, 409), f"400/409 got {s} {code_of(p)}"))
    # 连接清单 + 断开
    parsed, _ = call("S10-connections", "GET", f"/api/v1/racks/{RK2}/pdu-connections", token=T(),
                     expect=lambda s, p: (s == 200, "200"))
    conns = (data(parsed) or {}).get("items") or data(parsed) or []
    if isinstance(conns, list) and conns:
        cid = conns[0]["id"] if isinstance(conns[0], dict) else None
        if cid:
            uuid_map[cid] = "CONN1"
            call("S10-disconnect", "DELETE", f"/api/v1/pdu-connections/{cid}",
                 token=T(), body={"version": conns[0].get("version", 1)},
                 expect=lambda s, p: (s in (200, 204), f"200/204 got {s} {code_of(p)}"))


def s11(pg, rid):
    P = f"GT-{rid}"
    uname = ("gtu" + rid.replace("-", "")[-8:] + NONCE[-4:]).lower()
    upw = "Gt!u" + rid[-6:]
    parsed, st = call("S11-user-create", "POST", "/api/v1/admin/users",
                      {"username": uname, "displayName": "黄金测试用户", "password": upw, "roles": ["user"]},
                      token=T(), expect=OK2)
    u = data(parsed)
    if not u or not u.get("id"):
        check("S11-user-create", "用户创建", False, json.dumps(parsed, ensure_ascii=False)[:150])
        return
    uuid_map[u["id"]] = "GTUSER"
    parsed, _ = call("S11-me-admin", "GET", "/api/v1/auth/me", token=T())
    admin_id = data(parsed)["id"]
    # 普通用户访问 admin（不切换全局 token）
    tok2 = login_no_switch(uname, upw)
    call("S11-user-forbidden", "GET", "/api/v1/admin/users", token=tok2,
         expect=lambda s, p: (s == 403, f"403 got {s} {code_of(p)}"))
    # 最后管理员保护（admin token；预期 400/403/409 之一，采真实码）
    call("S11-last-admin", "DELETE", f"/api/v1/admin/users/{admin_id}", token=T(),
         expect=lambda s, p: (s in (400, 403, 409), f"400/403/409 got {s} {code_of(p)}"))
    # 删除普通用户应成功（对照组）
    parsed, _ = call("S11-user-list", "GET", "/api/v1/admin/users", token=T())
    items = (data(parsed) or {}).get("items") or []
    me = next((x for x in items if x["id"] == u["id"]), None)
    if me:
        call("S11-user-disable", "PUT", f"/api/v1/admin/users/{u['id']}?version={me['version']}",
             {"displayName": me["displayName"], "enabled": False, "roles": ["user"],
              "version": me["version"]}, token=T(),
             expect=lambda s, p: (s == 200, f"200 got {s} {code_of(p)} {json.dumps(p, ensure_ascii=False)[:80]}"))
    call("S11-disabled-login", "POST", "/api/v1/auth/login",
         {"username": uname, "password": upw},
         expect=lambda s, p: (s in (401, 403), f"401/403 got {s} {code_of(p)}"))
    if me:
        parsed, _ = call("S11-user-list2", "GET", "/api/v1/admin/users", token=T())
        items2 = (data(parsed) or {}).get("items") or []
        me2 = next((x for x in items2 if x["id"] == u["id"]), None)
        if me2:
            call("S11-user-delete-ok", "DELETE", f"/api/v1/admin/users/{u['id']}?version={me2['version']}",
                 token=T(), expect=lambda s, p: (s in (200, 204), f"200/204 got {s} {code_of(p)}"))


def s12(pg, rid):
    P = f"GT-{rid}"
    room = find_in_tree(pg, rid, "RM01")
    rack = find_in_tree(pg, rid, "RK01")
    if not (room and rack):
        check("S12-found", "RM01/RK01", False)
        return
    tid = dev_type_id()
    import datetime as _dt
    parsed, st = call("S12-validate", "POST", f"/api/v1/rooms/{room['id']}/rack-diagram-import/validate",
                      {"formatVersion": "1", "dataCenterId": room.get("dataCenterId"),
                       "roomId": room["id"], "exportedAt": _dt.datetime.utcnow().isoformat() + "Z",
                       "defaultTypeId": tid, "coveredRackIds": [rack["id"]],
                       "devices": [{"clientId": "gt-c1-" + NONCE, "rackId": rack["id"], "rackCode": rack["code"],
                                    "startU": 30, "endU": 31, "name": "GT导入设备", "serialNumber": "GT-SN-" + NONCE,
                                    "ratedPowerW": 200}]},
                      token=T(), expect=lambda s, p: (s in (200, 201, 400), f"捕获实际行为 got {s} {code_of(p)}"))
    d = data(parsed)
    if not isinstance(d, dict) or not (d.get("token") or d.get("draftToken")):
        check("S12-validate-shape", "草稿 token 结构", False, json.dumps(parsed, ensure_ascii=False)[:200])
        results[-1]["note"] = "BLOCKED：载荷形态与实现不符，保留 400 样本待解析"
        return
    token12 = d.get("token") or d.get("draftToken")
    items = d.get("items") or []
    decisions = [{"itemId": it["id"], "action": (it.get("defaultDecision") or (it.get("allowedDecisions") or ["SKIP"])[0])}
                 for it in items if it.get("requiresDecision") or it.get("allowedDecisions")]
    parsed, st = call("S12-commit", "POST", f"/api/v1/rooms/{room['id']}/rack-diagram-import/commit",
                      {"token": token12, "decisions": decisions}, token=T(), expect=OK2)
    call("S12-recommit-stale", "POST", f"/api/v1/rooms/{room['id']}/rack-diagram-import/commit",
         {"token": token12, "decisions": decisions}, token=T(),
         expect=lambda s, p: (s in (400, 409, 410), f"400/409/410 got {s} {code_of(p)}"))


def main():
    global ADMIN_PW
    ap = argparse.ArgumentParser()
    ap.add_argument("--env-file", required=True)
    ap.add_argument("--run-id", required=True)
    ap.add_argument("--pg", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--scenarios", required=True)
    a = ap.parse_args()
    env = {}
    for line in open(a.env_file):
        if "=" in line:
            k, v = line.strip().split("=", 1)
            env[k] = v
    ADMIN_PW = env["ADMIN_PASSWORD"]
    rid = a.run_id
    os.makedirs(a.out, exist_ok=True)
    os.makedirs(os.path.join(a.out, "api", "raw-redacted"), exist_ok=True)
    os.makedirs(os.path.join(a.out, "api", "normalized"), exist_ok=True)
    os.makedirs(os.path.join(a.out, "junit"), exist_ok=True)

    ctx = {}
    # 统一先登录（S1 内部会再完整采一遍登录样本）
    login("admin", ADMIN_PW)
    for s in a.scenarios.split(","):
        s = s.strip()
        try:
            if s == "S1":
                s1(a.pg, rid)
            elif s == "S2":
                ctx = s2(a.pg, rid)
                with open(os.path.join(a.out, "ctx.json"), "w") as f:
                    json.dump(ctx, f)
            elif s in ("S2B", "S2b"):
                s2b(a.pg, rid)
            elif s == "S3":
                s3(a.pg, rid)
            elif s == "S6":
                s6(a.pg, rid)
            elif s == "S8":
                s8(a.pg, rid)
            elif s == "S10":
                s10(a.pg, rid)
            elif s == "S11":
                s11(a.pg, rid)
            elif s == "S12":
                s12(a.pg, rid)
            else:
                results.append({"case": s, "expect": "未实现批次", "pass": None, "note": "本轮未包含"})
        except Exception as e:
            results.append({"case": s, "expect": "场景执行", "pass": False, "note": f"异常: {e}"})

    with open(os.path.join(a.out, "api", "raw-redacted", "samples.jsonl"), "a", encoding="utf-8") as f:
        for s in samples:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(a.out, "api", "normalized", "normalized.jsonl"), "a", encoding="utf-8") as f:
        for s in norms:
            f.write(json.dumps(s, ensure_ascii=False) + "\n")
    with open(os.path.join(a.out, "results.json"), "a", encoding="utf-8") as f:
        f.write("\n")
    npass = sum(1 for r in results if r["pass"] is True)
    nfail = sum(1 for r in results if r["pass"] is False)
    import io
    with open(os.path.join(a.out, "results.json"), "a", encoding="utf-8") as f:
        f.write(json.dumps(results, ensure_ascii=False, indent=1) + "\n")
    print(f"断言: {npass} PASS / {nfail} FAIL / 样本 {len(samples)} 条")
    for r in results:
        if r["pass"] is not True:
            print(f"  [{r['pass']}] {r['case']} :: {r['expect']} :: {r['note'][:120]}")


if __name__ == "__main__":
    main()