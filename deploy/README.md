# ale-app 部署即代码(P0-R3)

19500 独立前端的全部部署定义。验收口径:**一台干净主机,只用仓库文件与本目录声明的
环境输入,即可重建等价栈**,不依赖操作者记忆或服务器上未归档配置。

## 目录

| 文件 | 作用 |
|---|---|
| docker-compose.ale.yml | 单服务栈:ale-app(构建+运行),外接上游网络 |
| frontend/nginx-default.conf | 反代与静态策略:/api /health /metrics /legacy /assets 回落 SPA fallback + 安全头 |
| ../frontend/app/Dockerfile | 多阶段:node:22-alpine 构建 → nginx:alpine 运行 |
| .env.example | 环境输入声明(真实值不入仓库) |
| smoke.sh | 部署后冒烟(5 路,含旧页面标题与旧静态资源 200) |

## 首次部署(干净主机)

```bash
git clone <repo> && cd dcim-lite/deploy
cp .env.example .env        # 填 ALE_UPSTREAM_NETWORK / ALE_APP_PORT / APP_RELEASE
# 密钥注入说明:本栈无密钥;上游 backend 的 JWT/口令由后端栈 .env 管理,不经过本栈
docker compose -f docker-compose.ale.yml -p ale-app up -d --build
./smoke.sh "http://127.0.0.1:${ALE_APP_PORT}"
```

## 升级

```bash
cd dcim-lite && git pull
cd deploy
APP_RELEASE=<new-git-sha> docker compose -f docker-compose.ale.yml -p ale-app up -d --build
./smoke.sh "http://127.0.0.1:${ALE_APP_PORT}"
```

镜像以 `dcim-ale-app:<APP_RELEASE>` 留存,历史 tag 即回滚点。

## 回滚

```bash
APP_RELEASE=<previous-git-sha> docker compose -f docker-compose.ale.yml -p ale-app up -d
./smoke.sh "http://127.0.0.1:${ALE_APP_PORT}"
```

镜像若已被清理:`docker compose build` 检出旧 commit 重建(tag 取自该 commit sha)。

## 版本与 digest 规则

- `APP_RELEASE` 必须 = git commit sha(7+ 位),与 `docker image inspect dcim-ale-app:<sha>`
  的 `org.opencontainers.image.revision` 标签一致;
- 发布记录登记:commit sha + 镜像 sha256(RepoDigest 对本地构建为空,用 Image Id +
  构建时间戳;接入镜像仓库后改用 RepoDigest);
- CI 的 nightly field gate 同样上报候选镜像 digest,两处口径一致。

## 备份与恢复

- 本栈无状态(html/conf 均来自仓库或卷挂载),恢复 = 重新 `up -d --build`;
- 上游 backend/postgres 的备份恢复见后端栈文档(与 203 既有流程一致)。

## 与现网 19500 的关系

现网 ale-app 容器由本定义重建(2026-09-12 执行,见提交记录):旧的手工
`docker run` + 宿主机 ~/ale-app-dist 目录形态废弃;升级一律走本目录。
