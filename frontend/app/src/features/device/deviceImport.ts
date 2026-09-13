/**
 * 设备批量导入纯函数(第 7 轮 UI-P0-01;规格逐条复刻 v2 DeviceManagementView 编译产物):
 *  - 35 列模板字段 st 同序,别名表(如"设备类型"→typeCode)同 v2;
 *  - 匹配规则同 v2 Oe 口径:系统内重复库检查 → 设备编码唯一匹配(更新) →
 *    编码未匹配时序列号/资产编号联合识别,交叉冲突拒绝;
 *  - 更新载荷 = 现有设备全字段为基座,行内非空字段覆盖(空白可选字段保持原值);
 *  - 新增:类型编码必填,heightU/双路电源留空取类型默认值,编码留空由后端自动生成。
 * 本模块不依赖 xlsx 与网络,供 DeviceImportDialog 与单元测试共用。
 */
import { LIFECYCLE_STATUS_OPTIONS } from "@/features/device/statusLabel";

export interface ImportField {
  key: string;
  title: string;
  aliases?: string[];
  required: boolean;
  description: string;
  example: string;
  width: number;
}

/** 模板字段(v2 st 同序同文案) */
export const DEVICE_IMPORT_FIELDS: ImportField[] = [
  {
    key: "typeCode",
    title: "设备类型编码",
    aliases: ["设备类型"],
    required: true,
    description: "必须与系统中启用的设备类型编码或名称匹配;导出的设备信息可直接再次导入",
    example: "SERVER",
    width: 22,
  },
  {
    key: "code",
    title: "设备编码",
    required: false,
    description:
      "可选;填写已有编码会更新已有设备,填写新编码会新增设备,留空时根据序列号/资产编号匹配,否则由系统自动生成",
    example: "SRV-APP-001",
    width: 22,
  },
  {
    key: "name",
    title: "设备名称",
    required: true,
    description: "最多 150 个字符;更新时留空保持原值",
    example: "应用服务器001",
    width: 24,
  },
  {
    key: "lifecycleStatus",
    title: "生命周期状态",
    required: false,
    description: "支持英文状态值或系统显示名称;设备信息导入不会改变 U 位",
    example: "WAITING_RACK",
    width: 24,
  },
  {
    key: "heightU",
    title: "设备高度U",
    required: false,
    description: "1-100 的整数;新增时留空使用设备类型默认高度,更新时留空保持原值",
    example: "2",
    width: 18,
  },
  {
    key: "assetNumber",
    title: "资产编号",
    required: false,
    description: "建议设置为文本格式;可用于识别已有设备",
    example: "ASSET-2026-001",
    width: 22,
  },
  {
    key: "serialNumber",
    title: "序列号",
    required: false,
    description: "建议设置为文本格式;可用于识别已有设备",
    example: "SN20260831001",
    width: 22,
  },
  {
    key: "manufacturer",
    title: "厂商",
    required: false,
    description: "设备厂商",
    example: "示例厂商",
    width: 16,
  },
  {
    key: "modelNumber",
    title: "型号",
    required: false,
    description: "设备型号",
    example: "MODEL-X2",
    width: 16,
  },
  {
    key: "specification",
    title: "规格描述",
    aliases: ["规格"],
    required: false,
    description: "CPU、内存、端口等规格",
    example: "2U/双路CPU/256G内存",
    width: 24,
  },
  {
    key: "firmwareVersion",
    title: "固件版本",
    required: false,
    description: "设备固件版本",
    example: "v2.1.0",
    width: 14,
  },
  {
    key: "purchaseBatch",
    title: "采购批次",
    required: false,
    description: "采购批次或合同号",
    example: "2026-B01",
    width: 16,
  },
  {
    key: "warrantyExpiresAt",
    title: "保修到期日",
    required: false,
    description: "格式 YYYY-MM-DD",
    example: "2029-12-31",
    width: 16,
  },
  {
    key: "dualPowerRequired",
    title: "需要双路电源",
    aliases: ["是否双路供电"],
    required: false,
    description: "是/否、TRUE/FALSE、1/0;新增时留空使用类型默认值,更新时留空保持原值",
    example: "是",
    width: 20,
  },
  {
    key: "managementIp",
    title: "管理IP",
    required: false,
    description: "合法 IPv4 或 IPv6 地址",
    example: "192.168.10.21",
    width: 20,
  },
  {
    key: "businessIp",
    title: "业务IP",
    required: false,
    description: "合法 IPv4 或 IPv6 地址",
    example: "10.10.20.21",
    width: 20,
  },
  {
    key: "macAddress",
    title: "MAC地址",
    required: false,
    description: "建议格式 AA:BB:CC:DD:EE:FF",
    example: "00:11:22:33:44:55",
    width: 22,
  },
  {
    key: "managementProtocol",
    title: "管理协议",
    required: false,
    description: "SSH、SNMP、IPMI 等",
    example: "SSH",
    width: 14,
  },
  {
    key: "monitoringStatus",
    title: "监控状态",
    required: false,
    description: "接入监控的情况",
    example: "已接入",
    width: 14,
  },
  {
    key: "externalQrCode",
    title: "外部二维码",
    required: false,
    description: "外部资产二维码编号",
    example: "",
    width: 16,
  },
  {
    key: "externalQrCodeUrl",
    title: "二维码链接",
    required: false,
    description: "二维码指向的 URL",
    example: "",
    width: 20,
  },
  {
    key: "tags",
    title: "标签",
    required: false,
    description: "多个标签可使用逗号分隔",
    example: "生产,核心",
    width: 16,
  },
  {
    key: "organization",
    title: "所属组织",
    aliases: ["组织"],
    required: false,
    description: "资产归属组织",
    example: "技术中心",
    width: 16,
  },
  {
    key: "manager",
    title: "负责人",
    required: false,
    description: "设备负责人",
    example: "张三",
    width: 12,
  },
  {
    key: "contact",
    title: "联系方式",
    aliases: ["联系电话"],
    required: false,
    description: "电话或邮箱",
    example: "13800000000",
    width: 16,
  },
  {
    key: "businessSystem",
    title: "业务系统",
    required: false,
    description: "承载的业务系统",
    example: "计费系统",
    width: 16,
  },
  {
    key: "applicationName",
    title: "应用名称",
    required: false,
    description: "部署的应用",
    example: "billing-app",
    width: 16,
  },
  {
    key: "widthMm",
    title: "宽度mm",
    required: false,
    description: "毫米整数,不能为负数",
    example: "482",
    width: 12,
  },
  {
    key: "depthMm",
    title: "深度mm",
    required: false,
    description: "毫米整数,不能为负数",
    example: "600",
    width: 12,
  },
  {
    key: "heightMm",
    title: "高度mm",
    required: false,
    description: "毫米整数,不能为负数",
    example: "88",
    width: 12,
  },
  {
    key: "weightKg",
    title: "重量kg",
    required: false,
    description: "千克,不能为负数",
    example: "12.5",
    width: 12,
  },
  {
    key: "ratedPowerW",
    title: "额定功率W",
    aliases: ["额定功耗W"],
    required: false,
    description: "不能为负数;新增时留空可使用类型默认值,更新时留空保持原值",
    example: "500",
    width: 14,
  },
  {
    key: "peakPowerW",
    title: "峰值功率W",
    aliases: ["峰值功耗W"],
    required: false,
    description: "不能为负数;更新时留空保持原值",
    example: "1200",
    width: 14,
  },
  {
    key: "inputVoltage",
    title: "输入电压V",
    required: false,
    description: "伏特,不能为负数",
    example: "220",
    width: 14,
  },
  {
    key: "remarks",
    title: "备注",
    required: false,
    description: "补充说明",
    example: "批量导入示例",
    width: 24,
  },
];

