# 差分回放报告 v2（状态码+错误码 + 关键字段）：厂商原版 vs 重建版

- 基线用例（TEST/SETUP）：401；可比：391；并发对称（胜负互换，视为等价）：4；未产生：6
- **状态码+错误码口径一致（含命名等价/有意修复）：326 → 兼容率 83.4%**
- 关键字段口径一致：275 → 字段级兼容率 70.3%
- 关键字段差异：51；真实断裂：0

## 真实断裂（v2 口径）

- 无

## 关键字段差异明细（多为响应形状/嵌套填充差异，逐项修复属新源码重建范围）

| 用例 | 字段 | 基线 | 候选 |
|---|---|---|---|
| `SETUP-S02-RACK-CREATE` | data.templateSnapshot.revision | 0 | 1 |
| `SETUP-S02-RACK-CREATE` | data.templateSnapshot.uHeight | 0 | 42 |
| `S02-RESOURCE-TREE` | data.items[0].rooms[0].racks[0].templateSnapshot.revision | 0 | 1 |
| `S02-RESOURCE-TREE` | data.items[0].rooms[0].racks[0].templateSnapshot.uHeight | 0 | 42 |
| `S06-ASSIGN-SUCCESS` | data.device.type.status |  | ACTIVE |
| `S06-ASSIGN-SUCCESS` | data.device.type.version | 0 | 1 |
| `S06-ULAYOUT-READ` | data.positions[0].version | 1 | <absent> |
| `S06-ULAYOUT-READ` | data.rack.status | PARTIAL | <absent> |
| `S06-ULAYOUT-READ` | data.rack.uHeight | 42 | <absent> |
| `S06-ULAYOUT-READ` | data.rack.version | 2 | <absent> |
| `S06-ULAYOUT-READ` | …共 6 处 | | |
| `S06-ASSIGN-OVERLAP` | details.conflictDevices[0].device.heightU | 2 | <absent> |
| `S06-ASSIGN-OVERLAP` | details.conflictDevices[0].device.lifecycleStatus | RUNNING | <absent> |
| `S06-ASSIGN-OVERLAP` | details.conflictDevices[0].device.type.status | ACTIVE | <absent> |
| `S06-ASSIGN-OVERLAP` | details.conflictDevices[0].device.type.version | 1 | <absent> |
| `S06-ASSIGN-OVERLAP` | …共 12 处 | | |
| `S06-MOVE-SUCCESS` | data.device.type.status |  | ACTIVE |
| `S06-MOVE-SUCCESS` | data.device.type.version | 0 | 1 |
| `S06-DECOMMISSION` | data.device.heightU | 2 | <absent> |
| `S06-DECOMMISSION` | data.device.lifecycleStatus | OFF_RACK | <absent> |
| `S06-DECOMMISSION` | data.device.version | 4 | <absent> |
| `S06-DECOMMISSION` | data.executed | True | <absent> |
| `S06-DECOMMISSION` | …共 9 处 | | |
| `S07-DC-COPY-SUCCESS` | data.resource.status | PLANNING | <absent> |
| `S07-DC-COPY-SUCCESS` | data.resource.version | 1 | <absent> |
| `S07-DC-COPY-SUCCESS` | data.status | <absent> | OPERATING |
| `S07-DC-COPY-SUCCESS` | data.version | <absent> | 1 |
| `S07-ROOM-COPY-SUCCESS` | data.resource.status | PLANNING | <absent> |
| `S07-ROOM-COPY-SUCCESS` | data.resource.version | 1 | <absent> |
| `S07-ROOM-COPY-SUCCESS` | data.status | <absent> | OPERATING |
| `S07-ROOM-COPY-SUCCESS` | data.version | <absent> | 1 |
| `S07-RACK-COPY-SUCCESS` | data.resource.status | PLANNING | <absent> |
| `S07-RACK-COPY-SUCCESS` | data.resource.uHeight | 42 | <absent> |
| `S07-RACK-COPY-SUCCESS` | data.resource.version | 1 | <absent> |
| `S07-RACK-COPY-SUCCESS` | data.status | <absent> | AVAILABLE |
| `S07-RACK-COPY-SUCCESS` | …共 8 处 | | |
| `S08-ASSIGN-CREATES-APPROVAL` | data.approval.device.heightU | 0 | 2 |
| `S08-ASSIGN-CREATES-APPROVAL` | data.approval.device.lifecycleStatus |  | WAITING_RACK |
| `S08-ASSIGN-CREATES-APPROVAL` | data.approval.device.type.status |  | ACTIVE |
| `S08-ASSIGN-CREATES-APPROVAL` | data.approval.device.type.version | 0 | 1 |
| `S08-ASSIGN-CREATES-APPROVAL` | …共 5 处 | | |
| `S08-APPROVE-SUCCESS` | data.device.heightU | 2 | <absent> |
| `S08-APPROVE-SUCCESS` | data.device.lifecycleStatus | WAITING_RACK | <absent> |
| `S08-APPROVE-SUCCESS` | data.device.version | 1 | <absent> |
| `S08-APPROVE-SUCCESS` | data.heightU | <absent> | 2 |
| `S08-APPROVE-SUCCESS` | …共 8 处 | | |
| `S08-APPROVALS-LIST` | data.items[0].device.type.status |  | ACTIVE |
| `S08-APPROVALS-LIST` | data.items[0].device.type.version | 0 | 1 |
| `S08-STALE-ASSIGN-CREATES-APPROVAL` | data.approval.device.heightU | 0 | 2 |
| `S08-STALE-ASSIGN-CREATES-APPROVAL` | data.approval.device.lifecycleStatus |  | WAITING_RACK |
| `S08-STALE-ASSIGN-CREATES-APPROVAL` | data.approval.device.type.status |  | ACTIVE |
| `S08-STALE-ASSIGN-CREATES-APPROVAL` | data.approval.device.type.version | 0 | 1 |
| `S08-STALE-ASSIGN-CREATES-APPROVAL` | …共 5 处 | | |
| `S08-REJECTABLE-ASSIGN` | data.approval.device.heightU | 0 | 2 |
| `S08-REJECTABLE-ASSIGN` | data.approval.device.lifecycleStatus |  | WAITING_RACK |
| `S08-REJECTABLE-ASSIGN` | data.approval.device.type.status |  | ACTIVE |
| `S08-REJECTABLE-ASSIGN` | data.approval.device.type.version | 0 | 1 |
| `S08-REJECTABLE-ASSIGN` | …共 5 处 | | |
| `S09-TPL-NEW-VERSION` | data.isSystem | <absent> | True |
| `S09-TPL-NEW-VERSION` | data.revision | 2 | <absent> |
| `S09-TPL-NEW-VERSION` | data.status | <absent> | ACTIVE |
| `S09-TPL-NEW-VERSION` | data.uHeight | 45 | <absent> |
| `S09-TPL-NEW-VERSION` | …共 11 处 | | |
| `SETUP-S09-RACKB-CREATE` | data.templateSnapshot.revision | 0 | 2 |
| `SETUP-S09-RACKB-CREATE` | data.templateSnapshot.uHeight | 0 | 45 |
| `SETUP-S09-RACKB-CREATE` | data.uHeight | 42 | 45 |
| `SETUP-S10-DEVA-ASSIGN` | data.device.type.status |  | ACTIVE |
| `SETUP-S10-DEVA-ASSIGN` | data.device.type.version | 0 | 1 |
| `SETUP-S10-DEVX-ASSIGN` | data.device.type.status |  | ACTIVE |
| `SETUP-S10-DEVX-ASSIGN` | data.device.type.version | 0 | 1 |
| `S10-CONNECTIONS-LIST` | data.items[0].device.heightU | 2 | <absent> |
| `S10-CONNECTIONS-LIST` | data.items[0].device.lifecycleStatus | RUNNING | <absent> |
| `S10-CONNECTIONS-LIST` | data.items[0].device.version | 2 | <absent> |
| `S10-CONNECTIONS-LIST` | data.items[0].socket.status | CONNECTED | <absent> |
| `S10-CONNECTIONS-LIST` | …共 5 处 | | |
| `S10-PDU-LIST` | data.items[0].sockets | list[3] | <absent> |
| `S10-PDU-LIST` | data.items[0].sockets[0].status | AVAILABLE | <absent> |
| `S10-PDU-LIST` | data.items[0].sockets[0].version | 1 | <absent> |
| `S10-PDU-LIST` | data.items[0].sockets[1].status | AVAILABLE | <absent> |
| `S10-PDU-LIST` | …共 7 处 | | |
| `S05-DEV-LIST` | data.items[0].currentPosition.endU | 2 | <absent> |
| `S05-DEV-LIST` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S05-DEV-LIST` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S05-DEV-LIST` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S05-DEV-LIST` | …共 23 处 | | |
| `S05-DEV-LIST-FILTER` | data.items[0].currentPosition.endU | 2 | <absent> |
| `S05-DEV-LIST-FILTER` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S05-DEV-LIST-FILTER` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S05-DEV-LIST-FILTER` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S05-DEV-LIST-FILTER` | …共 23 处 | | |
| `S12-IMPORT-VALIDATE` | data.items[1].allowedActions | <absent> | list[2] |
| `S12-IMPORT-VALIDATE` | data.items[2].allowedActions | <absent> | list[2] |
| `S12-IMPORT-VALIDATE` | data.summary.decisions | 0 | 2 |
| `S12-IMPORT-VALIDATE` | data.summary.total | 3 | <absent> |
| `S12-IMPORT-COMMIT` | data.ignored | <absent> | 2 |
| `S11-USER-RESET-PASSWORD` | data.enabled | True | <absent> |
| `S11-USER-RESET-PASSWORD` | data.roles[0].version | 1 | <absent> |
| `S11-USER-RESET-PASSWORD` | data.version | 2 | <absent> |
| `SETUP-S08C-ASSIGN` | data.approval.device.heightU | 0 | 2 |
| `SETUP-S08C-ASSIGN` | data.approval.device.lifecycleStatus |  | WAITING_RACK |
| `SETUP-S08C-ASSIGN` | data.approval.device.type.status |  | ACTIVE |
| `SETUP-S08C-ASSIGN` | data.approval.device.type.version | 0 | 1 |
| `SETUP-S08C-ASSIGN` | …共 5 处 | | |
| `SETUP-S10C-DEV0-ASSIGN` | data.device.type.status |  | ACTIVE |
| `SETUP-S10C-DEV0-ASSIGN` | data.device.type.version | 0 | 1 |
| `SETUP-S10C-DEV1-ASSIGN` | data.device.type.status |  | ACTIVE |
| `SETUP-S10C-DEV1-ASSIGN` | data.device.type.version | 0 | 1 |
| `S10C-CONCURRENT-CONNECT-CLIENT1` | data.version | <absent> | 1 |
| `S10C-CONCURRENT-CONNECT-CLIENT0` | data.version | 1 | <absent> |
| `S14-S14-PAGE-1` | data.items[0].currentPosition.endU | 6 | <absent> |
| `S14-S14-PAGE-1` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S14-S14-PAGE-1` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S14-S14-PAGE-1` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S14-S14-PAGE-1` | …共 10 处 | | |
| `S14-S14-PAGE-2` | data.items[0].currentPosition.endU | 4 | <absent> |
| `S14-S14-PAGE-2` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S14-S14-PAGE-2` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S14-S14-PAGE-2` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S14-S14-PAGE-2` | …共 11 处 | | |
| `S14-S14-PAGE-FILTER-TYPE` | data.items[0].currentPosition.endU | 6 | <absent> |
| `S14-S14-PAGE-FILTER-TYPE` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S14-S14-PAGE-FILTER-TYPE` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S14-S14-PAGE-FILTER-TYPE` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S14-S14-PAGE-FILTER-TYPE` | …共 23 处 | | |
| `S14-S14-PAGE-FILTER-LIFE` | data.items[0].heightU | 2 | 4 |
| `S14-S14-PAGE-FILTER-LIFE` | data.items[1].version | 1 | 2 |
| `S14-S14-PAGE-FILTER-LIFE` | data.total | 5 | 4 |
| `S14-S14-PAGE-FILTER-SEARCH` | data.items[0].currentPosition.endU | 6 | <absent> |
| `S14-S14-PAGE-FILTER-SEARCH` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S14-S14-PAGE-FILTER-SEARCH` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S14-S14-PAGE-FILTER-SEARCH` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S14-S14-PAGE-FILTER-SEARCH` | …共 23 处 | | |
| `S14-S14-PAGE-FILTER-COMBO` | data.items[0].heightU | 2 | 4 |
| `S14-S14-PAGE-FILTER-COMBO` | data.items[1].version | 1 | 2 |
| `S14-S14-PAGE-FILTER-COMBO` | data.total | 5 | 4 |
| `S15-TPL-LIST-SUCCESS` | data.items[0].versions[0].revision | 2 | 1 |
| `S15-TPL-LIST-SUCCESS` | data.items[0].versions[0].uHeight | 45 | 42 |
| `S15-TPL-LIST-SUCCESS` | data.items[0].versions[1].revision | 1 | 2 |
| `S15-TPL-LIST-SUCCESS` | data.items[0].versions[1].uHeight | 42 | 45 |
| `S15-TPL-DUP-REVISION` | data.isSystem | <absent> | True |
| `S15-TPL-DUP-REVISION` | data.revision | 3 | <absent> |
| `S15-TPL-DUP-REVISION` | data.status | <absent> | ACTIVE |
| `S15-TPL-DUP-REVISION` | data.uHeight | 42 | <absent> |
| `S15-TPL-DUP-REVISION` | …共 14 处 | | |
| `S16-PAGE-1` | data.items[0].lifecycleStatus | WAITING_RACK | OFF_RACK |
| `S16-PAGE-1` | data.items[0].version | 1 | 4 |
| `S16-PAGE-2` | data.items[0].heightU | 3 | 4 |
| `S16-DEV-WAITING` | data.items[0].heightU | 2 | 4 |
| `S16-DEV-WAITING` | data.items[1].heightU | 3 | 2 |
| `S16-DEV-WAITING` | data.items[1].version | 1 | 2 |
| `S16-DEV-WAITING` | data.total | 7 | 6 |
| `S16-ASSIGN-OK` | data.device.heightU | 2 | 4 |
| `S16-ASSIGN-OK` | data.device.type.status |  | ACTIVE |
| `S16-ASSIGN-OK` | data.device.type.version | 0 | 1 |
| `S16-ASSIGN-OK` | data.position.endU | 39 | 41 |
| `S16-ASSIGN-OK` | …共 5 处 | | |
| `S16-ASSIGN-READBACK` | data.currentPosition.endU | 39 | <absent> |
| `S16-ASSIGN-READBACK` | data.currentPosition.heightU | 2 | <absent> |
| `S16-ASSIGN-READBACK` | data.currentPosition.orientation | NORMAL | <absent> |
| `S16-ASSIGN-READBACK` | data.currentPosition.rack.status | PARTIAL | <absent> |
| `S16-ASSIGN-READBACK` | …共 9 处 | | |
| `S16-MOVE-READBACK` | data.heightU | 3 | 2 |
| `S16-MOVE-READBACK` | data.version | 1 | 2 |
| `S17-DEV-PAGE-1` | data.items[0].currentPosition.endU | 39 | <absent> |
| `S17-DEV-PAGE-1` | data.items[0].currentPosition.heightU | 2 | <absent> |
| `S17-DEV-PAGE-1` | data.items[0].currentPosition.orientation | NORMAL | <absent> |
| `S17-DEV-PAGE-1` | data.items[0].currentPosition.rack.status | PARTIAL | <absent> |
| `S17-DEV-PAGE-1` | …共 10 处 | | |
| `S17-DEV-PAGE-2` | data.items[0].heightU | 3 | 4 |
| `S17-DEV-PAGE-2` | data.items[0].lifecycleStatus | WAITING_RACK | RUNNING |
| `S17-DEV-PAGE-2` | data.items[0].version | 1 | 2 |
| `S17-ASSIGN-EXECUTE` | data.device.heightU | 3 | 2 |
| `S17-ASSIGN-EXECUTE` | data.device.type.status |  | ACTIVE |
| `S17-ASSIGN-EXECUTE` | data.device.type.version | 0 | 1 |
| `S17-ASSIGN-EXECUTE` | data.device.version | 2 | 3 |
| `S17-ASSIGN-EXECUTE` | …共 7 处 | | |
| `S18-ASSIGN-EXEC-13` | data.device.type.status |  | ACTIVE |
| `S18-ASSIGN-EXEC-13` | data.device.type.version | 0 | 1 |
| `S18-ASSIGN-EXEC-13` | data.position.endU | 33 | 35 |
| `S18-ASSIGN-EXEC-13` | data.position.startU | 32 | 34 |
| `S18-ASSIGN-READ-13` | data.currentPosition.endU | 33 | <absent> |
| `S18-ASSIGN-READ-13` | data.currentPosition.heightU | 2 | <absent> |
| `S18-ASSIGN-READ-13` | data.currentPosition.orientation | NORMAL | <absent> |
| `S18-ASSIGN-READ-13` | data.currentPosition.rack.status | PARTIAL | <absent> |
| `S18-ASSIGN-READ-13` | …共 8 处 | | |
| `S18-ASSIGN-LAYOUT-13` | data.free[0].heightU | 23 | 21 |
| `S18-ASSIGN-LAYOUT-13` | data.free[0].startU | 7 | 9 |
| `S18-ASSIGN-LAYOUT-13` | data.free[1].heightU | 4 | 2 |
| `S18-ASSIGN-LAYOUT-13` | data.free[1].startU | 34 | 36 |
| `S18-ASSIGN-LAYOUT-13` | …共 23 处 | | |

## 级联缺失：6 项
- S14: 6
