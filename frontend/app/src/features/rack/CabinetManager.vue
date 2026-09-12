<script setup lang="ts">
/**
 * 屏4 机柜管理(复刻 v2 RackManagementView,scopeId data-v-dc499c1f 的类名逐条保留)。
 * 数据源 = 资源树 flatMap(跨机房汇总视图,与 v2 同口径);页头四统计卡随全量数据统计,
 * 筛选条即时生效(关键词点查询/回车生效),表格 8 列 + 分页,新增/编辑 900px 三段式对话框,
 * 520px 详情抽屉;批量导入入口见 RackImportDialog。
 */
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  createRack,
  deleteRack,
  fetchResourceTree,
  fetchRackTemplates,
  updateRack,
  type RackTemplate,
  type TreeDataCenter,
} from "@/features/resource/api";
import RackImportDialog from "@/features/rack/RackImportDialog.vue";
import {
  RACK_STATUS_OPTIONS,
  activeTemplateOptions,
  applyTemplateToForm,
  autoCode,
  formToRackPayload,
  rackErrMsg,
  rackFormDefaults,
  rackLocationText,
  rackStatusTagType,
  rackStatusLabel,
  rackToForm,
  type RackForm,
  type TreeRackRow,
} from "@/features/rack/rackShared";

const router = useRouter();