/** 提交载荷里的普通文本字段(空白不覆盖;v2 sl 同集合) */
const TEXT_KEYS = [
  "assetNumber",
  "serialNumber",
  "manufacturer",
  "modelNumber",
  "specification",
  "firmwareVersion",
  "purchaseBatch",
  "warrantyExpiresAt",
  "organization",
  "manager",
  "contact",
  "businessSystem",
  "applicationName",
  "managementIp",
  "businessIp",
  "macAddress",
  "managementProtocol",
  "monitoringStatus",
  "externalQrCode",
  "externalQrCodeUrl",
  "tags",
  "remarks",
] as const;

/** 提交载荷里的数值字段(空白不覆盖;v2 ul 同集合) */
const NUMBER_KEYS = [
  "widthMm",
  "depthMm",
  "heightMm",
  "weightKg",
  "ratedPowerW",
  "peakPowerW",
  "inputVoltage",
] as const;

/** 生命周期:英文枚举 → 中文(复用全局口径);导入反向映射在此派生 */
const LIFECYCLE_BY_CN = new Map<string, string>(
  LIFECYCLE_STATUS_OPTIONS.map((o) => [o.label, o.value] as const),
);

/** 导入行生命周期归一:英文枚举或中文显示名 → 枚举;无法识别返回 null */
export function normalizeLifecycle(input: string): string | null {
  const v = input.trim();
  if (!v) return null;
  if (LIFECYCLE_BY_CN.has(v) || LIFECYCLE_STATUS_OPTIONS.some((o) => o.value === v)) {
    return LIFECYCLE_BY_CN.get(v) ?? v;
  }
  return null;
}

