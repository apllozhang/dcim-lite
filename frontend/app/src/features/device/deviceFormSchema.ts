/**
 * 设备表单类型与工具函数(第6轮 屏3——从 DeviceFormFields 拆出避免 setup export)
 */
export interface DeviceForm {
  typeId: string;
  code: string;
  name: string;
  lifecycleStatus: string;
  assetNumber: string;
  serialNumber: string;
  manufacturer: string;
  modelNumber: string;
  specification: string;
  firmwareVersion: string;
  purchaseBatch: string;
  warrantyExpiresAt: string;
  heightU: number;
  widthMm?: number;
  depthMm?: number;
  heightMm?: number;
  weightKg?: number;
  ratedPowerW?: number;
  peakPowerW?: number;
  inputVoltage?: number;
  dualPowerRequired: boolean;
  organization: string;
  manager: string;
  contact: string;
  businessSystem: string;
  applicationName: string;
  managementIp: string;
  businessIp: string;
  macAddress: string;
  managementProtocol: string;
  monitoringStatus: string;
  externalQrCode: string;
  externalQrCodeUrl: string;
  tags: string;
  remarks: string;
}

export function defaultForm(): DeviceForm {
  return {
    typeId: "",
    code: "",
    name: "",
    lifecycleStatus: "WAITING_RACK",
    assetNumber: "",
    serialNumber: "",
    manufacturer: "",
    modelNumber: "",
    specification: "",
    firmwareVersion: "",
    purchaseBatch: "",
    warrantyExpiresAt: "",
    heightU: 1,
    widthMm: undefined,
    depthMm: undefined,
    heightMm: undefined,
    weightKg: undefined,
    ratedPowerW: undefined,
    peakPowerW: undefined,
    inputVoltage: undefined,
    dualPowerRequired: false,
    organization: "",
    manager: "",
    contact: "",
    businessSystem: "",
    applicationName: "",
    managementIp: "",
    businessIp: "",
    macAddress: "",
    managementProtocol: "",
    monitoringStatus: "",
    externalQrCode: "",
    externalQrCodeUrl: "",
    tags: "",
    remarks: "",
  };
}

export const DEVICE_FIELD_LABELS: Record<string, string> = {
  typeId: "设备类型",
  code: "设备编码",
  name: "设备名称",
  assetNumber: "资产编号",
  serialNumber: "序列号",
  manufacturer: "厂商",
  modelNumber: "型号",
  specification: "规格",
  firmwareVersion: "固件版本",
  purchaseBatch: "采购批次",
  warrantyExpiresAt: "保修到期日",
  organization: "所属组织",
  manager: "负责人",
  contact: "联系方式",
  businessSystem: "业务系统",
  applicationName: "应用名称",
  lifecycleStatus: "生命周期状态",
  heightU: "U 高度",
  widthMm: "宽度(mm)",
  depthMm: "深度(mm)",
  heightMm: "设备高度(mm)",
  weightKg: "重量(kg)",
  ratedPowerW: "额定功率(W)",
  peakPowerW: "峰值功率(W)",
  inputVoltage: "输入电压(V)",
  dualPowerRequired: "双路电源",
  managementIp: "管理 IP",
  businessIp: "业务 IP",
  macAddress: "MAC 地址",
  managementProtocol: "管理协议",
  monitoringStatus: "监控状态",
  externalQrCode: "外部二维码",
  externalQrCodeUrl: "二维码链接",
  tags: "标签",
  remarks: "备注",
};

export const FORM_SECTIONS = [
  { title: "基本信息", keys: ["typeId", "code", "name", "lifecycleStatus"] },
  {
    title: "资产与规格",
    keys: [
      "assetNumber",
      "serialNumber",
      "manufacturer",
      "modelNumber",
      "specification",
      "firmwareVersion",
      "purchaseBatch",
      "warrantyExpiresAt",
    ],
  },
  {
    title: "尺寸与电力",
    keys: [
      "heightU",
      "widthMm",
      "depthMm",
      "heightMm",
      "weightKg",
      "ratedPowerW",
      "peakPowerW",
      "inputVoltage",
      "dualPowerRequired",
    ],
  },
  {
    title: "组织与业务",
    keys: ["organization", "manager", "contact", "businessSystem", "applicationName"],
  },
  {
    title: "网络与管理",
    keys: ["managementIp", "businessIp", "macAddress", "managementProtocol", "monitoringStatus"],
  },
  { title: "扩展信息", keys: ["externalQrCode", "externalQrCodeUrl", "tags", "remarks"] },
];
