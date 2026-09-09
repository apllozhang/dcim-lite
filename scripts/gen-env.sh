#!/usr/bin/env bash
# Generate a one-time .env for the rebuild stack on the server.
# Usage: ./scripts/gen-env.sh > .env && chmod 600 .env
set -euo pipefail
rand() { openssl rand -base64 36 | tr -d '/+=' | cut -c1-32; }
cat <<EOF
APP_ENV=production
POSTGRES_HOST_PORT=15433
BACKEND_HOST_PORT=18080
POSTGRES_PASSWORD=$(rand)
JWT_SECRET=$(rand)$(rand)
ADMIN_USERNAME=admin
ADMIN_PASSWORD=$(rand)Rebuild#1
JWT_TTL_SECONDS=7200
EOF
