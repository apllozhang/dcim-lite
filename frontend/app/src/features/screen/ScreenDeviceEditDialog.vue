<script setup lang="ts">
/**
 * 大屏设备编辑对话框(第 7 轮 UI-P1-03;复刻 v2 RoomScreenView Bn/Wn 口径):
 * el-dialog 980px class screen-device-editor-dialog append-to-body destroy-on-close,
 * 打开时并行拉详情+类型表,复用 DeviceFormFields(positioned 在位约束),
 * 保存经 updateDevice 后 emit saved(父组件原地更新设备块,不整屏重载)。
 */
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  fetchDevice,
  fetchDeviceTypes,
  updateDevice,
  type Device,
  type DeviceType,
} from "@/features/resource/api";
import DeviceFormFields from "@/features/device/DeviceFormFields.vue";
import { defaultForm } from "@/features/device/deviceFormSchema";
import type { DeviceForm } from "@/features/device/deviceFormSchema";
import { reportError } from "@/api/errors";

const props = defineProps<{ modelValue: boolean; deviceId: string }>();
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "saved", device: Device): void;
}>();
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const types = ref<DeviceType[]>([]);
const form = ref<DeviceForm>(defaultForm());
const saving = ref(false);
const loading = ref(false);
const meta = reactive({ version: 1, positioned: false });
/** 服务端可空扩展字段(schema 未声明但详情返回) */
function ext<T>(obj: unknown, key: string): T | undefined {
  return (obj as Record<string, unknown> | null)?.[key] as T | undefined;
}

watch(
  () => [props.modelValue, props.deviceId] as const,
  async ([open, id]) => {
    if (!open || !id) return;
    loading.value = true;
    visible.value = true;
    try {
      const [full, typeList] = await Promise.all([fetchDevice(id), fetchDeviceTypes()]);
      types.value = typeList;
      meta.version = full.version ?? 1;
      meta.positioned = !!full.currentPosition;
      form.value = {
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
    } catch (e) {
      ElMessage.error("获取设备详情失败");
      reportError({
        message: `screen device edit load: ${e instanceof Error ? e.message : String(e)}`,
      });
      visible.value = false;
    } finally {
      loading.value = false;
    }
  },
  { immediate: true },
);

async function save() {
  const f = form.value;
  if (!f.typeId) {
    ElMessage.warning("请选择设备类型");
    return;
  }
  if (!f.code) {
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
  saving.value = true;
  try {
    // PUT:后端强制保持 lifecycleStatus(service/device.go:396),与编辑页同口径不携带
    await updateDevice(props.deviceId, meta.version, { ...f, lifecycleStatus: undefined });
    ElMessage.success("设备已保存");
    const saved = (await fetchDevice(props.deviceId)) as Device;
    emit("saved", saved);
    visible.value = false;
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : "保存失败");
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    title="编辑设备信息"
    width="980px"
    class="screen-device-editor-dialog"
    append-to-body
    destroy-on-close
  >
    <div v-loading="loading" class="screen-device-editor-body">
      <DeviceFormFields
        :form="form"
        :types="types"
        mode="edit"
        :loading="loading"
        :positioned="meta.positioned"
        :auto-code="false"
        @update:form="form = $event"
      />
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" data-test="screen-device-save-btn" @click="save">
        保存
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.screen-device-editor-body {
  min-height: 200px;
}
</style>
