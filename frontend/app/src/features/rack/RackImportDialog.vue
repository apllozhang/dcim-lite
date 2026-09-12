<script setup lang="ts">
/**
 * 机柜批量导入(复刻 v2 BatchImportDialog 的流程与文案)。
 * 与 v2 的差异:模板/结果文件为 CSV(Excel 可直接打开),解析在校验逻辑同 v2 口径:
 * 数据中心/机房存在且未停用归档、编码唯一(文件内 + 库内)、模板启用且有版本、
 * 状态枚举、数值范围、旋转角度 0/90/180/270、双路电源布尔解析。
 * dcim-lite 后端无 autoGenerateCode:留空编码在提交时由前端生成(前缀 RACK)。
 */
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { createRack, type RackTemplate, type TreeDataCenter } from "@/features/resource/api";
import {
  RACK_STATUS_OPTIONS,
  activeTemplateOptions,
  autoCode,
  rackErrMsg,
  rackFormDefaults,
  formToRackPayload,
  type RackForm,
  type TreeRackRow,
} from "@/features/rack/rackShared";

const props = defineProps<{
  modelValue: boolean;
  dataCenters: TreeDataCenter[];
  templates: RackTemplate[];
  existingRacks: TreeRackRow[];
}>();
const emit = defineEmits<{ (e: "update:modelValue", v: boolean): void; (e: "imported"): void }>();
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

/* ── 导入字段规格(v2 Ie 表逐条对应) ── */
interface ImportField {
  key: string;
  title: string;
  required: boolean;
  example: string;
}
const FIELDS: ImportField[] = [
  { key: "dataCenterCode", title: "数据中心编码", required: true, example: "DC-BJ-01" },
  { key: "roomCode", title: "机房编码", required: true, example: "ROOM-A01" },
  { key: "code", title: "机柜编码", required: false, example: "RACK-A-01" },
  { key: "name", title: "机柜名称", required: true, example: "A区01号机柜" },
  { key: "templateCode", title: "机柜模板编码", required: false, example: "RACK-TPL-42U" },
  { key: "status", title: "状态", required: false, example: "AVAILABLE" },
  { key: "type", title: "机柜类型", required: false, example: "STANDARD" },
  { key: "uHeight", title: "U位高度", required: false, example: "42" },
  { key: "widthMm", title: "宽度mm", required: false, example: "600" },
  { key: "depthMm", title: "深度mm", required: false, example: "1200" },
  { key: "heightMm", title: "高度mm", required: false, example: "2000" },
  { key: "manufacturer", title: "厂商", required: false, example: "示例厂商" },
  { key: "modelNumber", title: "型号", required: false, example: "CAB-42U" },
  { key: "serialNumber", title: "序列号", required: false, example: "SN20260831001" },
  { key: "assetNumber", title: "资产编号", required: false, example: "ASSET-RACK-001" },
  { key: "loadCapacityKg", title: "承重kg", required: false, example: "1000" },
  { key: "zone", title: "区域", required: false, example: "A" },
  { key: "row", title: "排", required: false, example: "01" },
  { key: "column", title: "列", required: false, example: "01" },
  { key: "aisle", title: "通道", required: false, example: "冷通道1" },
  { key: "xCoordinate", title: "X坐标", required: false, example: "100" },
  { key: "yCoordinate", title: "Y坐标", required: false, example: "100" },
  { key: "rotation", title: "旋转角度", required: false, example: "0" },
  { key: "manager", title: "负责人", required: false, example: "张三" },
  { key: "department", title: "所属部门", required: false, example: "基础设施部" },
  { key: "purpose", title: "用途", required: false, example: "核心业务" },
  { key: "dualPower", title: "双路电源", required: false, example: "是" },
  { key: "inputCircuits", title: "输入路数", required: false, example: "2" },
  { key: "ratedVoltage", title: "额定电压V", required: false, example: "220" },
  { key: "ratedCurrent", title: "额定电流A", required: false, example: "32" },
  { key: "ratedPowerKw", title: "额定功率kW", required: false, example: "7.04" },
  { key: "peakPowerKw", title: "峰值功率kW", required: false, example: "8" },
  { key: "pduCount", title: "PDU数量", required: false, example: "2" },
  { key: "sortOrder", title: "排序值", required: false, example: "10" },
  { key: "remarks", title: "备注", required: false, example: "批量导入示例" },
];