const tree = ref<TreeDataCenter[]>([]);
const templates = ref<RackTemplate[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const importVisible = ref(false);
const detailVisible = ref(false);
const mode = ref<"create" | "edit">("create");
const autoGenCode = ref(true);
const detailRow = ref<TreeRackRow | null>(null);
const editId = ref("");
const editVersion = ref(0);
const formDcId = ref("");
const formRoomId = ref("");
const page = ref(1);
const pageSize = ref(20);
const form = ref<RackForm>(rackFormDefaults());
const filters = reactive({ dataCenterId: "", roomId: "", status: "", keyword: "" });
const appliedKeyword = ref("");

const isActive = (s: string | null | undefined) => s !== "DISABLED" && s !== "ARCHIVED";

const allRacks = computed<TreeRackRow[]>(() =>
  tree.value.flatMap((dc) =>
    (dc.rooms ?? []).flatMap((room) =>
      (room.racks ?? []).map((r) => ({
        ...r,
        dataCenterName: dc.name ?? "",
        roomName: room.name ?? "",
        dataCenterStatus: dc.status,
        roomStatus: room.status,
      })),
    ),
  ),
);
const dcActiveRooms = (dc: TreeDataCenter) => (dc.rooms ?? []).filter((r) => isActive(r.status));
/** 可选 DC(启用且至少有一个未停用/归档机房;新建对话框左侧下拉) */
const creatableDcs = computed(() =>
  tree.value.filter((dc) => isActive(dc.status) && dcActiveRooms(dc).length > 0),
);
const dcRoomOptions = computed(
  () => creatableDcs.value.find((dc) => dc.id === formDcId.value)?.rooms ?? [],
);
/** DC→机房联级选项(新建时预选当前筛选所在位置;v2 H 口径) */
const dcRoomPairs = computed(() =>
  tree.value
    .filter((dc) => isActive(dc.status))
    .flatMap((dc) => dcActiveRooms(dc).map((room) => ({ dataCenter: dc, room }))),
);
/** 筛选条机房下拉:选了 DC 显示该 DC 全部机房,否则全部 */
const filterRoomOptions = computed(() =>
  filters.dataCenterId
    ? (tree.value.find((dc) => dc.id === filters.dataCenterId)?.rooms ?? [])
    : tree.value.flatMap((dc) => dc.rooms ?? []),
);
const filteredRacks = computed(() => {
  const kw = appliedKeyword.value.trim().toLowerCase();
  return allRacks.value.filter((r) => {
    if (filters.dataCenterId && r.dataCenterId !== filters.dataCenterId) return false;
    if (filters.roomId && r.roomId !== filters.roomId) return false;
    if (filters.status && r.status !== filters.status) return false;
    if (!kw) return true;
    return [
      r.code,
      r.name,
      r.dataCenterName,
      r.roomName,
      r.zone,
      r.rackRow,
      r.rackColumn,
      r.aisle,
      r.manufacturer,
      r.modelNumber,
      r.manager,
    ].some((v) =>
      String(v ?? "")
        .toLowerCase()
        .includes(kw),
    );
  });
});
const pagedRacks = computed(() =>
  filteredRacks.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
);
/* ── 四统计卡(随全量;v2 Me/qe/De/Ee/Pe 口径) ── */
const totalU = computed(() => filteredRacks.value.reduce((s, r) => s + (r.uHeight ?? 0), 0));
const statAvailable = computed(() => allRacks.value.filter((r) => r.status === "AVAILABLE").length);
const statDualPower = computed(() => allRacks.value.filter((r) => r.dualPower).length);
const statAttention = computed(
  () =>
    allRacks.value.filter((r) => ["FULL", "MAINTENANCE", "DISABLED"].includes(r.status ?? ""))
      .length,
);
const statDcCount = computed(() => new Set(allRacks.value.map((r) => r.dataCenterId)).size);
const statRoomCount = computed(() => new Set(allRacks.value.map((r) => r.roomId)).size);
const detailLocation = computed(() =>
  detailRow.value
    ? `${detailRow.value.dataCenterName ?? "-"} / ${detailRow.value.roomName ?? "-"}`
    : "-",
);
const templateOptions = computed(() => activeTemplateOptions(templates.value));

watch(
  () => [filters.dataCenterId, filters.roomId, filters.status, pageSize],
  () => {
    page.value = 1;
  },
);

async function loadAll() {
  loading.value = true;
  try {
    const [tr, tpl] = await Promise.all([fetchResourceTree(), fetchRackTemplates()]);
    tree.value = tr;
    templates.value = tpl;
    if (filters.dataCenterId && !tr.some((dc) => dc.id === filters.dataCenterId)) {
      filters.dataCenterId = "";
    }
    if (filters.roomId && !filterRoomOptions.value.some((r) => r.id === filters.roomId)) {
      filters.roomId = "";
    }
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    loading.value = false;
  }
}

function openImport() {
  importVisible.value = true;
}

function openCreate() {
  mode.value = "create";
  detailVisible.value = false;
  form.value = rackFormDefaults();
  autoGenCode.value = true;
  const hit =
    dcRoomPairs.value.find(
      (p) =>
        p.dataCenter.id === filters.dataCenterId &&
        (!filters.roomId || p.room.id === filters.roomId),
    ) ?? dcRoomPairs.value[0];
  formDcId.value = hit?.dataCenter.id ?? "";
  formRoomId.value = hit?.room.id ?? "";
  dialogVisible.value = true;
}

function openEdit(row: TreeRackRow) {
  detailVisible.value = false;
  mode.value = "edit";
  detailRow.value = row;
  editId.value = row.id ?? "";
  editVersion.value = row.version ?? 0;
  form.value = rackToForm(row);
  formDcId.value = row.dataCenterId ?? "";
  formRoomId.value = row.roomId ?? "";
  dialogVisible.value = true;
}

function openDetail(row: TreeRackRow) {
  detailRow.value = row;
  detailVisible.value = true;
}

function onFormDcChange() {
  formRoomId.value = dcRoomOptions.value[0]?.id ?? "";
}

function applySearch() {
  appliedKeyword.value = filters.keyword;
  page.value = 1;
}
function resetFilters() {
  filters.dataCenterId = "";
  filters.roomId = "";
  filters.status = "";
  filters.keyword = "";
  appliedKeyword.value = "";
  page.value = 1;
}

/** 选模板 → 规格落表单(v2 Te 口径:模板当前版本参数回填) */
function onTemplateChange(id: string | undefined) {
  if (!id) return;
  const hit = templateOptions.value.find((o) => o.template.id === id);
  if (hit?.version) applyTemplateToForm(form.value, hit.version);
}

async function save() {
  const auto = mode.value === "create" && autoGenCode.value;
  if (!form.value.name.trim() || (!auto && !form.value.code.trim())) {
    ElMessage.warning(auto ? "名称不能为空" : "编码和名称不能为空");
    return;
  }
  if (mode.value === "create" && !formRoomId.value) {
    ElMessage.warning("请选择机柜所属机房");
    return;
  }
  saving.value = true;
  try {
    const code = auto ? autoCode("RACK") : form.value.code.trim();
    if (mode.value === "create") {
      await createRack(formRoomId.value, formToRackPayload(form.value, code) as never);
    } else if (editId.value) {
      await updateRack(
        editId.value,
        editVersion.value,
        formToRackPayload(form.value, code) as never,
      );
    }
    dialogVisible.value = false;
    ElMessage.success("机柜已保存");
    await loadAll();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    saving.value = false;
  }
}

async function remove(row: TreeRackRow) {
  try {
    await ElMessageBox.confirm(
      `确定删除机柜“${row.name}”吗？如果机柜内仍有设备将禁止删除。`,
      "删除确认",
      { type: "warning" },
    );
  } catch {
    return;
  }
  try {
    await deleteRack(row.id ?? "", row.version ?? 0);
    ElMessage.success("机柜已删除");
    await loadAll();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  }
}

function gotoULayout(row: TreeRackRow) {
  router.push({ path: "/devices", query: { tab: "layout", rackId: row.id ?? "" } });
}

function kWText(v: number | null | undefined): string {
  return v === null || v === undefined ? "-" : `${v} kW`;
}

onMounted(loadAll);
</script>

<template>
  <div class="rack-page" data-test="rack-page">
    <div class="page-header">
      <div>
        <h2>机柜管理</h2>
        <p>集中查看和维护全部机柜台账，支持跨数据中心筛选、模板建柜和 U 位入口。</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="loadAll">刷新</el-button>
        <el-button :disabled="!dcRoomPairs.length" @click="openImport">批量导入</el-button>
        <el-button type="primary" :disabled="!dcRoomPairs.length" @click="openCreate">
          新增机柜
        </el-button>
      </div>
    </div>

    <el-row :gutter="16" class="summary-row">
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card total-card">
          <div class="summary-title">机柜总数</div>
          <div class="summary-value">{{ allRacks.length }}</div>
          <div class="summary-hint">
            覆盖 {{ statDcCount }} 个数据中心、{{ statRoomCount }} 个机房
          </div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card available-card">
          <div class="summary-title">空闲机柜</div>
          <div class="summary-value">{{ statAvailable }}</div>
          <div class="summary-hint">可安排设备上架</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card power-card">
          <div class="summary-title">双路供电</div>
          <div class="summary-value">{{ statDualPower }}</div>
          <div class="summary-hint">配置双路电源的机柜</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card warning-card">
          <div class="summary-title">需关注</div>
          <div class="summary-value">{{ statAttention }}</div>
          <div class="summary-hint">已满、维护中或已停用</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="filter-card">
      <div class="filter-bar">
        <el-select
          v-model="filters.dataCenterId"
          clearable
          placeholder="全部数据中心"
          style="width: 190px"
          data-test="rack-dc-filter"
          @change="filters.roomId = ''"
        >
          <el-option v-for="dc in tree" :key="dc.id" :label="dc.name" :value="dc.id ?? ''" />
        </el-select>
        <el-select
          v-model="filters.roomId"
          clearable
          placeholder="全部机房"
          style="width: 190px"
          data-test="rack-room-filter"
        >
          <el-option
            v-for="room in filterRoomOptions"
            :key="room.id"
            :label="room.name"
            :value="room.id ?? ''"
          />
        </el-select>
        <el-select
          v-model="filters.status"
          clearable
          placeholder="全部状态"
          style="width: 150px"
          data-test="rack-status-filter"
        >
          <el-option
            v-for="o in RACK_STATUS_OPTIONS"
            :key="o.value"
            :label="o.label"
            :value="o.value"
          />
        </el-select>
        <el-input
          v-model="filters.keyword"
          clearable
          placeholder="搜索编码、名称、位置、厂商或负责人"
          class="keyword-input"
          data-test="rack-search"
          @keyup.enter="applySearch"
        />
        <el-button type="primary" data-test="rack-search-btn" @click="applySearch">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </el-card>

    <el-card v-loading="loading" shadow="never" class="table-card">
      <template #header>
        <div class="table-header">
          <div>
            <strong>机柜台账</strong>
            <span>共 {{ filteredRacks.length }} 条，合计 {{ totalU }}U</span>
          </div>
          <el-tag type="info" effect="plain">跨机房汇总视图</el-tag>
        </div>
      </template>
      <el-table
        v-if="filteredRacks.length"
        :data="pagedRacks"
        stripe
        table-layout="fixed"
        data-test="rack-table"
      >
        <el-table-column label="机柜" min-width="190" fixed="left">
          <template #default="{ row }">
            <button class="rack-name-button" type="button" @click="openDetail(row)">
              {{ row.name }}
            </button>
            <div class="cell-subtitle">{{ row.code }}</div>
          </template>
        </el-table-column>
        <el-table-column label="所属位置" min-width="230">
          <template #default="{ row }">
            <div>{{ row.dataCenterName }} / {{ row.roomName }}</div>
            <div class="cell-subtitle">{{ rackLocationText(row) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="rackStatusTagType(row.status)">
              {{ rackStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="规格" width="145">
          <template #default="{ row }">
            <div>{{ row.uHeight }}U · {{ row.type || "标准机柜" }}</div>
            <div class="cell-subtitle">
              {{ row.widthMm }}×{{ row.depthMm }}×{{ row.heightMm }} mm
            </div>
          </template>
        </el-table-column>
        <el-table-column label="模板" min-width="150">
          <template #default="{ row }">
            <span v-if="row.templateName">{{ row.templateName }} V{{ row.templateRevision }}</span>
            <span v-else class="muted">自定义参数</span>
          </template>
        </el-table-column>
        <el-table-column label="电源 / PDU" width="135">
          <template #default="{ row }">
            <div>
              {{ row.dualPower ? "双路" : "单路"
              }}{{ row.inputCircuits > 0 ? ` · ${row.inputCircuits} 路` : "供电" }}
            </div>
            <div class="cell-subtitle" :class="{ muted: !row.pduCount }">
              {{ row.pduCount > 0 ? `${row.pduCount} 个 PDU` : "PDU 未配置" }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="manager" label="负责人" width="110">
          <template #default="{ row }">{{ row.manager || "-" }}</template>
        </el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="gotoULayout(row)">U 位</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="没有符合条件的机柜" />
      <div v-if="filteredRacks.length > pageSize" class="pagination">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          layout="total, sizes, prev, pager, next"
          :page-sizes="[10, 20, 50, 100]"
          :total="filteredRacks.length"
        />
      </div>
    </el-card>

    <!-- 新增/编辑机柜 -->
    <el-dialog
      v-model="dialogVisible"
      :title="mode === 'create' ? '新增机柜' : '编辑机柜'"
      width="900px"
      destroy-on-close
    >
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
      <el-form label-width="105px">
        <div v-if="mode === 'create'" class="batch-entry">
          <div>
            <strong>需要一次新增多个机柜？</strong>
            <span>使用 Excel 模板批量校验并导入，模板中已标明必填和可选字段。</span>
          </div>
          <div class="batch-entry-actions">
            <el-button @click="openImport">下载模板</el-button>
            <el-button type="primary" plain @click="openImport">批量导入机柜</el-button>
          </div>
        </div>
        <div class="form-section-title">归属与基本信息</div>
        <el-row v-if="mode === 'create'" :gutter="16">
          <el-col :span="12">
            <el-form-item label="数据中心" required>
              <el-select v-model="formDcId" style="width: 100%" @change="onFormDcChange">
                <el-option
                  v-for="dc in creatableDcs"
                  :key="dc.id"
                  :label="dc.name"
                  :value="dc.id ?? ''"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="机房" required>
              <el-select v-model="formRoomId" style="width: 100%">
                <el-option
                  v-for="room in dcRoomOptions"
                  :key="room.id"
                  :label="room.name"
                  :value="room.id ?? ''"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-alert
          v-else
          class="location-alert"
          :title="`所属位置：${detailLocation}`"
          type="info"
          :closable="false"
        />
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="编码" :required="mode === 'edit' || !autoGenCode">
              <div class="code-field">
                <el-input
                  v-model="form.code"
                  :disabled="mode === 'create' && autoGenCode"
                  :placeholder="
                    mode === 'create' && autoGenCode ? '保存时自动生成' : '请输入机柜编码'
                  "
                />
                <el-checkbox v-if="mode === 'create'" v-model="autoGenCode">自动生成</el-checkbox>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="名称" required>
              <el-input v-model="form.name" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
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
        </el-row>
        <el-form-item v-if="mode === 'create'" label="机柜模板">
          <el-select
            :model-value="form.templateId"
            clearable
            placeholder="不选择则使用自定义参数"
            style="width: 100%"
            @update:model-value="form.templateId = $event"
            @change="onTemplateChange"
          >
            <el-option
              v-for="o in templateOptions"
              :key="o.template.id"
              :label="`${o.template.name} · V${o.version?.revision} · ${o.version?.uHeight}U`"
              :value="o.template.id ?? ''"
            />
          </el-select>
        </el-form-item>
        <el-alert
          v-else-if="detailRow?.templateName"
          class="location-alert"
          :title="`实例模板快照：${detailRow?.templateName ?? ''} V${detailRow?.templateRevision ?? ''}`"
          type="info"
          :closable="false"
        />
        <div class="form-section-title">规格与位置</div>
        <el-row :gutter="16">
          <el-col :span="6">
            <el-form-item label="U 位">
              <el-input-number
                v-model="form.uHeight"
                :min="1"
                :max="100"
                :step="1"
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
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="厂商">
              <el-input v-model="form.manufacturer" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="型号">
              <el-input v-model="form.modelNumber" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="承重(kg)">
              <el-input-number
                v-model="form.loadCapacityKg"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
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
        <div class="form-section-title">电源与管理信息</div>
        <el-row :gutter="16">
          <el-col :span="6">
            <el-form-item label="双路电源">
              <el-switch v-model="form.dualPower" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="输入路数">
              <el-input-number
                v-model="form.inputCircuits"
                :min="0"
                :step="1"
                step-strictly
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="PDU 数量">
              <el-input-number
                v-model="form.pduCount"
                :min="0"
                :step="1"
                step-strictly
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="额定电压(V)">
              <el-input-number
                v-model="form.ratedVoltage"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="额定电流(A)">
              <el-input-number
                v-model="form.ratedCurrent"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="额定功率(kW)">
              <el-input-number
                v-model="form.ratedPowerKw"
                :min="0"
                :precision="2"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="峰值功率(kW)">
              <el-input-number
                v-model="form.peakPowerKw"
                :min="0"
                :precision="2"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="负责人">
              <el-input v-model="form.manager" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="用途">
              <el-input v-model="form.purpose" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注">
          <el-input v-model="form.remarks" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
    </el-dialog>

    <!-- 批量导入 -->
    <RackImportDialog
      v-model="importVisible"
      :data-centers="tree"
      :templates="templates"
      :existing-racks="allRacks"
      @imported="loadAll"
    />

    <!-- 详情抽屉 -->
    <el-drawer v-model="detailVisible" title="机柜详情" size="520px">
      <template v-if="detailRow">
        <div class="detail-heading">
          <div>
            <h3>{{ detailRow.name }}</h3>
            <p>{{ detailRow.code }}</p>
          </div>
          <el-tag :type="rackStatusTagType(detailRow.status ?? '')">
            {{ rackStatusLabel(detailRow.status ?? "") }}
          </el-tag>
        </div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="数据中心" :span="2">
            {{ detailRow.dataCenterName }}
          </el-descriptions-item>
          <el-descriptions-item label="机房" :span="2">
            {{ detailRow.roomName }}
          </el-descriptions-item>
          <el-descriptions-item label="物理位置" :span="2">
            {{ rackLocationText(detailRow) }}
          </el-descriptions-item>
          <el-descriptions-item label="模板" :span="2">
            {{
              detailRow.templateName
                ? `${detailRow.templateName} V${detailRow.templateRevision}`
                : "自定义参数"
            }}
          </el-descriptions-item>
          <el-descriptions-item label="规格">{{ detailRow.uHeight }}U</el-descriptions-item>
          <el-descriptions-item label="类型">{{ detailRow.type || "-" }}</el-descriptions-item>
          <el-descriptions-item label="尺寸" :span="2">
            {{ detailRow.widthMm }} × {{ detailRow.depthMm }} × {{ detailRow.heightMm }} mm
          </el-descriptions-item>
          <el-descriptions-item label="厂商">
            {{ detailRow.manufacturer || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="型号">
            {{ detailRow.modelNumber || "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="电源">
            {{ detailRow.dualPower ? "双路电源" : "单路电源" }}
          </el-descriptions-item>
          <el-descriptions-item label="输入路数">
            {{
              detailRow.inputCircuits && detailRow.inputCircuits > 0
                ? `${detailRow.inputCircuits} 路`
                : "未配置"
            }}
          </el-descriptions-item>
          <el-descriptions-item label="PDU">
            {{
              detailRow.pduCount && detailRow.pduCount > 0 ? `${detailRow.pduCount} 个` : "未配置"
            }}
          </el-descriptions-item>
          <el-descriptions-item label="额定功率">
            {{ kWText(detailRow.ratedPowerKw) }}
          </el-descriptions-item>
          <el-descriptions-item label="负责人">{{ detailRow.manager || "-" }}</el-descriptions-item>
          <el-descriptions-item label="用途">{{ detailRow.purpose || "-" }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">
            {{ detailRow.remarks || "-" }}
          </el-descriptions-item>
        </el-descriptions>
        <div class="drawer-actions">
          <el-button @click="openEdit(detailRow)">编辑机柜</el-button>
          <el-button type="primary" @click="gotoULayout(detailRow)">查看 U 位</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
/* 与 v2 RackManagementView-BL6u7R6o.css 逐条对应(去掉 data-v 哈希) */
.rack-page {
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
.header-actions {
  display: flex;
  gap: 10px;
}
.summary-row {
  margin-bottom: 16px;
}
.summary-card {
  position: relative;
  overflow: hidden;
  border-top: 3px solid #409eff;
}
.summary-card::after {
  content: "";
  position: absolute;
  width: 86px;
  height: 86px;
  border-radius: 50%;
  right: -28px;
  top: -36px;
  background: currentColor;
  opacity: 0.07;
}
.total-card {
  color: #409eff;
}
.available-card {
  color: #10b981;
  border-top-color: #10b981;
}
.power-card {
  color: #8b5cf6;
  border-top-color: #8b5cf6;
}
.warning-card {
  color: #f59e0b;
  border-top-color: #f59e0b;
}
.summary-title {
  color: #6b7280;
  font-size: 14px;
}
.summary-value {
  margin-top: 8px;
  color: #111827;
  font-size: 30px;
  line-height: 1.1;
  font-weight: 700;
}
.summary-value small {
  color: #9ca3af;
  font-size: 14px;
  font-weight: 400;
}
.summary-hint {
  margin-top: 9px;
  color: #9ca3af;
  font-size: 12px;
}
.filter-card {
  margin-bottom: 16px;
}
.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.keyword-input {
  min-width: 260px;
  flex: 1;
}
.table-card {
  min-height: 470px;
}
.table-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.table-header strong {
  margin-right: 12px;
  font-size: 16px;
}
.table-header span {
  color: #9ca3af;
  font-size: 13px;
}
.rack-name-button {
  padding: 0;
  border: 0;
  background: transparent;
  color: #2563eb;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.rack-name-button:hover {
  text-decoration: underline;
}
.cell-subtitle {
  margin-top: 4px;
  color: #9ca3af;
  font-size: 12px;
  line-height: 1.35;
}
.muted {
  color: #9ca3af;
}
.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
.form-section-title {
  margin: 4px 0 16px;
  padding-left: 10px;
  border-left: 3px solid #409eff;
  color: #374151;
  font-weight: 700;
}
.batch-entry {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  padding: 14px 16px;
  border: 1px dashed #93c5fd;
  border-radius: 8px;
  background: #eff6ff;
}
.batch-entry strong {
  display: block;
  color: #1d4ed8;
}
.batch-entry span {
  display: block;
  margin-top: 4px;
  color: #64748b;
  font-size: 13px;
}
.batch-entry-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
.location-alert {
  margin-bottom: 16px;
}
.detail-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 20px;
}
.detail-heading h3 {
  margin: 0 0 6px;
  font-size: 20px;
}
.detail-heading p {
  margin: 0;
  color: #9ca3af;
}
.drawer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 22px;
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
@media (max-width: 900px) {
  .page-header {
    gap: 14px;
  }
  .header-actions {
    flex-shrink: 0;
  }
}
</style>
