// jsdom 环境补齐:Element Plus 部分组件依赖 ResizeObserver
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
globalThis.ResizeObserver =
  globalThis.ResizeObserver ?? (ResizeObserverStub as unknown as typeof ResizeObserver);

// P1-R4:openapi-fetch 依赖 fetch/Request/Response/AbortSignal。
// Node 22 全局已提供;若 jsdom 环境遮蔽导致缺失,这里显式暴露,缺失即测试快速失败。
for (const key of ["fetch", "Request", "Response", "AbortSignal", "AbortController"]) {
  const g = globalThis as unknown as Record<string, unknown>;
  if (typeof g[key] === "undefined") {
    throw new Error(
      `tests/setup.ts: jsdom 环境缺少全局 ${key},openapi-fetch 无法工作,请在 vitest 配置中补齐 polyfill`,
    );
  }
}
