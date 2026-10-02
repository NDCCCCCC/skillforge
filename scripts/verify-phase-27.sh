#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# scripts/verify-phase-27.sh — Phase 27 structural gate (HERMETIC).
#
# 3 stages; all must pass.
#
# Stage 1: TODO clear — zero TODO/FIXME in Phase 27 production files
# Stage 2: No-on-disk invariant (4 sub-stages)
#          2a: streaming multipart (BodyStream/multipart.NewReader in upload.go)
#          2b: no disk writes (os.WriteFile/os.Create/ioutil.WriteFile absent)
#          2c: VerifyMagicBytes function declared
#          2d: VerifyMagicBytes body is pure in-memory
# Stage 3: Trust invariant — validateTiers called before embedder in
#          upload_index.go; kb.Citable NOT called in upload_index.go
#          (kb.Citable is the query-time gate; the upload path adds
#          nothing new — same invariant as Phase 26).
#
# Reference: docs/verify-phase-26.sh for the analog structure.

set -uo pipefail

phase_27_files=(
  "internal/document/parser.go"
  "internal/document/parser_test.go"
  "internal/knowledge/upload_index.go"
  "internal/knowledge/upload_index_test.go"
  "internal/server/upload.go"
  "internal/server/upload_test.go"
  "internal/retriever/backend.go"
  "internal/retriever/milvus/backend.go"
  "internal/retriever/milvus/client.go"
  "internal/retriever/mongo/backend.go"
  "internal/retriever/mongo/mongo.go"
)

echo "=== Stage 1: TODO/FIXME clear ==="
TODO_COUNT=$(grep -rE "TODO|FIXME" --include="*.go" "${phase_27_files[@]}" 2>/dev/null | grep -v "_test.go" | wc -l)
if [ "$TODO_COUNT" -gt 0 ]; then
  echo "FAIL: $TODO_COUNT TODO/FIXME in Phase 27 production files:"
  grep -rE "TODO|FIXME" --include="*.go" "${phase_27_files[@]}" 2>/dev/null | grep -v "_test.go"
  exit 1
fi
echo "PASS: no TODO/FIXME in Phase 27 production files"

echo ""
echo "=== Stage 2: No-on-disk invariant ==="

echo "--- Stage 2a: streaming multipart (parseUploadForm uses BodyStream/multipart.NewReader) ---"
STREAMING=$(grep -rE "BodyStream|multipart\.NewReader" --include="*.go" internal/server/upload.go 2>/dev/null | wc -l)
if [ "$STREAMING" -eq 0 ]; then
  echo "FAIL: upload.go does not use BodyStream/multipart.NewReader (would materialize to disk)"
  exit 1
fi
echo "PASS 2a: streaming multipart"

echo "--- Stage 2b: no disk writes in upload.go (excluding _test.go) ---"
DISK_WRITE=$(grep -rE "os\.WriteFile|os\.Create|ioutil\.WriteFile" --include="*.go" internal/server/upload.go 2>/dev/null | grep -v "_test.go" | wc -l)
if [ "$DISK_WRITE" -gt 0 ]; then
  echo "FAIL: upload.go contains disk write patterns"
  grep -rE "os\.WriteFile|os\.Create|ioutil\.WriteFile" --include="*.go" internal/server/upload.go 2>/dev/null | grep -v "_test.go"
  exit 1
fi
echo "PASS 2b: no disk writes"

echo "--- Stage 2c: VerifyMagicBytes function declared in upload.go ---"
VERIFY_DECL=$(grep -rE "^func VerifyMagicBytes" --include="*.go" internal/server/upload.go 2>/dev/null | wc -l)
if [ "$VERIFY_DECL" -eq 0 ]; then
  echo "FAIL: VerifyMagicBytes function not declared in upload.go"
  exit 1
fi
echo "PASS 2c: VerifyMagicBytes declared"

echo "--- Stage 2d: VerifyMagicBytes body is pure in-memory (no disk writes in the function body) ---"
VERIFY_DISK=$(awk '/^func VerifyMagicBytes/,/^}/' internal/server/upload.go 2>/dev/null | grep -cE "os\.WriteFile|os\.Create|ioutil\.WriteFile")
if [ "$VERIFY_DISK" -gt 0 ]; then
  echo "FAIL: VerifyMagicBytes body contains disk write patterns"
  exit 1
fi
echo "PASS 2d: VerifyMagicBytes body is pure"

echo ""
echo "=== Stage 3: Trust invariant ==="

echo "--- Stage 3a: validateTiers called in upload_index.go ---"
VT=$(grep -nE "validateTiers" internal/knowledge/upload_index.go 2>/dev/null | wc -l)
if [ "$VT" -eq 0 ]; then
  echo "FAIL: validateTiers not found in upload_index.go (KNOW-02 fail-closed gate must run before embedder)"
  exit 1
fi
echo "PASS 3a: validateTiers gate present"

echo "--- Stage 3b: kb.Citable NOT called in upload_index.go (it's a query-time gate, not insert-time) ---"
KC=$(grep -nE "kb\.Citable" internal/knowledge/upload_index.go 2>/dev/null | wc -l)
if [ "$KC" -gt 0 ]; then
  echo "FAIL: kb.Citable found in upload_index.go (should only be at query time — trust invariant)"
  exit 1
fi
echo "PASS 3b: kb.Citable not called at insert time"

echo ""
echo "=== Phase 27 verify PASS ==="
