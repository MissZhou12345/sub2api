#!/bin/bash
# 本地/服务器部署：拉取镜像并重启 docker-compose.local.yml 栈
set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f .env ]]; then
  echo "[ERROR] missing deploy/.env" >&2
  exit 1
fi

if [[ ! -f .version ]]; then
  echo "[ERROR] missing deploy/.version" >&2
  exit 1
fi

# 旧版 docker-compose 通常只支持一个 --env-file，BUILD_VERSION 从 .version 注入环境
set -a
# shellcheck disable=SC1091
source <(grep -E '^[A-Z_][A-Z0-9_]*=' .version | sed 's/\r$//')
set +a

if [[ -z "${BUILD_VERSION:-}" ]]; then
  echo "[ERROR] BUILD_VERSION not set in .version" >&2
  exit 1
fi

# 优先 docker compose (v2)，否则 docker-compose (v1)
if docker compose version >/dev/null 2>&1; then
  DC=(docker compose -f docker-compose.local.yml --env-file .env)
else
  DC=(docker-compose -f docker-compose.local.yml --env-file .env)
fi

echo "[INFO] BUILD_VERSION=${BUILD_VERSION}"
echo "[INFO] Running: ${DC[*]} ..."

# 记录当前 sub2api 镜像 ID，用于部署后清理
OLD_IMAGES=$("${DC[@]}" images -q sub2api 2>/dev/null || true)

"${DC[@]}" pull
"${DC[@]}" down

# 清理旧镜像
if [[ -n "${OLD_IMAGES}" ]]; then
  echo "[INFO] Removing old images..."
  # shellcheck disable=SC2086
  docker rmi ${OLD_IMAGES} 2>/dev/null || true
fi

"${DC[@]}" up -d
"${DC[@]}" ps

echo "[INFO] Deploy finished."
