# 多阶段构建:Vite 源码构建 → nginx 运行(P0-R3 部署即代码)
# 构建上下文 = frontend/app;产物含 security headers 与 SPA/legacy 路由策略(见 deploy/frontend/nginx-default.conf)
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# APP_RELEASE:构建时注入(git sha 或镜像 tag),供 ErrorReporter release 字段使用
ARG APP_RELEASE=dev
ENV APP_RELEASE=$APP_RELEASE
RUN npm run build

FROM nginx:alpine
ARG APP_RELEASE=dev
LABEL org.opencontainers.image.title="dcim-lite ale-app" \
      org.opencontainers.image.revision="${APP_RELEASE}" \
      org.opencontainers.image.source="https://github.com/apllozhang/dcim-lite"
# nginx 配置由 compose 以只读卷挂载(deploy/frontend/nginx-default.conf),便于审计与单独演进
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=10s --timeout=3s --retries=10 \
  CMD wget -q -O /dev/null http://127.0.0.1/login || exit 1
