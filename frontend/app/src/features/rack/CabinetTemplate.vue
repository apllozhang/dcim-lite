<script setup lang="ts">
/**
 * 屏5 机柜模板(复刻 v2 RackTemplateView,scopeId data-v-697c3598 的类名逐条保留)。
 * 列表 7 列;新增 920px 对话框(编码可自动生成 + 初始版本参数规格子表单);
 * 编辑仅元信息(版本发布后不可修改);新版本 820px 对话框(revision 递增)。
 * 自动编码为前端生成(v2 由服务端生成,后端无 autoGenerateCode,见 rackShared 注)。
 */
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  deleteRackTemplate,
  fetchRackTemplates,
  publishRackTemplateVersion,
  createRackTemplate,
  updateRackTemplate,
  type RackTemplate,
} from "@/features/resource/api";
import TemplateSpecForm from "@/features/rack/TemplateSpecForm.vue";
import {
  autoCode,
  latestVersion,
  rackErrMsg,
  submitWithAutoCode,
  type TemplateSpec,
} from "@/features/rack/rackShared";

const list = ref<RackTemplate[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const versionVisible = ref(false);
const mode = ref<"create" | "edit">("create");
const autoGenCode = ref(true);
const editing = ref<RackTemplate | null>(null);
const current = ref<TemplateSpec>(specDefaults());
const editingForm = ref({
  id: "",
  version: 0,
  code: "",
  name: "",
  description: "",
  status: "ACTIVE",
  remarks: "",
});

function specDefaults(): TemplateSpec {
  return {
    type: "STANDARD",
    manufacturer: "",
    modelNumber: "",
    uHeight: 42,
    widthMm: 600,
    depthMm: 1200,
    heightMm: 2000,
    dualPower: false,
    inputCircuits: 0,
    pduCount: 0,
    changeNote: "",
  };
}

/** 系统内置模板在编辑态禁改状态(v2 L computed 口径) */
const systemEditLocked = computed(
  () =>
    mode.value === "edit" &&
    list.value.find((t) => t.id === editingForm.value.id)?.isSystem === true,
);
const nextRevision = computed(() => (editing.value?.currentRevision ?? 0) + 1);

async function loadAll() {
  loading.value = true;
  try {
    list.value = await fetchRackTemplates();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  mode.value = "create";
  autoGenCode.value = true;
  editingForm.value = {
    id: "",
    version: 0,
    code: "",
    name: "",
    description: "",
    status: "ACTIVE",
    remarks: "",
  };
  current.value = specDefaults();
  dialogVisible.value = true;
}

function openEdit(t: RackTemplate) {
  mode.value = "edit";
  editingForm.value = {
    id: t.id ?? "",
    version: t.version ?? 0,
    code: t.code ?? "",
    name: t.name ?? "",
    description: t.description ?? "",
    status: t.status ?? "ACTIVE",
    remarks: t.remarks ?? "",
  };
  current.value = specDefaults();
  dialogVisible.value = true;
}

/** 新版本:默认带出最新一版的完整规格(v2 O 口径) */
function openVersion(t: RackTemplate) {
  editing.value = t;
  const v = latestVersion(t);
  current.value = v
    ? {
        type: v.type ?? "STANDARD",
        manufacturer: v.manufacturer ?? "",
        modelNumber: v.modelNumber ?? "",
        uHeight: v.uHeight ?? 42,
        widthMm: v.widthMm ?? 600,
        depthMm: v.depthMm ?? 1200,
        heightMm: v.heightMm ?? 2000,
        loadCapacityKg: v.loadCapacityKg ?? undefined,
        dualPower: v.dualPower ?? false,
        inputCircuits: v.inputCircuits ?? 0,
        ratedVoltage: v.ratedVoltage ?? undefined,
        ratedCurrent: v.ratedCurrent ?? undefined,
        ratedPowerKw: v.ratedPowerKw ?? undefined,
        peakPowerKw: v.peakPowerKw ?? undefined,
        pduCount: v.pduCount ?? 0,
        changeNote: "",
      }
    : specDefaults();
  versionVisible.value = true;
}

async function save() {
  const auto = mode.value === "create" && autoGenCode.value;
  if (!editingForm.value.name.trim() || (!auto && !editingForm.value.code.trim())) {
    ElMessage.warning(auto ? "模板名称不能为空" : "模板编码和名称不能为空");
    return;
  }
  saving.value = true;
  try {
    if (mode.value === "create") {
      // 自动编码:时间戳+随机段,唯一冲突时重生成重试一次(UI-P1-04)
      await submitWithAutoCode(
        auto,
        () => (auto ? autoCode("RTPL") : editingForm.value.code.trim()),
        (code) =>
          createRackTemplate({
            code,
            name: editingForm.value.name,
            description: editingForm.value.description,
            status: editingForm.value.status,
            remarks: editingForm.value.remarks,
            version: { ...current.value },
          } as never),
      );
    } else {
      await updateRackTemplate(editingForm.value.id, editingForm.value.version, {
        name: editingForm.value.name,
        description: editingForm.value.description,
        status: editingForm.value.status,
        remarks: editingForm.value.remarks,
      });
    }
    dialogVisible.value = false;
    ElMessage.success("模板已保存");
    await loadAll();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    saving.value = false;
  }
}

async function publishVersion() {
  if (!editing.value) return;
  saving.value = true;
  try {
    await publishRackTemplateVersion(editing.value.id ?? "", editing.value.version ?? 0, {
      ...current.value,
    });
    versionVisible.value = false;
    ElMessage.success("新版本已发布");
    await loadAll();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  } finally {
    saving.value = false;
  }
}

async function remove(t: RackTemplate) {
  try {
    await ElMessageBox.confirm(
      `确定删除模板“${t.name}”吗？已创建机柜的快照不会受影响。`,
      "删除确认",
      { type: "warning" },
    );
  } catch {
    return;
  }
  try {
    await deleteRackTemplate(t.id ?? "", t.version ?? 0);
    ElMessage.success("模板已删除");
    await loadAll();
  } catch (e) {
    ElMessage.error(rackErrMsg(e));
  }
}

onMounted(loadAll);
</script>

<template>
  <div class="template-page" data-test="rack-template-page">
    <div class="page-header">
      <div>
        <h2>机柜模板</h2>
        <p>维护可复用的机柜规格。模板版本发布后不可修改，已创建机柜保留当时的参数快照。</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="loadAll">刷新</el-button>
        <el-button type="primary" @click="openCreate">新增模板</el-button>
      </div>
    </div>
    <el-alert
      title="系统内置“标准 42U 机柜”模板不可删除或停用；新增版本不会自动修改已创建机柜。"
      type="info"
      show-icon
      :closable="false"
    />
    <el-card v-loading="loading" shadow="never" class="table-card">
      <el-table :data="list" stripe data-test="rack-template-table">
        <el-table-column prop="code" label="模板编码" min-width="145" />
        <el-table-column prop="name" label="模板名称" min-width="170">
          <template #default="{ row }">
            <span>{{ row.name }}</span>
            <el-tag v-if="row.isSystem" size="small" class="system-tag">系统</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" width="100">
          <template #default="{ row }">V{{ row.currentRevision }}</template>
        </el-table-column>
        <el-table-column label="主要规格" min-width="230">
          <template #default="{ row }">
            <span v-if="latestVersion(row)">
              {{ latestVersion(row)?.uHeight }}U · {{ latestVersion(row)?.widthMm }}×{{
                latestVersion(row)?.depthMm
              }}×{{ latestVersion(row)?.heightMm }}mm
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="电源/PDU" min-width="150">
          <template #default="{ row }">
            {{ latestVersion(row)?.dualPower ? "双路" : "单路/未配置" }} · PDU
            {{ latestVersion(row)?.pduCount ?? 0 }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'">
              {{ row.status === "ACTIVE" ? "启用" : "停用" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              :disabled="row.status !== 'ACTIVE'"
              @click="openVersion(row)"
            >
              新版本
            </el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" :disabled="row.isSystem" @click="remove(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !list.length" description="暂无机柜模板" />
    </el-card>

    <!-- 新增/编辑模板 -->
    <el-dialog
      v-model="dialogVisible"
      :title="mode === 'create' ? '新增机柜模板' : '编辑模板信息'"
      width="920px"
      destroy-on-close
    >
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="10">
            <el-form-item label="模板编码" :required="mode === 'edit' || !autoGenCode">
              <div class="code-field">
                <el-input
                  v-model="editingForm.code"
                  maxlength="50"
                  :disabled="mode === 'create' && autoGenCode"
                  :placeholder="
                    mode === 'create' && autoGenCode ? '保存时自动生成' : '请输入模板编码'
                  "
                />
                <el-checkbox v-if="mode === 'create'" v-model="autoGenCode">自动生成</el-checkbox>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="模板名称" required>
              <el-input v-model="editingForm.name" maxlength="150" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="状态">
              <el-select
                v-model="editingForm.status"
                style="width: 100%"
                :disabled="systemEditLocked"
              >
                <el-option label="启用" value="ACTIVE" />
                <el-option label="停用" value="DISABLED" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="说明">
          <el-input v-model="editingForm.description" type="textarea" :rows="2" />
        </el-form-item>
        <template v-if="mode === 'create'">
          <el-divider content-position="left">初始版本参数</el-divider>
          <TemplateSpecForm v-model="current" />
        </template>
        <el-form-item label="备注">
          <el-input v-model="editingForm.remarks" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
    </el-dialog>

    <!-- 发布新版本 -->
    <el-dialog
      v-model="versionVisible"
      :title="`发布新版本 · ${editing?.name ?? ''}`"
      width="820px"
      destroy-on-close
    >
      <template #footer>
        <el-button @click="versionVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="publishVersion">发布版本</el-button>
      </template>
      <el-alert
        :title="`将创建 V${nextRevision}，历史版本不会被覆盖。`"
        type="warning"
        show-icon
        :closable="false"
      />
      <el-form label-width="110px" class="version-form">
        <TemplateSpecForm v-model="current" />
      </el-form>
    </el-dialog>
  </div>
</template>

<style scoped>
/* 与 v2 RackTemplateView-BsPAlSep.css 逐条对应 */
.template-page {
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
.table-card {
  margin-top: 16px;
}
.system-tag {
  margin-left: 8px;
}
.version-form {
  margin-top: 18px;
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
</style>
