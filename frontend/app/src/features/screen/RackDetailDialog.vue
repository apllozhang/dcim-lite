<script setup lang="ts">
/**
 * 机柜详情与容量分析(屏6b;复刻 v2 RackDetailDialog,94vh 大对话框)。
 * 左栏完整 U 位立面图(浅色版,设备块按分类着色,空闲/占用图例,U 从上到下);
 * 右栏三组信息:机柜参数(参数网格)/容量与利用率(指标卡:可计算→百分比,
 * 缺字段→"无法计算"+原因,超限→红色"已超限",不按 0 估算)/PDU 与供电信息
 * (数值网格 + PDU 利用率说明 + PduManager 面板);底部 关闭/编辑/删除。
 */
import { computed } from "vue";
import type { Device } from "@/features/resource/api";
import type { ULayoutResponse } from "@/features/resource/api";
import PduManager from "@/features/screen/PduManager.vue";
import { U_PX, categoryLabel, deviceCategoryClass } from "@/features/screen/screenShared";

type TreeRackLike = {
  id?: string;
  code?: string;
  name?: string;
  type?: string;
  uHeight?: number;
  widthMm?: number;
  depthMm?: number;
  heightMm?: number;
  manufacturer?: string;
  modelNumber?: string;
  serialNumber?: string;
  assetNumber?: string;
  zone?: string;
  rackRow?: string;
  rackColumn?: string;
  aisle?: string;
  xCoordinate?: number | null;
  yCoordinate?: number | null;
  manager?: string;
  department?: string;
  purpose?: string;
  remarks?: string;
  status?: string;
  version?: number;
  templateName?: string;
  templateRevision?: number;
  rotation?: number;
  dualPower?: boolean;
  inputCircuits?: number | null;
  ratedVoltage?: number | null;
  ratedCurrentA?: number | null;
  ratedCurrent?: number | null;
  ratedPowerKw?: number | null;
  peakPowerKw?: number | null;
  pduCount?: number | null;
  loadCapacityKg?: number | null;
  dcName?: string;
};

const props = defineProps<{
  modelValue: boolean;
  rack: TreeRackLike | null;
  layout: ULayoutResponse | undefined;
  layoutLoading: boolean;
  dcName?: string;
  roomName?: string;
}>();
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "edit"): void;
  (e: "remove"): void;
  (e: "reload"): void;
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const devices = computed<ULayoutDeviceRow[]>(() => (props.layout?.devices ?? []) as never);
interface ULayoutDeviceRow {
  id: string;
  code: string;
  name: string;
  startU: number;
  endU: number;
  heightU: number;
  category?: string;
}

const usedU = computed(() => props.layout?.used ?? 0);
const totalU = computed(() => props.rack?.uHeight ?? 0);
const uRate = computed(() => (totalU.value ? Math.round((usedU.value / totalU.value) * 100) : 0));
const ratedPowerSum = computed(() =>
  devices.value.reduce(
    (s, d) => s + (Number((d as unknown as { ratedPowerW?: number }).ratedPowerW) || 0),
    0,
  ),
);
const dualPowered = computed(
  () => devices.value.filter((d) => (d as unknown as { dualPower?: boolean }).dualPower).length,
);

