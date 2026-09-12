<script setup lang="ts">
/**
 * 机柜图导入对话框(屏6b;复刻 v2 RackDiagramImportDialog 两阶段流程)。
 * 选择本系统导出的 .xlsx → 前端解析(rackDiagram.ts) → 后端 validate 拿
 * token 与逐行分类 → 人工确认(NEEDS_DECISION/REMOVE_MISSING 行必选处理方式)
 * → 二次确认 → commit 单事务提交 → 结果摘要。预校验不写数据。
 */
import { computed, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import {
  commitRackDiagram,
  fetchDeviceTypes,
  validateRackDiagram,
  type DeviceType,
  type DiagramImportItem,
  type DiagramValidateResult,
} from "@/features/resource/api";
import { parseRackDiagram } from "@/features/screen/rackDiagram";
import { rackErrMsg } from "@/features/rack/rackShared";

const props = defineProps<{ modelValue: boolean; roomId: string; dcId: string }>();
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "imported"): void;
}>();
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const fileInput = ref<HTMLInputElement | null>(null);
const fileName = ref("");
const parsedFile = ref<File | null>(null);
const defaultTypeId = ref("");
const types = ref<DeviceType[]>([]);
const typesLoading = ref(false);
const validating = ref(false);
const validated = ref<DiagramValidateResult | null>(null);
const decisions = reactive<Record<string, string>>({});
const committing = ref(false);
const confirmVisible = ref(false);
const resultText = ref("");

const summary = computed(() => validated.value?.summary);
const errorCount = computed(() => summary.value?.errors ?? 0);
const pendingDecisions = computed(
  () => (validated.value?.items ?? []).filter((i) => i.requiresDecision && !decisions[i.id]).length,
);
const headline = computed(() => {
  if (!validated.value) return "请先选择导出的机柜图并执行校验";
  if (errorCount.value) return "存在阻断错误，不能导入";
  if (pendingDecisions.value)
    return `还有 ${pendingDecisions.value} 项需要确认。所有待确认项处理完成后才可正式导入。`;
  return "校验通过。请确认下方变更内容后执行正式导入。";
});
const confirmText = computed(() => {
  const s = summary.value;
  if (!s) return "";
  return (
    `本次将新增 ${s.create} 项、迁移 ${s.move} 项，并按人工选择处理 ` +
    `${s.decisions} 个冲突和 ${s.removals} 个删除项。是否继续？`
  );
});

const KIND_TEXT: Record<string, string> = {
  CREATE_NEW: "新增",
  MOVE_EXISTING: "迁移",
  NEEDS_DECISION: "待确认",
  REMOVE_MISSING: "删除确认",
  ERROR: "错误",
  UNCHANGED: "保持不变",
  UPDATE_EXISTING: "保持不变",
};
const KIND_TAG: Record<string, "success" | "warning" | "danger" | "info"> = {
  CREATE_NEW: "success",
  MOVE_EXISTING: "warning",
  NEEDS_DECISION: "warning",
  REMOVE_MISSING: "danger",
  ERROR: "danger",
  UNCHANGED: "info",
  UPDATE_EXISTING: "info",
};
const ACTION_TEXT: Record<string, string> = {
  UPDATE_EXISTING: "保留编号，修改原设备",
  CREATE_NEW: "下架原设备，创建新设备",
  DECOMMISSION: "确认下架该设备",
  IGNORE: "忽略删除，保留原设备",
  SKIP: "忽略本项",
};
function allowedOf(item: DiagramImportItem): string[] {
  return item.allowedActions?.length
    ? item.allowedActions
    : ["UPDATE_EXISTING", "CREATE_NEW", "IGNORE"];
}
function itemKindText(item: DiagramImportItem): string {
  return KIND_TEXT[item.kind] ?? item.kind;
}
function deviceSpec(item: DiagramImportItem): string {
  return item.name ?? "无设备编号";
}
function sourceText(item: DiagramImportItem): string {
  return item.sourceDeviceCode ?? "无设备编号";
}

async function loadTypes() {
  if (types.value.length) return;
  typesLoading.value = true;
  try {
    types.value = await fetchDeviceTypes();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    typesLoading.value = false;
  }
}

