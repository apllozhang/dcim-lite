#!/usr/bin/env bash
# Rebuild 全量冒烟。在 203 部署目录执行：
#   cd /home/alec/cabinet-rebuild-deploy && bash /path/to/smoke-all.sh
# 或打包进 deploy 目录后 bash scripts/smoke-all.sh
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:19173}"
ENV_FILE="${ENV_FILE:-.env}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing $ENV_FILE" >&2
  exit 1
fi

USER=$(grep '^ADMIN_USERNAME=' "$ENV_FILE" | cut -d= -f2)
PASS=$(grep '^ADMIN_PASSWORD=' "$ENV_FILE" | cut -d= -f2)
SFX=$(date +%s | tail -c 6)
PASS_COUNT=0
FAIL_COUNT=0

note() { echo "== $* =="; }
ok() { echo "  OK $*"; PASS_COUNT=$((PASS_COUNT+1)); }
bad() { echo "  FAIL $*"; FAIL_COUNT=$((FAIL_COUNT+1)); }

export BASE_URL="$BASE" SMOKE_USER="$USER" SMOKE_PASS="$PASS"
TOKEN=$(python3 - <<'PY'
import json, os, re, urllib.request, base64
base = os.environ["BASE_URL"]
user = os.environ["SMOKE_USER"]
password = os.environ["SMOKE_PASS"]
cap = json.loads(urllib.request.urlopen(base + "/api/v1/auth/captcha", timeout=15).read())
cid = cap["data"]["id"]
img = cap["data"]["image"]
svg = base64.b64decode(img.split(",", 1)[1]).decode("utf-8", "replace")
digits = "".join(re.findall(r">([0-9])</text>", svg))
body = json.dumps({"username": user, "password": password, "captchaId": cid, "captcha": digits}).encode()
req = urllib.request.Request(base + "/api/v1/auth/login", data=body, headers={"Content-Type": "application/json"}, method="POST")
resp = json.loads(urllib.request.urlopen(req, timeout=15).read())
print(resp["data"]["token"])
PY
)
export TOKEN BASE SFX

python3 - <<'PY'
import json, os, urllib.request, urllib.error, threading, time, sys

base = os.environ["BASE"]
token = os.environ["TOKEN"]
sfx = os.environ["SFX"]
auth = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
fails = []

def call(method, path, body=None, headers=None):
    h = dict(auth)
    if headers:
        h.update(headers)
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(base + path, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"raw": raw[:200]}
    except Exception as e:
        return 0, {"err": str(e)}

def check(name, cond, detail=""):
    if cond:
        print(f"  OK {name} {detail}")
    else:
        print(f"  FAIL {name} {detail}")
        fails.append(name)

# --- health via frontend proxy ---
st, body = call("GET", "/health/live")
check("health-live", st == 200 and body.get("code") == "SUCCESS")

# --- auth ---
st, me = call("GET", "/api/v1/auth/me")
check("auth-me", st == 200 and "roles" in me.get("data", {}))
st, _ = call("GET", "/api/v1/resource-tree", headers={"Authorization": "Bearer bad"})
check("unauthorized-401", st == 401)

# --- resource hierarchy ---
st, dc = call("POST", "/api/v1/data-centers", {"code": f"SM{ sfx }", "name": "冒烟中心"})
check("create-dc", st == 200 and dc.get("code") == "SUCCESS", dc.get("data", {}).get("id", "")[:8])
dc_id = dc["data"]["id"]
st, room = call("POST", f"/api/v1/data-centers/{dc_id}/rooms", {"code": "R1", "name": "冒烟房"})
check("create-room", st == 200)
room_id = room["data"]["id"]
st, rack = call("POST", f"/api/v1/rooms/{room_id}/racks", {"code": "K1", "name": "冒烟柜", "uHeight": 20})
check("create-rack", st == 200 and rack["data"]["uHeight"] == 20)
rack_id = rack["data"]["id"]

st, dup = call("POST", "/api/v1/data-centers", {"code": f"SM{sfx}", "name": "重复"})
check("dup-code-409", st == 409 and dup.get("code") == "DUPLICATE_CODE")
st, child = call("DELETE", f"/api/v1/rooms/{room_id}?version={room['data']['version']}")
check("has-children-409", st == 409 and child.get("code") == "HAS_CHILDREN")

