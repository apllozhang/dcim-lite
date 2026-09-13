import { describe, expect, it } from "vitest";
import {
  buildRackIndex,
  deviceExportAoA,
  deviceExportFileName,
  deviceExportRow,
  EXPORT_HEADERS,
} from "@/features/device/deviceExport";
import type { Device } from "@/features/resource/api";

const DEV = {
  id: "d1",
  code: "SRV-001",
  name: "应用服务器001",
  typeId: "t1",
  type: { id: "t1", name: "服务器", category: "SERVER" },
  lifecycleStatus: "RUNNING",
  assetNumber: "ASSET-1",
  managementIp: "192.168.10.21",
  dualPowerRequired: true,
  heightU: 2,
  currentPosition: {
    startU: 10,
    endU: 11,
    rack: { id: "r1", code: "RACK-A-01", name: "A区01号机柜" },
  },
  remarks: "备注X",
} as unknown as Device;

const TREE = [
  {
    name: "北京数据中心",
    rooms: [{ name: "A机房", racks: [{ id: "r1" }] }],
  },
];

describe("设备导出", () => {
  it("表头 43 列与 v2 Hl 集合一致", () => {
    expect(EXPORT_HEADERS).toHaveLength(43);
    expect(EXPORT_HEADERS[2]).toBe("设备类型");
    expect(EXPORT_HEADERS[22]).toBe("管理IP");
    expect(EXPORT_HEADERS[35]).toBe("数据中心");
    expect(EXPORT_HEADERS[41]).toBe("上架状态");
  });

  it("行构造:类型/分类中文/生命周期中文/位置三段/上架状态", () => {
    const row = deviceExportRow(DEV, buildRackIndex(TREE));
    expect(row).toHaveLength(43);
    expect(row[0]).toBe("SRV-001");
    expect(row[2]).toBe("服务器");
    expect(row[3]).toBe("服务器"); // CATEGORY_CN
    expect(row[4]).toBe("运行中");
    expect(row[22]).toBe("192.168.10.21");
    expect(row[35]).toBe("北京数据中心");
    expect(row[36]).toBe("A机房");
    expect(row[37]).toBe("RACK-A-01");
    expect(row[39]).toBe(10);
    expect(row[40]).toBe(11);
    expect(row[41]).toBe("已上架");
  });

  it("未上架设备:位置段为空,上架状态=未上架;缺 type 时回退类型表", () => {
    const bare = {
      ...DEV,
      type: undefined,
      currentPosition: undefined,
      lifecycleStatus: "WAITING_RACK",
    } as unknown as Device;
    const row = deviceExportRow(bare, new Map(), {
      id: "t1",
      name: "服务器",
      category: "SERVER",
    });
    expect(row[2]).toBe("服务器"); // typeById 回退
    expect(row[37]).toBe("");
    expect(row[41]).toBe("未上架");
    expect(row[4]).toBe("待上架");
  });

  it("AoA 首行表头 + 每行 43 列", () => {
    const aoa = deviceExportAoA([DEV], buildRackIndex(TREE));
    expect(aoa[0]).toEqual(EXPORT_HEADERS);
    expect(aoa[1]).toHaveLength(43);
  });

  it("文件名:v2 Xl 口径(时间戳+数量)", () => {
    const name = deviceExportFileName(new Date(2026, 8, 13, 9, 5), 7);
    expect(name).toBe("设备信息_20260913-0905_7台.xlsx");
  });
});