function pickFile() {
  void loadTypes();
  fileInput.value?.click();
}
async function onFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const f = input.files?.[0];
  if (!f) return;
  fileName.value = f.name;
  parsedFile.value = f;
  validated.value = null;
  Object.keys(decisions).forEach((k) => delete decisions[k]);
  resultText.value = "";
  input.value = "";
}
async function validate() {
  if (!parsedFile.value) {
    ElMessage.warning("请先选择机柜图文件");
    return;
  }
  validating.value = true;
  try {
    const parsed = await parseRackDiagram(parsedFile.value, props.roomId, props.dcId);
    validated.value = await validateRackDiagram(props.roomId, {
      formatVersion: parsed.formatVersion,
      dataCenterId: parsed.dataCenterId,
      roomId: parsed.roomId,
      exportedAt: parsed.exportedAt,
      coveredRackIds: parsed.coveredRackIds,
      devices: parsed.devices,
      defaultTypeId: defaultTypeId.value || undefined,
    } as never);
  } catch (e) {
    validated.value = null;
    ElMessage.error(rackErrMsg(e));
  } finally {
    validating.value = false;
  }
}
function openConfirm() {
  if (errorCount.value || pendingDecisions.value) return;
  confirmVisible.value = true;
}
async function doCommit() {
  if (!validated.value) return;
  committing.value = true;
  try {
    const list = validated.value.items
      .filter((i) => decisions[i.id])
      .map((i) => ({ itemId: i.id, action: decisions[i.id] }));
    const r = await commitRackDiagram(props.roomId, {
      token: validated.value.token,
      decisions: list,
    });
    confirmVisible.value = false;
    resultText.value = `导入完成：新增 ${r.created}，更新 ${r.updated}，迁移 ${r.moved}，下架 ${r.decommissioned}`;
    ElMessage.success(resultText.value);
    validated.value = null;
    parsedFile.value = null;
    fileName.value = "";
    emit("imported");
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    committing.value = false;
  }
}
function reset() {
  validated.value = null;
  parsedFile.value = null;
  fileName.value = "";
  resultText.value = "";
  Object.keys(decisions).forEach((k) => delete decisions[k]);
}
</script>