# --- templates ---
st, tpls = call("GET", "/api/v1/rack-templates")
sys_t = next((t for t in tpls["data"]["items"] if t.get("isSystem")), None)
check("system-template", sys_t is not None)
if sys_t:
    st, _ = call("DELETE", f"/api/v1/rack-templates/{sys_t['id']}?version={sys_t['version']}")
    check("system-template-protect", st == 409)
    st, rack2 = call("POST", f"/api/v1/rooms/{room_id}/racks", {
        "code": "K2", "name": "模板柜", "templateId": sys_t["id"]
    })
    snap = (rack2.get("data") or {}).get("templateSnapshot") or {}
    check("rack-template-snapshot", st == 200 and snap.get("code") == sys_t["code"])

# --- devices + U slot ---
st, types = call("GET", "/api/v1/device-types")
tid = next(t["id"] for t in types["data"]["items"] if t["code"] == "SERVER")
check("seed-device-types", len(types["data"]["items"]) >= 6)
st, d1 = call("POST", "/api/v1/devices", {"typeId": tid, "code": f"S1{sfx}", "name": "机1", "heightU": 2})
st, d2 = call("POST", "/api/v1/devices", {"typeId": tid, "code": f"S2{sfx}", "name": "机2", "heightU": 2})
check("create-devices", st == 200)
st, a1 = call("POST", f"/api/v1/devices/{d1['data']['id']}/assign",
              {"targetRackId": rack_id, "startU": 1})
check("assign", st == 200 and a1["data"]["lifecycleStatus"] == "RUNNING")
st, a2 = call("POST", f"/api/v1/devices/{d2['data']['id']}/assign",
              {"targetRackId": rack_id, "startU": 2})
check("u-conflict-409", st == 409 and a2.get("code") == "U_SLOT_CONFLICT")
st, a3 = call("POST", f"/api/v1/devices/{d2['data']['id']}/assign",
              {"targetRackId": rack_id, "startU": 3})
check("assign-adjacent", st == 200)
st, layout = call("GET", f"/api/v1/racks/{rack_id}/u-layout")
pos = layout["data"].get("positions") or []
check("ulayout-positions", len(pos) == 2 and all(p.get("device") for p in pos))
st, hist = call("GET", f"/api/v1/devices/{d1['data']['id']}/history")
check("history", len(hist["data"]["items"]) >= 1)

# --- admin users ---
st, roles = call("GET", "/api/v1/admin/roles")
check("admin-roles", {r["code"] for r in roles["data"]["items"]} >= {"system_admin", "user"})
st, u = call("POST", "/api/v1/admin/users", {
    "username": f"smoke{ sfx }", "displayName": "冒烟用户",
    "password": "Smoke#2026!", "roleCodes": ["user"], "enabled": True
})
check("create-user", st == 200)
uid = u["data"]["id"]
st, ul = call("GET", "/api/v1/admin/users")
admin = next(x for x in ul["data"]["items"] if x["username"] == "admin")
st, last = call("PUT", f"/api/v1/admin/users/{admin['id']}?version={admin['version']}", {
    "username": admin["username"], "displayName": admin["displayName"],
    "authSource": "local", "enabled": False, "roleCodes": ["user"]
})
check("last-admin-409", st == 409 and last.get("code") == "LAST_ADMIN")

# --- approval ---
st, pol = call("PUT", "/api/v1/admin/approval-policy", {"assignApprovalEnabled": True})
check("policy-on", st == 200 and pol["data"]["assignApprovalEnabled"] is True)
st, d3 = call("POST", "/api/v1/devices", {"typeId": tid, "code": f"S3{sfx}", "name": "待批", "heightU": 1})
st, pend = call("POST", f"/api/v1/devices/{d3['data']['id']}/assign",
                {"targetRackId": rack_id, "startU": 10, "reason": "审批"})
check("assign-pending", st == 200 and pend["data"].get("status") == "PENDING")
st, ap_ok = call("POST", f"/api/v1/admin/approvals/{pend['data']['id']}/approve", {"comment": "ok"})
check("approve", st == 200 and ap_ok["data"]["lifecycleStatus"] == "RUNNING")
call("PUT", "/api/v1/admin/approval-policy", {"assignApprovalEnabled": False})

# --- PDU ---
st, p1 = call("POST", f"/api/v1/racks/{rack_id}/pdus", {"code": f"P{sfx}", "name": "PDU"})
check("create-pdu", st == 200)
st, p2 = call("POST", f"/api/v1/racks/{rack_id}/pdus", {"code": f"P{sfx}", "name": "重复"})
check("pdu-dup-409", st == 409)
st, s1 = call("POST", f"/api/v1/pdus/{p1['data']['id']}/sockets",
              {"socketNo": 1, "standard": "GB", "amperageA": 16})
check("create-socket", st == 200)
st, c1 = call("POST", f"/api/v1/pdu-sockets/{s1['data']['id']}/connection",
              {"deviceId": d1["data"]["id"], "redundancyRole": "PRIMARY"})
