---
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  gap_snapshot: "unknown::scenarios=0"
---

# Phase 27 LIVE Acceptance Runbook

**Phase:** 27 — 文档上传入库 (Document Upload Pipeline)
**Operator checkpoint:** HUMAN VERIFY (mirrors Phase 26 26-UAT.md §6)

## Prerequisites

- Server running with `SKILLFORGE_ADMIN_TOKEN` set
- At least one active KB slug in the registry (verify via `/admin` Knowledge Bases)
- A test `.md` file with content `Test upload 2026-09`
- `SKILLFORGE_UPLOAD_MAX_BYTES` unset (uses 100 MB default)

## Steps

1. **Upload a .md file via curl:**
   ```bash
   curl -X POST http://localhost:8888/api/admin/kb/authority/upload \
     -H "X-Admin-Token: $SKILLFORGE_ADMIN_TOKEN" \
     -F "file=@test.md;type=text/markdown"
   ```
   Expected: HTTP 200, JSON `{"chunks":1,"id_range":[N,N]}`

2. **Verify audit row:**
   ```bash
   sqlite3 skillforge.db "SELECT action, json_extract(frozen_args, '$.slug'), json_extract(frozen_args, '$.sha256'), json_extract(frozen_args, '$.chunk_count') FROM audit WHERE action='action_doc_uploaded' ORDER BY id DESC LIMIT 1;"
   ```
   Expected: `action_doc_uploaded|authority|<sha256>|1`

3. **Try upload without token:**
   ```bash
   curl -X POST http://localhost:8888/api/admin/kb/authority/upload \
     -F "file=@test.md;type=text/markdown"
   ```
   Expected: HTTP 403

4. **Try upload to deactivated slug:**
   - Deactivate via POST /api/admin/kb/:slug/deactivate (replace with a temporary slug from step 1, or use `uat-test`)
   - Then upload to that slug
   Expected: HTTP 409

6. **Upload via FE:**
   - Navigate to /admin → scroll to 上传文档 panel
   - Select KB → choose test.md → submit
   Expected: green toast "1 个片段已入库"

7. **Verify chunks are searchable:**
   - Send a chat query that should match the uploaded content via `/api/chat`
   Expected: Citation appears from the uploaded document (look for the new chunk's id in the verdict)

## Pass Criteria

All 5 verification steps (1-5 + 6 + 7) must return the expected result. Any deviation is a Phase 27 regression — record it in `.planning/STATE.md` Blockers and escalate per the env-intranet flakiness memory.
