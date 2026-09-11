/**
 * 错误上报(P1-R05;P1-R6 升级为远程落点):
 * - 双写:console 保留本地排查;remote reporter 火后不理地 POST 到
 *   /api/v1/telemetry/frontend-errors(后端字段白名单 + 环形缓冲,admin 接口检索);
 * - 字段白名单:release/route/message/apiStatus/apiCode/requestId/role(匿名化);
 *   实现方不得读取 token/密码/验证码等敏感值,上报载荷只含本接口字段;
 * - 告警阈值基线(部署侧按此配置规则):错误率 >5%/分钟、白屏(空 window.onerror 捕获)>
 *   0、401 激增 >10/分钟、接口 5xx >10/分钟。
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

const TELEMETRY_ENDPOINT = "/api/v1/telemetry/frontend-errors";

const consoleReporter: ErrorReporter = {
  report(e) {
    // 本地排查落点;远程上报失败时它是唯一痕迹
    console.error("[ErrorReporter]", JSON.stringify(e));
  },
};

/** 远程 reporter:白名单字段直发后端遥测端点;任何失败只回退 console,不再抛错 */
const remoteReporter: ErrorReporter = {
  report(e) {
    consoleReporter.report(e);
    try {
      void fetch(TELEMETRY_ENDPOINT, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          release: e.release,
          route: e.route,
          message: e.message,
          apiStatus: e.apiStatus,
          apiCode: e.apiCode,
          requestId: e.requestId,
          role: e.role,
        }),
        signal: AbortSignal.timeout(5000),
      }).catch(() => {});
    } catch {
      /* 上报通道异常绝不放大原错误 */
    }
  },
};

let active: ErrorReporter = remoteReporter;

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

/** 受控异常触发点:仅 E2E/排障使用,注入一条标记化上报用于端到端检索验证 */
declare global {
  interface Window {
    __ALE_FORCE_REPORT__?: (marker: string) => void;
  }
}
if (typeof window !== "undefined") {
  window.__ALE_FORCE_REPORT__ = (marker: string) => {
    reportError({ message: `e2e controlled exception: ${marker}` });
  };
}
