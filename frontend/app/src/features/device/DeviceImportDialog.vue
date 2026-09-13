<script setup lang="ts">
/**
 * 设备批量导入(第 7 轮 UI-P0-01;复刻 v2 BatchImportDialog 设备流程):
 * 真正的 Excel xlsx 模板/解析/结果回执(与机柜导入的 CSV 不同,勿混写登记)。
 * 校验与匹配规则集中在 deviceImport.ts(v2 同口径);提交逐行调用
 * createDevice / updateDevice,失败行记录原因并可导出"设备批量导入结果.xlsx"。
 */
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { createDevice, fetchDevices, updateDevice, type DeviceType } from "@/features/resource/api";
import {
  DEVICE_IMPORT_FIELDS,
  rowsFromMatrix,
  validateImportRows,
  type ImportDeviceLite,
  type ImportRowResult,
} from "@/features/device/deviceImport";

const props = defineProps<{
  modelValue: boolean;
  types: DeviceType[];
}>();
const emit = defineEmits<{ (e: "update:modelValue", v: boolean): void; (e: "imported"): void }>();
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const IMPORT_DESCRIPTION =
  "支持新增和更新设备。系统优先按设备编码匹配已有设备，编码为空时按序列号或资产编号匹配；匹配到已有设备则更新，未匹配到则新增并自动生成编码。导入前会校验重复标识和冲突，更新时空白可选字段默认保持原值，设备位置不会被修改。";

const rows = ref<ImportRowResult[]>([]);
const loadingFile = ref(false);
const importing = ref(false);
const importedCount = ref(0);
const failedCount = ref(0);
const fileInput = ref<HTMLInputElement | null>(null);

const validRows = computed(() => rows.value.filter((r) => r.errors.length === 0));
const errorCount = computed(() => rows.value.length - validRows.value.length);
const done = computed(() => importedCount.value > 0 || failedCount.value > 0);

function rowStatus(r: ImportRowResult): string {
  if (r.errors.length === 0) return done.value ? "已提交" : "待导入";
  return r.errors.some((e) => e.startsWith("提交失败")) ? "提交失败" : "校验错误";
}
function rowTagType(r: ImportRowResult): "danger" | "info" | "success" {
  const s = rowStatus(r);
  return s === "校验错误" || s === "提交失败" ? "danger" : s === "待导入" ? "info" : "success";
}

/* ── 模板下载:表头(深蓝)/字段说明/示例 三行 + 列宽(v2 il 同规格) ── */
async function downloadTemplate() {
  const XLSX = await import("xlsx");
  const headerStyle = {
    font: { name: "微软雅黑", sz: 10, bold: true, color: { rgb: "FFFFFF" } },
    alignment: { horizontal: "center", vertical: "center", wrapText: true },
    fill: { patternType: "solid", fgColor: { rgb: "1F4E78" } },
  };
  const noteStyle = {
    font: { name: "微软雅黑", sz: 9, italic: true, color: { rgb: "6B7280" } },
    alignment: { vertical: "center", wrapText: true },
  };
  const cellStyle = {
    font: { name: "微软雅黑", sz: 10, color: { rgb: "1F2937" } },
    alignment: { vertical: "center", wrapText: true },
  };
  const aoa = [
    DEVICE_IMPORT_FIELDS.map((f) => (f.required ? `${f.title}（必填）` : f.title)),
    DEVICE_IMPORT_FIELDS.map((f) => f.description),
    DEVICE_IMPORT_FIELDS.map((f) => f.example),
  ];
  const sheet = XLSX.utils.aoa_to_sheet(aoa);
  const range = XLSX.utils.decode_range(sheet["!ref"] ?? "A1");
  const styles = [headerStyle, noteStyle, cellStyle];
  for (let r = range.s.r; r <= range.e.r; r += 1) {
    for (let c = range.s.c; c <= range.e.c; c += 1) {
      const ref = XLSX.utils.encode_cell({ r, c });
      sheet[ref] = { ...(sheet[ref] ?? { v: "", t: "s" }), s: styles[r] };
    }
  }
  sheet["!cols"] = DEVICE_IMPORT_FIELDS.map((f) => ({ wch: f.width }));
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, sheet, "设备导入");
  const buf = XLSX.write(wb, { bookType: "xlsx", type: "array", cellStyles: true });
  saveBlob(new Blob([buf], { type: "application/octet-stream" }), "设备批量导入模板.xlsx");
}

function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}

/* ── 文件解析:第一张工作表 → 行对象 → 全量设备库 → 逐行校验 ── */
async function onFileChange(ev: Event) {
  const file = (ev.target as HTMLInputElement).files?.[0];
  (ev.target as HTMLInputElement).value = "";
  if (!file) return;
  loadingFile.value = true;
  try {
    const XLSX = await import("xlsx");
    const wb = XLSX.read(await file.arrayBuffer(), { type: "array" });
    const sheet = wb.Sheets[wb.SheetNames[0]];
    if (!sheet) throw new Error("文件中没有工作表");
    const matrix = XLSX.utils.sheet_to_json<unknown[]>(sheet, {
      header: 1,
      raw: false,
      defval: "",
    });
    const [lineRows, unknownHeaders] = rowsFromMatrix(matrix);
    if (!lineRows.length) throw new Error("没有解析到数据行(第一行必须是表头)");
    if (unknownHeaders.length) {
      ElMessage.warning(`已忽略无法识别的表头列：${unknownHeaders.slice(0, 6).join("、")}`);
    }
    const devices = (await fetchAllDevices()) as unknown as ImportDeviceLite[];
    rows.value = validateImportRows(lineRows, { types: props.types, devices });
    importedCount.value = 0;
    failedCount.value = 0;
    if (!validRows.value.length) {
      ElMessage.error("没有可导入的行，请查看每行的错误说明");
    }
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : "文件解析失败");
  } finally {
    loadingFile.value = false;
  }
}