check("connect", st == 200)
st, c2 = call("POST", f"/api/v1/pdu-sockets/{s1['data']['id']}/connection",
              {"deviceId": d1["data"]["id"], "redundancyRole": "PRIMARY"})
check("connect-dup-409", st == 409)

# --- rack list + device sort ---
st, racks = call("GET", "/api/v1/racks-page?page=1&pageSize=5&sortBy=code&sortDir=asc")
check("racks-list", st == 200 and "items" in racks.get("data", {}) and "total" in racks.get("data", {}),
      "st=%s total=%s" % (st, racks.get("data", {}).get("total")))
st, dev_sorted = call("GET", "/api/v1/devices?page=1&pageSize=10&sortBy=code&sortDir=desc")
check("devices-sort", st == 200)
codes = [d.get("code") for d in dev_sorted.get("data", {}).get("items") or []]
if len(codes) >= 2:
    # desc: first >= last alphabetically
    check("devices-sort-order", codes[0] >= codes[-1], str(codes[:4]))

# --- ldap config surface ---
st, ldap = call("GET", "/api/v1/admin/ldap")
check("ldap-get", st == 200)

# --- rack diagram import ---
st, imp_dc = call("POST", "/api/v1/data-centers", {"code": f"IM{sfx}", "name": "导入中心"})
st, imp_room = call("POST", f"/api/v1/data-centers/{imp_dc['data']['id']}/rooms",
                    {"code": "IR", "name": "导入房"})
st, imp_rack = call("POST", f"/api/v1/rooms/{imp_room['data']['id']}/racks",
                    {"code": "IK", "name": "导入柜", "uHeight": 20})
imp_rid = imp_rack["data"]["id"]
st, imp_dev = call("POST", "/api/v1/devices",
                   {"typeId": tid, "code": f"IE{sfx}", "name": "已有设备", "heightU": 2})
call("POST", f"/api/v1/devices/{imp_dev['data']['id']}/assign",
     {"targetRackId": imp_rid, "startU": 1})
st, imp_val = call("POST", f"/api/v1/rooms/{imp_room['data']['id']}/rack-diagram-import/validate", {
    "formatVersion": "1",
    "dataCenterId": imp_dc["data"]["id"],
    "roomId": imp_room["data"]["id"],
    "coveredRackIds": [imp_rid],
    "defaultTypeId": tid,
    "devices": [
        {"clientId": "a", "rackId": imp_rid, "startU": 1, "endU": 2, "name": "已有设备",
         "sourceDeviceId": imp_dev["data"]["id"], "sourceDeviceCode": imp_dev["data"]["code"]},
        {"clientId": "b", "rackId": imp_rid, "startU": 5, "endU": 6, "name": "新设备B"},
    ],
})
imp_sum = imp_val.get("data", {}).get("summary") or {}
check("import-validate", st == 200 and imp_sum.get("errors") == 0 and imp_sum.get("create") == 1,
      str(imp_sum))
st, imp_com = call("POST", f"/api/v1/rooms/{imp_room['data']['id']}/rack-diagram-import/commit",
                   {"token": imp_val["data"]["token"], "decisions": []})
check("import-commit", st == 200 and imp_com["data"]["created"] == 1, str(imp_com.get("data")))
st, imp_again = call("POST", f"/api/v1/rooms/{imp_room['data']['id']}/rack-diagram-import/commit",
                     {"token": imp_val["data"]["token"], "decisions": []})
check("import-token-once", st == 400)

# --- concurrent race ---
results = []
def race(dev_id):
    st, body = call("POST", f"/api/v1/devices/{dev_id}/assign",
                    {"targetRackId": rack_id, "startU": 15})
    results.append((st, body.get("code")))
st, cdev1 = call("POST", "/api/v1/devices", {"typeId": tid, "code": f"C1{sfx}", "name": "A", "heightU": 2})
st, cdev2 = call("POST", "/api/v1/devices", {"typeId": tid, "code": f"C2{sfx}", "name": "B", "heightU": 2})
t1 = threading.Thread(target=race, args=(cdev1["data"]["id"],))
t2 = threading.Thread(target=race, args=(cdev2["data"]["id"],))
t1.start(); t2.start(); t1.join(); t2.join()
oks = [r for r in results if r[0] == 200]
conf = [r for r in results if r[0] == 409]
check("concurrent-1win", len(oks) == 1 and len(conf) == 1, str(results))

print("SMOALL_PASS" if not fails else "SMOALL_FAIL " + ",".join(fails))
sys.exit(0 if not fails else 1)
PY

echo "done base=$BASE"
