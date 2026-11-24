#!/usr/bin/env bash
# verify-phase-54.sh -- Phase 54 (ingest async pipeline) acceptance gate.
# ING-04 async contract / ING-05 staging+retry+partial / ONT-01 ECL trigger.
# Zero waiver: any SKIP/WAIVED must not hold a green light (the
# verify-phase-47 CI-compatibility red line). No live Milvus/BGE deps --
# everything runs on fake seams (intranet flakiness does not gate this).
set -euo pipefail
cd "$(dirname "$0")"

pass=0; fail=0
ok()   { echo "--- PASS: $1"; pass=$((pass+1)); }
bad()  { echo "--- FAIL: $1"; fail=$((fail+1)); }

echo "=== P1: CGO build + vet ==="
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go vet ./...
ok "P1 build+vet (CGO_ENABLED=1)"

echo "=== P2: package suites ==="
go test ./internal/ingest/ ./internal/knowledge/ ./internal/knowledge/ecl/ ./internal/server/ ./internal/store/sqlite/ ./internal/kb/ ./internal/retriever/... -count=1
ok "P2 ingest+knowledge+server+sqlite+kb+retriever suites green"

echo "=== P3: named gates (54 plan deliverables) ==="
# a. ING-01: upload persist survives the boot reseed (53-04 delivery, the
#    async pipeline depends on it)
go test ./internal/knowledge/ -run '^TestUploadPersistReseedSurvival$' -count=1 \
  && ok "a. reseed survival: persisted upload survives the tier purge" \
  || bad "a. reseed survival"
# b. ING-02: cross-path chunk equivalence (53-04 delivery, standing gate)
go test ./internal/knowledge/ -run '^TestUploadIndexCrossPathEquivalence$' -count=1 \
  && ok "b. cross-path equivalence: upload vs directory chunk sequences byte-identical" \
  || bad "b. cross-path equivalence"
# c. 54-01: lifecycle happy path (Enqueue -> done, ecl 4 skipped non-blocking)
go test ./internal/ingest/ -run '^TestIngestLifecycleHappyPath$' -count=1 \
  && ok "c. lifecycle happy path: Enqueue lands done, ecl skipped non-blocking" \
  || bad "c. lifecycle happy path"
# d. 54-03: crash recovery asymmetry (running->failed+retryable, queued
#    auto-resumes -> failed on lost raw, re-upload recovers)
go test ./internal/ingest/ -run '^TestRecoverOrphansAsymmetry$' -count=1 \
  && ok "d. crash recovery asymmetry pinned (running/queued divergence)" \
  || bad "d. crash recovery asymmetry"
# e. 54-03: classifier table (transient whitelist 5 / deterministic 6 incl
#    default-closed)
go test ./internal/ingest/ -run '^TestClassifyTable$' -count=1 \
  && ok "e. classifier table pinned (transient whitelist + default-closed)" \
  || bad "e. classifier table"
# f. 54-03: retry loop attempt counts (transient 1+3, deterministic 1,
#    mid-loop success stops)
go test ./internal/ingest/ -run '^TestRetryLoopAttempts$' -count=1 \
  && ok "f. retry loop attempt counts pinned" \
  || bad "f. retry loop attempts"
# g. 54-02/53-03: embed cache reuse (same content_hash -> zero embedder
#    calls; model change -> cache miss) via the sqlite embed_cache table
go test ./internal/store/sqlite/ -run 'TestEmbedCache' -count=1 \
  && ok "g. embed_cache blob round-trip + uniqueness" \
  || bad "g. embed cache"
# h. 54-02/53-03: upsert dedup (hash-unchanged re-upload skips re-embed;
#    parse retry reads the archive) -- server package async contract suite
go test ./internal/server/ -run 'TestPersist|TestUpload' -count=1 \
  && ok "h. upload async contract suite (persist/archive/dedup/serial)" \
  || bad "h. upload async contract suite"

echo "=== P4: full internal smoke (not deterministic evidence) ==="
go test ./internal/... -count=1
ok "P4 ./internal/... full smoke"

echo ""
echo "=== verify-phase-54 summary: PASS=$pass FAIL=$fail ==="
if [ "$fail" -ne 0 ]; then
  echo "GATE: RED -- Phase 54 not closed (zero-waiver discipline: no SKIP masking)"
  exit 1
fi
echo "GATE: GREEN -- Phase 54 ingest async pipeline closed"
