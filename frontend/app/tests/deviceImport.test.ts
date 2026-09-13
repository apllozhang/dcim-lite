import { describe, expect, it } from "vitest";
import {
  isValidIp,
  matchColumn,
  normalizeLifecycle,
  parseBool,
  rowsFromMatrix,
  validateImportRows,
  type ImportDeviceLite,
  type ImportTypeLite,
} from "@/features/device/deviceImport";

const TYPES: ImportTypeLite[] = [
  {
    id: "t1",
    code: "SERVER",
    name: "服务器",
    status: "ACTIVE",
    defaultHeightU: 2,
    defaultDualPower: true,
    defaultRatedPowerW: 500,
  },
  { id: "t2", code: "NET", name: "交换机", status: "DISABLED", defaultHeightU: 1 },
];

const DEV = (over: Partial<ImportDeviceLite>): ImportDeviceLite => ({
  id: "d1",
  code: "SRV-001",
  name: "旧名",
  version: 3,
  serialNumber: "SN-1",
  assetNumber: "A-1",
  lifecycleStatus: "RUNNING",
  heightU: 2,
  typeId: "t1",
  ...over,
});

describe("表头与枚举归一", () => {
  it("表头按 title/别名/key 匹配", () => {
    expect(matchColumn("设备类型")).toBe("typeCode"); // v2 别名
    expect(matchColumn("管理IP")).toBe("managementIp");
    expect(matchColumn("设备名称")).toBe("name");
    expect(matchColumn("规格")).toBe("specification");
    expect(matchColumn("typeCode")).toBe("typeCode");
    expect(matchColumn("未知列")).toBeNull();
  });

  it("表头容忍模板的（必填）/（可选）后缀", () => {
    expect(matchColumn("设备类型编码（必填）")).toBe("typeCode");
    expect(matchColumn("资产编号（可选）")).toBe("assetNumber");
    expect(matchColumn("设备名称（必填）")).toBe("name");
  });

  it("生命周期支持中文与英文", () => {
    expect(normalizeLifecycle("运行中")).toBe("RUNNING");
    expect(normalizeLifecycle("OFF_RACK")).toBe("OFF_RACK");
    expect(normalizeLifecycle("随便")).toBeNull();
    expect(normalizeLifecycle("")).toBeNull();
  });

  it("布尔与 IP 校验", () => {
    expect(parseBool("是")).toBe(true);
    expect(parseBool("FALSE")).toBe(false);
    expect(parseBool("1")).toBe(true);
    expect(parseBool("2")).toBeNull();
    expect(isValidIp("192.168.10.21")).toBe(true);
    expect(isValidIp("256.1.1.1")).toBe(false);
    expect(isValidIp("::1")).toBe(true);
    expect(isValidIp("abc")).toBe(false);
  });
});

describe("rowsFromMatrix", () => {
  it("按表头构造行对象并跳过全空行", () => {
    const [rows, unknown] = rowsFromMatrix([
      ["设备类型", "设备编码", "设备名称", "乱列"],
      ["SERVER", "C1", "名一", "x"],
      ["", "", "", ""],
      ["SERVER", "", "名二", ""],
    ]);
    expect(unknown).toEqual(["乱列"]);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toEqual({ typeCode: "SERVER", code: "C1", name: "名一" });
  });
});

describe("validateImportRows", () => {
  it("新增行:typeCode 必填、类型匹配、默认值、自动编码摘要", () => {
    const [r] = validateImportRows([{ typeCode: "SERVER", name: "新设备" }], {
      types: TYPES,
      devices: [],
    });
    expect(r.errors).toEqual([]);
    expect(r.operation).toBe("CREATE");
    expect(r.summary).toBe("自动编码 · 新设备");
    expect(r.payload).toMatchObject({
      typeId: "t1",
      name: "新设备",
      heightU: 2, // 类型默认高度
      dualPowerRequired: true, // 类型默认双路
      ratedPowerW: 500, // 类型默认功率
      lifecycleStatus: "WAITING_RACK",
    });
  });

  it("新增行缺 typeCode/名称 报错;停用类型不匹配", () => {
    const [a, b, c] = validateImportRows(
      [{ name: "无类型" }, { typeCode: "SERVER", name: "" }, { typeCode: "NET", name: "停用类型" }],
      { types: TYPES, devices: [] },
    );
    expect(a.errors).toContain("设备类型编码不能为空");
    expect(b.errors).toContain("设备名称不能为空");
    expect(c.errors).toContain("启用的设备类型编码或名称 NET 不存在");
  });

  it("按编码匹配更新:空白字段保持原值,状态字段不携带(后端保持原状态)", () => {
    const [r] = validateImportRows([{ typeCode: "服务器", code: "srv-001", name: "新名" }], {
      types: TYPES,
      devices: [DEV({})],
    });
    expect(r.operation).toBe("UPDATE");
    expect(r.errors).toEqual([]);
    expect(r.summary).toBe("更新 · SRV-001（旧名）");
    expect(r.payload).toMatchObject({
      id: "d1",
      version: 3,
      name: "新名",
      assetNumber: "A-1", // 留空保持原值
      serialNumber: "SN-1",
    });
    expect(r.payload?.lifecycleStatus).toBeUndefined();
  });

  it("编码未匹配但序列号已匹配 → 冲突报错", () => {
    const [r] = validateImportRows(
      [{ typeCode: "SERVER", code: "NEW", name: "x", serialNumber: "SN-1" }],
      { types: TYPES, devices: [DEV({})] },
    );
    expect(r.errors.join()).toContain("设备编码未匹配到已有设备");
  });

  it("编码匹配与序列号指向不同设备 → 交叉冲突", () => {
    const [r] = validateImportRows(
      [{ typeCode: "SERVER", code: "SRV-001", name: "x", serialNumber: "SN-2" }],
      {
        types: TYPES,
        devices: [DEV({}), DEV({ id: "d2", code: "SRV-002", serialNumber: "SN-2" })],
      },
    );
    expect(r.errors.join()).toContain("请确认是新设备还是原设备改编码");
  });

  it("Excel 内编码重复与同设备多行更新都被拦截", () => {
    const [a, b] = validateImportRows(
      [
        { typeCode: "SERVER", code: "DUP", name: "甲" },
        { typeCode: "SERVER", code: "dup", name: "乙" },
      ],
      { types: TYPES, devices: [] },
    );
    expect(a.errors.join()).toContain("Excel 内设备编码重复");
    expect(b.errors.join()).toContain("Excel 内设备编码重复");

    const [c, d] = validateImportRows(
      [
        { typeCode: "SERVER", serialNumber: "SN-1", name: "甲" },
        { typeCode: "SERVER", assetNumber: "A-1", name: "乙" },
      ],
      { types: TYPES, devices: [DEV({})] },
    );
    expect(c.errors.join()).toContain("同一设备被多行同时更新");
    expect(d.errors.join()).toContain("同一设备被多行同时更新");
    expect(c.payload).toBeNull();
  });

  it("格式校验:IP/高度/负数/布尔", () => {
    const [r] = validateImportRows(
      [
        {
          typeCode: "SERVER",
          name: "x",
          managementIp: "999.1.1.1",
          heightU: "0",
          weightKg: "-3",
          dualPowerRequired: "可能",
        },
      ],
      { types: TYPES, devices: [] },
    );
    const joined = r.errors.join();
    expect(joined).toContain("管理IP格式无效");
    expect(joined).toContain("设备高度U");
    expect(joined).toContain("weightKg");
    expect(joined).toContain("需要双路电源");
  });
});
