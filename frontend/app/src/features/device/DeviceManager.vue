<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  fetchDevices,
  fetchDeviceTypes,
  createDevice,
  updateDevice,
  deleteDevice,
  fetchDevice,
  createDeviceType,
  updateDeviceType,
  deleteDeviceType,
  fetchULayout,
  fetchRacks,
} from "@/features/resource/api";
import type { Device, DeviceType, Rack, ULayoutDevice } from "@/features/resource/api";
import {
  lifecycleLabel,
  lifecycleTagType,
  LIFECYCLE_STATUS_OPTIONS,
} from "@/features/device/statusLabel";
import { defaultForm } from "@/features/device/deviceFormSchema";
import type { DeviceForm } from "@/features/device/deviceFormSchema";
import DeviceFormFields from "@/features/device/DeviceFormFields.vue";
import { reportError } from "@/api/errors";

/* ── 设备台账 Tab ── */
const deviceRows = ref<Device[]>([]);
const deviceTotal = ref(0);
const devicePage = ref(1);
const devicePageSize = ref(20);
const deviceSearch = ref("");
const deviceStatus = ref("");
const deviceTypeFilter = ref("");
const deviceLoading = ref(false);
const tabActive = ref("devices");

/** 从后端对象读取契约 schema 未声明但实际返回的扩展字段(如 warrantyExpiresAt) */
function ext<T>(obj: unknown, key: string): T | undefined {
  return (obj as Record<string, unknown> | null)?.[key] as T | undefined;
}
/** 错误消息提取(catch 子句统一 unknown) */
function errMsg(e: unknown, fallback: string): string {
  return e instanceof Error ? e.message : fallback;
}

async function loadDevices() {
  deviceLoading.value = true;
  try {
    const pg = await fetchDevices({
      page: devicePage.value,
      pageSize: devicePageSize.value,
      search: deviceSearch.value || undefined,
      lifecycleStatus: deviceStatus.value || undefined,
      typeId: deviceTypeFilter.value || undefined,
    });
    deviceRows.value = pg.items;
    deviceTotal.value = pg.total;
  } catch (e) {
    ElMessage.error("设备列表加载失败");
    reportError({ message: `devices load: ${e instanceof Error ? e.message : String(e)}` });
  } finally {
    deviceLoading.value = false;
  }
}

const deviceRacked = computed(
  () =>
    deviceRows.value.filter(
      (d) => d.lifecycleStatus === "RUNNING" || d.lifecycleStatus === "MAINTENANCE",
    ).length,
);
const deviceUnracked = computed(
  () =>
    deviceRows.value.filter(
      (d) => d.lifecycleStatus === "WAITING_RACK" || d.lifecycleStatus === "OFF_RACK",
    ).length,
);

function searchDevices() {
  devicePage.value = 1;
  loadDevices();
}
onMounted(() => {
  loadDevices();
  loadTypes();
});

/* ── 设备新增/编辑对话框 ── */
const deviceDialog = reactive({
  visible: false,
  editing: false,
  id: "",
  version: 1,
  loading: false,
  positioned: false,
});
const deviceForm = ref<DeviceForm>(defaultForm());
const deviceAutoCode = ref(true);