interface Metric {
  label: string;
  value: string;
  rate: number | null;
  state: "ok" | "over" | "unavailable";
  reason?: string;
}
const metrics = computed<Metric[]>(() => {
  const out: Metric[] = [
    {
      label: "U 位利用率",
      value: `${uRate.value}%`,
      rate: uRate.value,
      state: uRate.value > 100 ? "over" : "ok",
    },
  ];
  const ratedW = props.rack?.ratedPowerKw;
  if (ratedW && ratedW > 0) {
    const rate = Math.round((ratedPowerSum.value / (ratedW * 1000)) * 100);
    out.push({
      label: "额定功耗利用率",
      value: `${ratedPowerSum.value}W / ${ratedW}kW`,
      rate,
      state: rate > 100 ? "over" : "ok",
    });
  } else {
    out.push({
      label: "额定功耗利用率",
      value: "无法计算",
      rate: null,
      state: "unavailable",
      reason: "机柜未填写额定功率容量",
    });
  }
  const peakW = props.rack?.peakPowerKw;
  if (peakW && peakW > 0) {
    out.push({
      label: "峰值功耗利用率",
      value: "容量数据完整后自动计算",
      rate: null,
      state: "unavailable",
      reason: "机柜未填写峰值功率容量",
    });
  } else {
    out.push({
      label: "峰值功耗利用率",
      value: "无法计算",
      rate: null,
      state: "unavailable",
      reason: "机柜未填写峰值功率容量",
    });
  }
  const load = props.rack?.loadCapacityKg;
  if (load && load > 0) {
    out.push({
      label: "承重利用率",
      value: "无法计算",
      rate: null,
      state: "unavailable",
      reason: "系统未维护设备重量数据",
    });
  } else {
    out.push({
      label: "承重利用率",
      value: "无法计算",
      rate: null,
      state: "unavailable",
      reason: "机柜未填写承重上限",
    });
  }
  if (props.rack?.pduCount && props.rack.pduCount > 0) {
    out.push({
      label: "PDU 端口利用率",
      value: "无法计算",
      rate: null,
      state: "unavailable",
      reason: "系统未维护 PDU 插座总数和设备实际占用插座数",
    });
  } else {
    out.push({
      label: "PDU 端口利用率",
      value: "无法计算",
      rate: null,
      state: "unavailable",
      reason: "机柜未填写 PDU 数量",
    });
  }
  return out;
});

function uRows(uHeight: number) {
  const rows: { u: number; device?: ULayoutDeviceRow }[] = [];
  for (let u = uHeight; u >= 1; u -= 1) {
    rows.push({
      u,
      device: devices.value.find((d) => u >= d.startU && u <= d.endU),
    });
  }
  return rows;
}
function blockStyle(d: ULayoutDeviceRow, uHeight: number) {
  return {
    top: `${(uHeight - d.endU) * U_PX}px`,
    height: `${Math.max(1, d.endU - d.startU + 1) * U_PX}px`,
  };
}
function deviceTip(d: ULayoutDeviceRow): string {
  return `${d.name} - ${categoryLabel(
    (d as unknown as { category?: string }).category,
  )} - ${d.startU}-${d.endU}U - ${d.heightU}U`;
}
function deviceMeta(d: ULayoutDeviceRow): string {
  const p = (d as unknown as { ratedPowerW?: number }).ratedPowerW;
  return p ? `${p}W` : "";
}
function statusText(s?: string): string {
  return (
    {
      AVAILABLE: "空闲",
      PARTIAL: "部分使用",
      FULL: "已满",
      PLANNING: "规划中",
      MAINTENANCE: "维护中",
      DISABLED: "停用",
    }[s ?? ""] ??
    s ??
    "-"
  );
}
function statusColor(s?: string): string {
  return (
    {
      AVAILABLE: "#24d6a1",
      PARTIAL: "#2dc8f0",
      FULL: "#ffb547",
      PLANNING: "#7188a8",
      MAINTENANCE: "#a78bfa",
      DISABLED: "#ef6470",
    }[s ?? ""] ?? "#7bdfff"
  );
}
const dimension = computed(() =>
  props.rack?.widthMm
    ? `${props.rack.widthMm} × ${props.rack.depthMm} × ${props.rack.heightMm} mm`
    : "-",
);
function dash(v: unknown): string {
  const s = String(v ?? "").trim();
  return s && s !== "null" && s !== "undefined" ? s : "-";
}
</script>