/** "是/否、TRUE/FALSE、1/0" → boolean;其它 null */
export function parseBool(input: string): boolean | null {
  const v = input.trim().toUpperCase();
  if (["是", "TRUE", "1", "Y", "YES"].includes(v)) return true;
  if (["否", "FALSE", "0", "N", "NO"].includes(v)) return false;
  return null;
}

/** IPv4/IPv6 宽松校验(v2 lt 口径:点分十进制 0-255 或冒号十六进制) */
export function isValidIp(v: string): boolean {
  const s = v.trim();
  if (!s) return false;
  if (s.includes(":")) {
    return /^[0-9A-Fa-f:]+$/.test(s) && s.includes(":") && (s.match(/:/g) ?? []).length >= 2;
  }
  const parts = s.split(".");
  if (parts.length !== 4) return false;
  return parts.every((p) => /^\d{1,3}$/.test(p) && Number(p) >= 0 && Number(p) <= 255);
}

/** 非负数解析;非法/负数返回 null */
export function parseNonNegative(input: string): number | null {
  const s = input.trim();
  if (!s) return null;
  const n = Number(s);
  return Number.isFinite(n) && n >= 0 ? n : null;
}

/** 匹配/校验上下文:启用的设备类型 + 系统全部设备(导入前全量拉取) */
export interface ImportDeviceLite {
  id: string;
  code?: string | null;
  name?: string | null;
  version?: number;
  serialNumber?: string | null;
  assetNumber?: string | null;
  lifecycleStatus?: string | null;
  heightU?: number | null;
  typeId?: string | null;
  [k: string]: unknown;
}
export interface ImportTypeLite {
  id?: string | null;
  code?: string | null;
  name?: string | null;
  status?: string | null;
  defaultHeightU?: number | null;
  defaultDualPower?: boolean | null;
  defaultRatedPowerW?: number | null;
}

export interface ImportRowResult {
  rowNumber: number;
  operation: "" | "CREATE" | "UPDATE";
  matchedDeviceId?: string;
  summary: string;
  errors: string[];
  payload: Record<string, unknown> | null;
}

const empty = (v: unknown): boolean => v === undefined || v === null || String(v).trim() === "";
const text = (v: unknown): string => (empty(v) ? "" : String(v).trim());
const eq = (a?: string | null, b?: string | null): boolean =>
  text(a).toUpperCase() === text(b).toUpperCase() && text(a) !== "";

/** 类型匹配:v2 ql 口径——先编码后名称,仅启用状态 */
export function matchType(typeCode: string, types: ImportTypeLite[]): ImportTypeLite | undefined {
  const key = typeCode.trim();
  if (!key) return undefined;
  const active = types.filter((t) => t.status === "ACTIVE");
  return active.find((t) => eq(t.code, key)) ?? active.find((t) => eq(t.name, key));
}

/**
 * 匹配已有设备(v2 Oe 精确口径):
 * 系统内重复库 → 编码唯一匹配 → 交叉冲突 → 序列号/资产编号联合识别。
 * 返回 [匹配设备, 错误列表]。
 */
export function matchExistingDevice(
  row: Record<string, string>,
  devices: ImportDeviceLite[],
): [ImportDeviceLite | undefined, string[]] {
  const errors: string[] = [];
  const code = text(row.code);
  const sn = text(row.serialNumber);
  const asset = text(row.assetNumber);
  const byCode = code ? devices.filter((d) => eq(d.code, code)) : [];
  const bySn = sn ? devices.filter((d) => eq(d.serialNumber, sn)) : [];
  const byAsset = asset ? devices.filter((d) => eq(d.assetNumber, asset)) : [];
  if (byCode.length > 1) errors.push("系统中存在重复设备编码，无法安全更新");
  if (bySn.length > 1) errors.push("系统中存在重复序列号，无法安全识别设备");
  if (byAsset.length > 1) errors.push("系统中存在重复资产编号，无法安全识别设备");
  if (errors.length) return [undefined, errors];
  const snAsset = [...bySn, ...byAsset].filter(
    (d, i, arr) => arr.findIndex((x) => x.id === d.id) === i,
  );
  if (byCode.length === 1 && snAsset.some((d) => d.id !== byCode[0].id)) {
    errors.push(
      `设备编码匹配“${byCode[0].code}”，但序列号或资产编号匹配其他设备，请确认是新设备还是原设备改编码`,
    );
    return [undefined, errors];
  }
  if (byCode.length === 1) return [byCode[0], errors];
  if (code && snAsset.length) {
    errors.push(
      "设备编码未匹配到已有设备，但序列号或资产编号已匹配已有设备，请确认是新设备还是原设备改编码",
    );
    return [undefined, errors];
  }
  if (snAsset.length > 1) {
    errors.push("序列号与资产编号分别匹配到不同设备，无法安全导入");
    return [undefined, errors];
  }
  return [snAsset[0], errors];
}

