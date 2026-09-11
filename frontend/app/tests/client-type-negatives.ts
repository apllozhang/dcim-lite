/**
 * P1-R4 类型负例:强类型客户端的契约约束以"编译失败"验证。
 * 这些用例由 vue-tsc(npm run build)检查而非 vitest:
 *   - 若某负例不再报错(约束被削弱),@ts-expect-error 变为"未使用指令"→ 构建失败;
 *   - 若误报,后续行为由单测/门禁兜底。
 * 每个负例对应第四轮复评 P1-R4 的验收要求。
 */
import { api } from "@/api/client";

/** 负例 1:GET 专用路径用于 POST → 编译失败 */
export function postOnGetPath() {
  // @ts-expect-error /api/v1/auth/me 仅声明 GET,不得用于 POST
  return api.POST("/api/v1/auth/me", { body: {} });
}

/** 负例 2:未知路径 → 编译失败(keyof paths 约束) */
export function unknownPath() {
  // @ts-expect-error 路径不在 OpenAPI paths 内
  return api.GET("/api/v1/definitely-not-a-route");
}

/** 负例 3:query 参数类型错误 → 编译失败(page 为 number) */
export function wrongQueryParamType() {
  // @ts-expect-error page 为 number,传 string 必须编译失败
  return api.GET("/api/v1/devices", { params: { query: { page: "not-a-number" } } });
}

/** 正例锚点:同一调用在正确类型下必须通过且响应被推导 */
export async function typedCallAnchor(): Promise<void> {
  const res = await api.GET("/api/v1/devices", { params: { query: { page: 1 } } });
  if (res.data?.data?.items) {
    // items 元素类型来自生成 schema,不存在则说明推导断裂
    const first = res.data.data.items[0];
    void first?.code;
  }
}
