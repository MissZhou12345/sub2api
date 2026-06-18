#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

COMPOSE="docker-compose -f docker-compose.local.yml --env-file .env --env-file .version"

$COMPOSE pull
$COMPOSE down
$COMPOSE up -d

$COMPOSE ps