interface ImportRow {
  rowNumber: number;
  summary: string;
  form: RackForm | null;
  roomId: string;
  errors: string[];
}

const rows = ref<ImportRow[]>([]);
const importing = ref(false);
const importedCount = ref(0);
const fileInput = ref<HTMLInputElement | null>(null);

const validRows = computed(() => rows.value.filter((r) => r.errors.length === 0));
const errorCount = computed(() => rows.value.length - validRows.value.length);
const done = computed(() => importedCount.value > 0);

function isActive(s: string | null | undefined) {
  return s !== "DISABLED" && s !== "ARCHIVED";
}
const norm = (s: unknown) =>
  String(s ?? "")
    .trim()
    .toUpperCase();

/* ── CSV 模板生成(表头带必填/可选标识,与 v2 模板字段说明同文案) ── */
function downloadTemplate() {
  const header = FIELDS.map((f) => (f.required ? `${f.title}（必填）` : `${f.title}（可选）`));
  const desc = FIELDS.map((f) => f.title);
  const example = FIELDS.map((f) => f.example);
  const guide = [
    "请只在第一张工作表/本列结构中填写待导入数据。",
    "枚举字段按英文值填写;布尔值可填写:是/否、TRUE/FALSE、1/0。",
    "机柜编码可留空:导入提交时由系统自动生成;手工填写时校验唯一性。",
  ];
  const csv = [header, desc, example, [], guide]
    .map((line) => (Array.isArray(line) ? line.join(",") : line))
    .map((l) => l.replace(/,/g, "，"))
    .join("\r\n");
  saveFile("\uFEFF" + csv, "机柜批量导入模板.csv", "text/csv");
}

function saveFile(content: string, name: string, mime: string) {
  const blob = new Blob([content], { type: `${mime};charset=utf-8` });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

/* ── CSV 解析(支持引号包裹与转义) ── */
function parseCsv(text: string): string[][] {
  const out: string[][] = [];
  let cur: string[] = [];
  let cell = "";
  let inQuote = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (inQuote) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          cell += '"';
          i++;
        } else inQuote = false;
      } else cell += ch;
    } else if (ch === '"') {
      inQuote = true;
    } else if (ch === ",") {
      cur.push(cell);
      cell = "";
    } else if (ch === "\n" || ch === "\r") {
      if (ch === "\r" && text[i + 1] === "\n") i++;
      cur.push(cell);
      cell = "";
      if (cur.some((c) => c.trim() !== "")) out.push(cur);
      cur = [];
    } else cell += ch;
  }
  cur.push(cell);
  if (cur.some((c) => c.trim() !== "")) out.push(cur);
  return out;
}

