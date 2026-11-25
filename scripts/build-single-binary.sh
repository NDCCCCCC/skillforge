#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# build-single-binary.sh -- produce the single-file SkillForge release build.
#
# The result is ONE executable that serves both the API and the web UI
# from the same origin, with NO sibling web/dist directory to ship,
# lose, or mount wrongly.
#
# How it works:
#   1. build the frontend with Vite (web/dist)
#   2. copy web/dist into internal/webui/dist -- the //go:embed directive
#      in internal/webui/embed_on.go resolves against that directory
#   3. go build -tags embed_webui ./cmd/server
#
# The DEFAULT build (no tag) is untouched: it still serves web/dist from
# disk, which is what the hermetic dev loop and every existing test need.
#
# Usage:
#   bash scripts/build-single-binary.sh [--skip-frontend] [--out NAME]
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

SKIP_FRONTEND=0
OUT="skillforge"
while [ $# -gt 0 ]; do
  case "$1" in
    --skip-frontend) SKIP_FRONTEND=1; shift ;;
    --out) OUT="${2:?--out needs a name}"; shift 2 ;;
    *) echo "[FAIL] unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ "$SKIP_FRONTEND" = "0" ]; then
  echo "[INFO] building the frontend (web/dist)"
  ( cd web && npm run build ) || { echo "[FAIL] frontend build failed" >&2; exit 1; }
fi

if [ ! -f web/dist/index.html ]; then
  echo "[FAIL] web/dist/index.html is missing -- run 'cd web && npm run build' first" >&2
  exit 1
fi

echo "[INFO] staging web/dist into internal/webui/dist (embed source)"
rm -rf internal/webui/dist
mkdir -p internal/webui/dist
cp -r web/dist/. internal/webui/dist/
# The placeholder is tracked in git so the //go:embed directive resolves
# on a clean checkout (before the copy above runs).
if [ ! -f internal/webui/dist/.gitkeep ]; then
  printf '%s\n'     'placeholder: the release build copies web/dist here before'     '`go build -tags embed_webui ./cmd/server`. Keeping this directory in git'     'lets the embed directive resolve on a clean checkout.' > internal/webui/dist/.gitkeep
fi

echo "[INFO] compiling the single-binary build (-tags embed_webui)"
CGO_ENABLED="${CGO_ENABLED:-1}" go build -tags embed_webui -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/server || {
  echo "[FAIL] go build failed" >&2
  exit 1
}

SIZE=$(wc -c < "$OUT" | tr -d ' ')
echo "[OK] built $OUT ($SIZE bytes, embedded web UI included)"
echo "[INFO] run it with: bash scripts/run-single-binary.sh"