/** 设备显示名(摘要用):CODE（名称） */
function deviceLabel(d: ImportDeviceLite): string {
  return `${d.code ?? ""}${d.name ? `（${d.name}）` : ""}`;
}

/** 单行校验+载荷构造(v2 行级规则逐条) */
function buildRow(
  rowNumber: number,
  row: Record<string, string>,
  types: ImportTypeLite[],
  devices: ImportDeviceLite[],
): ImportRowResult {
  const errors: string[] = [];
  const [matched, matchErrors] = matchExistingDevice(row, devices);
  errors.push(...matchErrors);
  const operation: "CREATE" | "UPDATE" = matched ? "UPDATE" : "CREATE";

  const typeCode = text(row.typeCode);
  const type = typeCode ? matchType(typeCode, types) : undefined;
  if (typeCode) {
    if (!type) errors.push(`启用的设备类型编码或名称 ${typeCode} 不存在`);
  } else if (operation === "CREATE") {
    errors.push("设备类型编码不能为空");
  }

  const code = text(row.code);
  if (code.length > 80) errors.push("设备编码不能超过 80 个字符");
  const name = text(row.name);
  if (name.length > 150) errors.push("设备名称不能超过 150 个字符");
  if (!name && operation === "CREATE") errors.push("设备名称不能为空");

  const lifecycle = normalizeLifecycle(text(row.lifecycleStatus));
  if (text(row.lifecycleStatus) && !lifecycle) {
    errors.push(`生命周期状态 ${text(row.lifecycleStatus)} 无效`);
  }

  let heightU: number | undefined;
  if (!empty(row.heightU)) {
    const n = Number(text(row.heightU));
    if (!Number.isInteger(n) || n < 1 || n > 100) {
      errors.push("设备高度U必须是 1 到 100 的整数");
    } else {
      heightU = n;
    }
  }

  for (const [key, label] of [
    ["managementIp", "管理IP"],
    ["businessIp", "业务IP"],
  ] as const) {
    if (text(row[key]) && !isValidIp(row[key])) errors.push(`${label}格式无效`);
  }

  const nums: Record<string, number> = {};
  for (const key of NUMBER_KEYS) {
    if (empty(row[key])) continue;
    const n = parseNonNegative(row[key]);
    if (n === null) errors.push(`${String(key)}必须是不小于 0 的数字`);
    else nums[key] = n;
  }

  let dual: boolean | undefined;
  if (!empty(row.dualPowerRequired)) {
    const b = parseBool(row.dualPowerRequired);
    if (b === null) errors.push("需要双路电源填写 是/否、TRUE/FALSE 或 1/0");
    else dual = b;
  }

  const summary = matched
    ? `更新 · ${deviceLabel(matched)}`
    : code
      ? `新增 · ${code}`
      : `自动编码 · ${name || "未填写设备名称"}`;

  if (errors.length) {
    return { rowNumber, operation, summary, errors, payload: null, matchedDeviceId: matched?.id };
  }

  // 载荷:更新以现有设备为基座(空白保持原值);新增按类型默认补齐
  const payload: Record<string, unknown> = matched
    ? { ...(matched as Record<string, unknown>), version: matched.version ?? 1 }
    : {};
  if (type) payload.typeId = type.id;
  if (code) payload.code = code;
  if (name) payload.name = name;
  // 契约差异(dcim-lite 后端 PUT 设备强制保持原状态,service/device.go:396):
  // 更新行不携带 lifecycleStatus(与单设备编辑页同口径),状态流转只能走上架/下架。
  if (operation === "CREATE") payload.lifecycleStatus = lifecycle ?? "WAITING_RACK";
  else delete payload.lifecycleStatus;
  if (heightU !== undefined) payload.heightU = heightU;
  else if (operation === "CREATE") payload.heightU = type?.defaultHeightU ?? 1;
  if (dual !== undefined) payload.dualPowerRequired = dual;
  else if (operation === "CREATE") payload.dualPowerRequired = type?.defaultDualPower ?? false;
  if (operation === "CREATE" && nums.ratedPowerW === undefined && type?.defaultRatedPowerW) {
    payload.ratedPowerW = type.defaultRatedPowerW;
  }
  for (const key of NUMBER_KEYS) if (nums[key] !== undefined) payload[key] = nums[key];
  for (const key of TEXT_KEYS) if (text(row[key])) payload[key] = text(row[key]);
  if (operation === "CREATE") {
    delete payload.id;
    delete payload.currentPosition;
    delete payload.type;
    delete payload.createdAt;
    delete payload.updatedAt;
  }

  return { rowNumber, operation, summary, errors: [], payload, matchedDeviceId: matched?.id };
}

