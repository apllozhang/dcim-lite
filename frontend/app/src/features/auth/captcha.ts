/** SVG 验证码解析:从 base64 SVG 提取数字(与后端 renderCaptchaSVG 的 <text>d</text> 结构对应)。 */
export function parseCaptchaCode(svgBase64: string): string {
  const raw = svgBase64.includes(",") ? svgBase64.split(",", 2)[1] : svgBase64;
  const svg = atob(raw);
  const matches = svg.matchAll(/>(\d)<\/text>/g);
  return [...matches].map((m) => m[1]).join("");
}
