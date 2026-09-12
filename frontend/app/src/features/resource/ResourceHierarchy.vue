<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  fetchResourceTree,
  fetchRackTemplates,
  createDataCenter,
  updateDataCenter,
  deleteDataCenter,
  copyDataCenter,
  createRoom,
  updateRoom,
  deleteRoom,
  copyRoom,
  moveRoom,
  createRack,
  updateRack,
  deleteRack,
  copyRack,
  moveRack,
} from "@/features/resource/api";
import type {
  TreeDataCenter,
  RackTemplate,
  DataCenterInput,
  RoomInput,
  RackInput,
  CopyMoveInput,
} from "@/features/resource/api";
import type { Rack } from "@/features/resource/api";
import { dcRoomStatus, rackStatus } from "@/features/resource/resourceStatus";
import { reportError } from "@/api/errors";

/**
 * 资源层级(第 6 轮 100% 复刻 v2 ResourceManagementView):
 * 页头(基础资源管理+刷新/新增数据中心)、统计条、三栏(数据中心/机房/机柜)
 * 级联选择与 CRUD/复制/移动。文案、状态映射、结构均按 v2 编译产物逐项对照。
 * 差异说明:v2 机柜模板下拉由模板接口提供,此处同样接入 GET /rack-templates。
 */

const tree = ref<TreeDataCenter[]>([]);
const templates = ref<RackTemplate[]>([]);
const loading = ref(false);
const selectedDcId = ref("");
const selectedRoomId = ref("");

const selectedDc = computed(() => tree.value.find((d) => d.id === selectedDcId.value));
const rooms = computed(() => selectedDc.value?.rooms ?? []);
const selectedRoom = computed(() => rooms.value.find((r) => r.id === selectedRoomId.value));
const racks = computed(() => selectedRoom.value?.racks ?? []);
const dcCount = computed(() => tree.value.length);
const roomCount = computed(() => tree.value.reduce((s, d) => s + (d.rooms?.length ?? 0), 0));
const rackCount = computed(() =>
  tree.value.reduce(
    (s, d) => s + (d.rooms ?? []).reduce((r, room) => r + (room.racks?.length ?? 0), 0),
    0,
  ),
);

function selectDc(id: string) {
  selectedDcId.value = id;
  selectedRoomId.value = rooms.value[0]?.id ?? "";
}
function selectRoom(id: string) {
  selectedRoomId.value = id;
}

async function load(keepSelection = true) {
  loading.value = true;
  try {
    tree.value = await fetchResourceTree();
    if (!keepSelection || !tree.value.some((d) => d.id === selectedDcId.value)) {
      selectedDcId.value = tree.value[0]?.id ?? "";
    }
    if (!keepSelection || !rooms.value.some((r) => r.id === selectedRoomId.value)) {
      selectedRoomId.value = rooms.value[0]?.id ?? "";
    }
  } catch (e) {
    reportError({
      message: `resource tree load failed: ${e instanceof Error ? e.message : String(e)}`,
    });
    ElMessage.error("资源加载失败");
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  await load(false);
  fetchRackTemplates()
    .then((t) => (templates.value = t))
    .catch(() => (templates.value = []));
});

/* ── 通用小工具 ── */
function genCode(prefix: string): string {
  return `${prefix}-${Math.floor(1000 + Math.random() * 9000)}`;
}
function fail(e: unknown, fallback: string) {
  ElMessage.error(e instanceof Error ? e.message : fallback);
}
const DONE = {
  dc: "数据中心已保存",
  room: "机房已保存",
  rack: "机柜已保存",
  copied: "复制成功",
  moved: "移动成功",
  deleted: "已删除",
};

/* ── 数据中心对话框(新增/编辑;v2 字段:编码[自动生成]/名称/负责人/地址/联系方式/服务商/备注) ── */
const dcDialog = reactive({ visible: false, editing: false, id: "", version: 1, autoCode: true });
const dcForm = reactive<DataCenterInput>({ code: "", name: "" });