/** 全量设备(匹配库;v2 We 同款:页大小 100,5 页并发) */
async function fetchAllDevices(): Promise<ImportDeviceLite[]> {
  const first = await fetchDevices({ page: 1, pageSize: 100 });
  const items = [...first.items];
  const pages = Math.ceil(first.total / 100);
  for (let start = 2; start <= pages; start += 5) {
    const batch = Array.from({ length: Math.min(5, pages - start + 1) }, (_, i) => start + i);
    const results = await Promise.all(batch.map((p) => fetchDevices({ page: p, pageSize: 100 })));
    results.forEach((r) => items.push(...r.items));
  }
  return items as unknown as ImportDeviceLite[];
}

/* ── 提交:逐行 create/update;失败行回写原因 ── */
async function startImport() {
  if (importing.value) return;
  importing.value = true;
  importedCount.value = 0;
  failedCount.value = 0;
  let lastError = "";
  try {
    for (const row of validRows.value) {
      try {
        const payload = row.payload ?? {};
        if (row.operation === "UPDATE") {
          const version = Number(payload.version ?? 1);
          const body = { ...(payload as Record<string, unknown>) };
          delete body.version;
          delete body.id;
          await updateDevice(row.matchedDeviceId ?? "", version, body);
        } else {
          await createDevice(payload);
        }
        importedCount.value += 1;
      } catch (e) {
        failedCount.value += 1;
        lastError = e instanceof Error ? e.message : "提交失败";
        row.errors.push(`提交失败：${lastError}`);
      }
    }
    if (failedCount.value) {
      ElMessage.warning(`导入完成：成功 ${importedCount.value} 行，失败 ${failedCount.value} 行`);
    } else {
      ElMessage.success(`导入完成：成功 ${importedCount.value} 行`);
    }
    emit("imported");
  } finally {
    importing.value = false;
  }
}

/* ── 结果导出(设备批量导入结果.xlsx) ── */
async function exportResult() {
  const XLSX = await import("xlsx");
  const headerStyle = {
    font: { name: "微软雅黑", sz: 10, bold: true, color: { rgb: "FFFFFF" } },
    fill: { patternType: "solid", fgColor: { rgb: "1F4E78" } },
  };
  const aoa = [
    ["Excel行号", "操作", "数据标识", "结果", "说明"],
    ...rows.value.map((r) => [
      r.rowNumber,
      r.operation || "—",
      r.summary,
      rowStatus(r),
      r.errors.length ? r.errors.join("; ") : r.summary,
    ]),
  ];
  const sheet = XLSX.utils.aoa_to_sheet(aoa);
  sheet["!cols"] = [{ wch: 10 }, { wch: 10 }, { wch: 32 }, { wch: 10 }, { wch: 60 }];
  const range = XLSX.utils.decode_range(sheet["!ref"] ?? "A1");
  for (let c = range.s.c; c <= range.e.c; c += 1) {
    const ref = XLSX.utils.encode_cell({ r: 0, c });
    sheet[ref] = { ...(sheet[ref] ?? { v: "", t: "s" }), s: headerStyle };
  }
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, sheet, "导入结果");
  const buf = XLSX.write(wb, { bookType: "xlsx", type: "array", cellStyles: true });
  saveBlob(new Blob([buf], { type: "application/octet-stream" }), "设备批量导入结果.xlsx");
}

function openChange(v: boolean) {
  if (v) {
    rows.value = [];
    importedCount.value = 0;
    failedCount.value = 0;
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    title="批量导入设备"
    width="920px"
    destroy-on-close
    @open="openChange(true)"
  >
    <el-alert
      :title="IMPORT_DESCRIPTION"
      type="info"
      show-icon
      :closable="false"
      class="import-alert"
    />
    <div class="import-actions">
      <el-button data-test="device-import-template-btn" @click="downloadTemplate">
        下载 Excel 导入模板
      </el-button>
      <el-button
        type="primary"
        plain
        :loading="loadingFile"
        data-test="device-import-file-btn"
        @click="fileInput?.click()"
      >
        选择 Excel 文件
      </el-button>
      <input
        ref="fileInput"
        type="file"
        accept=".xlsx,.xls"
        style="display: none"
        @change="onFileChange"
      />
    </div>
    <div v-if="rows.length" class="import-stats" data-test="device-import-stats">
      <span>总行数 {{ rows.length }}</span>
      <span>可导入 {{ validRows.length }}</span>
      <span>校验错误 {{ errorCount }}</span>
      <span v-if="importedCount">已成功 {{ importedCount }}</span>
      <span v-if="failedCount" class="import-errors">失败 {{ failedCount }}</span>
    </div>
    <el-table v-if="rows.length" :data="rows" size="small" max-height="360">
      <el-table-column prop="rowNumber" label="Excel 行" width="80" />
      <el-table-column prop="operation" label="操作" width="80" />
      <el-table-column prop="summary" label="数据标识" min-width="180" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="rowTagType(row)" size="small">{{ rowStatus(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="说明" min-width="220">
        <template #default="{ row }">
          <span v-if="row.errors.length" class="import-errors">{{ row.errors.join("；") }}</span>
          <span v-else class="import-ok">校验通过</span>
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
        data-test="device-import-submit"
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