<template>
  <el-dialog
    v-model="visible"
    title="导入机柜图"
    width="1080px"
    append-to-body
    destroy-on-close
    @open="reset"
  >
    <div class="import-guide">
      <strong>仅支持本页面“导出机柜图”生成的 .xlsx 文件</strong>
      <span>
        可以直接回导，也可以在 Excel
        中增删、改名或移动设备。预校验阶段不会写入数据；新增设备的编号在正式提交时自动生成。
      </span>
    </div>
    <div class="import-controls">
      <el-form-item label="新增设备默认类型" label-width="130px">
        <el-select
          v-model="defaultTypeId"
          :loading="typesLoading"
          filterable
          placeholder="请选择设备类型"
          style="width: 240px"
        >
          <el-option
            v-for="t in types"
            :key="t.id"
            :label="`${t.name}（${t.code}）`"
            :value="t.id ?? ''"
          />
        </el-select>
      </el-form-item>
      <div class="file-control">
        <el-button type="primary" plain @click="pickFile">选择机柜图</el-button>
        <span :class="{ muted: !fileName }">{{ fileName || "尚未选择文件" }}</span>
        <input
          ref="fileInput"
          type="file"
          accept=".xlsx"
          class="native-file-input"
          @change="onFileChange"
        />
        <el-button
          type="primary"
          :loading="validating"
          :disabled="!parsedFile"
          data-test="diagram-validate-btn"
          @click="validate"
        >
          解析并校验
        </el-button>
      </div>
    </div>

    <template v-if="validated">
      <div class="validation-summary">
        <div>
          <span>总项数</span>
          <strong>{{ validated.items.length }}</strong>
        </div>
        <div>
          <span>新增设备</span>
          <strong class="success">{{ summary?.create ?? 0 }}</strong>
        </div>
        <div>
          <span>位置迁移</span>
          <strong class="warning">{{ summary?.move ?? 0 }}</strong>
        </div>
        <div>
          <span>待人工确认</span>
          <strong class="warning">{{ summary?.decisions ?? 0 }}</strong>
        </div>
        <div>
          <span>错误</span>
          <strong :class="errorCount ? 'danger' : ''">{{ errorCount }}</strong>
        </div>
      </div>
      <el-alert
        :title="headline"
        :type="errorCount ? 'error' : pendingDecisions ? 'warning' : 'success'"
        :closable="false"
        show-icon
        class="import-alert"
      />
      <el-table :data="validated.items" size="small" max-height="380" class="validation-table">
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="KIND_TAG[row.kind as string] ?? 'info'">
              {{ itemKindText(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="目标位置" width="170">
          <template #default="{ row }">
            {{ row.rackCode }} · {{ row.startU }}-{{ row.endU }}U
          </template>
        </el-table-column>
        <el-table-column label="导入设备" min-width="170">
          <template #default="{ row }">
            <div class="device-name">{{ deviceSpec(row) }}</div>
            <small
              >功耗: {{ row.message?.includes("功耗") ? row.message : "无 SN / 功耗信息" }}</small
            >
          </template>
        </el-table-column>
        <el-table-column label="匹配到的原设备" min-width="140">
          <template #default="{ row }">{{ sourceText(row) }}</template>
        </el-table-column>
        <el-table-column label="校验说明" min-width="180">
          <template #default="{ row }">
            <span :class="{ 'no-decision': !row.message }">{{ row.message || "无需确认" }}</span>
          </template>
        </el-table-column>
        <el-table-column label="人工确认" width="210">
          <template #default="{ row }">
            <el-select
              v-if="row.requiresDecision"
              v-model="decisions[row.id as string]"
              size="small"
              placeholder="必须选择处理方式"
              style="width: 100%"
            >
              <el-option
                v-for="a in allowedOf(row)"
                :key="a"
                :label="ACTION_TEXT[a] ?? a"
                :value="a"
              />
            </el-select>
            <span v-else class="no-decision">—</span>
          </template>
        </el-table-column>
      </el-table>
    </template>
    <el-empty v-else description="请选择本系统导出的机柜图文件并执行解析校验" :image-size="72" />

    <template #footer>
      <div class="dialog-footer">
        <span v-if="resultText" class="footer-tip">{{ resultText }}</span>
        <el-button @click="visible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="committing"
          :disabled="!validated || !!errorCount || !!pendingDecisions"
          data-test="diagram-commit-btn"
          @click="openConfirm"
        >
          确认导入
        </el-button>
      </div>
    </template>

    <!-- 二次确认 -->
    <el-dialog
      v-model="confirmVisible"
      title="确认导入机柜图"
      width="480px"
      append-to-body
      destroy-on-close
    >
      <p style="margin: 0; color: #303133; line-height: 1.7">{{ confirmText }}</p>
      <template #footer>
        <el-button @click="confirmVisible = false">返回检查</el-button>
        <el-button type="primary" :loading="committing" @click="doCommit">确认执行</el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<style scoped>
/* v2 245b8e7f 逐条对应 */
.import-guide {
  margin-bottom: 14px;
  padding: 12px 16px;
  border: 1px solid #cfe3ef;
  border-radius: 8px;
  background: #f3f9fd;
}
.import-guide strong {
  display: block;
  color: #12324a;
  font-size: 13px;
}
.import-guide span {
  display: block;
  margin-top: 4px;
  color: #6991a8;
  font-size: 12px;
  line-height: 1.6;
}
.import-controls {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.import-controls .el-form-item {
  margin: 0;
}
.import-controls .el-select {
  min-width: 220px;
}
.file-control {
  display: flex;
  align-items: center;
  gap: 10px;
}
.file-control span {
  color: #303133;
  font-size: 12px;
}
.file-control span.muted {
  color: #a8abb2;
}
.native-file-input {
  display: none;
}
.import-alert {
  margin: 12px 0;
}
.validation-summary {
  display: flex;
  gap: 26px;
  margin-bottom: 4px;
}
.validation-summary div {
  text-align: left;
}
.validation-summary span {
  display: block;
  color: #8a9aa5;
  font-size: 11px;
}
.validation-summary strong {
  font-size: 20px;
  color: #12324a;
}
.validation-summary .success strong {
  color: #1d8a5f;
}
.validation-summary strong.success {
  color: #1d8a5f;
}
.validation-summary strong.warning {
  color: #c07f1d;
}
.validation-summary strong.danger {
  color: #c0483f;
}
.validation-table .device-name {
  color: #234257;
  font-size: 12px;
  font-weight: 600;
}
.validation-table small {
  color: #8a9aa5;
  font-size: 10px;
}
.no-decision {
  color: #a8abb2;
}
.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}
.footer-tip {
  margin-right: auto;
  color: #1d8a5f;
  font-size: 12px;
}
</style>