function openDcCreate() {
  Object.assign(dcDialog, { visible: true, editing: false, id: "", version: 1, autoCode: true });
  Object.assign(dcForm, { code: "", name: "" });
}
function openDcEdit() {
  const dc = selectedDc.value;
  if (!dc) return;
  Object.assign(dcDialog, {
    visible: true,
    editing: true,
    id: dc.id,
    version: dc.version ?? 1,
    autoCode: false,
  });
  Object.assign(dcForm, {
    code: dc.code ?? "",
    name: dc.name ?? "",
    manager: dc.manager ?? "",
    address: dc.address ?? "",
    contact: dc.contact ?? "",
    serviceProvider: dc.serviceProvider ?? "",
    remarks: dc.remarks ?? "",
  });
}
async function submitDc() {
  if (!dcForm.name) {
    ElMessage.warning("名称不能为空");
    return;
  }
  if (!dcDialog.editing && dcDialog.autoCode && !dcForm.code) {
    dcForm.code = genCode("DC");
  }
  if (!dcForm.code) {
    ElMessage.warning("资源编码不能为空");
    return;
  }
  try {
    if (dcDialog.editing) await updateDataCenter(dcDialog.id, dcDialog.version, dcForm);
    else await createDataCenter(dcForm);
    dcDialog.visible = false;
    ElMessage.success(DONE.dc);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}

/* ── 机房对话框(编码/名称/面积/楼栋/楼层/房间号/用途/平面图启用/每排机柜数) ── */
const roomDialog = reactive({ visible: false, editing: false, id: "", version: 1 });
const roomForm = reactive<RoomInput>({ code: "", name: "" });

function openRoomCreate() {
  if (!selectedDc.value) {
    ElMessage.warning("请先选择数据中心");
    return;
  }
  Object.assign(roomDialog, { visible: true, editing: false, id: "", version: 1 });
  Object.assign(roomForm, {
    code: "",
    name: "",
    building: "",
    floor: "",
    roomNumber: "",
    purpose: "",
    floorPlanEnabled: false,
    racksPerRow: 8,
  });
}
function openRoomEdit() {
  const room = selectedRoom.value;
  if (!room) return;
  Object.assign(roomDialog, {
    visible: true,
    editing: true,
    id: room.id,
    version: room.version ?? 1,
  });
  Object.assign(roomForm, {
    code: room.code ?? "",
    name: room.name ?? "",
    building: room.building ?? "",
    floor: room.floor ?? "",
    roomNumber: room.roomNumber ?? "",
    areaSquareMeters: room.areaSquareMeters,
    purpose: room.purpose ?? "",
    floorPlanEnabled: room.floorPlanEnabled ?? false,
    racksPerRow: room.racksPerRow ?? 8,
  });
}
async function submitRoom() {
  if (!roomForm.code || !roomForm.name) {
    ElMessage.warning("编码和名称不能为空");
    return;
  }
  try {
    if (roomDialog.editing) await updateRoom(roomDialog.id, roomDialog.version, roomForm);
    else if (selectedDc.value?.id) await createRoom(selectedDc.value.id, roomForm);
    roomDialog.visible = false;
    ElMessage.success(DONE.room);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}

/* ── 机柜对话框(编码/名称/模板/宽深高/厂商/型号/区域/通道/双路电源/输入路数) ── */
const rackDialog = reactive({ visible: false, editing: false, id: "", version: 1 });
const rackForm = reactive<RackInput>({ code: "", name: "" });

function openRackCreate() {
  if (!selectedRoom.value) {
    ElMessage.warning("请先选择机房");
    return;
  }
  Object.assign(rackDialog, { visible: true, editing: false, id: "", version: 1 });
  Object.assign(rackForm, {
    code: "",
    name: "",
    templateId: undefined,
    manufacturer: "",
    modelNumber: "",
    widthMm: 600,
    depthMm: 1200,
    heightMm: 2000,
    zone: "",
    aisle: "",
    dualPower: false,
    inputCircuits: 0,
  });
}
function openRackEdit(rack: Rack) {
  Object.assign(rackDialog, {
    visible: true,
    editing: true,
    id: rack.id ?? "",
    version: rack.version ?? 1,
  });
  Object.assign(rackForm, {
    code: rack.code ?? "",
    name: rack.name ?? "",
    templateId: rack.templateId ?? undefined,
    manufacturer: rack.manufacturer ?? "",
    modelNumber: rack.modelNumber ?? "",
    widthMm: rack.widthMm ?? 600,
    depthMm: rack.depthMm ?? 1200,
    heightMm: rack.heightMm ?? 2000,
    zone: rack.zone ?? "",
    aisle: rack.aisle ?? "",
    dualPower: rack.dualPower ?? false,
    inputCircuits: rack.inputCircuits ?? 0,
  });
}
async function submitRack() {
  if (!rackForm.code || !rackForm.name) {
    ElMessage.warning("编码和名称不能为空");
    return;
  }
  try {
    if (rackDialog.editing) await updateRack(rackDialog.id, rackDialog.version, rackForm);
    else if (selectedRoom.value?.id) await createRack(selectedRoom.value.id, rackForm);
    rackDialog.visible = false;
    ElMessage.success(DONE.rack);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}

/* ── 复制/移动对话框(v2 CopyMoveBody:目标数据中心/目标机房/副本名称/资源编码) ── */
type CopyKind = "dc" | "room" | "rack";
const copyDialog = reactive<{
  visible: boolean;
  kind: CopyKind;
  id: string;
  version: number;
  name: string;
  targetDcId: string;
  targetRoomId: string;
}>({ visible: false, kind: "dc", id: "", version: 1, name: "", targetDcId: "", targetRoomId: "" });
const copyForm = reactive<CopyMoveInput>({ code: "", name: "" });

const COPY_EXPLAIN: Record<CopyKind, string> = {
  dc: "复制数据中心及其下属机房、机柜结构；不会复制设备、PDU 实例和资产唯一编号。",
  room: "复制机房及其下属机柜结构；不会复制设备、PDU 实例和资产唯一编号。",
  rack: "只复制机柜参数和模板快照，不复制设备、PDU 实例、序列号和资产编号。",
};
const COPY_TITLE: Record<CopyKind, string> = {
  dc: "复制数据中心",
  room: "复制机房",
  rack: "复制机柜",
};

function openCopy(
  kind: CopyKind,
  id: string | undefined,
  version: number | undefined,
  name: string,
) {
  Object.assign(copyDialog, {
    visible: true,
    kind,
    id: id ?? "",
    version: version ?? 1,
    name,
    targetDcId: kind === "room" ? (selectedDc.value?.id ?? "") : "",
    targetRoomId: kind === "rack" ? (selectedRoom.value?.id ?? "") : "",
  });
  Object.assign(copyForm, { code: "", name: "" });
}
async function submitCopy() {
  if (!copyForm.name) {
    ElMessage.warning("副本名称不能为空");
    return;
  }
  if (copyDialog.kind !== "dc" && !copyForm.code) {
    copyForm.code = genCode(copyDialog.kind === "room" ? "R" : "K");
  }
  try {
    const body = { ...copyForm };
    if (copyDialog.kind === "dc") await copyDataCenter(copyDialog.id, copyDialog.version, body);
    else if (copyDialog.kind === "room") await copyRoom(copyDialog.id, copyDialog.version, body);
    else await copyRack(copyDialog.id, copyDialog.version, body);
    copyDialog.visible = false;
    ElMessage.success(DONE.copied);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}

/* ── 移动对话框 ── */
const moveDialog = reactive<{
  visible: boolean;
  kind: "room" | "rack";
  id: string;
  version: number;
  targetDcId: string;
  targetRoomId: string;
}>({ visible: false, kind: "room", id: "", version: 1, targetDcId: "", targetRoomId: "" });

const MOVE_EXPLAIN = {
  room: "移动后，下属机柜、设备 U 位和 PDU 关系保持不变，并同步到目标数据中心。",
  rack: "移动后，设备、U 位和 PDU 关系保持不变；机柜在大屏中的坐标会清空。",
};
const MOVE_TITLE = { room: "移动到其他数据中心", rack: "移动到其他机房" };

function openMove(kind: "room" | "rack", id: string | undefined, version: number | undefined) {
  Object.assign(moveDialog, {
    visible: true,
    kind,
    id: id ?? "",
    version: version ?? 1,
    targetDcId:
      kind === "room"
        ? (tree.value.find((d) => d.id !== selectedDc.value?.id)?.id ?? "")
        : (selectedDc.value?.id ?? ""),
    targetRoomId: "",
  });
}
async function submitMove() {
  if (moveDialog.kind === "room" && !moveDialog.targetDcId) {
    ElMessage.warning("请选择目标数据中心");
    return;
  }
  if (moveDialog.kind === "rack" && !moveDialog.targetRoomId) {
    ElMessage.warning("请选择目标机房");
    return;
  }
  const body: CopyMoveInput = {
    code: genCode(moveDialog.kind === "room" ? "R" : "K"),
    name: moveDialog.kind === "room" ? "副本" : "副本",
    targetDataCenterId: moveDialog.targetDcId || undefined,
    targetRoomId: moveDialog.targetRoomId || undefined,
  };
  try {
    if (moveDialog.kind === "room") await moveRoom(moveDialog.id, moveDialog.version, body);
    else await moveRack(moveDialog.id, moveDialog.version, body);
    moveDialog.visible = false;
    ElMessage.success(DONE.moved);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}

/* ── 删除(v2:删除确认 → 已删除) ── */
async function removeDc() {
  const dc = selectedDc.value;
  if (!dc) return;
  try {
    await ElMessageBox.confirm(
      `确认删除数据中心「${dc.name}」及其下属全部机房与机柜？`,
      "删除确认",
      {
        type: "warning",
      },
    );
  } catch {
    return;
  }
  try {
    await deleteDataCenter(dc.id ?? "", dc.version ?? 1);
    ElMessage.success(DONE.deleted);
    await load(false);
  } catch (e) {
    fail(e, "操作失败");
  }
}
async function removeRoom() {
  const room = selectedRoom.value;
  if (!room) return;
  try {
    await ElMessageBox.confirm(`确认删除机房「${room.name}」及其下属机柜？`, "删除确认", {
      type: "warning",
    });
  } catch {
    return;
  }
  try {
    await deleteRoom(room.id ?? "", room.version ?? 1);
    ElMessage.success(DONE.deleted);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}
async function removeRack(rack: Rack) {
  try {
    await ElMessageBox.confirm(`确认删除机柜「${rack.name}」？`, "删除确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deleteRack(rack.id ?? "", rack.version ?? 1);
    ElMessage.success(DONE.deleted);
    await load();
  } catch (e) {
    fail(e, "操作失败");
  }
}

const rackLocation = (rack: Rack) => {
  const row = rack.rackRow ?? "";
  const col = rack.rackColumn ?? "";
  return row || col ? `${row}${col}` : "-";
};
</script>

<template>
  <div v-loading="loading" class="resource-page" data-test="resource-page">
    <div class="page-header">
      <div>
        <h2>基础资源管理</h2>
        <p>按数据中心、机房、机柜逐级维护资源，当前支持整 U 机柜和可选电源参数。</p>
      </div>
      <div class="header-actions">
        <el-button @click="load()">刷新</el-button>
        <el-button type="primary" data-test="dc-create-btn" @click="openDcCreate">
          新增数据中心
        </el-button>
      </div>
    </div>

    <el-row class="summary-row" :gutter="16">
      <el-col :xs="24" :sm="8">
        <div class="summary-box">
          <div class="summary">
            <span>数据中心</span><strong data-test="summary-dc">{{ dcCount }}</strong>
          </div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="8">
        <div class="summary-box">
          <div class="summary">
            <span>机房</span><strong data-test="summary-room">{{ roomCount }}</strong>
          </div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="8">
        <div class="summary-box">
          <div class="summary">
            <span>机柜</span><strong data-test="summary-rack">{{ rackCount }}</strong>
          </div>
        </div>
      </el-col>
    </el-row>

    <el-row class="resource-columns" :gutter="16">
      <!-- 数据中心列 -->
      <el-col :xs="24" :md="8">
        <el-card class="resource-card" shadow="never">
          <template #header>
            <div class="card-title">
              <span>数据中心</span>
              <el-tag size="small" type="info" effect="plain">{{ dcCount }} 个</el-tag>
            </div>
          </template>
          <div v-if="tree.length === 0" class="empty-hint">暂无数据中心</div>
          <div
            v-for="dc in tree"
            :key="dc.id"
            class="resource-item"
            :class="{ active: dc.id === selectedDcId }"
            data-test="dc-item"
            @click="selectDc(dc.id ?? '')"
          >
            <div class="item-main">
              <div class="item-name">{{ dc.name }}</div>
              <div class="item-code">{{ dc.code }} · {{ dc.rooms?.length ?? 0 }} 个机房</div>
            </div>
            <el-tag size="small" :type="dcRoomStatus(dc.status).tag">
              {{ dcRoomStatus(dc.status).label }}
            </el-tag>
            <el-dropdown
              trigger="click"
              @command="
                (cmd: string) =>
                  cmd === 'edit'
                    ? openDcEdit()
                    : cmd === 'copy'
                      ? openCopy('dc', dc.id, dc.version, dc.name ?? '')
                      : removeDc()
              "
            >
              <span class="item-more" @click.stop>···</span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">编辑</el-dropdown-item>
                  <el-dropdown-item command="copy">复制数据中心</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-card>
      </el-col>

      <!-- 机房列 -->
      <el-col :xs="24" :md="8">
        <el-card class="resource-card" shadow="never">
          <template #header>
            <div class="card-title">
              <span>机房</span>
              <el-button link type="primary" data-test="room-create-btn" @click="openRoomCreate">
                新增
              </el-button>
            </div>
          </template>
          <div v-if="!selectedDc" class="empty-hint">请先选择数据中心</div>
          <div v-else-if="rooms.length === 0" class="empty-hint">暂无机房</div>
          <div
            v-for="room in rooms"
            :key="room.id"
            class="resource-item"
            :class="{ active: room.id === selectedRoomId }"
            data-test="room-item"
            @click="selectRoom(room.id ?? '')"
          >
            <div class="item-main">
              <div class="item-name">{{ room.name }}</div>
              <div class="item-code">{{ room.code }} · {{ room.racks?.length ?? 0 }} 个机柜</div>
              <div class="item-meta">未设置物理位置</div>
            </div>
            <el-tag size="small" :type="dcRoomStatus(room.status).tag">
              {{ dcRoomStatus(room.status).label }}
            </el-tag>
            <el-dropdown
              trigger="click"
              @command="
                (cmd: string) =>
                  cmd === 'edit'
                    ? openRoomEdit()
                    : cmd === 'copy'
                      ? openCopy('room', room.id, room.version, room.name ?? '')
                      : cmd === 'move'
                        ? openMove('room', room.id, room.version)
                        : removeRoom()
              "
            >
              <span class="item-more" @click.stop>···</span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">编辑</el-dropdown-item>
                  <el-dropdown-item command="copy">复制机房</el-dropdown-item>
                  <el-dropdown-item command="move">移动到其他数据中心</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-card>
      </el-col>

      <!-- 机柜列 -->
      <el-col :xs="24" :md="8">
        <el-card class="resource-card rack-card" shadow="never">
          <template #header>
            <div class="card-title">
              <span>机柜</span>
              <el-button link type="primary" data-test="rack-create-btn" @click="openRackCreate">
                新增
              </el-button>
            </div>
          </template>
          <div v-if="!selectedRoom" class="empty-hint">请先选择机房</div>
          <el-table
            v-else
            :data="racks"
            size="small"
            :empty-text="'暂无机柜'"
            data-test="rack-table"
          >
            <el-table-column prop="code" label="编码" min-width="70" show-overflow-tooltip />
            <el-table-column prop="name" label="名称" min-width="80" show-overflow-tooltip />
            <el-table-column label="模板" min-width="90">
              <template #default="{ row }">
                {{ row.templateName || "自定义参数" }}
              </template>
            </el-table-column>
            <el-table-column label="位置" width="60">
              <template #default="{ row }">{{ rackLocation(row) }}</template>
            </el-table-column>
            <el-table-column label="容量" width="60">
              <template #default="{ row }">{{ row.uHeight }}U</template>
            </el-table-column>
            <el-table-column label="状态" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="rackStatus(row.status).tag">
                  {{ rackStatus(row.status).label }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="70">
              <template #default="{ row }">
                <el-dropdown
                  trigger="click"
                  @command="
                    (cmd: string) =>
                      cmd === 'edit'
                        ? openRackEdit(row)
                        : cmd === 'copy'
                          ? openCopy('rack', row.id, row.version, row.name ?? '')
                          : cmd === 'move'
                            ? openMove('rack', row.id, row.version)
                            : removeRack(row)
                  "
                >
                  <el-button link size="small">更多</el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="edit">编辑</el-dropdown-item>
                      <el-dropdown-item command="copy">复制机柜</el-dropdown-item>
                      <el-dropdown-item command="move">移动到其他机房</el-dropdown-item>
                      <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- 数据中心对话框 -->
    <el-dialog
      v-model="dcDialog.visible"
      :title="dcDialog.editing ? '编辑数据中心' : '新增数据中心'"
      width="520px"
    >
      <el-form label-width="90px">
        <el-form-item label="编码">
          <div class="code-field">
            <el-input
              v-model="dcForm.code"
              :disabled="dcDialog.editing || dcDialog.autoCode"
              placeholder="请输入数据中心编码"
            />
            <el-checkbox v-if="!dcDialog.editing" v-model="dcDialog.autoCode">
              保存时自动生成
            </el-checkbox>
            <el-button
              v-if="!dcDialog.editing && !dcDialog.autoCode"
              @click="dcForm.code = genCode('DC')"
            >
              自动生成
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="名称"><el-input v-model="dcForm.name" /></el-form-item>
        <el-form-item label="负责人"><el-input v-model="dcForm.manager" /></el-form-item>
        <el-form-item label="地址"><el-input v-model="dcForm.address" /></el-form-item>
        <el-form-item label="联系方式"><el-input v-model="dcForm.contact" /></el-form-item>
        <el-form-item label="服务商"><el-input v-model="dcForm.serviceProvider" /></el-form-item>
        <el-form-item label="备注">
          <el-input v-model="dcForm.remarks" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dcDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="submitDc">保存</el-button>
      </template>
    </el-dialog>

    <!-- 机房对话框 -->
    <el-dialog
      v-model="roomDialog.visible"
      :title="roomDialog.editing ? '编辑机房' : '新增机房'"
      width="520px"
    >
      <el-form label-width="90px">
        <el-form-item label="编码">
          <el-input v-model="roomForm.code" placeholder="请输入机房编码" />
        </el-form-item>
        <el-form-item label="名称"><el-input v-model="roomForm.name" /></el-form-item>
        <el-form-item label="面积(㎡)">
          <el-input-number v-model="roomForm.areaSquareMeters" :min="0" :controls="false" />
        </el-form-item>
        <el-form-item label="楼栋"><el-input v-model="roomForm.building" /></el-form-item>
        <el-form-item label="楼层"><el-input v-model="roomForm.floor" /></el-form-item>
        <el-form-item label="房间号"><el-input v-model="roomForm.roomNumber" /></el-form-item>
        <el-form-item label="用途"><el-input v-model="roomForm.purpose" /></el-form-item>
        <el-form-item label="平面图">
          <el-switch v-model="roomForm.floorPlanEnabled" active-text="启用" inactive-text="关闭" />
        </el-form-item>
        <el-form-item v-if="roomForm.floorPlanEnabled" label="每排机柜数">
          <el-input-number v-model="roomForm.racksPerRow" :min="1" />
          <div class="form-tip">用于控制机房资源大屏每排最多展示的机柜数量，超过后自动换行。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roomDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="submitRoom">保存</el-button>
      </template>
    </el-dialog>

    <!-- 机柜对话框 -->
    <el-dialog
      v-model="rackDialog.visible"
      :title="rackDialog.editing ? '编辑机柜' : '新增机柜'"
      width="540px"
    >
      <el-form label-width="90px">
        <el-form-item label="编码">
          <el-input v-model="rackForm.code" placeholder="请输入机柜编码" />
        </el-form-item>
        <el-form-item label="名称"><el-input v-model="rackForm.name" /></el-form-item>
        <el-form-item label="机柜模板">
          <el-select
            v-model="rackForm.templateId"
            clearable
            filterable
            placeholder="不选择则使用自定义参数"
            style="width: 100%"
          >
            <el-option
              v-for="t in templates"
              :key="t.id"
              :label="`${t.name}（${t.code}）`"
              :value="t.id ?? ''"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="宽(mm)">
          <el-input-number v-model="rackForm.widthMm" :min="1" :controls="false" />
        </el-form-item>
        <el-form-item label="深(mm)">
          <el-input-number v-model="rackForm.depthMm" :min="1" :controls="false" />
        </el-form-item>
        <el-form-item label="高(mm)">
          <el-input-number v-model="rackForm.heightMm" :min="1" :controls="false" />
        </el-form-item>
        <el-form-item label="厂商"><el-input v-model="rackForm.manufacturer" /></el-form-item>
        <el-form-item label="型号"><el-input v-model="rackForm.modelNumber" /></el-form-item>
        <el-form-item label="区域"><el-input v-model="rackForm.zone" /></el-form-item>
        <el-form-item label="通道"><el-input v-model="rackForm.aisle" /></el-form-item>
        <el-form-item label="双路电源">
          <el-switch v-model="rackForm.dualPower" />
        </el-form-item>
        <el-form-item label="输入路数">
          <el-input-number v-model="rackForm.inputCircuits" :min="0" :controls="false" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rackDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="submitRack">保存</el-button>
      </template>
    </el-dialog>

    <!-- 复制对话框 -->
    <el-dialog v-model="copyDialog.visible" :title="COPY_TITLE[copyDialog.kind]" width="480px">
      <el-alert
        type="info"
        :closable="false"
        :title="COPY_EXPLAIN[copyDialog.kind]"
        class="copy-tip"
      />
      <el-form label-width="110px">
        <el-form-item v-if="copyDialog.kind === 'room'" label="目标数据中心">
          <el-select v-model="copyDialog.targetDcId" style="width: 100%">
            <el-option v-for="d in tree" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="copyDialog.kind === 'rack'" label="目标机房">
          <el-select v-model="copyDialog.targetRoomId" style="width: 100%">
            <el-option
              v-for="opt in tree.flatMap((d) =>
                (d.rooms ?? []).map((r) => ({ id: r.id ?? '', label: `${d.name} / ${r.name}` })),
              )"
              :key="opt.id"
              :label="opt.label"
              :value="opt.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="副本名称">
          <el-input v-model="copyForm.name" />
        </el-form-item>
        <el-form-item label="资源编码">
          <el-input v-model="copyForm.code" placeholder="提交时自动生成" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="copyDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="submitCopy">确认</el-button>
      </template>
    </el-dialog>

    <!-- 移动对话框 -->
    <el-dialog v-model="moveDialog.visible" :title="MOVE_TITLE[moveDialog.kind]" width="480px">
      <el-alert
        type="info"
        :closable="false"
        :title="MOVE_EXPLAIN[moveDialog.kind]"
        class="copy-tip"
      />
      <el-form label-width="110px">
        <el-form-item v-if="moveDialog.kind === 'room'" label="目标数据中心">
          <el-select v-model="moveDialog.targetDcId" style="width: 100%">
            <el-option
              v-for="d in tree.filter((x) => x.id !== selectedDcId)"
              :key="d.id"
              :label="d.name"
              :value="d.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="moveDialog.kind === 'rack'" label="目标机房">
          <el-select v-model="moveDialog.targetRoomId" style="width: 100%">
            <el-option
              v-for="room in tree
                .filter((d) => d.id === selectedDcId)
                .flatMap((d) => d.rooms ?? [])
                .filter((r) => r.id !== selectedRoomId)"
              :key="room.id"
              :label="`${room.name}（${room.code}）`"
              :value="room.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="moveDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="submitMove">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* 版面复刻自 v2 ResourceManagementView-Ca99QyBJ.css(去 data-v 哈希) */
.resource-page {
  max-width: 1500px;
  margin: 0 auto;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 18px;
}
.page-header h2 {
  margin: 0 0 8px;
}
.page-header p {
  margin: 0;
  color: #6b7280;
}
.header-actions {
  display: flex;
  gap: 10px;
}
.summary-row {
  margin-bottom: 16px;
}
.summary-box {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 14px 18px;
}
.summary {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.summary span {
  color: #6b7280;
}
.summary strong {
  font-size: 28px;
  color: #111827;
}
.resource-columns {
  min-height: 560px;
}
.resource-card {
  height: 100%;
  min-height: 560px;
}
.card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 700;
}
.resource-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 13px 10px;
  margin-bottom: 8px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  cursor: pointer;
  transition: 0.15s ease;
}
.resource-item:hover {
  border-color: #93c5fd;
  background: #f8fbff;
}
.resource-item.active {
  border-color: #409eff;
  background: #ecf5ff;
}
.item-main {
  min-width: 0;
  flex: 1;
}
.item-name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.item-code,
.item-meta {
  margin-top: 4px;
  color: #6b7280;
  font-size: 12px;
}
.item-more {
  cursor: pointer;
  color: #6b7280;
  font-weight: 700;
  letter-spacing: 1px;
  padding: 0 4px;
}
.empty-hint {
  color: #909399;
  text-align: center;
  padding: 40px 0;
}
.rack-card :deep(.el-card__body) {
  padding-top: 12px;
}
.code-field {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
}
.code-field .el-input {
  flex: 1;
}
.code-field .el-checkbox {
  flex: none;
}
.copy-tip {
  margin-bottom: 16px;
}
.form-tip {
  font-size: 12px;
  color: #909399;
  line-height: 1.55;
  padding-top: 4px;
}
</style>
