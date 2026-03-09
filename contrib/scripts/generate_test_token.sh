#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${1:-}"

if [ -z "$DB_PATH" ]; then
  echo "Usage: $0 <path-to-env.db>"
  exit 1
fi

if [ ! -f "$DB_PATH" ]; then
  echo "Error: database file not found: $DB_PATH"
  exit 1
fi

if ! command -v sqlite3 &>/dev/null; then
  echo "Error: sqlite3 is not installed"
  exit 1
fi

ENV_ID=$(sqlite3 "$DB_PATH" "SELECT env_id FROM environments LIMIT 1;")

if [ -z "$ENV_ID" ]; then
  echo "Error: no environments found in database"
  exit 1
fi

TOKEN=$(openssl rand -hex 16)

sqlite3 "$DB_PATH" "INSERT OR REPLACE INTO tokens (env_id, token) VALUES ('$ENV_ID', '$TOKEN');"

ENV_NAME=$(sqlite3 "$DB_PATH" "SELECT name FROM environments WHERE env_id = '$ENV_ID';")
ENV_NS=$(sqlite3 "$DB_PATH" "SELECT namespace FROM environments WHERE env_id = '$ENV_ID';")

echo "Token generated for environment:"
echo "  env_id:    $ENV_ID"
echo "  name:      $ENV_NAME"
echo "  namespace: $ENV_NS"
echo "  token:     $TOKEN"
echo ""
echo "Extend URL: http://localhost:8080/extend?env_id=${ENV_ID}&token=${TOKEN}"