/** 批量校验入口:逐行构造 + 跨行冲突(同设备多行更新/Excel 内标识重复) */
export function validateImportRows(
  rows: Record<string, string>[],
  ctx: { types: ImportTypeLite[]; devices: ImportDeviceLite[] },
): ImportRowResult[] {
  const results = rows.map((row, i) => buildRow(i + 2, row, ctx.types, ctx.devices)); // 行1=表头

  // Excel 内标识重复(v2 同口径)
  const dup = (key: string, label: string) => {
    const seen = new Map<string, number[]>();
    rows.forEach((r, i) => {
      const v = text(r[key]).toUpperCase();
      if (v) seen.set(v, [...(seen.get(v) ?? []), results[i].rowNumber]);
    });
    for (const lineNos of seen.values()) {
      if (lineNos.length > 1) {
        for (const rn of lineNos) {
          results[rn - 2].errors.push(`Excel 内${label}重复(第 ${lineNos.join("、")} 行)`);
          results[rn - 2].payload = null;
        }
      }
    }
  };
  dup("code", "设备编码");
  dup("serialNumber", "序列号");
  dup("assetNumber", "资产编号");

  // 同一设备被多行同时匹配更新
  const byDevice = new Map<string, number[]>();
  for (const r of results) {
    if (r.matchedDeviceId)
      byDevice.set(r.matchedDeviceId, [...(byDevice.get(r.matchedDeviceId) ?? []), r.rowNumber]);
  }
  for (const lineNos of byDevice.values()) {
    if (lineNos.length > 1) {
      for (const rn of lineNos) {
        results[rn - 2].errors.push(
          `同一设备被多行同时更新(第 ${lineNos.join("、")} 行),请合并为一行`,
        );
        results[rn - 2].payload = null;
      }
    }
  }
  return results;
}

/** 表头 → 字段 key(精确 title → alias → key 本身;大小写不敏感)。
 * 模板表头带"（必填）/（可选）"标记,匹配前剔除(与机柜 CSV 解析同口径) */
export function matchColumn(header: string): string | null {
  const h = header
    .trim()
    .replace(/（(必填|可选)）$/, "")
    .trim()
    .toUpperCase();
  if (!h) return null;
  const byTitle = DEVICE_IMPORT_FIELDS.find((f) => f.title.toUpperCase() === h);
  if (byTitle) return byTitle.key;
  const byAlias = DEVICE_IMPORT_FIELDS.find((f) =>
    (f.aliases ?? []).some((a) => a.toUpperCase() === h),
  );
  if (byAlias) return byAlias.key;
  const byKey = DEVICE_IMPORT_FIELDS.find((f) => f.key.toUpperCase() === h);
  return byKey ? byKey.key : null;
}

/**
 * 工作表二维数组 → 行对象列表。
 * 首行表头逐列匹配;无法识别的表头列忽略;全空行跳过。
 * 返回 [行对象列表, 无法识别的表头]。
 */
export function rowsFromMatrix(matrix: unknown[][]): [Record<string, string>[], string[]] {
  const headers = (matrix[0] ?? []).map((c) => String(c ?? ""));
  const unknownHeaders: string[] = [];
  const colKeys: (string | null)[] = headers.map((h) => {
    const k = matchColumn(h);
    if (!k && h.trim()) unknownHeaders.push(h.trim());
    return k;
  });
  const rows: Record<string, string>[] = [];
  for (const line of matrix.slice(1)) {
    const obj: Record<string, string> = {};
    let hasValue = false;
    colKeys.forEach((key, c) => {
      if (!key) return;
      const v = String(line[c] ?? "").trim();
      if (v) hasValue = true;
      obj[key] = v;
    });
    if (hasValue) rows.push(obj);
  }
  return [rows, unknownHeaders];
}
