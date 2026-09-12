import { describe, expect, it } from "vitest";
import {
  autoSlot,
  categoryLabel,
  deviceColor,
  rackCardHeight,
  rackPositionText,
  rowTops,
  shortName,
  statusColor,
  statusLabel,
} from "@/features/screen/screenShared";

describe("screenShared(机房大屏口径,复刻 v2)", () => {
  it("状态中文与色板(v2 ie/ke 映射)", () => {
    expect(statusLabel("AVAILABLE")).toBe("空闲");
    expect(statusLabel("PARTIAL")).toBe("部分使用");
    expect(statusLabel("FULL")).toBe("已满");
    expect(statusLabel("PLANNING")).toBe("规划中");
    expect(statusLabel("MAINTENANCE")).toBe("维护中");
    expect(statusLabel("DISABLED")).toBe("停用");
    expect(statusLabel("OTHER")).toBe("OTHER");
    expect(statusColor("AVAILABLE")).toBe("#24d6a1");
    expect(statusColor("MAINTENANCE")).toBe("#a78bfa");
    expect(statusColor("WHAT")).toBe("#7bdfff");
  });

  it("设备分类色与中文(v2 Xe/Vl 口径)", () => {
    expect(deviceColor("SERVER")).toBe("#2d8cff");
    expect(deviceColor("POWER_ENVIRONMENT")).toBe("#ef6470");
    expect(deviceColor(undefined)).toBe("#5f86a5");
    expect(categoryLabel("SERVER")).toBe("服务器");
    expect(categoryLabel("ACCESSORY")).toBe("附件");
    expect(categoryLabel(undefined)).toBe("其他设备");
  });

  it("卡内短名 >9 字截断(v2 Zn)", () => {
    expect(shortName("abc")).toBe("abc");
    expect(shortName("1234567890")).toBe("12345678 …");
    expect(shortName(null)).toBe("");
  });

  it("机柜卡高度 = 152 + U数×9(v2 $t)", () => {
    expect(rackCardHeight({ uHeight: 42 })).toBe(152 + 42 * 9);
    expect(rackCardHeight({ uHeight: 0 })).toBe(152);
  });

  it("行主序槽位与累计行顶(v2 st/Je)", () => {
    expect(autoSlot(0, 8)).toEqual({ row: 0, column: 0 });
    expect(autoSlot(7, 8)).toEqual({ row: 0, column: 7 });
    expect(autoSlot(8, 8)).toEqual({ row: 1, column: 0 });
    const tops = rowTops([530, 200]);
    expect(tops[0]).toBe(72);
    expect(tops[1]).toBe(72 + 530 + 32);
  });

  it("位置文案:区域/排/列/通道,自动排布兜底(v2 oe;zone 不加'区'字,与机柜页 ke 不同)", () => {
    expect(rackPositionText({ zone: "A", rackRow: "01", rackColumn: "02", aisle: "冷通道" })).toBe(
      "A · 01排 · 02列 · 冷通道",
    );
    expect(rackPositionText({})).toBe("自动排布");
  });
});
