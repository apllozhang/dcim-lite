import { describe, expect, it } from "vitest";
import { toTreeNodes } from "@/features/resource/api";

describe("toTreeNodes(P0-R02:两级子字段规整为统一 children)", () => {
  it("maps dc→rooms→racks into label/children nodes", () => {
    const nodes = toTreeNodes([
      {
        id: "dc1",
        code: "DC1",
        name: "一号数据中心",
        rooms: [
          {
            id: "rm1",
            code: "R1",
            name: "机房一",
            racks: [{ id: "rk1", code: "K1", name: "机柜一" }],
          },
        ],
      },
    ]);
    expect(nodes).toHaveLength(1);
    expect(nodes[0].kind).toBe("dc");
    expect(nodes[0].children?.[0].label).toBe("机房一");
    expect(nodes[0].children?.[0].kind).toBe("room");
    expect(nodes[0].children?.[0].children?.[0]).toMatchObject({
      id: "rk1",
      label: "机柜一",
      kind: "rack",
    });
  });

  it("tolerates empty/missing arrays", () => {
    const nodes = toTreeNodes([{ id: "dc2", code: "DC2", name: "空" }]);
    expect(nodes[0].children ?? []).toHaveLength(0);
  });
});