async function onFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  importedCount.value = 0;
  try {
    const text = await file.text();
    const grid = parseCsv(text.replace(/^\uFEFF/, ""));
    if (grid.length < 2) throw new Error("Excel 内容为空");
    // 表头匹配:标题 / 标题（必填）/ 标题（可选）/ key(v2 ce 同口径)
    const header = grid[0].map(norm);
    const colOf = new Map<string, number>();
    FIELDS.forEach((f) => {
      const candidates = [
        norm(f.title),
        norm(`${f.title}（必填）`),
        norm(`${f.title}（可选）`),
        norm(f.key),
      ];
      const idx = header.findIndex((h) => candidates.includes(h));
      if (idx >= 0) colOf.set(f.key, idx);
    });
    const dcByCode = new Map(props.dataCenters.map((dc) => [norm(dc.code), dc] as const));
    const tplByCode = new Map(
      activeTemplateOptions(props.templates).map((o) => [norm(o.template.code), o] as const),
    );
    const usedCodes = new Set(props.existingRacks.map((r) => norm(r.code)));
    const fileCodes = new Map<string, number>();

    rows.value = grid
      .slice(1)
      .map((cells, i) => {
        const get = (key: string) => {
          const idx = colOf.get(key);
          return idx === undefined ? "" : String(cells[idx] ?? "").trim();
        };
        // 全空行跳过(v2:所有映射字段均空白的行不进入校验)
        if (!FIELDS.some((f) => get(f.key) !== "")) return null;
        const errors: string[] = [];
        const base = rackFormDefaults();
        const dcCode = norm(get("dataCenterCode"));
        const roomCode = norm(get("roomCode"));
        const code = get("code");
        const name = get("name");
        const dc = dcByCode.get(dcCode);
        const room = dc?.rooms?.find((r) => norm(r.code) === roomCode);
        if (!dcCode) errors.push("数据中心编码不能为空");
        else if (!dc) errors.push(`数据中心编码 ${dcCode} 不存在`);
        else if (!isActive(dc.status)) errors.push("所属数据中心已停用或归档");
        if (!roomCode) errors.push("机房编码不能为空");
        else if (dc && !room) errors.push(`机房编码 ${roomCode} 不属于数据中心 ${dcCode}`);
        else if (room && !isActive(room.status)) errors.push("所属机房已停用或归档");
        if (code) {
          if (code.length > 50) errors.push("机柜编码不能超过 50 个字符");
          const n = norm(code);
          fileCodes.set(n, (fileCodes.get(n) ?? 0) + 1);
          if ((fileCodes.get(n) ?? 0) > 1) errors.push("Excel 内机柜编码重复");
          if (usedCodes.has(n)) errors.push("机柜编码已存在");
        }
        if (!name) errors.push("机柜名称不能为空");
        else if (name.length > 150) errors.push("机柜名称不能超过 150 个字符");
        const tplCode = norm(get("templateCode"));
        const tpl = tplCode ? tplByCode.get(tplCode) : undefined;
        if (tplCode && !tpl) errors.push(`启用的机柜模板编码 ${tplCode} 不存在`);
        if (tpl) {
          base.templateId = tpl.template.id;
          if (tpl.version) {
            base.uHeight = tpl.version.uHeight ?? base.uHeight;
            base.widthMm = tpl.version.widthMm ?? base.widthMm;
            base.depthMm = tpl.version.depthMm ?? base.depthMm;
            base.heightMm = tpl.version.heightMm ?? base.heightMm;
            base.dualPower = tpl.version.dualPower ?? false;
            base.inputCircuits = tpl.version.inputCircuits ?? 0;
            base.pduCount = tpl.version.pduCount ?? 0;
          }
        }
        const status = norm(get("status")) || base.status;
        if (!RACK_STATUS_OPTIONS.some((o) => o.value === status)) {
          errors.push(`状态 ${status} 无效`);
        }
        base.status = status;
        base.code = code;
        base.name = name;
        base.type = get("type") || base.type;
        const num = (
          key: string,
          label: string,
          opt: { min: number; exclusive?: boolean } = { min: 0 },
        ) => {
          const raw = get(key);
          if (raw === "") return undefined;
          const v = Number(raw);
          if (Number.isNaN(v) || (opt.exclusive ? v <= opt.min : v < opt.min)) {
            errors.push(`${label}数值无效`);
            return undefined;
          }
          return v;
        };
        const uHeight = num("uHeight", "U位高度", { min: 1 });
        if (uHeight !== undefined && (uHeight < 1 || uHeight > 100 || !Number.isInteger(uHeight)))
          errors.push("U位高度必须是整数且不能大于 100");
        else if (uHeight !== undefined) base.uHeight = uHeight;
        const w = num("widthMm", "宽度mm", { min: 0, exclusive: true });
        if (w !== undefined) base.widthMm = w;
        const d = num("depthMm", "深度mm", { min: 0, exclusive: true });
        if (d !== undefined) base.depthMm = d;
        const h = num("heightMm", "高度mm", { min: 0, exclusive: true });
        if (h !== undefined) base.heightMm = h;
        const load = num("loadCapacityKg", "承重kg");
        if (load !== undefined) base.loadCapacityKg = load;
        const x = num("xCoordinate", "X坐标");
        if (x !== undefined) base.xCoordinate = x;
        const y = num("yCoordinate", "Y坐标");
        if (y !== undefined) base.yCoordinate = y;
        const rotation = get("rotation");
        if (rotation !== "") {
          const rv = Number(rotation);
          if (Number.isNaN(rv) || ![0, 90, 180, 270].includes(rv)) {
            errors.push("旋转角度仅支持 0、90、180、270");
          } else base.rotation = rv;
        }
        const dp = get("dualPower");
        if (dp !== "") {
          const up = dp.toUpperCase();
          if (["是", "TRUE", "1"].includes(up)) base.dualPower = true;
          else if (["否", "FALSE", "0"].includes(up)) base.dualPower = false;
          else errors.push("双路电源只能填写是/否、TRUE/FALSE、1/0");
        }
        const int = (key: string, label: string) => {
          const raw = get(key);
          if (raw === "") return undefined;
          const v = Number(raw);
          if (Number.isNaN(v) || v < 0 || !Number.isInteger(v)) {
            errors.push(`${label}必须是不小于 0 的整数`);
            return undefined;
          }
          return v;
        };
        const ic = int("inputCircuits", "输入路数");
        if (ic !== undefined) base.inputCircuits = ic;
        const rv2 = num("ratedVoltage", "额定电压V");
        if (rv2 !== undefined) base.ratedVoltage = rv2;
        const rc = num("ratedCurrent", "额定电流A");
        if (rc !== undefined) base.ratedCurrent = rc;
        const rp = num("ratedPowerKw", "额定功率kW");
        if (rp !== undefined) base.ratedPowerKw = rp;
        const pp = num("peakPowerKw", "峰值功率kW");
        if (pp !== undefined) base.peakPowerKw = pp;
        const pc = int("pduCount", "PDU数量");
        if (pc !== undefined) base.pduCount = pc;
        const so = int("sortOrder", "排序值");
        if (so !== undefined) base.sortOrder = so;
        base.manufacturer = get("manufacturer");
        base.modelNumber = get("modelNumber");
        base.serialNumber = get("serialNumber");
        base.assetNumber = get("assetNumber");
        base.zone = get("zone");
        base.row = get("row");
        base.column = get("column");
        base.aisle = get("aisle");
        base.manager = get("manager");
        base.department = get("department");
        base.purpose = get("purpose");
        base.remarks = get("remarks");
        return {
          rowNumber: i + 2,
          summary: code
            ? `${code} · ${name || "未填写名称"}`
            : `自动编码 · ${name || "未填写机柜名称"}`,
          form: errors.length ? null : base,
          roomId: room?.id ?? "",
          errors,
        } satisfies ImportRow;
      })
      .filter((r): r is ImportRow => r !== null);
    const bad = rows.value.filter((r) => r.errors.length > 0).length;
    if (!rows.value.length) ElMessage.warning("没有可导入的数据行");
    else if (bad) ElMessage.warning(`已读取 ${rows.value.length} 行，其中 ${bad} 行需要修正`);
    else ElMessage.success(`已读取并校验 ${rows.value.length} 行数据`);
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : "Excel 文件解析失败");
  } finally {
    input.value = "";
  }
}

