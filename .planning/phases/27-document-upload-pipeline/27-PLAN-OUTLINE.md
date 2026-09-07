# Phase 27 Plan Outline: 文档上传入库 (Document Upload Pipeline)

**Phase:** 27
**Plans:** 7 plans in 4 waves
**Requirements covered:** DOC-01, DOC-02, DOC-03, DOC-04, DOC-05, DOC-06
**Total tasks:** 19

---

## Wave Structure

| Wave | Plans | Depends On | Autonomous | Key Deliverable |
|------|-------|-----------|------------|-----------------|
| 1 | 27-01, 27-02, 27-03, 27-04 | None | Yes | Parser registry + UploadIndex + HTTP endpoint + IndexerBackend |
| 2 | 27-05, 27-07 | 27-03 | Partial | FE panel + degraded modes |
| 3 | 27-06 | All of above | No (human checkpoint) | Verify script + Makefile + STRIDE + LIVE runbook |

---

## Dependency Graph

```
Wave 1 (parallel — no file conflicts):
  27-01 ─────────────────────────────────────────────────────────────┐
    │  (parser registry + go get)                                    │
    ├────────────────────────────────────────────────────────────────┤
  27-02 ─────────────────────────────────────────────────────────────┤
    │  (UploadIndex + nextID coordination)                          │
    ├────────────────────────────────────────────────────────────────┤
  27-03 ─────────────────────────────────────────────────────────────┤
    │  (upload handler + multipart streaming)                        │
    ├────────────────────────────────────────────────────────────────┤
  27-04 ─────────────────────────────────────────────────────────────┤
       (IndexerBackend interface + Milvus/Mongo adapters)            │
                                                                       │
Wave 2:                                                                 │
  27-05 depends on 27-03 ─────────────────────────────────────────────┤
       (FE upload panel)                                               │
                                                                       │
  27-07 depends on 27-03 ─────────────────────────────────────────────┤
       (degraded modes + deduplication)                               │
                                                                       │
Wave 3:                                                                 │
  27-06 depends on all ───────────────────────────────────────────────┘
       (verify script + hygiene gate + human LIVE checkpoint)
```

---

## Plan Summary

| Plan | Objective | Tasks | Files | Requirements |
|------|----------|-------|-------|--------------|
| 27-01 | Parser abstraction + format registry | 3 | parser.go, go.mod | DOC-01, DOC-02, DOC-03 |
| 27-02 | UploadIndex entry point + nextID | 3 | upload_index.go | DOC-02, DOC-03, DOC-05 |
| 27-03 | HTTP upload endpoint | 3 | upload.go, deps.go | DOC-01, DOC-03, DOC-04, DOC-05 |
| 27-04 | Dual-backend IndexerBackend | 4 | backend.go, main.go | DOC-02, DOC-05 |
| 27-05 | FE upload panel | 3 | DocUpload.tsx, App.tsx | DOC-01, DOC-06 |
| 27-06 | Engineering hygiene + LIVE gate | 3 | verify-phase-27.sh, STRIDE-27.md | DOC-03, DOC-04, DOC-05 |
| 27-07 | Degraded modes + dedup | 3 | upload.go, audit_store.go | DOC-03, DOC-04 |

---

## Decisions Made by Planner

| Decision ID | Question | Answer |
|-------------|----------|--------|
| D27-01-1 | SKILLFORGE_UPLOAD_MAX_BYTES configurable? | Yes — env var defaulting to 100 MB |
| D27-01-2 | Memory buffer vs streaming? | bytes.Buffer in memory (100 MB max bounded) |
| D27-01-3 | Parser library choice? | Try eino-ext first; pure-Go fallback if CGO |
| D27-02-1 | Upload to retrieval_enabled=false slug? | Reject with 409 (fail-closed via validateTiers) |
| D27-03-1 | Upload max bytes configurable? | Yes — Deps.UploadMaxBodyBytes, env SKILLFORGE_UPLOAD_MAX_BYTES |
| D27-03-2 | Per-slug concurrency? | Per-slug sync.Map mutexes (serialized per slug) |

---

## Requirement Coverage

| Requirement | Plan(s) | Status |
|-------------|---------|--------|
| DOC-01: Upload UI + endpoint | 27-01, 27-03, 27-05 | Planned |
| DOC-02: Parse → chunk → embed → index | 27-01, 27-02, 27-04 | Planned |
| DOC-03: No disk, type/size guards | 27-01, 27-03, 27-06, 27-07 | Planned |
| DOC-04: Configurable limits + dedup | 27-03, 27-06, 27-07 | Planned |
| DOC-05: INSERT-only audit row | 27-02, 27-03, 27-06, 27-07 | Planned |
| DOC-06: Immediately visible | 27-02, 27-04, 27-05 | Planned |

---

## Execution Order

**Wave 1 (parallel, no conflicts):**
```bash
# Three plans can run in parallel:
27-01  # Parser registry + go get
27-02  # UploadIndex + nextID
27-03  # Upload handler + multipart
27-04  # IndexerBackend interface + adapters
```

**Wave 2 (after 27-03 completes):**
```bash
27-05  # FE upload panel
27-07  # Degraded modes + dedup
```

**Wave 3 (after all above complete):**
```bash
27-06  # Engineering hygiene + human LIVE checkpoint
```

---

## Next Steps

Execute Wave 1 plans in parallel:
- `/gsd:execute-phase 27 --plans 27-01,27-02,27-03,27-04 --parallel`

After Wave 1 lands, execute Wave 2:
- `/gsd:execute-phase 27 --plans 27-05,27-07 --parallel`

After Wave 2 lands, execute Wave 3 (contains human checkpoint):
- `/gsd:execute-phase 27 --plans 27-06`
