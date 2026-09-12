<script setup lang="ts">
/**
 * PDU 与供电连接面板(屏6b;挂机柜容量对话框右栏,复刻 v2 PDUManager)。
 * PDU 卡(状态 tag/编辑/删除 + 厂商型号/额定功率/输入电流 + 插座明细表
 * 制式国标欧标/状态可用已连接停用/供电设备/断开+连接设备)与三组对话框
 * (PDU 编辑含自动生成编码/插座编辑 10A·16A 选档/连接设备 主路·冗余路)。
 * 全部真后端:pdus/sockets/connections 三组端点;socket 状态由连接事实维护
 * (后端收权 C10,body status 会被忽略——保留 v2 的表单项但如实知悉)。
 */
import { computed, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  connectSocket,
  createPDU,
  createSocket,
  deletePDU,
  deleteSocket,
  disconnectPDUConnection,
  fetchPDUConnections,
  fetchPDUs,
  fetchSockets,
  updatePDU,
  updateSocket,
  type Device,
  type PDU,
  type PDUConnection,
  type PDUSocket,
} from "@/features/resource/api";
import { autoCode, rackErrMsg } from "@/features/rack/rackShared";

const props = defineProps<{ rackId: string; devices: Device[] }>();
defineExpose({ reload: loadAll });

const loading = ref(false);
const saving = ref(false);
const errorText = ref("");
const pdus = ref<PDU[]>([]);
const connections = ref<PDUConnection[]>([]);
const socketsByPdu = reactive<Record<string, PDUSocket[]>>({});

const pduDialog = reactive({
  visible: false,
  mode: "create" as "create" | "edit",
  id: "",
  version: 0,
  autoGenerateCode: true,
});
const pduForm = reactive({
  code: "",
  name: "",
  manufacturer: "",
  modelNumber: "",
  inputVoltage: undefined as number | undefined,
  ratedPowerW: undefined as number | undefined,
  ratedCurrentA: undefined as number | undefined,
  status: "ACTIVE",
  remarks: "",
  autoGenerateCode: true,
});
const socketDialog = reactive({
  visible: false,
  mode: "create" as "create" | "edit",
  pduId: "",
  id: "",
  version: 1,
});
const socketForm = reactive({
  socketNo: 1,
  standard: "CN",
  amperageA: 10,
  label: "",
  status: "AVAILABLE",
});
const connDialog = reactive({ visible: false, socketId: "", socketNo: 0 });
const connForm = reactive({
  deviceId: "",
  powerW: undefined as number | undefined,
  circuit: "",
  redundancyRole: "PRIMARY",
});

const rackedDevices = computed(() => props.devices.filter((d) => !!d.id));
function deviceLabel(d: Device): string {
  const pos = d.currentPosition
    ? `${d.currentPosition.startU}-${d.currentPosition.endU} U`
    : "已上架";
  return `${d.name}（${d.code}） · ${pos}`;
}
function standardText(s: string): string {
  return s === "CN" ? "国标" : "欧标";
}
function socketStatusText(s: string): string {
  return { AVAILABLE: "可用", CONNECTED: "已连接", DISABLED: "停用" }[s] ?? s;
}
function socketStatusType(s: string): "success" | "info" | "warning" {
  return s === "AVAILABLE" ? "success" : s === "CONNECTED" ? "warning" : "info";
}
function connectionOf(socket: PDUSocket): PDUConnection | undefined {
  return connections.value.find((c) => c.socketId === socket.id);
}
function connectionText(socket: PDUSocket): string {
  const c = connectionOf(socket);
  if (!c) return "未连接";
  const d = rackedDevices.value.find((x) => x.id === c.deviceId);
  return d ? `${d.name ?? ""}（${d.code ?? ""}）` : (c.deviceId ?? "-");
}

