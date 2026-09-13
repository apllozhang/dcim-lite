<script setup lang="ts">
/**
 * 大屏设备详情抽屉(第 7 轮 UI-P1-03;复刻 v2 RoomScreenView On 口径):
 * el-drawer 720px append-to-body,标题"设备详细信息",内容分组
 * 基本信息/资产与规格/网络与管理/组织与业务/当前位置,footer 关闭+下架设备。
 */
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { fetchDevice, type Device } from "@/features/resource/api";
import { lifecycleLabel } from "@/features/device/statusLabel";

const props = defineProps<{ modelValue: boolean; deviceId: string; reloadToken?: number }>();
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "offline", device: Device): void;
}>();
const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const detail = ref<Device | null>(null);
const loading = ref(false);
watch(
  () => [props.modelValue, props.deviceId, props.reloadToken ?? 0] as const,
  async ([open, id]) => {
    if (!open || !id) return;
    loading.value = true;
    detail.value = null;
    try {
      detail.value = await fetchDevice(id);
    } catch {
      ElMessage.error("获取设备详情失败");
      visible.value = false;
    } finally {
      loading.value = false;
    }
  },
  { immediate: true },
);

const pos = computed(() => detail.value?.currentPosition ?? null);
const groups = computed(() => {
  const d = detail.value;
  if (!d) return [];
  const v = (x: unknown) => (x === null || x === undefined || x === "" ? "—" : String(x));
  const typeName = (d as unknown as { type?: { name?: string } }).type?.name ?? v(d.typeId);
  return [
    {
      title: "基本信息",
      items: [
        ["设备编码", v(d.code)],
        ["设备名称", v(d.name)],
        ["设备类型", typeName],
        ["生命周期状态", lifecycleLabel(d.lifecycleStatus) || v(d.lifecycleStatus)],
        ["设备高度U", `${d.heightU ?? "—"}U`],
        ["双路电源", d.dualPowerRequired ? "是" : "否"],
      ],
    },
    {
      title: "资产与规格",
      items: [
        ["资产编号", v(d.assetNumber)],
        ["序列号", v(d.serialNumber)],
        ["厂商", v(d.manufacturer)],
        ["型号", v(d.modelNumber)],
        ["规格", v(d.specification)],
        ["固件版本", v(d.firmwareVersion)],
        ["采购批次", v(d.purchaseBatch)],
        ["保修到期日", v(d.warrantyExpiresAt)],
      ],
    },
    {
      title: "网络与管理",
      items: [
        ["管理 IP", v(d.managementIp)],
        ["业务 IP", v(d.businessIp)],
        ["MAC 地址", v(d.macAddress)],
        ["管理协议", v(d.managementProtocol)],
        ["监控状态", v(d.monitoringStatus)],
      ],
    },
    {
      title: "组织与业务",
      items: [
        ["所属组织", v(d.organization)],
        ["负责人", v(d.manager)],
        ["联系方式", v(d.contact)],
        ["业务系统", v(d.businessSystem)],
        ["应用名称", v(d.applicationName)],
        ["标签", v(d.tags)],
      ],
    },
    {
      title: "当前位置",
      items: [
        [
          "机柜",
          v(pos.value?.rack?.name ? `${pos.value.rack.name}（${pos.value.rack.code}）` : null),
        ],
        ["U 位区间", pos.value ? `U${pos.value.startU}-${pos.value.endU}` : "—"],
        ["上架时间", v(pos.value?.installedAt)],
        ["备注", v(d.remarks)],
      ],
    },
  ];
});

function offline() {
  if (detail.value) emit("offline", detail.value);
}
</script>

<template>
  <el-drawer
    v-model="visible"
    title="设备详细信息"
    size="720px"
    class="screen-detail-drawer"
    append-to-body
    destroy-on-close
  >
    <div v-loading="loading" class="device-detail-body">
      <template v-if="detail">
        <section v-for="g in groups" :key="g.title" class="detail-group">
          <h4>{{ g.title }}</h4>
          <div class="detail-grid">
            <div v-for="[label, value] in g.items" :key="label" class="detail-item">
              <span class="detail-label">{{ label }}</span>
              <span class="detail-value">{{ value }}</span>
            </div>
          </div>
        </section>
      </template>
    </div>
    <template #footer>
      <div class="drawer-footer">
        <el-button @click="visible = false">关闭</el-button>
        <el-button v-if="detail" type="danger" @click="offline">下架设备</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.device-detail-body {
  min-height: 160px;
}
.detail-group {
  margin-bottom: 18px;
}
.detail-group h4 {
  margin: 0 0 10px;
  font-size: 14px;
  color: #111827;
  border-left: 3px solid #1f4e78;
  padding-left: 8px;
}
.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 20px;
}
.detail-item {
  display: flex;
  gap: 10px;
  font-size: 13px;
  line-height: 22px;
}
.detail-label {
  flex: 0 0 76px;
  color: #6b7280;
}
.detail-value {
  color: #111827;
  word-break: break-all;
}
.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