async function startImport() {
  if (importing.value) return;
  importing.value = true;
  let ok = 0;
  for (const row of validRows.value) {
    if (!row.form) continue;
    try {
      const code = row.form.code || autoCode("RACK");
      await createRack(row.roomId, formToRackPayload(row.form, code) as never);
      ok++;
      importedCount.value = ok;
    } catch (e) {
      row.errors = [rackErrMsg(e)];
    }
  }
  importing.value = false;
  const fail = validRows.value.length - ok;
  if (fail)
    ElMessage.warning(`导入完成：成功 ${ok} 条，失败 ${fail} 条，可导出结果查看原因并重试失败项`);
  else ElMessage.success(`成功导入 ${ok} 条数据`);
  emit("imported");
}

function exportResult() {
  const lines = [
    ["Excel行号", "数据标识", "导入状态", "错误原因"].join(","),
    ...rows.value.map((r) =>
      [
        r.rowNumber,
        r.summary,
        r.errors.length === 0
          ? importedCount.value > 0
            ? "导入成功"
            : "校验通过"
          : r.errors.join("; "),
      ]
        .map((c) => `"${String(c).replace(/"/g, '""')}"`)
        .join(","),
    ),
  ].join("\r\n");
  saveFile("\uFEFF" + lines, "机柜批量导入结果.csv", "text/csv");
}