<template>
  <el-dialog
    v-model="visible"
    width="1240px"
    append-to-body
    destroy-on-close
    class="rack-capacity-dialog-host"
  >
    <template #header>
      <div class="dialog-title">
        <div>
          <small>RACK CAPACITY ANALYSIS</small>
          <h2>机柜详情与容量分析</h2>
        </div>
        <span class="status-pill" :style="{ '--status-color': statusColor(rack?.status) }">
          {{ statusText(rack?.status) }}
        </span>
      </div>
    </template>

    <div v-if="rack" class="rack-detail-shell">
      <!-- 左:完整 U 位立面图 -->
      <div class="rack-visual-column">
        <div class="rack-identity">
          <small>{{ dash(rack.code) }}</small>
          <h3>{{ rack.name }}</h3>
          <span>{{ rack.uHeight }}U · {{ usedU }}U 已用 · {{ devices.length }} 台设备</span>
        </div>
        <div class="rack-location">
          <span>当前机房</span>
          <b>{{ dcName ?? "-" }} / {{ roomName ?? "-" }}</b>
        </div>
        <div class="rack-diagram-panel">
          <div class="rack-diagram-toolbar">
            <b>完整 U 位图</b>
            <span>{{ devices.length }} 台设备</span>
          </div>
          <div v-if="layoutLoading" class="rack-diagram-state">
            <div class="loading-ring" />
            正在加载 U 位布局…
          </div>
          <div v-else-if="!layout" class="rack-diagram-state error">
            ⚠ U 位布局读取失败，左侧机柜图暂时无法展示
          </div>
          <div v-else class="rack-frame">
            <div class="rack-frame-top"><i /><i /><i /></div>
            <div class="rack-u-stage" :style="{ height: `${(rack.uHeight ?? 0) * U_PX}px` }">
              <div
                v-for="row in uRows(rack.uHeight ?? 0)"
                :key="`du-${row.u}`"
                class="rack-u-line"
                :class="{ major: row.u % 5 === 0 }"
              >
                <span>{{ row.u }}</span>
                <i />
              </div>
              <div
                v-for="d in devices"
                :key="`dd-${d.id}`"
                class="rack-device-block"
                :class="deviceCategoryClass(d.category)"
                :style="blockStyle(d, rack.uHeight ?? 0)"
                :title="deviceTip(d)"
              >
                <strong>{{ d.name }}</strong>
                <span>{{ d.startU }}-{{ d.endU }}U</span>
                <small>{{ deviceMeta(d) }}</small>
              </div>
            </div>
            <div class="rack-frame-bottom"><i /><i /></div>
          </div>
          <div class="rack-legend">
            <span><i class="empty" />空闲 U 位</span>
            <span><i class="occupied" />设备占用</span>
            <span class="legend-note">U 位从上到下递减</span>
          </div>
        </div>
      </div>

      <!-- 右:参数/利用率/PDU -->
      <div class="rack-information-column">
        <section class="info-section">
          <div class="section-heading">
            <h3>机柜参数</h3>
            <span>独立规格</span>
          </div>
          <div class="parameter-grid">
            <div>
              <span>机柜编码</span><b>{{ dash(rack.code) }}</b>
            </div>
            <div>
              <span>机柜名称</span><b>{{ dash(rack.name) }}</b>
            </div>
            <div>
              <span>类型</span><b>{{ dash(rack.type) }}</b>
            </div>
            <div>
              <span>尺寸（宽×深×高）</span><b>{{ dimension }}</b>
            </div>
            <div>
              <span>厂商 / 型号</span
              ><b>{{ dash(rack.manufacturer) }} / {{ dash(rack.modelNumber) }}</b>
            </div>
            <div>
              <span>序列号 / 资产编号</span
              ><b>{{ dash(rack.serialNumber) }} / {{ dash(rack.assetNumber) }}</b>
            </div>
            <div>
              <span>区域 / 排 / 列</span
              ><b>{{ dash(rack.zone) }} / {{ dash(rack.rackRow) }} / {{ dash(rack.rackColumn) }}</b>
            </div>
            <div>
              <span>通道 / 坐标</span
              ><b
                >{{ dash(rack.aisle) }} / {{ dash(rack.xCoordinate) }},{{
                  dash(rack.yCoordinate)
                }}</b
              >
            </div>
            <div>
              <span>负责人 / 部门</span
              ><b>{{ dash(rack.manager) }} / {{ dash(rack.department) }}</b>
            </div>
            <div>
              <span>用途</span><b>{{ dash(rack.purpose) }}</b>
            </div>
            <div>
              <span>模板版本</span
              ><b>{{
                rack.templateName
                  ? `${rack.templateName} V${rack.templateRevision ?? ""}`
                  : "自定义参数"
              }}</b>
            </div>
            <div>
              <span>旋转角度</span><b>{{ dash(rack.rotation) }}</b>
            </div>
          </div>
        </section>

        <section class="info-section">
          <div class="section-heading">
            <h3>容量与利用率</h3>
            <span>缺少必需字段时不按 0 估算</span>
          </div>
          <div class="metric-grid">
            <div
              v-for="m in metrics"
              :key="m.label"
              class="metric-card"
              :class="{ warning: m.state === 'over', unavailable: m.state === 'unavailable' }"
            >
              <div class="metric-card-head">
                <span>{{ m.label }}</span>
                <b>{{
                  m.state === "over" ? "已超限" : m.state === "unavailable" ? "无法计算" : "可计算"
                }}</b>
              </div>
              <div class="metric-value" :class="{ 'unavailable-value': m.state === 'unavailable' }">
                <strong>{{ m.value }}</strong>
                <span v-if="m.rate !== null">{{ m.rate }}%</span>
              </div>
              <div v-if="m.rate !== null" class="metric-progress">
                <i :style="{ width: `${Math.min(100, m.rate)}%` }" />
              </div>
              <div v-if="m.reason" class="metric-reasons">{{ m.reason }}</div>
            </div>
          </div>
          <div class="calculation-rule">
            已知值：U 位 {{ usedU }}/{{ totalU }}U；额定功耗合计 {{ ratedPowerSum }}W；双路供电设备
            {{ dualPowered }} 台。
          </div>
        </section>

        <section class="info-section">
          <div class="section-heading">
            <h3>PDU 与供电信息</h3>
            <span>容量数据完整后自动计算</span>
          </div>
          <div class="power-grid">
            <div>
              <span>PDU 数量</span><b>{{ dash(rack.pduCount) }} 台</b>
            </div>
            <div>
              <span>输入回路</span><b>{{ dash(rack.inputCircuits) }} 路</b>
            </div>
            <div>
              <span>机柜双路供电</span><b>{{ rack.dualPower ? "是" : "否" }}</b>
            </div>
            <div>
              <span>双路供电设备</span><b>{{ dualPowered }} 台</b>
            </div>
            <div>
              <span>额定电压</span><b>{{ dash(rack.ratedVoltage) }}</b>
            </div>
            <div>
              <span>额定电流</span><b>{{ dash(rack.ratedCurrent ?? rack.ratedCurrentA) }}</b>
            </div>
            <div>
              <span>机柜额定功率</span
              ><b>{{ rack.ratedPowerKw ? `${rack.ratedPowerKw} kW` : "未填写" }}</b>
            </div>
            <div>
              <span>机柜峰值功率</span
              ><b>{{ rack.peakPowerKw ? `${rack.peakPowerKw} kW` : "未填写" }}</b>
            </div>
          </div>
          <div class="pdu-notice">
            <b>PDU 利用率说明</b>
            <p>
              当前模型仅记录 PDU 数量和输入回路，未记录每台 PDU
              的插座总数、已占用插座及实际电流，因此系统会明确显示“无法计算”，不会生成可能误导的百分比。
            </p>
          </div>
          <PduManager v-if="rack.id" :rack-id="rack.id" :devices="devices as unknown as Device[]" />
        </section>

        <section class="info-section">
          <div class="section-heading">
            <h3>备注</h3>
          </div>
          <div class="remarks-section">
            <p>{{ rack.remarks || "暂无备注" }}</p>
          </div>
        </section>
      </div>
    </div>

    <template #footer>
      <div class="dialog-actions">
        <el-button @click="visible = false">关闭</el-button>
        <el-button @click="emit('edit')">编辑机柜</el-button>
        <el-button type="danger" @click="emit('remove')">删除机柜</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
