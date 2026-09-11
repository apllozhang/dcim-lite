import { describe, expect, it } from "vitest";
import { parseCaptchaCode } from "@/features/auth/captcha";

describe("parseCaptchaCode", () => {
  it("extracts digits from the captcha SVG", () => {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg"><text x="10" y="20">4</text><text x="30" y="20">9</text><text x="50" y="20">0</text><text x="70" y="20">9</text></svg>`;
    const b64 = Buffer.from(svg).toString("base64");
    expect(parseCaptchaCode(`data:image/svg+xml;base64,${b64}`)).toBe("4909");
  });

  it("returns empty string for svg without digits", () => {
    const svg = `<svg xmlns="http://www.w3.org/2000/svg"></svg>`;
    const b64 = Buffer.from(svg).toString("base64");
    expect(parseCaptchaCode(b64)).toBe("");
  });
});