function rowStatus(r: ImportRow): string {
  if (r.errors.length === 0) return done.value ? "导入成功" : "待导入";
  return "校验错误";
}

function rowTagType(r: ImportRow): "danger" | "info" | "success" {
  const s = rowStatus(r);
  return s === "校验错误" ? "danger" : s === "待导入" ? "info" : "success";
}

function openChange(v: boolean) {
  if (v) {
    rows.value = [];
    importedCount.value = 0;
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    title="批量导入机柜"
    width="900px"
    destroy-on-close
    @open="openChange(true)"
  >
    <el-alert
      title="请使用机柜导入模板填写数据。机柜编码可留空，导入提交时自动生成；手工填写时校验编码唯一性。系统还会校验数据中心、机房、模板和数值范围，只有校验通过的行才会提交。"
      type="info"
      show-icon
      :closable="false"
      class="import-alert"
    />
    <div class="import-actions">
      <el-button @click="downloadTemplate">下载 Excel 导入模板</el-button>
      <el-button type="primary" plain @click="fileInput?.click()">选择 Excel 文件</el-button>
      <input
        ref="fileInput"
        type="file"
        accept=".csv,.xlsx,.xls"
        style="display: none"
        @change="onFileChange"
      />
    </div>
    <div v-if="rows.length" class="import-stats">
      <span>总行数 {{ rows.length }}</span>
      <span>可导入 {{ validRows.length }}</span>
      <span>校验错误 {{ errorCount }}</span>
      <span v-if="done">已成功 {{ importedCount }}</span>
    </div>
    <el-table v-if="rows.length" :data="rows" size="small" max-height="360">
      <el-table-column prop="rowNumber" label="Excel 行" width="80" />
      <el-table-column prop="summary" label="数据标识" min-width="180" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="rowTagType(row)" size="small">{{ rowStatus(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="说明" min-width="220">
        <template #default="{ row }">
          <span v-if="row.errors.length" class="import-errors">{{ row.errors.join("；") }}</span>
          <span v-else class="import-ok">校验通过，可以导入</span>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else description="请下载模板填写后，选择 Excel 文件进行校验" :image-size="72" />
    <template #footer>
      <el-button :disabled="!rows.length" @click="exportResult">导出校验/导入结果</el-button>
      <el-button
        type="primary"
        :loading="importing"
        :disabled="!validRows.length"
        data-test="rack-import-submit"
        @click="startImport"
      >
        开始导入（{{ validRows.length }} 条）
      </el-button>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.import-alert {
  margin-bottom: 14px;
}
.import-actions {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}
.import-stats {
  display: flex;
  gap: 18px;
  margin-bottom: 10px;
  color: #6b7280;
  font-size: 13px;
}
.import-errors {
  color: #dc2626;
  font-size: 12px;
}
.import-ok {
  color: #16a34a;
  font-size: 12px;
}
</style>
