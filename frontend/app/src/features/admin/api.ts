/**
 * 管理模块只读查询(第 5 轮 P1-D 三权限对照的最小切片)。
 * 用户列表 GET /admin/users:admin 角色可见;user 角色由后端 403 +
 * 前端路由守卫(meta.requiresAdmin)双重拦截。
 */
import type { components } from "@/api/generated/schema";
import { api, unwrapData } from "@/api/client";

export type AdminUser = components["schemas"]["User"];

export async function fetchAdminUsers(params?: { search?: string }): Promise<AdminUser[]> {
  const data = await unwrapData(
    await api.GET("/api/v1/admin/users", { params: params ? { query: params } : undefined }),
    "GET /api/v1/admin/users",
  );
  return data.items ?? [];
}