/* v2 d7b54227 对话框样式逐条对应(浅色) */
.dialog-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding-right: 34px;
}
.dialog-title small,
.section-heading small {
  color: #6991a8;
  font-size: 10px;
  letter-spacing: 1.2px;
}
.dialog-title h2 {
  margin: 4px 0 0;
  color: #12324a;
  font-size: 22px;
}
.status-pill {
  align-self: center;
  padding: 6px 13px;
  border: 1px solid color-mix(in srgb, var(--status-color) 70%, transparent);
  border-radius: 999px;
  color: var(--status-color);
  background: color-mix(in srgb, var(--status-color) 12%, white);
  font-size: 12px;
  font-weight: 700;
}
.rack-detail-shell {
  display: grid;
  grid-template-columns: 360px 1fr;
  gap: 22px;
  align-items: start;
}
.rack-visual-column {
  position: sticky;
  top: 0;
}
.rack-identity small {
  display: block;
  color: #6991a8;
  font-size: 11px;
}
.rack-identity h3 {
  margin: 4px 0 6px;
  color: #12324a;
  font-size: 22px;
}
.rack-identity > span {
  color: #6d8290;
  font-size: 12px;
}
.rack-location {
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid #e5edf2;
  border-radius: 8px;
  background: #f6f9fb;
}
.rack-location span {
  display: block;
  color: #8a9aa5;
  font-size: 11px;
}
.rack-location b {
  display: block;
  margin-top: 3px;
  color: #234257;
  font-size: 13px;
}
.rack-diagram-panel {
  margin-top: 14px;
  padding: 14px;
  border: 1px solid #dbe7ee;
  border-radius: 10px;
  background: #fbfdff;
}
.rack-diagram-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.rack-diagram-toolbar b {
  color: #19384e;
  font-size: 13px;
}
.rack-diagram-toolbar span {
  color: #8a9aa5;
  font-size: 11px;
}
.rack-frame {
  padding: 10px 10px 0;
  border: 3px solid #38576b;
  border-radius: 8px;
  background: #0d2135;
}
.rack-frame-top,
.rack-frame-bottom {
  display: flex;
  justify-content: space-between;
  padding: 4px 10px;
}
.rack-frame-top i,
.rack-frame-bottom i {
  width: 26px;
  height: 5px;
  border-radius: 2px;
  background: #2c4a5e;
}
.rack-u-stage {
  position: relative;
  margin: 6px 0;
  overflow: hidden;
  border: 1px solid #315167;
  background: #0a1825;
}
.rack-u-line {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 9px;
  padding-right: 4px;
  border-bottom: 1px solid #1d3a4e;
}
.rack-u-line.major {
  border-bottom-color: #31576f;
}
.rack-u-line span {
  flex: 0 0 20px;
  color: #54778c;
  font-family: Consolas, monospace;
  font-size: 6px;
  line-height: 1;
  text-align: right;
}
.rack-u-line i {
  flex: 1;
  border-top: 1px dashed #16303f;
}
.rack-device-block {
  position: absolute;
  left: 24px;
  right: 5px;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  padding: 0 6px;
  border: 1px solid #0f2c3f;
  border-left: 3px solid currentColor;
  border-radius: 3px;
}
.rack-device-block strong {
  overflow: hidden;
  color: #eaf6ff;
  font-size: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rack-device-block span {
  flex: 0 0 auto;
  color: #b7d9ea;
  font-size: 8px;
}
.rack-device-block small {
  flex: 0 0 auto;
  color: #9dc3d6;
  font-size: 8px;
}
.rack-device-block.category-server {
  color: #d3b400;
  background: #fff7cc;
}
.rack-device-block.category-network {
  color: #4d8a2f;
  background: #e2f4d9;
}
.rack-device-block.category-storage {
  color: #4a5f94;
  background: #dde4f5;
}
.rack-device-block.category-security {
  color: #a86a2c;
  background: #fdeadd;
}
.rack-device-block.category-power-environment {
  color: #a83842;
  background: #fbdde0;
}
.rack-device-block.category-other,
.rack-device-block.category-accessory {
  color: #6b7f5e;
  background: #edf3e6;
}
.rack-legend {
  display: flex;
  gap: 14px;
  margin-top: 12px;
  color: #6d8290;
  font-size: 11px;
}
.rack-legend span {
  display: flex;
  align-items: center;
  gap: 5px;
}
.rack-legend i {
  width: 9px;
  height: 9px;
  border-radius: 2px;
}
.rack-legend i.empty {
  border: 1px solid #9db8c8;
  background: #fff;
}
.rack-legend i.occupied {
  background: #ffd966;
}
.rack-legend .legend-note {
  margin-left: auto;
  color: #9aa9b3;
}
.rack-diagram-state {
  padding: 40px 0;
  color: #6d8290;
  font-size: 12px;
  text-align: center;
}
.rack-diagram-state.error {
  color: #b4555f;
}
.loading-ring {
  width: 20px;
  height: 20px;
  margin: 0 auto 10px;
  border: 2px solid #c9dbe6;
  border-top-color: #2b7ba8;
  border-radius: 50%;
  animation: rack-loading 0.8s linear infinite;
}
@keyframes rack-loading {
  to {
    transform: rotate(360deg);
  }
}
.rack-information-column {
  min-width: 0;
}
.info-section + .info-section {
  margin-top: 18px;
}
.section-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid #e5edf2;
}
.section-heading h3 {
  margin: 0;
  color: #19384e;
  font-size: 15px;
}
.section-heading > span {
  color: #8a9aa5;
  font-size: 11px;
}
.calculation-rule {
  margin-top: 10px;
  color: #8a9aa5;
  font-size: 11px;
}
.parameter-grid,
.power-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 10px 16px;
}
.parameter-grid > div,
.power-grid > div {
  min-width: 0;
}
.parameter-grid span,
.power-grid span {
  display: block;
  color: #8a9aa5;
  font-size: 11px;
}
.parameter-grid b,
.power-grid b {
  display: block;
  margin-top: 2px;
  overflow: hidden;
  color: #234257;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
}
.metric-card {
  padding: 10px 12px;
  border: 1px solid #e5edf2;
  border-radius: 8px;
  background: #fff;
}
.metric-card.warning {
  border-color: #f0c36d;
  background: #fffaf0;
}
.metric-card.unavailable {
  border-style: dashed;
  background: #fafcfd;
}
.metric-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.metric-card-head span {
  color: #6d8290;
  font-size: 11px;
}
.metric-card-head b {
  color: #3d8f5f;
  font-size: 10px;
}
.metric-card.warning .metric-card-head b {
  color: #c07f1d;
}
.metric-card.unavailable .metric-card-head b {
  color: #9aa9b3;
}
.metric-value {
  margin-top: 6px;
}
.metric-value strong {
  color: #19384e;
  font-size: 16px;
}
.metric-value span {
  margin-left: 8px;
  color: #2b7ba8;
  font-size: 13px;
  font-weight: 700;
}
.unavailable-value strong {
  color: #9aa9b3;
  font-size: 13px;
  font-weight: 600;
}
.metric-progress {
  height: 4px;
  margin-top: 8px;
  overflow: hidden;
  border-radius: 4px;
  background: #eef3f6;
}
.metric-progress i {
  display: block;
  height: 100%;
  border-radius: 4px;
  background: linear-gradient(90deg, #24b39a, #2b7ba8);
}
.metric-card.warning .metric-progress i {
  background: linear-gradient(90deg, #f0a13d, #e2634f);
}
.metric-reasons {
  margin-top: 6px;
  color: #9aa9b3;
  font-size: 10px;
  line-height: 1.5;
}
.pdu-notice {
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px dashed #c9dbe6;
  border-radius: 8px;
  background: #f6f9fb;
}
.pdu-notice b {
  color: #536b7a;
  font-size: 12px;
}
.pdu-notice p {
  margin: 4px 0 0;
  color: #8a9aa5;
  font-size: 11px;
  line-height: 1.6;
}
.remarks-section p {
  margin: 0;
  color: #536b7a;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
}
.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
@media (max-width: 1100px) {
  .rack-detail-shell {
    grid-template-columns: 1fr;
  }
  .rack-visual-column {
    position: static;
  }
  .parameter-grid,
  .power-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
