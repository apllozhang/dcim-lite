<script setup lang="ts">
/**
 * 屏6 机房资源大屏(复刻 v2 RoomScreenView;暗色主题来自全局 ale-theme.css
 * 的 .screen-page 段,类名与其保持一致即自动套用)。
 * 左侧控制面板:品牌/数据中心+机房选择器/机房信息卡/四统计/状态图例(可筛选)/
 * 设备类型图例/当前选中卡/返回+全屏。右侧:屏头(面包屑+实时时钟)/画布工具栏
 * (搜索+新增+编辑+详情)/筛选 chips/机房边界(网格底+排参考线+固定槽位框+
 * 机柜卡片:U 位立面图 9px/格,设备块按分类着色)。
 * 交互:机柜卡标题拖动交换槽位(持久化 xCoordinate/yCoordinate)、U 位点击/右键
 * 上架、设备块点击/右键 查看/迁移/下架、机柜新增/编辑/删除/详情。
 * 机柜图 Excel 导出/导入依赖 SheetJS,本版入口置灰禁用(见 PR 说明)。
 */
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  assignDevice,
  createRack,
  decommissionDevice,
  deleteRack,
  fetchDevices,
  fetchResourceTree,
  fetchRackTemplates,
  fetchULayout,
  moveDevice,
  updateRack,
  type Device,
  type Rack,
  type RackTemplate,
  type TreeDataCenter,
} from "@/features/resource/api";
import type { ULayoutResponse } from "@/features/resource/api";
import {
  CLOCK_WEEKDAYS,
  COL_PITCH,
  DEVICE_LEGENDS,
  MAX_PER_ROW,
  RACK_W,
  STATUS_LEGENDS,
  U_PX,
  autoSlot,
  categoryLabel,
  deviceColor,
  ext,
  rackCardHeight,
  rackPositionText,
  rowTops,
  shortName,
  slotX,
  statusColor,
  statusLabel,
} from "@/features/screen/screenShared";
import {
  RACK_STATUS_OPTIONS,
  activeTemplateOptions,
  applyTemplateToForm,
  autoCode,
  formToRackPayload,
  rackErrMsg,
  rackFormDefaults,
  rackToForm,
  type RackForm,
} from "@/features/rack/rackShared";

type TreeRack = Rack & { dcName: string };
interface SlotPos {
  row: number;
  column: number;
}

const router = useRouter();

/* ── 数据 ── */
const loading = ref(false);
const tree = ref<TreeDataCenter[]>([]);
const templates = ref<RackTemplate[]>([]);
const layouts = reactive<Record<string, ULayoutResponse>>({});
const dcId = ref("");
const roomId = ref("");
const statusFilter = ref("ALL");
const rackKeyword = ref("");
const selectedRack = ref<TreeRack | null>(null);
const searchHitDeviceId = ref("");

const rooms = computed<NonNullable<TreeDataCenter["rooms"]>>(() =>
  dcId.value ? (tree.value.find((d) => d.id === dcId.value)?.rooms ?? []) : [],
);
const room = computed(() => rooms.value.find((r) => r.id === roomId.value));
const roomRacks = computed<TreeRack[]>(() =>
  (room.value?.racks ?? []).map((r) => ({
    ...r,
    dcName: tree.value.find((d) => d.id === dcId.value)?.name ?? "",
  })),
);
const racksPerRow = computed(() => {
  const n = Number(ext(room.value, "racksPerRow"));
  return Math.min(MAX_PER_ROW, Math.max(1, Number.isFinite(n) ? Math.round(n) : 8));
});
const keyword = computed(() => rackKeyword.value.trim().toLowerCase());
function rackHaystack(r: TreeRack): string {
  const devs = layouts[r.id ?? ""];
  return [
    r.code,
    r.name,
    ...((devs?.devices ?? []).map((d) => [d.name, d.code] as const).flat() ?? []),
  ]
    .join(" ")
    .toLowerCase();
}
const filteredRacks = computed<TreeRack[]>(() =>
  roomRacks.value.filter((r) => {
    const okStatus = statusFilter.value === "ALL" || r.status === statusFilter.value;
    if (!keyword.value) return okStatus;
    return okStatus && rackHaystack(r).includes(keyword.value);
  }),
);
/**
 * 全机房槽位(含被筛选隐藏的),卡片只渲染可见柜(v2 ln/nn 口径)。
 * 排布按 v2 Ft():持久坐标(x/y 均有限)的柜排前面,按 y/x 升序,其余按原序,
 * 全体行主序落槽——不用原始像素值直接当行列。
 */
const allSlots = computed(() => {
  const sorted = roomRacks.value
    .map((rack, index) => ({
      rack,
      index,
      x: Number(rack.xCoordinate),
      y: Number(rack.yCoordinate),
      persisted:
        Number.isFinite(Number(rack.xCoordinate)) && Number.isFinite(Number(rack.yCoordinate)),
    }))
    .sort((a, b) =>
      a.persisted !== b.persisted
        ? a.persisted
          ? -1
          : 1
        : a.persisted && b.persisted
          ? a.y - b.y || a.x - b.x
          : a.index - b.index,
    );
  const out: { rack: TreeRack; pos: SlotPos }[] = [];
  const seen = new Set<string>();
  sorted.forEach((s, i) => {
    out.push({ rack: s.rack, pos: autoSlot(i, racksPerRow.value) });
    seen.add(`${s.rack.id}`);
  });
  return out;
});
/** 可见柜槽位(按 allSlots 相同键,保证与框一致) */
const visibleSlots = computed(() =>
  allSlots.value.filter((s) => filteredRacks.value.some((r) => r.id === s.rack.id)),
);
const rowCount = computed(() =>
  allSlots.value.length ? Math.max(...allSlots.value.map((s) => s.pos.row)) + 1 : 0,
);
const rowMaxHeights = computed(() =>
  Array.from({ length: rowCount.value }, (_, row) =>
    Math.max(
      0,
      ...allSlots.value.filter((s) => s.pos.row === row).map((s) => rackCardHeight(s.rack)),
    ),
  ),
);
const rowTopList = computed(() => rowTops(rowMaxHeights.value));
const usedU = computed(() => roomRacks.value.reduce((s, r) => s + layoutUsed(r), 0));
const totalU = computed(() => roomRacks.value.reduce((s, r) => s + (r.uHeight ?? 0), 0));
const usageRate = computed(() =>
  totalU.value ? Math.round((usedU.value / totalU.value) * 100) : 0,
);
const canvasSize = computed(() => {
  const rows = rowCount.value;
  const cols = rows ? Math.min(racksPerRow.value, Math.max(1, allSlots.value.length)) : 0;
  const width = cols ? 38 + (cols - 1) * COL_PITCH + RACK_W + 38 : 900;
  const last = rows - 1;
  const height =
    last >= 0 ? (rowTopList.value[last] ?? 72) + (rowMaxHeights.value[last] ?? 0) + 36 : 600;
  return { width: Math.max(900, width), height: Math.max(600, height) };
});
const roomLocation = computed(() => {
  const parts = [room.value?.building, room.value?.floor, room.value?.roomNumber].filter(Boolean);
  return parts.join(" / ") || "未设置物理位置";
});
const designPower = computed(() => ext(room.value, "designPowerW") ?? "-");
const roomManager = computed(() => ext(room.value, "manager") ?? "-");
function layoutUsed(r: TreeRack): number {
  return layouts[r.id ?? ""]?.used ?? 0;
}
function statusCount(s: string): number {
  return roomRacks.value.filter((r) => r.status === s).length;
}
function setStatus(v: string) {
  statusFilter.value = v === "ALL" || statusFilter.value === v ? "ALL" : v;
  ensureSelection();
}
/** 无选中或选中已被过滤时自动选第一台可见柜(v2 同款:大屏常有一台当前选中) */
function ensureSelection() {
  if (!selectedRack.value || !filteredRacks.value.some((r) => r.id === selectedRack.value?.id)) {
    selectedRack.value = filteredRacks.value[0] ?? null;
  }
}
function selectRack(r: TreeRack) {
  selectedRack.value = r;
}
function selectedMetrics(r: TreeRack | null) {
  if (!r) return null;
  const used = layoutUsed(r);
  const total = r.uHeight ?? 0;
  return {
    used,
    total,
    remain: Math.max(0, total - used),
    rate: total ? Math.round((used / total) * 100) : 0,
  };
}