async function loadAll() {
  if (!props.rackId) return;
  loading.value = true;
  errorText.value = "";
  try {
    const [list, conns] = await Promise.all([
      fetchPDUs(props.rackId),
      fetchPDUConnections(props.rackId).catch(() => [] as PDUConnection[]),
    ]);
    pdus.value = list;
    connections.value = conns;
    const results = await Promise.all(
      list.map(async (p) => {
        try {
          return [p.id ?? "", await fetchSockets(p.id ?? "")] as const;
        } catch {
          return [p.id ?? "", [] as PDUSocket[]] as const;
        }
      }),
    );
    Object.keys(socketsByPdu).forEach((k) => delete socketsByPdu[k]);
    for (const [id, sockets] of results) socketsByPdu[id] = sockets;
  } catch (e) {
    errorText.value = "PDU 信息加载失败，请稍后重试";
    rackErrMsg(e);
  } finally {
    loading.value = false;
  }
}
watch(() => props.rackId, loadAll, { immediate: true });

function openCreatePDU() {
  pduDialog.mode = "create";
  pduDialog.autoGenerateCode = true;
  Object.assign(pduForm, {
    code: "",
    name: "",
    manufacturer: "",
    modelNumber: "",
    inputVoltage: undefined,
    ratedPowerW: undefined,
    ratedCurrentA: undefined,
    status: "ACTIVE",
    remarks: "",
  });
  pduDialog.visible = true;
}
function openEditPDU(p: PDU) {
  pduDialog.mode = "edit";
  pduDialog.id = p.id ?? "";
  pduDialog.version = p.version ?? 0;
  Object.assign(pduForm, {
    code: p.code ?? "",
    name: p.name ?? "",
    manufacturer: p.manufacturer ?? "",
    modelNumber: p.modelNumber ?? "",
    inputVoltage: p.inputVoltage ?? undefined,
    ratedPowerW: p.ratedPowerW ?? undefined,
    ratedCurrentA: p.ratedCurrentA ?? undefined,
    status: p.status ?? "ACTIVE",
    remarks: p.remarks ?? "",
  });
  pduDialog.visible = true;
}
async function savePDU() {
  if (
    !pduForm.name.trim() ||
    (pduDialog.mode === "edit" && !pduForm.code.trim()) ||
    (pduDialog.mode === "create" && !pduForm.autoGenerateCode && !pduForm.code.trim())
  ) {
    ElMessage.warning("请输入 PDU 名称");
    return;
  }
  saving.value = true;
  try {
    const payload = { ...pduForm, code: pduForm.code.trim() };
    if (pduDialog.mode === "create") {
      const code = pduForm.autoGenerateCode ? autoCode("PDU") : payload.code;
      await createPDU(props.rackId, { ...payload, code } as never);
    } else {
      await updatePDU(pduDialog.id, pduDialog.version, payload as never);
    }
    pduDialog.visible = false;
    ElMessage.success("PDU 已保存");
    await loadAll();
  } catch {
    ElMessage.error("PDU 保存失败，请检查编码和版本后重试");
  } finally {
    saving.value = false;
  }
}
async function removePDU(p: PDU) {
  try {
    await ElMessageBox.confirm(`确定删除 PDU“${p.name}”吗？`, "删除确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deletePDU(p.id ?? "", p.version ?? 0);
    ElMessage.success("PDU 已删除");
    await loadAll();
  } catch {
    ElMessage.error("PDU 删除失败，可能仍存在插座或连接");
  }
}

function openCreateSocket(p: PDU) {
  socketDialog.mode = "create";
  socketDialog.pduId = p.id ?? "";
  Object.assign(socketForm, {
    socketNo: (socketsByPdu[p.id ?? ""]?.length ?? 0) + 1,
    standard: "CN",
    amperageA: 10,
    label: "",
    status: "AVAILABLE",
  });
  socketDialog.visible = true;
}
function openEditSocket(p: PDU, s: PDUSocket) {
  socketDialog.mode = "edit";
  socketDialog.pduId = p.id ?? "";
  socketDialog.id = s.id ?? "";
  socketDialog.version = s.version ?? 1;
  Object.assign(socketForm, {
    socketNo: s.socketNo ?? 1,
    standard: s.standard ?? "CN",
    amperageA: s.amperageA ?? 10,
    label: s.label ?? "",
    status: s.status ?? "AVAILABLE",
  });
  socketDialog.visible = true;
}
async function saveSocket() {
  saving.value = true;
  try {
    if (socketDialog.mode === "create") {
      await createSocket(socketDialog.pduId, { ...socketForm } as never);
    } else {
      await updateSocket(socketDialog.id, socketDialog.version, { ...socketForm } as never);
    }
    socketDialog.visible = false;
    ElMessage.success("插座已保存");
    await loadAll();
  } catch {
    ElMessage.error("插座保存失败，请检查编号、制式和电流参数");
  } finally {
    saving.value = false;
  }
}
async function removeSocket(s: PDUSocket) {
  try {
    await ElMessageBox.confirm(`确定删除插座 ${s.socketNo} 吗？`, "删除确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deleteSocket(s.id ?? "", s.version ?? 1);
    ElMessage.success("插座已删除");
    await loadAll();
  } catch {
    ElMessage.error("插座删除失败，已连接插座不能直接删除");
  }
}

function openConnect(_p: PDU, s: PDUSocket) {
  connDialog.socketId = s.id ?? "";
  connDialog.socketNo = s.socketNo ?? 0;
  Object.assign(connForm, {
    deviceId: "",
    powerW: undefined,
    circuit: "",
    redundancyRole: "PRIMARY",
  });
  connDialog.visible = true;
}
async function confirmConnect() {
  if (!connForm.deviceId) {
    ElMessage.warning("请选择已上架设备");
    return;
  }
  saving.value = true;
  try {
    await connectSocket(connDialog.socketId, { ...connForm } as never);
    connDialog.visible = false;
    ElMessage.success("供电连接已建立");
    await loadAll();
  } catch {
    ElMessage.error("供电连接失败，请确认设备已上架且未重复连接");
  } finally {
    saving.value = false;
  }
}
async function disconnect(c: PDUConnection) {
  try {
    await ElMessageBox.confirm("确定断开该设备的供电连接吗？", "断开确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await disconnectPDUConnection(c.id ?? "", c.version ?? 1);
    ElMessage.success("供电连接已断开");
    await loadAll();
  } catch {
    ElMessage.error("断开供电连接失败");
  }
}
</script>

<template>
  <div v-loading="loading" class="pdu-manager">
    <div class="pdu-manager-heading">
      <div>
        <small>PDU &amp; POWER CONNECTIONS</small>
        <h3>PDU 与供电连接</h3>
      </div>
      <div class="pdu-heading-actions">
        <el-button size="small" :loading="loading" @click="loadAll">刷新</el-button>
        <el-button size="small" type="primary" @click="openCreatePDU">新增 PDU</el-button>
      </div>
    </div>
    <el-alert
      v-if="errorText"
      :title="errorText"
      type="warning"
      :closable="false"
      show-icon
      class="pdu-empty"
    />
    <div v-if="!loading && !pdus.length" class="pdu-empty">
      当前机柜尚未配置 PDU，PDU 参数为可选项。
    </div>
    <div v-for="p in pdus" :key="p.id" class="pdu-card">
      <div class="pdu-card-head">
        <div>
          <strong>{{ p.name }}</strong>
          <span>{{ p.code }}</span>
        </div>
        <div class="pdu-card-actions">
          <el-tag size="small" :type="p.status === 'ACTIVE' ? 'success' : 'info'">
            {{ p.status === "ACTIVE" ? "启用" : "停用" }}
          </el-tag>
          <el-button link type="primary" @click="openEditPDU(p)">编辑</el-button>
          <el-button link type="danger" @click="removePDU(p)">删除</el-button>
        </div>
      </div>
      <div class="pdu-meta">
        <span
          >厂商/型号：{{ [p.manufacturer, p.modelNumber].filter(Boolean).join(" / ") || "-" }}</span
        >
        <span>额定功率：{{ p.ratedPowerW ? `${p.ratedPowerW} W` : "未填写" }}</span>
        <span>输入电流：{{ p.ratedCurrentA ? `${p.ratedCurrentA} A` : "未填写" }}</span>
      </div>
      <div class="socket-toolbar">
        <span>插座明细（{{ (socketsByPdu[p.id ?? ""] ?? []).length }} 个）</span>
        <el-button size="small" link type="primary" @click="openCreateSocket(p)">
          新增插座
        </el-button>
      </div>
      <el-table
        v-if="(socketsByPdu[p.id ?? ''] ?? []).length"
        :data="socketsByPdu[p.id ?? '']"
        size="small"
        border
      >
        <el-table-column prop="socketNo" label="插座" width="60" />
        <el-table-column label="制式/电流" width="110">
          <template #default="{ row }">
            {{ standardText(row.standard) }} · {{ row.amperageA }}A
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="socketStatusType(row.status)">
              {{ socketStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="供电设备" min-width="160">
          <template #default="{ row }">
            <template v-if="connectionOf(row)">
              {{ connectionText(row) }}
              <small v-if="connectionOf(row)?.circuit">（{{ connectionOf(row)?.circuit }}）</small>
            </template>
            <span v-else class="pdu-unconnected">未连接</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button
              v-if="connectionOf(row)"
              link
              type="danger"
              size="small"
              @click="disconnect(connectionOf(row)!)"
            >
              断开
            </el-button>
            <el-button
              v-else
              link
              type="primary"
              size="small"
              :disabled="row.status !== 'AVAILABLE'"
              @click="openConnect(p, row)"
            >
              连接设备
            </el-button>
            <el-button link type="primary" size="small" @click="openEditSocket(p, row)">
              编辑
            </el-button>
            <el-button link type="danger" size="small" @click="removeSocket(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-else class="pdu-empty-sockets">暂无插座</div>
    </div>

    <!-- PDU 编辑 -->
    <el-dialog
      v-model="pduDialog.visible"
      :title="pduDialog.mode === 'create' ? '新增 PDU' : '编辑 PDU'"
      width="560px"
      append-to-body
      destroy-on-close
    >
      <el-form label-width="100px">
        <el-form-item
          label="PDU 编码"
          :required="pduDialog.mode === 'edit' || !pduForm.autoGenerateCode"
        >
          <div class="code-field">
            <el-input
              v-model="pduForm.code"
              :disabled="pduDialog.mode === 'create' && pduForm.autoGenerateCode"
              :placeholder="
                pduDialog.mode === 'create' && pduForm.autoGenerateCode
                  ? '可留空自动生成'
                  : '请输入 PDU 编码'
              "
            />
            <el-checkbox v-if="pduDialog.mode === 'create'" v-model="pduForm.autoGenerateCode">
              自动生成编码
            </el-checkbox>
          </div>
        </el-form-item>
        <el-form-item label="名称" required>
          <el-input v-model="pduForm.name" placeholder="请输入 PDU 名称" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="厂商">
              <el-input v-model="pduForm.manufacturer" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="型号">
              <el-input v-model="pduForm.modelNumber" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="输入电压">
              <el-input-number
                v-model="pduForm.inputVoltage"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="额定功率">
              <el-input-number
                v-model="pduForm.ratedPowerW"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="额定电流">
              <el-input-number
                v-model="pduForm.ratedCurrentA"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="状态">
          <el-select v-model="pduForm.status" style="width: 100%">
            <el-option label="启用" value="ACTIVE" />
            <el-option label="停用" value="DISABLED" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="pduForm.remarks" type="textarea" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pduDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="savePDU">保存</el-button>
      </template>
    </el-dialog>

    <!-- 插座编辑 -->
    <el-dialog
      v-model="socketDialog.visible"
      :title="socketDialog.mode === 'create' ? '新增插座' : '编辑插座'"
      width="460px"
      append-to-body
      destroy-on-close
    >
      <el-form label-width="90px">
        <el-form-item label="插座编号" required>
          <el-input-number
            v-model="socketForm.socketNo"
            :min="1"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="插座制式" required>
          <el-select v-model="socketForm.standard" style="width: 100%">
            <el-option label="国标" value="CN" />
            <el-option label="欧标" value="EU" />
          </el-select>
        </el-form-item>
        <el-form-item label="额定电流" required>
          <el-select v-model="socketForm.amperageA" style="width: 100%">
            <el-option :label="'10A'" :value="10" />
            <el-option :label="'16A'" :value="16" />
          </el-select>
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="socketForm.label" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="socketForm.status" style="width: 100%">
            <el-option label="可用" value="AVAILABLE" />
            <el-option label="停用" value="DISABLED" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="socketDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveSocket">保存</el-button>
      </template>
    </el-dialog>

    <!-- 连接设备 -->
    <el-dialog
      v-model="connDialog.visible"
      :title="`连接设备到插座 ${connDialog.socketNo}`"
      width="500px"
      append-to-body
      destroy-on-close
    >
      <el-form label-width="100px">
        <el-form-item label="设备" required>
          <el-select
            v-model="connForm.deviceId"
            filterable
            placeholder="请选择已上架设备"
            style="width: 100%"
          >
            <el-option
              v-for="d in rackedDevices"
              :key="d.id"
              :label="deviceLabel(d)"
              :value="d.id ?? ''"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="供电角色">
          <el-select v-model="connForm.redundancyRole" style="width: 100%">
            <el-option label="主路" value="PRIMARY" />
            <el-option label="冗余路" value="STAND_BY" />
          </el-select>
        </el-form-item>
        <el-form-item label="功耗 W">
          <el-input-number
            v-model="connForm.powerW"
            :min="0"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="回路">
          <el-input v-model="connForm.circuit" placeholder="例如 A 路" />
        </el-form-item>
      </el-form>
      <el-alert
        title="只能选择当前机柜中已经上架的设备，一个插座只能建立一个有效连接。"
        type="info"
        :closable="false"
        show-icon
      />
      <template #footer>
        <el-button @click="connDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="confirmConnect">建立连接</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* v2 606a5166 面板样式逐条对应(浅色,位于容量对话框内) */
.pdu-manager {
  margin-top: 14px;
  padding: 16px 18px;
  border: 1px solid #dbe7ee;
  border-radius: 10px;
  background: #fbfdff;
  box-shadow: 0 5px 18px #2348600d;
}
.pdu-manager-heading,
.pdu-card-head,
.socket-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.pdu-manager-heading small {
  color: #6991a8;
  font-size: 10px;
  letter-spacing: 1.2px;
}
.pdu-manager-heading h3 {
  margin: 3px 0 0;
  color: #19384e;
  font-size: 16px;
}
.pdu-heading-actions,
.pdu-card-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.pdu-empty {
  margin-top: 10px;
  padding: 12px;
  color: #718692;
  background: #f3f8fa;
  border-radius: 6px;
  font-size: 12px;
}
.pdu-card {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid #e5edf2;
  border-radius: 8px;
  background: #fff;
}
.pdu-card-head strong {
  display: block;
  color: #234257;
  font-size: 13px;
}
.pdu-card-head span {
  display: block;
  margin-top: 3px;
  color: #8a9aa5;
  font-size: 11px;
}
.pdu-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin: 10px 0;
  color: #6d8290;
  font-size: 11px;
}
.socket-toolbar {
  margin: 8px 0;
  color: #536b7a;
  font-size: 11px;
  font-weight: 700;
}
.pdu-empty-sockets {
  padding: 10px;
  color: #8a9aa5;
  background: #f6f9fb;
  border-radius: 6px;
  font-size: 12px;
  text-align: center;
}
.pdu-unconnected {
  color: #9aa9b3;
  font-size: 12px;
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
</style>
