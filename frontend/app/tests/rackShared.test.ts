import { describe, expect, it, vi } from "vitest";
import {
  autoCode,
  isConflictError,
  latestVersion,
  submitWithAutoCode,
} from "@/features/rack/rackShared";

describe("autoCode 并发安全(UI-P1-04)", () => {
  it("格式:前缀-时间戳base36-随机段", () => {
    const c = autoCode("RACK");
    expect(c).toMatch(/^RACK-[0-9A-Z]+-[0-9A-Z]{7}$/);
  });

  it("大批量生成无碰撞(同毫秒也由随机段区分)", () => {
    const seen = new Set<string>();
    for (let i = 0; i < 20000; i += 1) seen.add(autoCode("RACK"));
    expect(seen.size).toBe(20000);
  });
});

describe("isConflictError", () => {
  it("识别 409 状态与编码已存在文案", () => {
    expect(isConflictError({ status: 409 })).toBe(true);
    expect(isConflictError({ response: { status: 409 } })).toBe(true);
    expect(isConflictError({ message: "机柜编码已存在" })).toBe(true);
    expect(isConflictError({ response: { data: { message: "code already exists" } } })).toBe(true);
    expect(isConflictError({ message: "not found" })).toBe(false);
    expect(isConflictError(new Error("网络错误"))).toBe(false);
  });
});

describe("submitWithAutoCode", () => {
  it("自动编码:唯一冲突时重生成编码重试一次", async () => {
    const codes: string[] = [];
    let calls = 0;
    await submitWithAutoCode(
      true,
      () => {
        const c = `RACK-${codes.length}`;
        codes.push(c);
        return c;
      },
      async (code) => {
        calls += 1;
        if (code === "RACK-0") {
          const e = new Error("conflict") as Error & { status: number };
          e.status = 409;
          throw e;
        }
      },
    );
    expect(codes).toEqual(["RACK-0", "RACK-1"]);
    expect(calls).toBe(2);
  });

  it("手工编码:冲突直接抛出不重试", async () => {
    const gen = vi.fn(() => "FIXED");
    const submit = vi.fn(async () => {
      throw new Error("已存在");
    });
    await expect(submitWithAutoCode(false, gen, submit)).rejects.toThrow("已存在");
    expect(gen).toHaveBeenCalledTimes(1);
    expect(submit).toHaveBeenCalledTimes(1);
  });

  it("非冲突错误不重试", async () => {
    const submit = vi.fn(async () => {
      throw new Error("服务器内部错误");
    });
    await expect(submitWithAutoCode(true, () => "C", submit)).rejects.toThrow("服务器内部错误");
    expect(submit).toHaveBeenCalledTimes(1);
  });
});

describe("latestVersion 回归", () => {
  it("按 revision 降序取最新", () => {
    const v = latestVersion({
      versions: [{ revision: 2 }, { revision: 5 }, { revision: 1 }],
    } as never);
    expect(v?.revision).toBe(5);
  });
});