/* ── 时钟 ── */
const now = ref(new Date());
let clockTimer = 0;
const clockTime = computed(() => {
  const d = now.value;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
});
const clockDate = computed(() => {
  const d = now.value;
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日星期${CLOCK_WEEKDAYS[d.getDay()]}`;
});

/* ── 全屏 ── */
const isFullscreen = ref(false);
function toggleFullscreen() {
  if (document.fullscreenElement) {
    void document.exitFullscreen();
  } else {
    void document.documentElement.requestFullscreen();
  }
}
function onFsChange() {
  isFullscreen.value = !!document.fullscreenElement;
}

/* ── 加载 ── */
async function loadTree(keepSelection = true) {
  loading.value = true;
  try {
    const [tr, tpl] = await Promise.all([fetchResourceTree(), fetchRackTemplates()]);
    tree.value = tr;
    templates.value = tpl;
    if (!keepSelection || !tr.some((d) => d.id === dcId.value)) dcId.value = tr[0]?.id ?? "";
    if (!rooms.value.some((r) => r.id === roomId.value)) roomId.value = rooms.value[0]?.id ?? "";
    await loadLayouts();
    ensureSelection();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    loading.value = false;
  }
}
async function loadLayouts() {
  const list = roomRacks.value;
  const missing = list.filter((r) => !layouts[r.id ?? ""]);
  const results = await Promise.all(
    missing.map(async (r) => {
      try {
        return [r.id ?? "", await fetchULayout(r.id ?? "")] as const;
      } catch {
        return [r.id ?? "", null] as const;
      }
    }),
  );
  for (const [id, lay] of results) {
    if (lay) layouts[id] = lay;
  }
}

function onRoomChange() {
  selectedRack.value = null;
  void loadLayouts();
}

/* ── U 位立面 ── */
interface URow {
  u: number;
  occupied: boolean;
  device: ULayoutResponse["devices"][number] | undefined;
}
function uRows(rack: TreeRack): URow[] {
  const lay = layouts[rack.id ?? ""];
  const h = rack.uHeight ?? 0;
  const rows: URow[] = [];
  for (let u = h; u >= 1; u--) {
    const device = (lay?.devices ?? []).find((d) => u >= d.startU && u <= d.endU);
    rows.push({ u, occupied: !!device, device });
  }
  return rows;
}
function deviceBlockStyle(rack: TreeRack, d: ULayoutResponse["devices"][number]) {
  const h = rack.uHeight ?? 0;
  return {
    top: `${(h - d.endU) * U_PX}px`,
    height: `${Math.max(1, d.heightU) * U_PX}px`,
    "--device-color": deviceColor(ext(d as unknown as object, "category") as string | undefined),
  };
}
function deviceTitle(_rack: TreeRack, d: ULayoutResponse["devices"][number]): string {
  return `${d.name} - ${categoryLabel(
    ext(d as unknown as object, "category") as string | undefined,
  )} - ${d.startU}-${d.endU}U - ${d.heightU}U`;
}
function uTitle(_rack: TreeRack, u: number, occupied: boolean): string {
  return occupied ? `${u} U 已占用` : `${u} U 空闲，点击或右键可进行上架操作`;
}
/* ── 卡片坐标/拖拽交换 ── */
const dragId = ref("");
const dragSaving = ref(false);
function slotOf(rackId: string | undefined): SlotPos {
  const hit = allSlots.value.find((s) => s.rack.id === rackId);
  return hit?.pos ?? { row: 0, column: 0 };
}
function cardPos(rack: TreeRack) {
  const pos = slotOf(rack.id);
  return { left: `${slotX(pos)}px`, top: `${rowTopList.value[pos.row] ?? 72}px` };
}
function slotFrameStyle(pos: SlotPos) {
  return {
    left: `${slotX(pos)}px`,
    top: `${rowTopList.value[pos.row] ?? 72}px`,
    width: `${RACK_W}px`,
    height: "auto",
  };
}
function rowGuideStyle(row: number) {
  return { top: `${(rowTopList.value[row] ?? 72) - 22}px` };
}
let dragPointerId: number | null = null;
let dragStart: { x: number; y: number } | null = null;
let dragMoved = false;
function startRackDrag(ev: PointerEvent, rack: TreeRack) {
  if (ev.button !== 0 || dragSaving.value || dragId.value) return;
  dragPointerId = ev.pointerId;
  dragStart = { x: ev.clientX, y: ev.clientY };
  dragMoved = false;
  dragId.value = rack.id ?? "";
  document.body.classList.add("is-rack-dragging");
}
function onCanvasPointerMove(ev: PointerEvent) {
  if (!dragId.value || ev.pointerId !== dragPointerId) return;
  const dx = ev.clientX - (dragStart?.x ?? 0);
  const dy = ev.clientY - (dragStart?.y ?? 0);
  if (Math.abs(dx) + Math.abs(dy) > 6) dragMoved = true;
}
async function onCanvasPointerUp(ev: PointerEvent) {
  if (!dragId.value || ev.pointerId !== dragPointerId) return;
  const dragged = roomRacks.value.find((r) => r.id === dragId.value);
  dragId.value = "";
  document.body.classList.remove("is-rack-dragging");
  dragPointerId = null;
  if (!dragged || !dragMoved) return;
  // 命中目标槽位:找指针下方的其它机柜卡(简化:最近槽位 + 该槽位上的柜)
  const canvas = document.querySelector(".room-layout-canvas");
  if (!canvas) return;
  const rect = canvas.getBoundingClientRect();
  const px = ev.clientX - rect.left;
  const py = ev.clientY - rect.top;
  const target = nearestSlotRack(px, py, dragged.id ?? "");
  await swapRacks(dragged, target);
}
function nearestSlotRack(px: number, py: number, excludeId: string): TreeRack | null {
  let best: { rack: TreeRack; dist: number } | null = null;
  for (const s of allSlots.value) {
    if (s.rack.id === excludeId) continue;
    const x = slotX(s.pos);
    const y = rowTopList.value[s.pos.row] ?? 72;
    const dx = px < x ? x - px : px > x + RACK_W ? px - x - RACK_W : 0;
    const h = rackCardHeight(s.rack);
    const dy = py < y ? y - py : py > y + h ? py - y - h : 0;
    const dist = dx * dx + dy * dy;
    if (!best || dist < best.dist) best = { rack: s.rack, dist };
  }
  return best && best.dist < 400 * 400 ? best.rack : null;
}
async function swapRacks(a: TreeRack, b: TreeRack | null) {
  const posA = slotOf(a.id);
  const posB = b ? slotOf(b.id) : null;
  if (!posB || (posA.row === posB.row && posA.column === posB.column)) return;
  dragSaving.value = true;
  try {
    const updates = [{ rack: a, pos: posB }, ...(b ? [{ rack: b, pos: posA }] : [])];
    await Promise.all(
      updates.map(({ rack, pos }) =>
        updateRack(
          rack.id ?? "",
          rack.version ?? 0,
          rackPayloadWithPos(rack, slotX(pos), rowTopList.value[pos.row] ?? 72) as never,
        ),
      ),
    );
    ElMessage.success(b ? `${a.name} 与 ${b.name} 已交换位置` : `${a.name} 位置已保存`);
    await loadTree();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    dragSaving.value = false;
  }
}
/** 全量载荷 + 新坐标(后端 PUT 缺省字段会落默认值,必须带全) */
function rackPayloadWithPos(r: TreeRack, x: number, y: number): Record<string, unknown> {
  const form = rackToForm(r);
  return { ...formToRackPayload(form, r.code ?? ""), xCoordinate: x, yCoordinate: y };
}

/* ── U 位操作菜单(点击/右键) ── */
const ctxMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  kind: "" as "" | "u" | "device",
  rack: null as TreeRack | null,
  u: 0,
  device: null as ULayoutResponse["devices"][number] | null,
});
function openUMenu(ev: MouseEvent, rack: TreeRack, u: number, occupied: boolean) {
  if (occupied) return;
  ctxMenu.visible = true;
  ctxMenu.x = ev.clientX;
  ctxMenu.y = ev.clientY;
  ctxMenu.kind = "u";
  ctxMenu.rack = rack;
  ctxMenu.u = u;
  ctxMenu.device = null;
}
function openDeviceMenu(ev: MouseEvent, rack: TreeRack, d: ULayoutResponse["devices"][number]) {
  ctxMenu.visible = true;
  ctxMenu.x = ev.clientX;
  ctxMenu.y = ev.clientY;
  ctxMenu.kind = "device";
  ctxMenu.rack = rack;
  ctxMenu.u = d.startU;
  ctxMenu.device = d;
}
function closeCtx() {
  ctxMenu.visible = false;
}
async function assignAt(u: number) {
  const rack = ctxMenu.rack;
  closeCtx();
  if (!rack) return;
  assignTarget.rackId = rack.id ?? "";
  assignTarget.rackName = rack.name ?? "";
  assignTarget.startU = u;
  assignTarget.visible = true;
  await loadAssignable();
}
const assignTarget = reactive({ visible: false, rackId: "", rackName: "", startU: 1 });
const assignable = ref<Device[]>([]);
const assignableLoading = ref(false);
async function loadAssignable() {
  assignableLoading.value = true;
  try {
    const waiting = await fetchDevices({ page: 1, pageSize: 100, lifecycleStatus: "WAITING_RACK" });
    const off = await fetchDevices({ page: 1, pageSize: 100, lifecycleStatus: "OFF_RACK" });
    assignable.value = [...waiting.items, ...off.items];
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    assignableLoading.value = false;
  }
}
async function confirmAssign(deviceId: string) {
  try {
    await assignDevice(deviceId, { rackId: assignTarget.rackId, startU: assignTarget.startU });
    ElMessage.success(`已上架到 ${assignTarget.rackName} U${assignTarget.startU}`);
    assignTarget.visible = false;
    await reloadLayouts();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  }
}
const moveTarget = reactive({
  visible: false,
  deviceId: "",
  deviceName: "",
  rackId: "",
  startU: 1,
});
function openMove() {
  const d = ctxMenu.device;
  closeCtx();
  if (!d) return;
  moveTarget.deviceId = d.id;
  moveTarget.deviceName = d.name;
  moveTarget.rackId = "";
  moveTarget.startU = d.startU;
  moveTarget.visible = true;
}
async function confirmMove() {
  if (!moveTarget.rackId) {
    ElMessage.warning("请选择目标机柜");
    return;
  }
  try {
    await moveDevice(moveTarget.deviceId, {
      rackId: moveTarget.rackId,
      startU: moveTarget.startU,
    });
    ElMessage.success("已移位");
    moveTarget.visible = false;
    await reloadLayouts();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  }
}
async function offlineDevice() {
  const d = ctxMenu.device;
  closeCtx();
  if (!d) return;
  try {
    await ElMessageBox.confirm(`确认将“${d.name}”下架吗？`, "下架确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await decommissionDevice(d.id, "机房大屏下架");
    ElMessage.success("已下架");
    await reloadLayouts();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  }
}
async function reloadLayouts() {
  Object.keys(layouts).forEach((k) => delete layouts[k]);
  await loadLayouts();
}

/* ── 机柜新增/编辑/删除 ── */
const editor = reactive({
  visible: false,
  mode: "create" as "create" | "edit",
  id: "",
  version: 0,
  autoCode: true,
});
const form = ref<RackForm>(rackFormDefaults());
const saving = ref(false);
const templateOptions = computed(() => activeTemplateOptions(templates.value));
function openCreateRack() {
  if (!room.value) {
    ElMessage.warning("请先选择机房");
    return;
  }
  editor.mode = "create";
  editor.autoCode = true;
  form.value = rackFormDefaults();
  editor.visible = true;
}
function openEditRack(rack: TreeRack | null = selectedRack.value) {
  if (!rack) return;
  editor.mode = "edit";
  editor.id = rack.id ?? "";
  editor.version = rack.version ?? 0;
  form.value = rackToForm(rack);
  editor.visible = true;
}
function onTemplatePick(id: string | undefined) {
  const hit = templateOptions.value.find((o) => o.template.id === id);
  if (hit?.version) applyTemplateToForm(form.value, hit.version);
}
async function saveRack() {
  const auto = editor.mode === "create" && editor.autoCode;
  if (!form.value.name.trim() || (!auto && !form.value.code.trim())) {
    ElMessage.warning(auto ? "名称不能为空" : "编码和名称不能为空");
    return;
  }
  saving.value = true;
  try {
    const code = auto ? autoCode("RACK") : form.value.code.trim();
    if (editor.mode === "create") {
      await createRack(roomId.value, formToRackPayload(form.value, code) as never);
    } else {
      await updateRack(editor.id, editor.version, formToRackPayload(form.value, code) as never);
    }
    editor.visible = false;
    ElMessage.success("机柜已保存");
    await loadTree();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    saving.value = false;
  }
}
async function removeRack(rack: TreeRack | null = selectedRack.value) {
  if (!rack) return;
  try {
    await ElMessageBox.confirm(
      `确定删除机柜“${rack.name}”吗？如果机柜内仍有设备将禁止删除。`,
      "删除确认",
      { type: "warning" },
    );
  } catch {
    return;
  }
  try {
    await deleteRack(rack.id ?? "", rack.version ?? 0);
    ElMessage.success("机柜已删除");
    selectedRack.value = null;
    await loadTree();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  }
}

/* ── 机柜详情对话框 ── */
const detailVisible = ref(false);
function openDetail(rack: TreeRack | null = selectedRack.value) {
  if (!rack) return;
  selectedRack.value = rack;
  detailVisible.value = true;
}

/* ── 搜索 ── */
function applySearch() {
  if (!keyword.value) return;
  const hit = filteredRacks.value.find((r) => rackHaystack(r).includes(keyword.value));
  if (hit) {
    selectedRack.value = hit;
    const lay = layouts[hit.id ?? ""];
    searchHitDeviceId.value =
      lay?.devices.find(
        (d) =>
          d.name.toLowerCase().includes(keyword.value) ||
          d.code.toLowerCase().includes(keyword.value),
      )?.id ?? "";
    ElMessage.success(`已定位机柜“${hit.name}”`);
  } else {
    ElMessage.info("未找到匹配的机柜或设备");
  }
}

/* ── 导航 ── */
function goBack() {
  router.push("/");
}

onMounted(() => {
  void loadTree(false);
  clockTimer = window.setInterval(() => (now.value = new Date()), 1000);
  document.addEventListener("fullscreenchange", onFsChange);
});
onBeforeUnmount(() => {
  window.clearInterval(clockTimer);
  document.removeEventListener("fullscreenchange", onFsChange);
  document.body.classList.remove("is-rack-dragging");
});
</script>

<template>
  <div class="screen-page" data-test="room-screen">
    <!-- ── 左侧控制面板 ── -->
    <aside class="control-panel">
      <div class="brand-block">
        <div class="brand-mark">DC</div>
        <div>
          <h1>机房资源大屏</h1>
          <p>Data Center Visualization</p>
        </div>
      </div>
      <div class="selector-section">
        <label>数据中心</label>
        <el-select v-model="dcId" size="large" class="screen-select" data-test="screen-dc-select">
          <el-option v-for="d in tree" :key="d.id" :label="d.name" :value="d.id ?? ''" />
        </el-select>
        <label>机房</label>
        <el-select
          v-model="roomId"
          size="large"
          class="screen-select"
          data-test="screen-room-select"
          @change="onRoomChange"
        >
          <el-option v-for="r in rooms" :key="r.id" :label="r.name" :value="r.id ?? ''" />
        </el-select>
      </div>

      <div v-if="room" class="room-card">
        <div class="section-title"><span />机房信息</div>
        <div class="room-name">{{ room.name }}</div>
        <div class="room-code">{{ room.code }}</div>
        <div class="room-meta">
          <div>
            <span>所在位置</span>
            <strong>{{ roomLocation }}</strong>
          </div>
          <div>
            <span>机房面积</span>
            <strong>{{ room.areaSquareMeters ? `${room.areaSquareMeters} ㎡` : "-" }}</strong>
          </div>
          <div>
            <span>设计功率</span>
            <strong>{{ designPower }}</strong>
          </div>
          <div>
            <span>负责人</span>
            <strong>{{ roomManager ?? "-" }}</strong>
          </div>
        </div>
        <div class="room-card-actions">
          <button type="button" @click="openCreateRack()">新增机柜</button>
          <button type="button" disabled title="机柜图导出随后续轮次交付">导出机柜图</button>
          <button type="button" disabled title="机柜图导入随后续轮次交付">导入机柜图</button>
          <button type="button" @click="loadTree()">刷新数据</button>
        </div>
      </div>

      <div class="stats-grid">
        <div class="stat-item cyan">
          <span>机柜总数</span>
          <strong>{{ roomRacks.length }}</strong>
          <small>RACKS</small>
        </div>
        <div class="stat-item green">
          <span>空闲</span>
          <strong>{{ statusCount("AVAILABLE") }}</strong>
          <small>AVAILABLE</small>
        </div>
        <div class="stat-item blue">
          <span>已用 U 位</span>
          <strong>{{ usedU }}</strong>
          <small>/ {{ totalU }}U</small>
        </div>
        <div class="stat-item orange">
          <span>U 位使用率</span>
          <strong>{{ usageRate }}</strong>
          <small>%</small>
        </div>
      </div>

      <div class="legend-card">
        <div class="section-title">
          <span />机柜状态图例
          <span v-if="statusFilter !== 'ALL'" class="filter-hint">已筛选</span>
        </div>
        <div class="status-legend-list">
          <button
            type="button"
            class="status-legend-item"
            :class="{ active: statusFilter === 'ALL' }"
            data-test="screen-status-ALL"
            @click="setStatus('ALL')"
          >
            <i class="all-dot" />
            <span>全部</span>
            <b>{{ roomRacks.length }}</b>
          </button>
          <button
            v-for="s in STATUS_LEGENDS"
            :key="s.value"
            type="button"
            class="status-legend-item"
            :class="{ active: statusFilter === s.value }"
            :data-test="`screen-status-${s.value}`"
            @click="setStatus(s.value)"
          >
            <i :style="{ color: s.color, background: s.color }" />
            <span>{{ s.label }}</span>
            <b>{{ statusCount(s.value) }}</b>
          </button>
        </div>
        <div class="u-note">点击任意 U 位可打开操作菜单，直接进行上架、查看、迁移或下架操作。</div>
      </div>

      <div class="legend-card">
        <div class="section-title"><span />设备类型图例</div>
        <div class="device-legend-list">
          <div v-for="d in DEVICE_LEGENDS" :key="d.value">
            <i :style="{ color: d.color, background: d.color }" />
            {{ d.label }}
          </div>
        </div>
        <div class="u-note">机柜统一展示一套 U 位，所有操作按 U 区间管理。</div>
      </div>

      <div v-if="selectedRack" class="selected-card">
        <div class="selected-header">
          <div>
            <small>当前选中</small>
            <strong>{{ selectedRack.name }}</strong>
          </div>
          <button
            type="button"
            class="selected-status-button rack-status-chip"
            :style="{ '--status-color': statusColor(selectedRack.status ?? '') }"
            @click="setStatus(selectedRack.status ?? 'ALL')"
          >
            <i />{{ statusLabel(selectedRack.status ?? "") }}
          </button>
        </div>
        <div class="selected-grid">
          <div>
            <span>编码</span>
            <b>{{ selectedRack.code }}</b>
          </div>
          <div>
            <span>规格</span>
            <b>{{ selectedRack.uHeight }}U</b>
          </div>
          <div>
            <span>已使用</span>
            <b>{{ layoutUsed(selectedRack) }}U</b>
          </div>
          <div>
            <span>剩余</span>
            <b>{{ selectedMetrics(selectedRack)?.remain }}U</b>
          </div>
          <div>
            <span>使用率</span>
            <b>{{ selectedMetrics(selectedRack)?.rate }}%</b>
          </div>
          <div>
            <span>位置</span>
            <b>{{ rackPositionText(selectedRack) }}</b>
          </div>
        </div>
        <div class="selected-progress">
          <i :style="{ width: `${selectedMetrics(selectedRack)?.rate ?? 0}%` }" />
        </div>
        <div class="selected-actions">
          <button type="button" @click="openDetail()">查看详情</button>
          <button type="button" @click="openEditRack()">编辑机柜</button>
          <button type="button" class="danger" @click="removeRack()">删除机柜</button>
        </div>
        <button type="button" class="u-button" @click="openDetail()">查看完整 U 位详情</button>
      </div>

      <div class="panel-actions">
        <button type="button" @click="goBack">返回管理平台</button>
        <button type="button" @click="toggleFullscreen">
          {{ isFullscreen ? "退出全屏" : "进入全屏" }}
        </button>
      </div>
    </aside>

    <!-- ── 右侧大屏区 ── -->
    <main class="visual-area">
      <header class="screen-header">
        <div>
          <h2>
            {{ tree.find((d) => d.id === dcId)?.name ?? "-" }}
            <span>/</span>
            {{ room?.name ?? "-" }}
          </h2>
          <p>机柜 U 位立面图 · 拖动机柜标题即可交换固定槽位</p>
        </div>
        <div class="header-status">
          <div class="online"><i />数据实时在线</div>
          <div class="clock">
            <strong>{{ clockTime }}</strong>
            <span>{{ clockDate }}</span>
          </div>
        </div>
      </header>

      <section class="canvas-shell">
        <div class="canvas-toolbar">
          <span><i class="pulse-dot" />机柜 U 位总览</span>
          <div class="toolbar-right">
            <el-input
              v-model="rackKeyword"
              size="small"
              placeholder="查询机柜编码、名称或设备名称"
              class="rack-search"
              data-test="screen-search"
              @keyup.enter="applySearch"
            />
            <button type="button" class="canvas-action canvas-search-button" @click="applySearch">
              查询
            </button>
            <button type="button" class="canvas-action primary" @click="openCreateRack()">
              ＋新增机柜
            </button>
            <button
              type="button"
              class="canvas-action"
              :disabled="!selectedRack"
              @click="openEditRack()"
            >
              编辑
            </button>
            <button
              type="button"
              class="canvas-action"
              :disabled="!selectedRack"
              @click="openDetail()"
            >
              详情
            </button>
            <span class="canvas-info">
              {{ rowCount }} 排 · {{ filteredRacks.length }} 个机柜 · {{ usedU }}/{{ totalU }}U
              已使用
            </span>
            <span class="drag-tip">拖动标题可与其他机柜交换槽位</span>
          </div>
        </div>
        <div class="filter-bar" role="group" aria-label="机柜状态快速筛选">
          <button
            type="button"
            class="filter-chip"
            :class="{ active: statusFilter === 'ALL' }"
            @click="setStatus('ALL')"
          >
            全部
            <span>{{ filteredRacks.length }}</span>
          </button>
          <button
            v-for="s in STATUS_LEGENDS"
            :key="s.value"
            type="button"
            class="filter-chip"
            :class="{ active: statusFilter === s.value }"
            @click="setStatus(s.value)"
          >
            <i :style="{ background: s.color }" />
            {{ s.label }}
            <span>{{ statusCount(s.value) }}</span>
          </button>
        </div>

        <!-- 空态三分支(v2 同款) -->
        <div v-if="!room" class="empty-screen">
          <div class="empty-icon">⌗</div>
          <h3>请选择需要展示的机房</h3>
          <p>在左侧依次选择数据中心和机房</p>
        </div>
        <div v-else-if="!roomRacks.length" class="empty-screen">
          <div class="empty-icon">▥</div>
          <h3>该机房还没有机柜</h3>
          <p>请先在资源层级或机柜管理中添加机柜</p>
        </div>
        <div v-else-if="!filteredRacks.length" class="empty-screen">
          <div class="empty-icon">◍</div>
          <h3>没有“{{ statusFilter === "ALL" ? "全部" : statusLabel(statusFilter) }}”状态的机柜</h3>
          <p>点击筛选栏中的“全部”查看该机房的全部机柜</p>
        </div>

        <div
          v-else
          v-loading="loading"
          class="rack-room-scroller"
          @pointermove="onCanvasPointerMove"
          @pointerup="onCanvasPointerUp"
        >
          <div
            class="room-boundary"
            :style="{
              width: `${canvasSize.width + 40}px`,
              height: `${canvasSize.height}px`,
            }"
          >
            <div class="room-boundary-header">
              <div>
                <strong>{{ room?.name }}</strong>
                <span>{{ room?.code }} · {{ roomLocation }}</span>
              </div>
              <div class="capacity-summary">
                <span>U 位总容量</span>
                <strong
                  >{{ usedU }}<small> / {{ totalU }}U</small></strong
                >
              </div>
            </div>
            <div
              class="room-layout-canvas"
              :style="{
                width: `${canvasSize.width}px`,
                height: `${canvasSize.height}px`,
              }"
            >
              <div
                v-for="row in rowCount"
                :key="`guide-${row}`"
                class="layout-row-guide"
                :style="rowGuideStyle(row - 1)"
              >
                <span>第 {{ row }} 排</span>
                <i />
              </div>
              <div
                v-for="s in allSlots"
                :key="`frame-${s.rack.id}`"
                class="rack-slot-frame"
                :style="slotFrameStyle(s.pos)"
              />
              <article
                v-for="s in visibleSlots"
                :key="s.rack.id"
                class="rack-card rack-card-floating"
                :class="{
                  selected: selectedRack?.id === s.rack.id,
                  dragging: dragId === s.rack.id,
                }"
                :style="{
                  ...cardPos(s.rack),
                  '--rack-card-height': `${rackCardHeight(s.rack)}px`,
                }"
                :data-rack-id="s.rack.id"
                :aria-label="s.rack.name"
                tabindex="0"
                @click="selectRack(s.rack)"
              >
                <div
                  class="rack-card-header rack-drag-handle"
                  :title="s.rack.name"
                  @pointerdown="startRackDrag($event, s.rack)"
                >
                  <div>
                    <strong>{{ s.rack.name }}</strong>
                    <span>{{ s.rack.code }} · {{ s.rack.uHeight }}U</span>
                  </div>
                  <button
                    type="button"
                    class="rack-status-chip"
                    :style="{ '--status-color': statusColor(s.rack.status ?? '') }"
                  >
                    <i />{{ statusLabel(s.rack.status ?? "") }}
                  </button>
                </div>
                <div class="rack-usage-line">
                  <span>已用 {{ layoutUsed(s.rack) }}U</span>
                  <b>
                    {{
                      s.rack.uHeight ? Math.round((layoutUsed(s.rack) / s.rack.uHeight) * 100) : 0
                    }}%
                  </b>
                  <span>剩余 {{ Math.max(0, (s.rack.uHeight ?? 0) - layoutUsed(s.rack)) }}U</span>
                </div>
                <div class="rack-usage-bar">
                  <i
                    :style="{
                      width: `${
                        s.rack.uHeight ? Math.round((layoutUsed(s.rack) / s.rack.uHeight) * 100) : 0
                      }%`,
                    }"
                  />
                </div>
                <div class="cabinet-shell" :class="{ 'is-full': s.rack.status === 'FULL' }">
                  <div class="cabinet-top"><span /><b>U 位面板</b><span /></div>
                  <div
                    class="cabinet-frame"
                    :style="{ '--rack-height': `${(s.rack.uHeight ?? 0) * U_PX}px` }"
                  >
                    <i class="rack-rail left" />
                    <i class="rack-rail right" />
                    <div class="u-scale" aria-hidden="true">
                      <span v-for="row in uRows(s.rack)" :key="`s-${s.rack.id}-${row.u}`">
                        {{ row.u }}
                      </span>
                    </div>
                    <div class="u-grid">
                      <button
                        v-for="row in uRows(s.rack)"
                        :key="`g-${s.rack.id}-${row.u}`"
                        type="button"
                        class="u-line"
                        :class="{ major: row.u % 5 === 0, occupied: row.occupied }"
                        :title="uTitle(s.rack, row.u, row.occupied)"
                        @click.stop="openUMenu($event, s.rack, row.u, row.occupied)"
                        @contextmenu.prevent.stop="openUMenu($event, s.rack, row.u, row.occupied)"
                      />
                      <div
                        v-for="d in layouts[s.rack.id ?? '']?.devices ?? []"
                        :key="`${s.rack.id}-${d.id}`"
                        class="device-block"
                        :class="{ 'search-hit': searchHitDeviceId === d.id }"
                        :style="deviceBlockStyle(s.rack, d)"
                        :data-position-id="d.id"
                        :title="deviceTitle(s.rack, d)"
                        @click.stop="openDeviceMenu($event, s.rack, d)"
                        @contextmenu.prevent.stop="openDeviceMenu($event, s.rack, d)"
                      >
                        <strong>{{ shortName(d.name) }}</strong>
                        <small>{{ d.startU }}-{{ d.endU }}U</small>
                      </div>
                    </div>
                  </div>
                  <div class="cabinet-base"><span /><span /></div>
                </div>
                <div class="rack-card-footer">
                  <span>{{ rackPositionText(s.rack) }}</span>
                  <button type="button" @click.stop="openDetail(s.rack)">详情</button>
                </div>
              </article>
            </div>
            <div class="room-door"><i /><span>机房入口</span></div>
          </div>
        </div>
      </section>
    </main>

    <!-- U 位/设备 右键菜单 -->
    <teleport to="body">
      <div
        v-if="ctxMenu.visible"
        class="rack-context-menu"
        :style="{ left: `${ctxMenu.x}px`, top: `${ctxMenu.y}px` }"
        @click.stop
      >
        <div class="context-menu-header">
          <small>{{ ctxMenu.kind === "u" ? "空闲 U 位" : "在位设备" }}</small>
          <strong>{{
            ctxMenu.kind === "u" ? `${ctxMenu.rack?.name} · U${ctxMenu.u}` : ctxMenu.device?.name
          }}</strong>
          <span v-if="ctxMenu.device"
            >{{ ctxMenu.device.code }} · {{ ctxMenu.device.startU }}-{{
              ctxMenu.device.endU
            }}U</span
          >
        </div>
        <template v-if="ctxMenu.kind === 'u'">
          <button type="button" @click="assignAt(ctxMenu.u)">
            <b>上架设备到此位置</b>
            <span>选择一台待上架/已下架设备放置到 U{{ ctxMenu.u }}</span>
          </button>
        </template>
        <template v-else>
          <button type="button" @click="openMove()">
            <b>迁移设备</b>
            <span>移动到其它机柜的指定 U 位</span>
          </button>
          <button type="button" class="danger" @click="offlineDevice()">
            <b>下架设备</b>
            <span>设备将转为已下架并释放 U 位</span>
          </button>
        </template>
      </div>
    </teleport>

    <!-- 上架设备选择 -->
    <el-dialog v-model="assignTarget.visible" title="上架设备" width="520px">
      <p style="margin: 0 0 12px; color: #6b7280">
        目标位置：{{ assignTarget.rackName }} · U{{ assignTarget.startU }}
      </p>
      <el-table
        v-loading="assignableLoading"
        :data="assignable"
        size="small"
        max-height="360"
        data-test="screen-assign-table"
      >
        <el-table-column prop="code" label="编码" min-width="120" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="高度" width="70">
          <template #default="{ row }">{{ row.heightU }}U</template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{ row }">
            <el-button link type="primary" @click="confirmAssign(row.id)">上架</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 迁移设备 -->
    <el-dialog v-model="moveTarget.visible" title="迁移设备" width="520px">
      <el-form label-width="90px">
        <el-form-item label="设备">
          <span>{{ moveTarget.deviceName }}</span>
        </el-form-item>
        <el-form-item label="目标机柜" required>
          <el-select v-model="moveTarget.rackId" filterable style="width: 100%">
            <el-option
              v-for="r in roomRacks"
              :key="r.id"
              :label="`${r.name}（${r.code}）`"
              :value="r.id ?? ''"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="起始 U" required>
          <el-input-number v-model="moveTarget.startU" :min="1" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="moveTarget.visible = false">取消</el-button>
        <el-button type="primary" @click="confirmMove">确认移位</el-button>
      </template>
    </el-dialog>

    <!-- 机柜详情 -->
    <el-dialog
      v-model="detailVisible"
      :title="`机柜详情 · ${selectedRack?.name ?? ''}`"
      width="760px"
    >
      <template v-if="selectedRack">
        <el-descriptions :column="3" border>
          <el-descriptions-item label="编码">{{ selectedRack.code }}</el-descriptions-item>
          <el-descriptions-item label="规格">{{ selectedRack.uHeight }}U</el-descriptions-item>
          <el-descriptions-item label="状态">
            {{ statusLabel(selectedRack.status ?? "") }}
          </el-descriptions-item>
          <el-descriptions-item label="尺寸" :span="2">
            {{ selectedRack.widthMm }} × {{ selectedRack.depthMm }} × {{ selectedRack.heightMm }} mm
          </el-descriptions-item>
          <el-descriptions-item label="位置">
            {{ rackPositionText(selectedRack) }}
          </el-descriptions-item>
          <el-descriptions-item label="已使用">
            {{ layoutUsed(selectedRack) }}U
          </el-descriptions-item>
          <el-descriptions-item label="剩余">
            {{ selectedMetrics(selectedRack)?.remain }}U
          </el-descriptions-item>
          <el-descriptions-item label="使用率">
            {{ selectedMetrics(selectedRack)?.rate }}%
          </el-descriptions-item>
          <el-descriptions-item label="所属位置" :span="3">
            {{ selectedRack.dcName }} / {{ room?.name }}
          </el-descriptions-item>
        </el-descriptions>
        <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 14px">
          <el-button @click="openEditRack()">编辑机柜</el-button>
          <el-button type="danger" plain @click="removeRack()">删除机柜</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 机柜新增/编辑 -->
    <el-dialog
      v-model="editor.visible"
      :title="editor.mode === 'create' ? '新增机柜' : '编辑机柜'"
      width="640px"
      destroy-on-close
    >
      <el-form label-width="90px">
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="编码" :required="editor.mode === 'edit' || !editor.autoCode">
              <div class="code-field">
                <el-input
                  v-model="form.code"
                  :disabled="editor.mode === 'create' && editor.autoCode"
                  :placeholder="
                    editor.mode === 'create' && editor.autoCode
                      ? '保存时自动生成'
                      : '请输入机柜编码'
                  "
                />
                <el-checkbox v-if="editor.mode === 'create'" v-model="editor.autoCode">
                  自动生成
                </el-checkbox>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="名称" required>
              <el-input v-model="form.name" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="状态">
              <el-select v-model="form.status" style="width: 100%">
                <el-option
                  v-for="o in RACK_STATUS_OPTIONS"
                  :key="o.value"
                  :label="o.label"
                  :value="o.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="editor.mode === 'create'" :span="12">
            <el-form-item label="机柜模板">
              <el-select
                :model-value="form.templateId"
                clearable
                placeholder="不选择则使用自定义参数"
                style="width: 100%"
                @update:model-value="form.templateId = $event"
                @change="onTemplatePick"
              >
                <el-option
                  v-for="o in templateOptions"
                  :key="o.template.id"
                  :label="`${o.template.name} · V${o.version?.revision} · ${o.version?.uHeight}U`"
                  :value="o.template.id ?? ''"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="6">
            <el-form-item label="U 位">
              <el-input-number
                v-model="form.uHeight"
                :min="1"
                :max="100"
                step-strictly
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="宽(mm)">
              <el-input-number
                v-model="form.widthMm"
                :min="1"
                :step="10"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="深(mm)">
              <el-input-number
                v-model="form.depthMm"
                :min="1"
                :step="10"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="高(mm)">
              <el-input-number
                v-model="form.heightMm"
                :min="1"
                :step="10"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="6">
            <el-form-item label="区域">
              <el-input v-model="form.zone" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="排">
              <el-input v-model="form.row" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="列">
              <el-input v-model="form.column" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="通道">
              <el-input v-model="form.aisle" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注">
          <el-input v-model="form.remarks" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editor.visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" data-test="screen-rack-save" @click="saveRack">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style>
/* 全局样式(与 v2 RoomScreenView CSS 一致不带 data-v,便于 ale-theme.css 暗色段覆盖)。
   仅保留本屏用到的规则;对话框走 el-form/el-descriptions 默认皮肤。 */
.screen-page {
  width: 100vw;
  min-width: 1180px;
  height: 100vh;
  min-height: 720px;
  display: flex;
  overflow: hidden;
  color: #d8efff;
  background: radial-gradient(circle at 75% 15%, #0a3153 0, #061a31 34%, #030d1d 75%);
  font-family: "Microsoft YaHei", "PingFang SC", sans-serif;
}
.control-panel {
  width: 330px;
  flex: 0 0 330px;
  height: 100%;
  padding: 24px 22px 18px;
  box-sizing: border-box;
  overflow-y: auto;
  border-right: 1px solid rgba(45, 200, 240, 0.28);
  background: linear-gradient(180deg, #071b31fa, #030e1dfa);
  box-shadow: 12px 0 40px #00000047;
}
.control-panel::-webkit-scrollbar,
.rack-room-scroller::-webkit-scrollbar {
  width: 7px;
  height: 7px;
}
.control-panel::-webkit-scrollbar-thumb,
.rack-room-scroller::-webkit-scrollbar-thumb {
  border-radius: 8px;
  background: #174666;
}
.brand-block {
  display: flex;
  align-items: center;
  gap: 13px;
  padding-bottom: 22px;
  border-bottom: 1px solid rgba(80, 170, 210, 0.2);
}
.brand-mark {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border: 1px solid #2dc8f0;
  color: #65dcff;
  font-weight: 800;
  font-size: 18px;
  background: #2dc8f01a;
  box-shadow:
    inset 0 0 18px #2dc8f029,
    0 0 15px #2dc8f01f;
  clip-path: polygon(12% 0, 88% 0, 100% 12%, 100% 88%, 88% 100%, 12% 100%, 0 88%, 0 12%);
}
.brand-block h1 {
  margin: 0 0 4px;
  font-size: 21px;
  letter-spacing: 2px;
  color: #e8f8ff;
}
.brand-block p {
  margin: 0;
  color: #547c99;
  font-size: 10px;
  letter-spacing: 1.4px;
}
.selector-section {
  display: grid;
  gap: 9px;
  padding: 20px 0 6px;
}
.selector-section label {
  margin-top: 3px;
  color: #7da5bf;
  font-size: 12px;
}
.screen-select {
  width: 100%;
}
.screen-select .el-select__wrapper {
  min-height: 40px;
  border: 1px solid #174d70;
  background: #071d34;
  box-shadow: none;
}
.screen-select .el-select__selected-item {
  color: #d8efff;
}
.room-card,
.legend-card,
.selected-card {
  margin-top: 16px;
  padding: 15px;
  border: 1px solid rgba(50, 139, 184, 0.28);
  border-radius: 5px;
  background: linear-gradient(135deg, #0c2c48b8, #041628a8);
}
.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  color: #92b8d0;
  font-size: 12px;
}
.section-title > span {
  width: 3px;
  height: 13px;
  background: #2dc8f0;
  box-shadow: 0 0 8px #2dc8f0;
}
.room-name {
  color: #f0fbff;
  font-size: 18px;
  font-weight: 700;
}
.room-code {
  margin-top: 3px;
  color: #4f7894;
  font-size: 11px;
}
.room-meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 10px;
  margin-top: 15px;
}
.room-meta span,
.selected-grid span {
  display: block;
  color: #557b94;
  font-size: 10px;
}
.room-meta strong {
  display: block;
  margin-top: 3px;
  overflow: hidden;
  color: #a8ccdf;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.room-card-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 14px;
}
.room-card-actions button,
.selected-actions button {
  padding: 7px 6px;
  border: 1px solid #1e5d7e;
  color: #7bdfff;
  background: #0a2940cc;
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
}
.room-card-actions button:hover,
.selected-actions button:hover {
  border-color: #2dc8f0;
  background: #2dc8f02e;
}
.room-card-actions button:disabled {
  opacity: 0.42;
  cursor: not-allowed;
}
.stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 16px;
}
.stat-item {
  min-height: 70px;
  padding: 10px 11px;
  border: 1px solid rgba(45, 119, 153, 0.24);
  background: #051c31b8;
  box-sizing: border-box;
}
.stat-item span {
  display: block;
  color: #6f98b1;
  font-size: 11px;
}
.stat-item strong {
  display: inline-block;
  margin-top: 5px;
  color: currentColor;
  font-size: 27px;
}
.stat-item small {
  margin-left: 7px;
  color: #496c84;
  font-size: 8px;
}
.cyan {
  color: #2dc8f0;
}
.green {
  color: #24d6a1;
}
.blue {
  color: #4d8fff;
}
.orange {
  color: #ffb547;
}
.status-legend-list {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.status-legend-item {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 9px;
  border: 1px solid #173f5c;
  border-radius: 4px;
  color: #89adc3;
  font-size: 11px;
  font-family: inherit;
  text-align: left;
  background: #061b2f8c;
  cursor: pointer;
  transition:
    border-color 0.18s,
    background 0.18s;
}
.status-legend-item:hover {
  border-color: #2dc8f0;
}
.status-legend-item.active {
  border-color: #2dc8f0;
  background: #2dc8f024;
  color: #d9f6ff;
}
.status-legend-item i {
  width: 9px;
  height: 9px;
  flex: 0 0 9px;
  border-radius: 2px;
  box-shadow: 0 0 7px currentColor;
}
.status-legend-item i.all-dot {
  background: #d8efff;
}
.status-legend-item b {
  margin-left: auto;
  color: #d5ecf8;
  font-family: Consolas, monospace;
  font-size: 11px;
}
.filter-hint {
  margin-left: auto;
  color: #4f7891;
  font-size: 10px;
}
.device-legend-list {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 9px 14px;
}
.device-legend-list div {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #89adc3;
  font-size: 11px;
}
.device-legend-list i {
  width: 9px;
  height: 9px;
  border-radius: 2px;
  box-shadow: 0 0 7px currentColor;
}
.u-note {
  margin-top: 11px;
  padding-top: 10px;
  border-top: 1px dashed #174562;
  color: #4f7891;
  font-size: 10px;
  line-height: 1.6;
}
.selected-card {
  border-color: #2dc8f080;
}
.selected-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}
.selected-header small {
  display: block;
  color: #547d98;
  font-size: 10px;
}
.selected-header strong {
  display: block;
  margin-top: 3px;
  color: #e9f9ff;
  font-size: 16px;
}
.selected-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 13px;
}
.selected-grid b {
  display: block;
  margin-top: 3px;
  overflow: hidden;
  color: #a9ccdf;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.selected-progress {
  height: 4px;
  margin-top: 12px;
  overflow: hidden;
  border-radius: 4px;
  background: #0a2a44;
}
.selected-progress i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, #24d6a1, #2dc8f0, #ffb547);
  box-shadow: 0 0 8px #2dc8f0;
}
.selected-actions {
  display: flex;
  gap: 6px;
  margin-top: 12px;
}
.selected-actions button {
  flex: 1;
}
.selected-actions button.danger {
  color: #ff9ca3;
  border-color: #ef64708c;
}
.selected-status-button {
  padding: 2px 0;
  border: 0;
  font: inherit;
  background: transparent;
  cursor: pointer;
}
.selected-status-button:hover {
  text-decoration: underline;
}
.u-button {
  width: 100%;
  margin-top: 14px;
  padding: 9px;
  border: 1px solid #268db5;
  color: #7bdfff;
  background: #18759a29;
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
}
.u-button:hover {
  background: #2dc8f03d;
}
.panel-actions {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}
.panel-actions button {
  flex: 1;
  padding: 9px 5px;
  border: 1px solid #174b6b;
  color: #719ab4;
  background: #061a2d;
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
}
.panel-actions button:hover {
  color: #bfeaff;
  border-color: #2b90b8;
}
.visual-area {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0 28px 24px;
}
.screen-header {
  height: 86px;
  flex: 0 0 86px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid rgba(54, 140, 182, 0.2);
}
.screen-header h2 {
  margin: 0 0 5px;
  color: #e6f7ff;
  font-size: 22px;
  letter-spacing: 1px;
}
.screen-header h2 span {
  margin: 0 12px;
  color: #2dc8f0;
}
.screen-header p {
  margin: 0;
  color: #527a94;
  font-size: 11px;
}
.header-status {
  display: flex;
  align-items: center;
  gap: 28px;
}
.online {
  color: #58bda5;
  font-size: 11px;
}
.online i {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 7px;
  border-radius: 50%;
  background: #24d6a1;
  box-shadow: 0 0 9px #24d6a1;
}
.clock {
  text-align: right;
}
.clock strong {
  display: block;
  color: #d9f3ff;
  font-family: Consolas, monospace;
  font-size: 22px;
  font-weight: 500;
  letter-spacing: 2px;
}
.clock span {
  color: #587e97;
  font-size: 10px;
}
.canvas-shell {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  margin-top: 20px;
  overflow: hidden;
  border: 1px solid rgba(41, 136, 180, 0.36);
  background: #041222c2;
  box-shadow:
    inset 0 0 50px #05324f38,
    0 10px 45px #0003;
}
.canvas-toolbar {
  min-height: 48px;
  flex: 0 0 48px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 18px;
  border-bottom: 1px solid rgba(35, 121, 159, 0.25);
  color: #98bbd0;
  font-size: 12px;
}
.pulse-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 8px;
  border-radius: 50%;
  background: #2dc8f0;
  box-shadow: 0 0 9px #2dc8f0;
}
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.rack-search {
  width: 220px;
}
.rack-search .el-input__wrapper {
  min-height: 28px;
  border: 1px solid #174d70;
  box-shadow: none;
  background: #061a2e;
}
.rack-search .el-input__inner {
  color: #d8efff;
}
.canvas-action {
  padding: 6px 9px;
  border: 1px solid #174d70;
  border-radius: 4px;
  color: #8db5ca;
  font: inherit;
  font-size: 11px;
  background: #061a2e;
  cursor: pointer;
}
.canvas-action:hover:not(:disabled) {
  border-color: #2dc8f0;
  color: #d9f7ff;
}
.canvas-action.primary {
  border-color: #268db5;
  color: #9eeeff;
  background: #18759a38;
}
.canvas-action:disabled {
  opacity: 0.42;
  cursor: not-allowed;
}
.canvas-info {
  color: #527b96;
}
.drag-tip {
  color: #5a8da7;
  font-size: 10px;
  white-space: nowrap;
}
.screen-page .filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-bottom: 1px solid rgba(45, 119, 153, 0.25);
  background: #04111f8c;
}
.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border: 1px solid #173f5c;
  border-radius: 999px;
  color: #89adc3;
  font-size: 11px;
  font-family: inherit;
  background: #061b2f8c;
  cursor: pointer;
  transition:
    border-color 0.18s,
    color 0.18s,
    background 0.18s;
}
.filter-chip:hover {
  border-color: #2dc8f0;
  color: #d9f6ff;
}
.filter-chip.active {
  border-color: #2dc8f0;
  color: #d9f6ff;
  background: #2dc8f029;
  box-shadow: 0 0 8px #2dc8f040;
}
.filter-chip i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}
.filter-chip span {
  min-width: 16px;
  padding: 0 4px;
  border-radius: 8px;
  text-align: center;
  color: #dff4ff;
  font-family: Consolas, monospace;
  font-size: 10px;
  background: #2dc8f02e;
}
.rack-room-scroller {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 16px;
}
.room-boundary {
  position: relative;
  min-width: 980px;
  min-height: 100%;
  padding: 18px 20px 64px;
  border: 1px solid #174b70;
  border-radius: 8px;
  box-sizing: border-box;
  background-color: #07172b;
  background-image:
    linear-gradient(rgba(21, 49, 80, 0.45) 1px, transparent 1px),
    linear-gradient(90deg, rgba(21, 49, 80, 0.45) 1px, transparent 1px);
  background-size: 24px 24px;
  box-shadow: inset 0 0 60px #0837532e;
}
.room-boundary-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 5px 4px 18px;
  border-bottom: 1px solid #173c5a;
}
.room-boundary-header strong {
  display: block;
  color: #bdeeff;
  font-size: 16px;
}
.room-boundary-header span {
  display: block;
  margin-top: 4px;
  color: #4f7894;
  font-size: 10px;
}
.capacity-summary {
  text-align: right;
}
.capacity-summary span {
  display: block;
  color: #4f7894;
  font-size: 10px;
}
.capacity-summary strong {
  color: #2dc8f0;
  font-size: 22px;
}
.capacity-summary strong small {
  color: #567b93;
  font-size: 11px;
}
.room-layout-canvas {
  position: relative;
  margin-top: 18px;
  overflow: visible;
  border: 1px dashed rgba(45, 200, 240, 0.22);
  background: #04142347;
}
.layout-row-guide {
  position: absolute;
  left: 12px;
  right: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  pointer-events: none;
}
.layout-row-guide span {
  flex: 0 0 auto;
  padding: 3px 7px;
  border: 1px solid rgba(45, 200, 240, 0.28);
  border-radius: 3px;
  color: #5b92ad;
  font-size: 9px;
  background: #051829db;
}
.layout-row-guide i {
  flex: 1;
  border-top: 1px dashed rgba(45, 200, 240, 0.16);
}
.rack-slot-frame {
  position: absolute;
  z-index: 1;
  width: 218px;
  height: auto;
  box-sizing: border-box;
  border: 1px dashed rgba(45, 200, 240, 0.34);
  border-radius: 10px;
  background: linear-gradient(180deg, #124b6933, #05182a14);
  box-shadow: inset 0 0 22px #2dc8f00d;
  pointer-events: none;
}
.rack-slot-frame::before {
  content: "固定机柜槽位";
  position: absolute;
  top: 7px;
  left: 9px;
  padding: 2px 5px;
  border-radius: 3px;
  color: #69abc7b8;
  font-size: 8px;
  letter-spacing: 1px;
  background: #051829b8;
}
.rack-card-floating {
  position: absolute;
  z-index: 2;
  margin: 0;
}
.rack-card-floating:hover {
  z-index: 10;
}
.rack-card-floating.dragging {
  z-index: 20;
  border-color: #6ce5ff;
  box-shadow:
    0 0 0 2px #2dc8f040,
    0 18px 32px #0000006b;
}
.rack-drag-handle {
  cursor: grab;
  touch-action: none;
}
.rack-card-floating.dragging .rack-drag-handle {
  cursor: grabbing;
}
body.is-rack-dragging {
  user-select: none;
  cursor: grabbing;
}
.screen-page .rack-card {
  width: 218px;
  min-width: 218px;
  max-width: 218px;
  height: var(--rack-card-height);
  min-height: var(--rack-card-height);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 10px;
  border: 1px solid #1b4d6b;
  border-radius: 7px;
  outline: none;
  background: linear-gradient(180deg, #0a263f, #061a2d);
  box-shadow: 0 7px 18px #00000040;
  box-sizing: border-box;
  cursor: pointer;
  transition:
    border-color 0.18s,
    transform 0.18s,
    box-shadow 0.18s;
}
.screen-page .rack-card:hover,
.screen-page .rack-card.selected {
  border-color: #2dc8f0;
  box-shadow:
    0 0 0 1px #2dc8f038,
    0 8px 24px #0000004d;
}
.screen-page .rack-card.selected {
  background: linear-gradient(180deg, #0c3150, #071d33);
}
.rack-card-floating:hover,
.rack-card-floating.selected,
.rack-card-floating.dragging {
  transform: none;
}
.rack-card-header {
  min-height: 31px;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 1px 2px 8px;
  box-sizing: border-box;
}
.rack-card-header > div {
  min-width: 0;
}
.rack-card-header strong {
  display: block;
  max-width: 168px;
  overflow: hidden;
  color: #d9f5ff;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rack-card-header span {
  display: block;
  margin-top: 3px;
  color: #557d96;
  font-size: 9px;
}
.rack-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 2px;
  padding: 2px 7px;
  border: 1px solid var(--status-color);
  border-radius: 999px;
  color: var(--status-color);
  font-size: 9px;
  font-weight: 600;
  background: color-mix(in srgb, var(--status-color) 12%, transparent);
  font-family: inherit;
  white-space: nowrap;
  cursor: pointer;
  appearance: none;
}
.rack-status-chip:hover {
  filter: brightness(1.18);
}
.rack-status-chip i {
  width: 6px;
  height: 6px;
  margin: 0;
  border-radius: 50%;
  background: var(--status-color);
  box-shadow: 0 0 6px var(--status-color);
}
.rack-usage-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #557d96;
  font-size: 8px;
}
.rack-usage-line b {
  color: #8fdfff;
  font-size: 10px;
}
.rack-usage-bar {
  height: 3px;
  margin: 5px 0 9px;
  overflow: hidden;
  background: #081929;
}
.rack-usage-bar i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, #24d6a1, #2dc8f0, #ffb547);
}
.cabinet-shell {
  padding: 0 7px;
}
.cabinet-shell.is-full .cabinet-frame {
  border-color: #ffb5478c;
  box-shadow:
    inset 0 0 12px #000,
    0 5px 12px #00000059,
    0 0 10px #ffb54738;
}
.cabinet-top {
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 7px;
  border: 1px solid #3d5c70;
  border-bottom: 0;
  border-radius: 4px 4px 0 0;
  color: #7698aa;
  background: linear-gradient(#293f4d, #162d3d);
  box-shadow: inset 0 1px #ffffff1f;
}
.cabinet-top span {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #071521;
  box-shadow: inset 0 0 0 1px #5d7787;
}
.cabinet-top b {
  font-size: 8px;
  font-weight: 500;
  letter-spacing: 2px;
}
.cabinet-frame {
  position: relative;
  height: var(--rack-height);
  padding: 0 12px 0 30px;
  border: 3px solid #38576b;
  border-top-width: 2px;
  border-bottom-width: 2px;
  box-sizing: content-box;
  background: #06111d;
  box-shadow:
    inset 0 0 12px #000,
    0 5px 12px #00000059;
}
.rack-rail {
  position: absolute;
  z-index: 4;
  top: 0;
  bottom: 0;
  width: 6px;
  background: repeating-linear-gradient(
    to bottom,
    #324d5e 0,
    #324d5e 2px,
    #0b1821 2px,
    #0b1821 5px
  );
  box-shadow: inset 0 0 2px #7992a0;
  pointer-events: none;
}
.rack-rail.left {
  left: 20px;
}
.rack-rail.right {
  right: 3px;
}
.u-scale {
  position: absolute;
  top: 0;
  left: 0;
  width: 20px;
  height: var(--rack-height);
  display: flex;
  flex-direction: column;
  background: #102536;
}
.u-scale span {
  height: 9px;
  flex: 0 0 9px;
  display: grid;
  place-items: center;
  border-bottom: 1px solid #294355;
  box-sizing: border-box;
  color: #6e91a5;
  font-family: Consolas, monospace;
  font-size: 6px;
  line-height: 1;
}
.u-grid {
  position: relative;
  height: var(--rack-height);
  overflow: hidden;
  border: 1px solid #315167;
  box-sizing: border-box;
  background: #0a1825;
}
.u-line {
  width: 100%;
  height: 9px;
  display: block;
  padding: 0;
  box-sizing: border-box;
  border: 0;
  border-bottom: 1px solid #263c4c;
  background: linear-gradient(90deg, #14304359, #08172333);
  cursor: context-menu;
}
.u-line:hover:not(.occupied) {
  background: linear-gradient(90deg, #2dc8f038, #2dc8f014);
}
.u-line.major {
  border-bottom-color: #496b7e;
}
.u-line.occupied {
  cursor: default;
}
.device-block {
  position: absolute;
  z-index: 2;
  left: 1px;
  right: 1px;
  width: calc(100% - 2px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 3px;
  min-height: 8px;
  padding: 0 4px;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--device-color) 72%, #d9f7ff);
  box-sizing: border-box;
  color: #f2fbff;
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--device-color) 78%, #071523),
    color-mix(in srgb, var(--device-color) 52%, #071523)
  );
  box-shadow:
    inset 3px 0 var(--device-color),
    inset 0 0 5px #ffffff1f;
  cursor: pointer;
}
.device-block:hover {
  z-index: 5;
  filter: brightness(1.22);
  box-shadow:
    inset 3px 0 var(--device-color),
    0 0 8px var(--device-color);
}
.device-block strong {
  min-width: 0;
  overflow: hidden;
  font-size: 7px;
  font-weight: 600;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.device-block small {
  flex: 0 0 auto;
  font-size: 6px;
  opacity: 0.82;
  white-space: nowrap;
}
.screen-page .device-block.search-hit {
  z-index: 4;
  outline: 2px solid #ffe08a;
  box-shadow:
    0 0 0 3px #ffe08a38,
    0 0 18px #ffe08abf;
  animation: device-search-hit 1.1s ease-in-out infinite alternate;
}
@keyframes device-search-hit {
  0% {
    filter: brightness(1);
  }
  to {
    filter: brightness(1.45);
  }
}
.rack-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 13px;
  padding: 0 2px;
  color: #547a91;
  font-size: 9px;
}
.rack-card-footer button {
  padding: 3px 8px;
  border: 1px solid #1e5d7e;
  color: #68cbe9;
  background: #0a2940;
  cursor: pointer;
  font-family: inherit;
  font-size: 9px;
}
.rack-card-footer button:hover {
  border-color: #2dc8f0;
  color: #c9f7ff;
}
.room-door {
  position: absolute;
  right: 35px;
  bottom: 20px;
  display: flex;
  align-items: center;
  gap: 9px;
  color: #4f8ba9;
  font-size: 10px;
}
.room-door i {
  width: 36px;
  height: 20px;
  border-top: 3px solid #2dc8f0;
  border-left: 3px solid #2dc8f0;
  border-radius: 18px 0 0;
}
.empty-screen {
  flex: 1;
  display: grid;
  place-content: center;
  text-align: center;
}
.empty-icon {
  width: 70px;
  height: 70px;
  display: grid;
  place-items: center;
  margin: 0 auto 18px;
  border: 1px solid #1e5372;
  color: #2e84a8;
  font-size: 30px;
  background: #0f3d5b3d;
  transform: rotate(45deg);
}
.empty-screen h3 {
  margin: 0 0 9px;
  color: #86aec5;
  font-size: 17px;
}
.empty-screen p {
  margin: 0;
  color: #426980;
  font-size: 11px;
}
.rack-context-menu {
  position: fixed;
  z-index: 5000;
  width: 236px;
  padding: 7px;
  border: 1px solid rgba(45, 200, 240, 0.55);
  border-radius: 8px;
  color: #dff7ff;
  background: linear-gradient(180deg, #082035fc, #03101dfc);
  box-shadow:
    0 16px 42px #00000094,
    inset 0 0 24px #2dc8f00f;
  font-family: "Microsoft YaHei", "PingFang SC", sans-serif;
}
.context-menu-header {
  padding: 8px 10px 10px;
  border-bottom: 1px solid rgba(45, 200, 240, 0.18);
}
.context-menu-header small {
  display: block;
  color: #53819c;
  font-size: 10px;
}
.context-menu-header strong {
  display: block;
  margin-top: 3px;
  overflow: hidden;
  color: #dcf8ff;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.context-menu-header span {
  display: block;
  margin-top: 4px;
  color: #729bb3;
  font-size: 10px;
}
.rack-context-menu > button {
  width: 100%;
  display: block;
  margin-top: 4px;
  padding: 8px 10px;
  border: 0;
  border-radius: 5px;
  color: #bfe8f7;
  text-align: left;
  background: transparent;
  cursor: pointer;
  font-family: inherit;
}
.rack-context-menu > button:hover {
  color: #ecfbff;
  background: #2dc8f021;
}
.rack-context-menu > button b,
.rack-context-menu > button span {
  display: block;
}
.rack-context-menu > button b {
  font-size: 12px;
}
.rack-context-menu > button span {
  margin-top: 3px;
  color: #527b94;
  font-size: 9px;
}
.rack-context-menu > button.danger b {
  color: #ff9099;
}
.rack-context-menu > button.danger:hover {
  background: #ef647021;
}
@media (max-width: 1280px) {
  .control-panel {
    width: 300px;
    flex-basis: 300px;
    padding-left: 17px;
    padding-right: 17px;
  }
  .visual-area {
    padding-left: 18px;
    padding-right: 18px;
  }
  .screen-page {
    min-width: 1100px;
  }
}
</style>
