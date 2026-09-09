# dcim-lite

Lightweight DCIM backend for data-center room / rack / device inventory.

**Self-hosted · Go + PostgreSQL · Docker-ready · API-first**

中文：面向机房资源台账的轻量 DCIM 后端（数据中心 → 机房 → 机柜 → 设备 / U 位）。

---

## Features

| Area | Capability |
| --- | --- |
| Hierarchy | Data center → Room → Rack CRUD, copy/move, resource tree |
| Devices | Types & inventory, lifecycle, assign / move / decommission |
| U-slots | Exclusive occupancy (DB exclusion + service checks), position history |
| Templates | Versioned rack templates, snapshot on rack create |
| PDU | PDUs, sockets, device power connections |
| Admin | Users & roles, last-admin protection, reset password |
| Approval | Optional assign/move approval policy |
| Import | Rack-diagram two-phase import (validate → commit) |
| Auth | JWT + 4-digit captcha on login |

## Stack

- Go 1.24+, Gin, GORM, PostgreSQL 16
- JWT (HS256), bcrypt
- Numbered SQL migrations applied at startup

## Quick start (Docker)

```bash
cp .env.example .env
# set DATABASE_URL, JWT_SECRET, ADMIN_PASSWORD
docker compose up -d --build
curl -sS http://127.0.0.1:8080/health/ready
```

## Quick start (local)

```bash
export DATABASE_URL='postgres://user:pass@127.0.0.1:5432/dcimlite?sslmode=disable'
export JWT_SECRET='change-me-to-a-long-random-secret-at-least-32-bytes'
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD='ChangeMe#Now1!'
export MIGRATIONS_DIR=migrations
go run ./cmd/server
```

## API (summary)

- Auth: `POST /api/v1/auth/captcha`, `POST /api/v1/auth/login`, `GET /api/v1/auth/me`
- Tree / list: `GET /api/v1/resource-tree`, `GET /api/v1/racks-page`
- Resources: data-centers / rooms / racks CRUD + copy/move
- Devices: CRUD, assign/move/decommission, `GET /api/v1/racks/:id/u-layout`
- Templates & PDU; admin users/roles/approvals/ldap
- `GET /api/v1/devices/import-template` (xlsx)

Envelope: `{ "code", "message", "data", "requestId" }` + header `X-Request-Id`.

## Frontends

| Path | Description |
| --- | --- |
| `frontend/clean/` | Open-source UI (vanilla JS): login + captcha, tree, racks, devices |
| `frontend/ale/` | Internal ALE-skinned package — **gitignored**, do not publish |

See `frontend/README.md`.

## Smoke test

```bash
BASE=http://127.0.0.1:8080 ENV_FILE=.env bash scripts/smoke-all.sh
```

## Layout

```text
cmd/server/     entrypoint
internal/       config, middleware, domain, handlers, repository
migrations/     versioned SQL
scripts/        gen-env + smoke
deploy/         sample nginx
```

## Security

- Strong `JWT_SECRET` and admin password; never commit `.env`
- Keep PostgreSQL on a private network; put TLS/reverse proxy in front if exposed
- Captcha is basic anti-automation, not a full WAF

## Scope

**Lite** DCIM: inventory, racks, U-slots, power.  
Not full enterprise DCIM (no auto-discovery, env monitoring, or ITSM).

## License

Choose a license before publishing (Apache-2.0 / MIT / proprietary).
