# Deferred Items — quick-261001-cz7

## 1. Pre-existing test failure: TestEmitSubCategoryInstanceCount

- **Package:** `internal/ontology/emitter` (emitter_test.go:83)
- **Symptom:** `SubCategory instances = 6, want 7` (KBCategory=5 assertion still passes)
- **Cause hypothesis:** corpus drift — the test emits the TBox from the live
  working-tree `skills/` + `knowledge/` dirs and pins the SubCategory count;
  knowledge/sql-guides subdirs changed after the count was pinned
  (e.g. quick-260928 entity/KB work). Unrelated to this quick task's diff
  (2 new live-gated test files + bootstrap .sh edits only).
- **Discovered during:** full `go test ./internal/...` hermetic sweep (verify step)
- **Action:** out of scope per executor scope-boundary rule; owner should
  re-pin the count (or make it corpus-derived) in a dedicated fix.
