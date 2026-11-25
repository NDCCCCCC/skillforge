#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# run-single-binary.sh -- launch the single-file v1.8 build.
#
# Sources .env (the operator's own secrets file) and execs the embedded
# binary. Nothing here reads or echoes a secret value.
#
# NOTE on .env parsing: the file is written by an operator (and by the
# admin panel), so a value may contain shell metacharacters that bash
# would mis-parse if the file were sourced directly. Values are therefore
# read LINE BY LINE as KEY=VALUE and exported literally -- no word
# splitting, no command substitution.
#
# Usage:  bash scripts/run-single-binary.sh [--no-env]
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

BIN="${SKILLFORGE_BIN:-$ROOT/skillforge.exe}"
if [ ! -f "$BIN" ]; then
  echo "[FAIL] $BIN not found -- build it first:" >&2
  echo "  cp -r web/dist/* internal/webui/dist/" >&2
  echo "  go build -tags embed_webui -trimpath -ldflags='-s -w' -o skillforge.exe ./cmd/server" >&2
  exit 1
fi

DQ=$(printf '\042')
SQ=$(printf '\047')

if [ "${1:-}" != "--no-env" ] && [ -f "$ROOT/.env" ]; then
  loaded=0
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in
      ''|'#'*) continue ;;
    esac
    case "$line" in
      *=*) ;;
      *) continue ;;
    esac
    key=${line%%=*}
    val=${line#*=}
    key=$(printf '%s' "$key" | tr -d '[:space:]')
    case "$key" in
      ''|*[!A-Za-z0-9_]*) continue ;;
    esac
    # Strip ONE matching outer quote pair, if present.
    first=${val%"${val#?}"}
    last=${val#"${val%?}"}
    if [ "$first" = "$DQ" ] && [ "$last" = "$DQ" ]; then
      val=${val#?}
      val=${val%?}
    elif [ "$first" = "$SQ" ] && [ "$last" = "$SQ" ]; then
      val=${val#?}
      val=${val%?}
    fi
    export "$key=$val"
    loaded=$((loaded + 1))
  done < "$ROOT/.env"
  echo "[INFO] loaded $loaded variable(s) from .env (values not echoed)"
fi

echo "[INFO] starting $(basename "$BIN") (embedded web UI)"
exec "$BIN"
