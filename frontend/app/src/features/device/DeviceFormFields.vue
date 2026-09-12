<script setup lang="ts">
/**
 * 设备表单字段(第6轮 屏3 设备台账——100%复刻 v2 DeviceFormFields-B18Ou5kv):
 * 六段37字段,按编译产物的字段名/标签/验证逐项对齐;
 * 与 v2 差异:不含 id/createdAt/updatedAt/type/currentPosition(后端管理)。
 * 类型与表单段定义在 deviceFormSchema.ts(<script setup> 不能 export)。
 */
import { LIFECYCLE_STATUS_OPTIONS } from "@/features/device/statusLabel";
import {
  DEVICE_FIELD_LABELS as FIELD_LABELS,
  FORM_SECTIONS,
} from "@/features/device/deviceFormSchema";
import type { DeviceForm } from "@/features/device/deviceFormSchema";

defineProps<{
  form: DeviceForm;
  types: { id?: string; code?: string; name?: string }[];
  mode: "create" | "edit";
  loading: boolean;
  positioned: boolean;
  autoCode: boolean;
}>();

const emit = defineEmits<{
  "update:form": [v: DeviceForm];
  "update:autoCode": [v: boolean];
}>();
</script>

<template>
  <div v-loading="loading" class="device-form-fields">
    <el-alert
      v-if="positioned"
      title="设备已上架：U 高度和位置相关状态受后端保护；如需调整 U 高度，请通过迁移功能重新校验机柜空间。"
      type="warning"
      :closable="false"
      show-icon
    />
    <section v-for="s in FORM_SECTIONS" :key="s.title">
      <h4>{{ s.title }}</h4>
      <div v-if="s.title === '基本信息'" class="form-grid">
        <el-form-item :label="FIELD_LABELS.typeId" required>
          <el-select
            style="width: 100%"
            filterable
            :model-value="form.typeId"
            @update:model-value="emit('update:form', { ...form, typeId: $event })"
          >
            <el-option v-for="t in types" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="FIELD_LABELS.code" :required="mode === 'edit' || !autoCode">
          <div class="code-field">
            <el-input
              :model-value="form.code"
              maxlength="80"
              :disabled="mode === 'create' && autoCode"
              :placeholder="mode === 'create' && autoCode ? '保存时自动生成' : '请输入设备编码'"
              @update:model-value="emit('update:form', { ...form, code: $event })"
            />
            <el-checkbox
              v-if="mode === 'create'"
              :model-value="autoCode"
              @update:model-value="emit('update:autoCode', !!$event)"
            >
              自动生成
            </el-checkbox>
          </div>
        </el-form-item>
        <el-form-item :label="FIELD_LABELS.name" required>
          <el-input
            :model-value="form.name"
            maxlength="150"
            @update:model-value="emit('update:form', { ...form, name: $event })"
          />
        </el-form-item>
        <el-form-item :label="FIELD_LABELS.lifecycleStatus">
          <el-select
            style="width: 100%"
            :model-value="form.lifecycleStatus"
            @update:model-value="emit('update:form', { ...form, lifecycleStatus: $event })"
          >
            <el-option
              v-for="o in LIFECYCLE_STATUS_OPTIONS"
              :key="o.value"
              :label="o.label"
              :value="o.value"
              :disabled="
                positioned
                  ? ['WAITING_RACK', 'OFF_RACK', 'SCRAPPED'].includes(o.value)
                  : o.value === 'RUNNING'
              "
            />
          </el-select>
        </el-form-item>
      </div>
      <!-- 资产与规格 2列网格 -->
      <div v-else-if="s.title === '资产与规格'" class="form-grid">
        <el-form-item v-for="k in s.keys" :key="k" :label="FIELD_LABELS[k]">
          <el-date-picker
            v-if="k === 'warrantyExpiresAt'"
            style="width: 100%"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="选择日期"
            clearable
            :model-value="(form as any)[k]"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
          <el-input
            v-else
            :model-value="(form as any)[k]"
            :maxlength="k === 'specification' ? 500 : 120"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
        </el-form-item>
      </div>
      <!-- 尺寸与电力 3列网格 -->
      <div v-else-if="s.title === '尺寸与电力'" class="form-grid form-grid--three">
        <el-form-item
          v-for="k in s.keys"
          :key="k"
          :label="FIELD_LABELS[k]"
          :required="k === 'heightU'"
        >
          <el-switch
            v-if="k === 'dualPowerRequired'"
            :model-value="(form as any)[k]"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
          <el-input-number
            v-else
            :model-value="(form as any)[k]"
            :min="k === 'heightU' ? 1 : 0"
            :max="k === 'heightU' ? 100 : undefined"
            :precision="
              ['weightKg', 'ratedPowerW', 'peakPowerW', 'inputVoltage'].includes(k) ? 2 : 0
            "
            :disabled="k === 'heightU' && positioned"
            controls-position="right"
            style="width: 100%"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
        </el-form-item>
      </div>
      <!-- 组织与业务 2列 -->
      <div v-else-if="s.title === '组织与业务'" class="form-grid">
        <el-form-item v-for="k in s.keys" :key="k" :label="FIELD_LABELS[k]">
          <el-input
            :model-value="(form as any)[k]"
            :maxlength="k === 'organization' ? 150 : 100"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
        </el-form-item>
      </div>
      <!-- 网络与管理 2列 -->
      <div v-else-if="s.title === '网络与管理'" class="form-grid">
        <el-form-item v-for="k in s.keys" :key="k" :label="FIELD_LABELS[k]">
          <el-input
            :model-value="(form as any)[k]"
            maxlength="64"
            :placeholder="k === 'managementProtocol' ? '如 SSH、SNMP、HTTPS' : undefined"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
        </el-form-item>
      </div>
      <!-- 扩展信息 2列 -->
      <div v-else class="form-grid">
        <el-form-item v-for="k in s.keys" :key="k" :label="FIELD_LABELS[k]">
          <el-input
            v-if="k === 'remarks'"
            :model-value="(form as any)[k]"
            type="textarea"
            :rows="3"
            maxlength="500"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
          <el-input
            v-else
            :model-value="(form as any)[k]"
            :maxlength="k === 'tags' ? 500 : 255"
            :placeholder="k === 'tags' ? '多个标签可使用逗号分隔' : undefined"
            @update:model-value="emit('update:form', { ...form, [k]: $event })"
          />
        </el-form-item>
      </div>
    </section>
  </div>
</template>

<style scoped>
.device-form-fields {
  max-height: 64vh;
  overflow-y: auto;
  padding-right: 8px;
}
section {
  margin-bottom: 16px;
}
h4 {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 24px;
}
.form-grid--three {
  grid-template-columns: 1fr 1fr 1fr;
}
.code-field {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}
.code-field .el-input {
  flex: 1;
}
</style>