function openDeviceCreate() {
  Object.assign(deviceDialog, {
    visible: true,
    editing: false,
    id: "",
    version: 1,
    positioned: false,
  });
  deviceForm.value = defaultForm();
  deviceAutoCode.value = true;
}
async function openDeviceEdit(dev: Device) {
  deviceDialog.loading = true;
  Object.assign(deviceDialog, {
    visible: true,
    editing: true,
    id: dev.id ?? "",
    version: dev.version ?? 1,
    positioned: !!dev.currentPosition,
  });
  try {
    const full = await fetchDevice(dev.id ?? "");
    deviceForm.value = {
      typeId: full.typeId ?? "",
      code: full.code ?? "",
      name: full.name ?? "",
      lifecycleStatus: full.lifecycleStatus ?? "WAITING_RACK",
      assetNumber: full.assetNumber ?? "",
      serialNumber: full.serialNumber ?? "",
      manufacturer: full.manufacturer ?? "",
      modelNumber: full.modelNumber ?? "",
      specification: full.specification ?? "",
      firmwareVersion: full.firmwareVersion ?? "",
      purchaseBatch: full.purchaseBatch ?? "",
      warrantyExpiresAt: ext<string>(full, "warrantyExpiresAt") ?? "",
      heightU: full.heightU ?? 1,
      widthMm: ext<number>(full, "widthMm"),
      depthMm: ext<number>(full, "depthMm"),
      heightMm: ext<number>(full, "heightMm"),
      weightKg: ext<number>(full, "weightKg"),
      ratedPowerW: ext<number>(full, "ratedPowerW"),
      peakPowerW: ext<number>(full, "peakPowerW"),
      inputVoltage: ext<number>(full, "inputVoltage"),
      dualPowerRequired: full.dualPowerRequired ?? false,
      organization: full.organization ?? "",
      manager: full.manager ?? "",
      contact: full.contact ?? "",
      businessSystem: full.businessSystem ?? "",
      applicationName: full.applicationName ?? "",
      managementIp: full.managementIp ?? "",
      businessIp: full.businessIp ?? "",
      macAddress: full.macAddress ?? "",
      managementProtocol: full.managementProtocol ?? "",
      monitoringStatus: full.monitoringStatus ?? "",
      externalQrCode: ext<string>(full, "externalQrCode") ?? "",
      externalQrCodeUrl: ext<string>(full, "externalQrCodeUrl") ?? "",
      tags: full.tags ?? "",
      remarks: full.remarks ?? "",
    };
  } catch {
    ElMessage.error("获取设备详情失败");
    deviceDialog.visible = false;
  } finally {
    deviceDialog.loading = false;
  }
  deviceAutoCode.value = false;
}
async function submitDevice() {
  const f = deviceForm.value;
  if (!f.typeId) {
    ElMessage.warning("请选择设备类型");
    return;
  }
  if (!deviceAutoCode.value && !f.code) {
    ElMessage.warning("请填写设备编码");
    return;
  }
  if (!f.name) {
    ElMessage.warning("请填写设备名称");
    return;
  }
  if (!Number.isInteger(f.heightU) || f.heightU < 1) {
    ElMessage.warning("U 高度必须是 1 到 100 的整数");
    return;
  }
  deviceDialog.loading = true;
  try {
    const body: Record<string, unknown> = { ...f };
    if (deviceAutoCode.value && !deviceDialog.editing) body.code = undefined;
    body.lifecycleStatus = undefined;
    if (deviceDialog.editing) {
      // PUT: status field needs special handling (backend preserves if not in body)
      await updateDevice(deviceDialog.id, deviceDialog.version, {
        ...body,
        lifecycleStatus: undefined,
      });
    } else {
      await createDevice(body);
    }
    deviceDialog.visible = false;
    ElMessage.success(deviceDialog.editing ? "设备已保存" : "设备已创建");
    loadDevices();
  } catch (e) {
    ElMessage.error(errMsg(e, "操作失败"));
  } finally {
    deviceDialog.loading = false;
  }
}
async function removeDevice(dev: Device) {
  try {
    await ElMessageBox.confirm(`确认删除设备「${dev.name}」？`, "删除确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deleteDevice(dev.id ?? "", dev.version ?? 1);
    ElMessage.success("已删除");
    loadDevices();
  } catch (e) {
    ElMessage.error(errMsg(e, "删除失败"));
  }
}

/* ── 设备类型 Tab ── */
const types = ref<DeviceType[]>([]);
const typeLoading = ref(false);
const typeDialog = reactive({ visible: false, editing: false, id: "", version: 1 });
const typeForm = reactive({ code: "", name: "", category: "SERVER", defaultHeightU: 1 });

async function loadTypes() {
  typeLoading.value = true;
  try {
    types.value = await fetchDeviceTypes();
  } catch {
    types.value = [];
  } finally {
    typeLoading.value = false;
  }
}
function openTypeCreate() {
  Object.assign(typeDialog, { visible: true, editing: false, id: "", version: 1 });
  Object.assign(typeForm, { code: "", name: "", category: "SERVER", defaultHeightU: 1 });
}
function openTypeEdit(t: DeviceType) {
  Object.assign(typeDialog, {
    visible: true,
    editing: true,
    id: t.id ?? "",
    version: t.version ?? 1,
  });
  Object.assign(typeForm, {
    code: t.code ?? "",
    name: t.name ?? "",
    category: t.category ?? "SERVER",
    defaultHeightU: t.defaultHeightU ?? 1,
  });
}
async function submitType() {
  if (!typeForm.code || !typeForm.name) {
    ElMessage.warning("编码和名称不能为空");
    return;
  }
  try {
    const body = { ...typeForm };
    if (typeDialog.editing) await updateDeviceType(typeDialog.id, typeDialog.version, body);
    else await createDeviceType(body);
    typeDialog.visible = false;
    ElMessage.success(typeDialog.editing ? "设备类型已保存" : "设备类型已创建");
    loadTypes();
  } catch (e) {
    ElMessage.error(errMsg(e, "操作失败"));
  }
}
async function removeType(t: DeviceType) {
  try {
    await ElMessageBox.confirm(`确认删除设备类型「${t.name}」？`, "删除确认", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deleteDeviceType(t.id ?? "", t.version ?? 1);
    ElMessage.success("已删除");
    loadTypes();
  } catch (e) {
    ElMessage.error(errMsg(e, "删除失败"));
  }
}

/* ── 机柜U位 Tab ── */
const racks = ref<Rack[]>([]);
const selectedRackId = ref("");
const uLayout = ref<{
  uHeight: number;
  used: number;
  free: number;
  devices: ULayoutDevice[];
} | null>(null);
const uLayoutLoading = ref(false);

async function loadRacks() {
  try {
    const pg = await fetchRacks({ page: 1, pageSize: 200 });
    racks.value = pg.items;
    if (!selectedRackId.value && racks.value.length) selectedRackId.value = racks.value[0].id ?? "";
  } catch {
    racks.value = [];
  }
}
watch(selectedRackId, async (id) => {
  if (!id) return;
  uLayoutLoading.value = true;
  try {
    uLayout.value = await fetchULayout(id);
  } catch {
    uLayout.value = null;
  } finally {
    uLayoutLoading.value = false;
  }
});
watch(tabActive, (val) => {
  if (val === "u-layout" && racks.value.length === 0) loadRacks();
});

function uRows(
  uHeight: number,
  devices: ULayoutDevice[],
): { u: number; label?: string; color?: string; deviceId?: string }[] {
  const map = new Map<number, { label: string; color: string; deviceId: string; span: number }>();
  for (const d of devices) {
    map.set(d.startU, {
      label: `${d.name}(${d.code})`,
      color: d.color || "#409eff",
      deviceId: d.id,
      span: d.endU - d.startU + 1,
    });
  }
  const rows: { u: number; label?: string; color?: string; deviceId?: string }[] = [];
  for (let u = uHeight; u >= 1; u--) {
    const d = map.get(u);
    rows.push({ u, ...(d ? d : {}) });
    if (d?.span && d.span > 1) {
      for (let i = 1; i < d.span; i++) {
        u--;
        rows.push({ u, label: "", color: d.color });
      }
    }
  }
  return rows;
}

const CAT_CATEGORIES = [
  "SERVER",
  "NETWORK",
  "STORAGE",
  "SECURITY",
  "POWER_ENVIRONMENT",
  "ACCESSORY",
  "OTHER",
];
</script>

<template>
  <div class="device-page" data-test="device-manager">
    <div class="page-header">
      <div>
        <h2>设备与 U 位管理</h2>
        <p>维护设备台账、统一 U 位占用和设备履历。</p>
      </div>
      <div class="actions" data-test="device-header-actions">
        <el-button @click="loadDevices()">刷新</el-button>
        <el-button data-test="batch-import-btn">批量导入</el-button>
        <el-button>导出设备信息</el-button>
        <el-button type="primary" data-test="device-create-btn" @click="openDeviceCreate">
          新增设备
        </el-button>
      </div>
    </div>

    <el-tabs v-model="tabActive" data-test="device-tabs">
      <!-- ──────── Tab1: 设备台账 ──────── -->
      <el-tab-pane label="设备台账" name="devices">
        <el-row :gutter="16" class="summary-row">
          <el-col :xs="24" :sm="6">
            <div class="stat-box">
              <div class="summary">
                <span>设备总数</span><strong>{{ deviceTotal }}</strong
                ><small>/ 3000</small>
              </div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="6">
            <div class="stat-box">
              <div class="summary">
                <span>当前页已上架</span><strong>{{ deviceRacked }}</strong>
              </div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="6">
            <div class="stat-box">
              <div class="summary">
                <span>设备类型</span><strong>{{ types.length }}</strong>
              </div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="6">
            <div class="stat-box">
              <div class="summary">
                <span>未上架设备</span><strong>{{ deviceUnracked }}</strong>
              </div>
            </div>
          </el-col>
        </el-row>

        <div class="toolbar" data-test="device-toolbar">
          <el-input
            v-model="deviceSearch"
            placeholder="名称、编码、资产号、序列号或 IP"
            clearable
            style="width: 280px"
            data-test="device-search"
            @change="searchDevices"
          />
          <el-select
            v-model="deviceTypeFilter"
            placeholder="设备类型"
            clearable
            style="width: 170px"
            data-test="device-type-filter"
            @change="searchDevices"
          >
            <el-option v-for="t in types" :key="t.id" :label="t.name ?? t.code" :value="t.id" />
          </el-select>
          <el-select
            v-model="deviceStatus"
            placeholder="生命周期"
            clearable
            style="width: 140px"
            data-test="device-status-filter"
            @change="searchDevices"
          >
            <el-option
              v-for="o in LIFECYCLE_STATUS_OPTIONS"
              :key="o.value"
              :label="o.label"
              :value="o.value"
            />
          </el-select>
          <el-button type="primary" plain data-test="device-search-btn" @click="searchDevices">
            查询
          </el-button>
          <el-button
            @click="
              deviceSearch = '';
              deviceStatus = '';
              deviceTypeFilter = '';
              searchDevices();
            "
          >
            重置
          </el-button>
        </div>

        <el-table
          v-loading="deviceLoading"
          :data="deviceRows"
          size="default"
          data-test="device-table"
        >
          <el-table-column prop="code" label="编码" min-width="130" show-overflow-tooltip />
          <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
          <el-table-column label="状态" width="95">
            <template #default="{ row }">
              <el-tag
                :type="lifecycleTagType(row.lifecycleStatus)"
                size="small"
                data-test="device-status-tag"
              >
                {{ lifecycleLabel(row.lifecycleStatus) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="高度" width="65" align="right">
            <template #default="{ row }">{{ row.heightU }}U</template>
          </el-table-column>
          <el-table-column label="位置" min-width="170">
            <template #default="{ row }">
              <span v-if="row.currentPosition" data-test="device-position" class="tabular-nums">
                {{ row.currentPosition.rack?.code ?? "" }} · U{{ row.currentPosition.startU }}-{{
                  row.currentPosition.endU
                }}
              </span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button link size="small" type="primary" @click="openDeviceEdit(row)">
                编辑
              </el-button>
              <el-button link size="small" type="danger" @click="removeDevice(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination">
          <el-pagination
            v-model:current-page="devicePage"
            v-model:page-size="devicePageSize"
            :total="deviceTotal"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            @current-change="loadDevices"
            @size-change="loadDevices"
          />
        </div>
      </el-tab-pane>

      <!-- ──────── Tab2: 设备类型 ──────── -->
      <el-tab-pane label="设备类型" name="types">
        <div class="tab-action">
          <el-button type="primary" data-test="type-create-btn" @click="openTypeCreate">
            新增设备类型
          </el-button>
        </div>
        <el-table v-loading="typeLoading" :data="types" size="default" data-test="type-table">
          <el-table-column prop="code" label="编码" min-width="120" />
          <el-table-column prop="name" label="名称" min-width="140" />
          <el-table-column prop="category" label="分类" width="110">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ row.category ?? "SERVER" }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="defaultHeightU" label="默认U高" width="90" align="right">
            <template #default="{ row }">{{ row.defaultHeightU ?? 1 }}U</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button link size="small" type="primary" @click="openTypeEdit(row)">
                编辑
              </el-button>
              <el-button link size="small" type="danger" @click="removeType(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <!-- 设备类型对话框 -->
        <el-dialog
          v-model="typeDialog.visible"
          :title="typeDialog.editing ? '编辑设备类型' : '新增设备类型'"
          width="420px"
        >
          <el-form label-width="100px">
            <el-form-item label="编码" required>
              <el-input v-model="typeForm.code" maxlength="50" />
            </el-form-item>
            <el-form-item label="名称" required>
              <el-input v-model="typeForm.name" maxlength="150" />
            </el-form-item>
            <el-form-item label="分类">
              <el-select v-model="typeForm.category" style="width: 100%">
                <el-option v-for="c in CAT_CATEGORIES" :key="c" :label="c" :value="c" />
              </el-select>
            </el-form-item>
            <el-form-item label="默认U高">
              <el-input-number
                v-model="typeForm.defaultHeightU"
                :min="1"
                :max="100"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="typeDialog.visible = false">取消</el-button
            ><el-button type="primary" @click="submitType">保存</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ──────── Tab3: 机柜U位 ──────── -->
      <el-tab-pane label="机柜 U 位" name="u-layout">
        <div class="toolbar">
          <el-select
            v-model="selectedRackId"
            placeholder="选择机柜"
            filterable
            style="width: 300px"
          >
            <el-option
              v-for="r in racks"
              :key="r.id"
              :label="`${r.name ?? r.code}（${r.code} :: ${r.uHeight}U）`"
              :value="r.id ?? ''"
            />
          </el-select>
        </div>
        <div v-if="uLayout" v-loading="uLayoutLoading" class="rack-u-view">
          <div class="rack-title">
            <div>
              <strong>U 位视图</strong
              ><span>
                · {{ uLayout.used }}U 已用 / {{ uLayout.free }}U 空闲 / {{ uLayout.uHeight }}U
                总高</span
              >
            </div>
          </div>
          <div class="u-panel">
            <header><strong>设备</strong><span>U 位</span></header>
            <div class="u-grid">
              <div
                v-for="row in uRows(uLayout.uHeight, uLayout.devices)"
                :key="row.u"
                class="u-row"
              >
                <div class="u-number">{{ row.u }}</div>
                <div
                  class="u-cell"
                  :class="{
                    occupied: !!row.label,
                    start:
                      row.label &&
                      row.u === uLayout.devices.find((d) => d.startU === row.u)?.startU,
                  }"
                  :style="row.color ? { '--device-color': row.color } : {}"
                >
                  <strong v-if="row.label && row.label.length > 0">{{ row.label }}</strong>
                  <small v-if="row.deviceId"
                    >U{{ uLayout.devices.find((d) => d.id === row.deviceId)?.startU }}-{{
                      uLayout.devices.find((d) => d.id === row.deviceId)?.endU
                    }}</small
                  >
                </div>
              </div>
            </div>
          </div>
          <div class="legend">
            <span><i class="empty-dot" /> 空闲</span><span><i class="device-dot" /> 已占用</span>
          </div>
        </div>
        <el-empty v-else description="请选择机柜查看 U 位占用" />
      </el-tab-pane>
    </el-tabs>

    <!-- 设备新增/编辑对话框 -->
    <el-dialog
      v-model="deviceDialog.visible"
      :title="deviceDialog.editing ? '编辑设备' : '新增设备'"
      width="720px"
      @open="deviceDialog.loading = false"
    >
      <DeviceFormFields
        :form="deviceForm"
        :types="types"
        :mode="deviceDialog.editing ? 'edit' : 'create'"
        :loading="deviceDialog.loading"
        :positioned="deviceDialog.positioned"
        :auto-code="deviceAutoCode"
        @update:form="deviceForm = $event"
        @update:auto-code="deviceAutoCode = $event"
      />
      <template #footer>
        <el-button @click="deviceDialog.visible = false">取消</el-button
        ><el-button type="primary" @click="submitDevice">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* 复刻 v2 DeviceManagementView-DQc6kKpV.css(去 data-v 哈希) */
.device-page {
  max-width: 1560px;
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
.actions,
.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
}
.summary-row {
  margin-bottom: 16px;
}
.stat-box {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 14px 18px;
}
.summary {
  display: flex;
  align-items: baseline;
  gap: 10px;
}
.summary span {
  color: #6b7280;
}
.summary strong {
  font-size: 28px;
  color: #111827;
}
.summary small {
  margin-left: auto;
  color: #9ca3af;
}
.toolbar {
  margin-bottom: 16px;
}
.tab-action {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.muted {
  color: #9ca3af;
}

/* U 位视图 */
.rack-u-view {
  min-width: 720px;
}
.rack-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}
.rack-title span {
  color: #6b7280;
  font-size: 13px;
}
.u-panel {
  border: 1px solid #dfe4ea;
  border-radius: 10px;
  overflow: hidden;
  background: #fff;
}
.u-panel header {
  display: flex;
  justify-content: space-between;
  padding: 10px 12px;
  background: #172033;
  color: #fff;
}
.u-panel header span {
  color: #cbd5e1;
  font-size: 12px;
}
.u-grid {
  padding: 8px;
  background: #f3f4f6;
}
.u-row {
  min-height: 26px;
  display: grid;
  grid-template-columns: 42px 1fr;
  align-items: stretch;
}
.u-number {
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
  font-size: 11px;
  border-bottom: 1px solid #d1d5db;
}
.u-cell {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 26px;
  padding: 2px 8px;
  border: 1px solid #d7dce2;
  border-top: 0;
  background: #fff;
  overflow: hidden;
}
.u-row:first-child .u-cell {
  border-top: 1px solid #d7dce2;
}
.u-cell.occupied {
  border-color: color-mix(in srgb, var(--device-color, #409eff) 76%, #172033);
  background: color-mix(in srgb, var(--device-color, #409eff) 86%, white);
}
.u-cell strong {
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.u-cell small {
  margin-left: auto;
  white-space: nowrap;
  opacity: 0.9;
}
.legend {
  display: flex;
  gap: 18px;
  align-items: center;
  margin-top: 12px;
  color: #6b7280;
  font-size: 12px;
}
.legend span {
  display: flex;
  align-items: center;
  gap: 6px;
}
.legend i {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  display: inline-block;
  border: 1px solid #d1d5db;
}
.empty-dot {
  background: #fff;
}
.device-dot {
  background: #409eff;
  border-color: #409eff !important;
}
</style>
