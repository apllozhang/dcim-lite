/**
 * 错误上报(P1-R05):错误边界 ≠ 错误监控。
 * - 可替换:默认 console,生产接真实采集时 setErrorReporter 换实现;
 * - 字段白名单:release/route/message/apiStatus/apiCode/requestId/role(匿名化);
 *   实现方不得读取 token/密码/验证码等敏感值,上报载荷只含本接口字段。
 */
export interface ErrorReport {
  release: string;
  route: string;
  message: string;
  apiStatus?: number;
  apiCode?: string;
  requestId?: string;
  /** 匿名角色(system_admin/user/anonymous),不含用户名等标识 */
  role?: string;
}

export interface ErrorReporter {
  report(e: ErrorReport): void;
}

declare const __APP_RELEASE__: string;

export const APP_RELEASE: string = typeof __APP_RELEASE__ !== "undefined" ? __APP_RELEASE__ : "dev";

const consoleReporter: ErrorReporter = {
  report(e) {
    // 生产接真实采集前的落点;保留完整字段便于排查
    console.error("[ErrorReporter]", JSON.stringify(e));
  },
};

let active: ErrorReporter = consoleReporter;

export function setErrorReporter(reporter: ErrorReporter): void {
  active = reporter;
}

export function reportError(
  partial: Omit<ErrorReport, "release" | "route"> & { route?: string },
): void {
  active.report({
    release: APP_RELEASE,
    route: partial.route ?? (typeof location !== "undefined" ? location.pathname : ""),
    message: partial.message,
    apiStatus: partial.apiStatus,
    apiCode: partial.apiCode,
    requestId: partial.requestId,
    role: partial.role,
  });
}
