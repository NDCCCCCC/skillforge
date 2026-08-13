# Phase 6: Eval Dashboard & Production Hardening - Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 31 (new + modified)
**Analogs found:** 29 / 31 (2 greenfield: `.github/workflows/eval-gate.yml`, `eval/reports/INDEX.json`)

This phase is a HARDENING phase on a Phase-3-shipped substrate. The recurring pattern is **thin adapter over existing code**: `/api/audit` is a 1:1 HTTP mapping of `store.QueryAudit`; the dashboard reads `eval.Report` (already shipped); `gateDecision` is a pure function over the existing `Report` schema; SC-4 scales an existing test. The temptation to rebuild is the trap — almost every SC is satisfied by extending, not rewriting.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/eval/frozen.go` (NEW) | model + utility | file-I/O | `internal/eval/dataset.go` (`LoadGolden`, `Case`) | exact |
| `internal/eval/archive.go` (NEW) | utility | file-I/O | `internal/eval/dataset.go` (`WriteReport`) + `internal/store/sqlite/audit.go` (atomic pattern) | role-match |
| `internal/eval/gate.go` (NEW) | utility (pure fn) | transform | `internal/eval/calibration.go` (`Calibrate`) | exact |
| `internal/eval/crosseval.go` (NEW) | utility (pure fn) | transform | `internal/eval/calibration.go` | role-match |
| `internal/eval/eval_test.go` (MODIFY) | test | batch | itself (`TestRunnerReproducibility`) | exact |
| `internal/eval/frozen_test.go` (NEW) | test | file-I/O | `internal/eval/eval_test.go` + `internal/eval/golden_test.go` | exact |
| `internal/eval/gate_test.go` (NEW) | test | transform | `internal/eval/eval_test.go::TestPercentile` | exact |
| `cmd/eval/main.go` (MODIFY) | config (CLI) | batch | itself (flag set) | exact |
| `internal/server/audit.go` (NEW) | controller | request-response | `internal/server/knowledge.go` (`graphHandler`) + `internal/server/skills.go` (`listSkillsHandler`) | exact |
| `internal/server/eval.go` (NEW) | controller | request-response | `internal/server/knowledge.go` (`graphHandler` + `chunkHandler` + path-validate `:id`) | exact |
| `internal/server/server.go` (MODIFY) | config | request-response | itself (`New()` route registration + `Deps`) | exact |
| `internal/server/chat.go` (MODIFY) | utility | transform | itself (`computePromptHash` L420-425) | exact |
| `internal/server/audit_test.go` (NEW) | test | request-response | `internal/server/knowledge_test.go` (`invokeGraphHandler` + `ut.PerformRequest`) | exact |
| `internal/server/eval_test.go` (NEW) | test | request-response | `internal/server/knowledge_test.go` | exact |
| `internal/server/chat_session_test.go` (MODIFY) | test | batch (concurrent) | itself (`TestSessionIsolationConcurrent` L253-313) | exact |
| `internal/store/store.go` (MODIFY) | model | CRUD | itself (`AuditFilter`) | exact |
| `internal/store/sqlite/audit.go` (MODIFY) | service | CRUD | itself (`QueryAudit` L85-168) | exact |
| `cmd/server/main.go` (MODIFY) | config | request-response | itself (Deps wiring) | exact |
| `skills/compliance/eval-gate.json` (NEW) | config | file-I/O | `skills/compliance/SKILL.md` (co-location D2-07) | partial (no prior JSON config file) |
| `eval/frozen/compliance.jsonl` (NEW) | data | file-I/O | `eval/golden/compliance.jsonl` + `internal/eval/testdata/frozen_5.jsonl` | exact |
| `eval/frozen/compliance.lock.json` (NEW) | config | file-I/O | `internal/eval/testdata/frozen_5.jsonl` (sidecar concept) | role-match |
| `eval/reports/INDEX.json` (NEW) | data | file-I/O | NO existing analog (greenfield format) | none |
| `.github/workflows/eval-gate.yml` (NEW) | config (CI) | batch | NO existing `.github/` (greenfield) | none |
| `web/src/components/eval/EvalDashboard.tsx` (NEW) | component (route) | request-response | `web/src/App.tsx` (`LiveApp` composition root) | role-match |
| `web/src/components/eval/SkillsTable.tsx` (NEW) + `.test.tsx` | component | request-response | `web/src/components/graph/KnowledgeGraphBrowser.tsx` + `__tests__/KnowledgeGraphBrowser.test.tsx` | exact |
| `web/src/components/eval/TrendPanel.tsx` (NEW) + `.test.tsx` | component | transform | `web/src/components/graph/*` (panel pattern) | role-match |
| `web/src/components/eval/CrossEvalTab.tsx` (NEW) | component | transform | `web/src/components/graph/*` | role-match |
| `web/src/components/eval/AuditTable.tsx` (NEW) + `.test.tsx` | component | request-response | `web/src/components/graph/GraphTree.tsx` + sketch-findings status-color tokens | role-match |
| `web/src/store/app-store.ts` (MODIFY) | store | request-response | itself (slices + D14 no-`messages` invariant) | exact |
| `web/src/App.tsx` (MODIFY) | component (root) | request-response | itself (`LiveApp`) | exact |
| `web/src/components/layout/LeftSidebar.tsx` (MODIFY) | component | event-driven | itself (D6-01 nav entry) | exact |

---

## Pattern Assignments

### `internal/eval/frozen.go` (NEW — model + utility, file-I/O)

**Analog:** `internal/eval/dataset.go` (lines 49-55 `Case`, 133-174 `LoadGolden`)

**Type definition pattern** (from `dataset.go:49-55`):
```go
// Case is one row of the golden-set JSONL.
type Case struct {
    ID             string   `json:"id"`
    Category       string   `json:"category"`
    Query          string   `json:"query"`
    GroundTruth    string   `json:"ground_truth"`
    ExpectedChunks []string `json:"expected_chunks"`
}
```

**LockFile struct to replicate** (RESEARCH Example 2):
```go
type LockFile struct {
    ModelVersion   string `json:"model_ver"`
    PromptHash     string `json:"prompt_hash"`     // MUST match chat.go::computePromptHash output
    Seed           int64  `json:"seed"`
    EvalSetVersion string `json:"eval_set_version"`
    SkillName      string `json:"skill_name"`
}
```

**LoadGolden signature to mirror for `LoadLock`** (`dataset.go:133`):
```go
func LoadGolden(path string) ([]Case, error)  // ← LoadLock(path string) (LockFile, error) mirrors this
```
- Use `os.Open` + `defer f.Close()` + `json.NewDecoder(f).Decode(&lock)` (single JSON object, not JSONL).
- Error wrapping convention: `fmt.Errorf("eval.LoadLock: open %q: %w", path, err)`.
- File header MUST be Apache-2.0 SPDX (copy lines 1-15 from `dataset.go`).

**AssertPin signature** (RESEARCH Example 2):
```go
func AssertPin(lock LockFile, liveModel, liveSkillName string) error
```
- MUST call `computePromptHash` from `internal/server` package OR duplicate it — RESEARCH Q1 BLOCKING: if Q1 (a) is chosen, the hash input changes, so lock + assertion + chat.go all change together. **Invariant:** the lock's `prompt_hash` field MUST be byte-identical to what `internal/server.computePromptHash` produces, whatever that ends up meaning post-Q1.
- Return errors naming the drifted field: `fmt.Errorf("frozen pin: model_ver drift (lock=%s, live=%s)", ...)`

---

### `internal/eval/archive.go` (NEW — utility, file-I/O)

**Analog:** `internal/eval/dataset.go::WriteReport` (lines 180-192) + `internal/store/sqlite/audit.go::QueryAudit` (parameter assembly)

**WriteReport pattern to extend** (`dataset.go:180-192`):
```go
func WriteReport(path string, r Report) error {
    if path == "" {
        return fmt.Errorf("eval.WriteReport: path is empty")
    }
    b, err := json.MarshalIndent(r, "", "  ")
    if err != nil { return fmt.Errorf("eval.WriteReport: marshal: %w", err) }
    if err := os.WriteFile(path, b, 0o644); err != nil {
        return fmt.Errorf("eval.WriteReport: write %q: %w", path, err)
    }
    return nil
}
```

**INDEX.json schema** (RESEARCH Pattern 2 — greenfield, no analog):
```json
{
  "version": 1,
  "runs": [
    { "ts": "2026-08-13T10-05-21Z", "skill": "compliance", "judge_model": "glm-5.2",
      "faithfulness": 0.92, "context_precision": 0.85, "answer_relevance": 0.88,
      "context_recall": 0.79, "refusal_accuracy": 0.95,
      "frozen_version": "compliance-2026-08-13-v1", "report_path": "2026-08-13T10-05-21Z.json" }
  ]
}
```

**`refusal_accuracy` derivation** (RESEARCH Pattern 3 — NO new metric):
- `refusal_accuracy = Report.PerCategory["refusal-required"]["context_recall"]`
- Verified at `internal/eval/metrics.go:184-195` — `ContextRecall` returns `1.0` when `groundTruth=="REFUSAL" && refused`, `0.0` otherwise. The category mean IS the refusal-accuracy rate.
- **Invariant:** do NOT introduce a new metric function. Read the existing `PerCategory` map.

**Atomic write pattern** (RESEARCH Pitfall 7):
- Write to `<path>.tmp` then `os.Rename(<path>.tmp, <path>)` — atomic on POSIX; Windows same-volume works since Go 1.5.
- For `INDEX.json`: `os.ReadFile` → append to `runs[]` → atomic write back. NOT safe under concurrent processes (document this — git is the canonical history).

**Full report copy** (`<iso-ts>.json`): use `WriteReport` directly with the archive path. ISO timestamp format: `time.Now().UTC().Format("2006-01-02T15-04-05Z")` (RESEARCH uses `T10-05-21Z` shape — colons are filesystem-hostile on Windows).

---

### `internal/eval/gate.go` (NEW — utility / pure fn, transform)

**Analog:** `internal/eval/calibration.go::Calibrate` (lines 87-149 — pure function, no I/O, returns a structured result + rationale)

**Pure-function signature** (RESEARCH Pitfall 6 — extract `gateDecision` for testability):
```go
// gateDecision evaluates per-Skill faithfulness against per-Skill thresholds.
// Returns the list of Skills that FAILED (faithfulness < threshold) — empty = pass.
type SkillResult struct {
    Skill       string
    Faithfulness float64
    Threshold    float64
    Pass         bool
}

func gateDecision(report Report, thresholds map[string]float64) []SkillResult
```
- **Pattern:** no I/O, no fmt.Errorf — pure data-in / data-out (mirrors `Calibrate` returning `Recommendation`).
- Default threshold `0.9` when a Skill is not in the map (D6-10).
- Phase 6 ships exactly ONE Skill (`compliance`) → the map has one entry.

**Caller pattern** (CI workflow calls this via `cmd/eval --gate-check` OR a new `cmd/evalgate`): the workflow runs the pure fn, formats the markdown table, exits non-zero on failure. **Invariant:** `cmd/eval` MUST NOT exit non-zero on low faithfulness — the gate decision belongs in the helper, not the eval CLI (`cmd/eval/main.go:53-56` comment is the contract).

---

### `internal/eval/crosseval.go` (NEW — utility / pure fn, transform)

**Analog:** `internal/eval/calibration.go` (pure fn pattern)

**Diff function:**
```go
// crossEvalDiff compares per-Skill faithfulness across judge models.
// Flags any Skill with >5% faithfulness delta (Pitfall 8 self-eval-bias signal).
type CrossEvalDelta struct {
    Skill         string
    FaithfulnessByJudge map[string]float64  // judge_model → score
    DeltaPct      float64
    Flagged       bool
}

func crossEvalDiff(reportsByJudge map[string]Report, deltaThresholdPct float64) []CrossEvalDelta
```
- `deltaThresholdPct` defaults to `5.0` (D6-14).
- **Invariant:** the `>5%` threshold is the exact numeric value `eval/README.md` already documents as the Pitfall 8 signal — Phase 6 operationalizes the doc into a visible alert (CONTEXT `<specifics>`).

---

### `internal/eval/eval_test.go` (MODIFY — test, batch)

**Analog:** itself — `TestRunnerReproducibility` (lines 51-76)

**Current tolerance to relax** (`eval_test.go:72`):
```go
assert.True(t, math.Abs(v1-v2) < 1e-9, ...)  // ← CHANGE to 1e-6 (D6-15 strict layer)
```

**New live-reproducibility test to ADD** (RESEARCH Example 3 — `//go:build liveeval`):
```go
//go:build liveeval
func TestRunnerLiveReproducibility(t *testing.T) {
    if os.Getenv("GLM_API_KEY") == "" { t.Skip("GLM_API_KEY unset") }
    // ... build real judge via eval.NewGLMChatModel, load frozen set, run TWICE
    // ... assert abs(run1-run2) < 1e-3 (D6-15 jitter tolerance)
}
```
**Pitfall 3 counter-measure (load-bearing invariant):** the live test MUST carry `//go:build liveeval` so the default `go test ./...` does NOT burn GLM quota. Only the eval-gate CI workflow runs `-tags liveeval`.

---

### `internal/eval/frozen_test.go` + `gate_test.go` (NEW — tests)

**Analog:** `internal/eval/eval_test.go` (table-driven + `require.NoError` + `assert.Equal`) and `internal/eval/golden_test.go`

**Test function signature pattern** (from `eval_test.go:51`):
```go
func TestAssertPin(t *testing.T) {
    // table-driven cases: matching pin → nil; model_ver drift → error; prompt_hash drift → error
}

func TestGateDecision(t *testing.T) {
    // table-driven: report with faithfulness 0.85 < threshold 0.9 → SkillResult{Pass:false}
    //               report with faithfulness 0.95 ≥ threshold 0.9 → SkillResult{Pass:true}
    //               missing threshold → defaults to 0.9
}
```
- Use `github.com/stretchr/testify/assert` + `require` (already in go.mod).
- File header: Apache-2.0 SPDX (copy from `eval_test.go:1-15`).

---

### `cmd/eval/main.go` (MODIFY — config / CLI, batch)

**Analog:** itself — flag set (lines 99-104) + bootstrap (lines 109-118)

**Current flag pattern to extend** (`main.go:99-104`):
```go
goldenPath := flag.String("golden", defaultGoldenPath, "path to the golden-set JSONL file")
reportPath := flag.String("report", defaultReportPath, "output path for the JSON report")
calibrationPath := flag.String("calibration", defaultCalibrationPath, "...")
limit := flag.Int("limit", 0, "...")
topK := flag.Int("topk", defaultTopK, "...")
flag.Parse()
```

**New flags to add** (RESEARCH Pattern 2 — flag matrix):
```go
archive := flag.Bool("archive", false, "append report to eval/reports/<ts>.json + update INDEX.json")
frozen := flag.String("frozen", "", "skill name; reads eval/frozen/<skill>.jsonl + .lock.json, asserts pin")
judgeModels := flag.String("judge-models", "", "comma-list (e.g. glm-5.2,deepseek-chat); cross-eval per judge")
```
**Flag matrix invariants (RESEARCH Pattern 2):**
- `--golden` + `--frozen` together → ERROR (mutually exclusive eval-set sources).
- `--frozen` alone → strict (StubJudge test path) OR live (`cmd/eval --frozen` real run, 1e-3 tolerance).
- `--judge-models` + `--archive` → writes per-judge reports under `eval/reports/<judge>/<ts>.json`.
- Existing flags unchanged — Phase 6 EXTENDS, never rewrites.

**`--archive` callsite** (after `eval.WriteReport(*reportPath, report)` at `main.go:236`):
```go
if *archive {
    if err := eval.WriteArchive("eval/reports", report, rec, *frozen != "", judgeName); err != nil {
        log.Fatalf("[FAIL] cmd/eval: archive: %v", err)
    }
}
```

**Exit-code invariant** (`main.go:53-56` comment): `cmd/eval` exits 0 on low faithfulness — the gate decision belongs to `gateDecision()` called from CI, NOT this CLI.

---

### `internal/server/audit.go` (NEW — controller, request-response)

**Analog:** `internal/server/knowledge.go::graphHandler` (lines 75-98) + `internal/server/skills.go::listSkillsHandler` (lines 129-169)

**Handler method-signature pattern** (`knowledge.go:75` — exact shape to mirror):
```go
func (s *Deps) auditHandler(ctx context.Context, c *app.RequestContext) {
    if s.Store == nil {
        c.JSON(http.StatusServiceUnavailable, graphResponseError{
            Error: "audit store not initialized (set Deps.Store at server construction)",
        })
        return
    }
    // ... build AuditFilter from query params ...
    // ... call s.Store.QueryAudit(ctx, f) ...
    // ... apply retention projection ...
    // ... c.JSON(http.StatusOK, rows) ...
}
```

**Imports to copy** (`knowledge.go:46-56` — same posture):
```go
import (
    "context"
    "crypto/subtle"   // NEW — constant-time token compare
    "errors"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "time"

    "github.com/cloudwego/hertz/pkg/app"

    "github.com/skillforge/skillforge/internal/store"
)
```

**Query-param → AuditFilter mapping** (RESEARCH Example 1, D6-05):
```go
f := store.AuditFilter{
    UserID:        string(c.Query("user")),
    SkillID:       string(c.Query("skill")),
    RefusalReason: string(c.Query("refusal_reason")),
    Since:         parseTime(c.Query("since")),
    Until:         parseTime(c.Query("until")),
    Limit:         atoiOr(c.Query("limit"), 100),
    Offset:        atoiOr(c.Query("offset"), 0),  // requires adding Offset to AuditFilter
}
```
- **`limit`/`offset` MUST be integer-clamped** (V5 ASVS — reject negative/huge).

**Retention projection pattern (LOAD-BEARING — Pattern 1 + Pitfall 4):**
```go
// redact applies the retention projection IN PLACE between QueryAudit and c.JSON.
// INVARIANT: this is a READ-TIME projection. The store gains NO UPDATE/DELETE path.
// T-3-AUDIT INSERT-only invariant MUST survive this phase.
func redact(rows []store.AuditRow, retention time.Duration, now time.Time, isAdmin bool) {
    if isAdmin { return }
    boundary := now.Add(-retention)
    for i := range rows {
        if rows[i].Timestamp.Before(boundary) {
            reason := fmt.Sprintf("REDACTED:%dd+ (ts=%s, boundary=%s)", ...)
            rows[i].Query = reason
            rows[i].VerdictJSON = reason
        }
    }
}
```

**Admin-token pattern (ASVS V6.2.1 — constant-time compare, NOT `==`):**
```go
func isAdminRequest(c *app.RequestContext) bool {
    expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
    if expected == "" { return false }  // D6-08: unset → no admin anywhere (demo works OOB)
    var got string
    c.Request.Header.VisitAll(func(k, v []byte) {
        if string(k) == "X-Admin-Token" { got = string(v) }
    })
    return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
```

**Error envelope reuse** (`knowledge.go:61-63`): reuse the `graphResponseError{Error string}` shape OR define a sibling `auditResponseError`. **Do NOT leak err.Error() in 500 responses** (W-3 convention from `knowledge.go:90-95`) — log full error at `[WARN]` server-side, return generic message.

---

### `internal/server/eval.go` (NEW — controller, request-response)

**Analog:** `internal/server/knowledge.go` (`graphHandler` lines 75-98 + `chunkHandler` lines 121-154 — including `:id` path-param parsing)

**EvalReporter seam on Deps (RESEARCH Pattern 4 — testability):**
```go
// Mirrors KnowledgeBuilder at server.go:98-101.
type EvalReporter interface {
    Index() (EvalIndex, error)                  // reads eval/reports/INDEX.json
    Report(ts string) (eval.Report, error)      // reads one eval/reports/<ts>.json
    FrozenSets() ([]FrozenSetInfo, error)       // scans eval/frozen/*.lock.json
}
```
- Production wires a filesystem-backed impl at `cmd/server/main.go` (mirrors how `KnowledgeBuilder` is wired).
- Tests inject an in-memory stub (mirrors `stubKnowledgeBuilder` at `knowledge_test.go:52`).

**Path-traversal protection (LOAD-BEARING — V5 ASVS):**
```go
// `/api/eval/reports/:ts` — the :ts MUST be regex-validated before os.ReadFile.
// Reject any path containing '/', '\', or '..'. Confine reads to eval/reports/.
var validTimestamp = regexp.MustCompile(`^[0-9T:-]+Z$`)

func (s *Deps) evalReportHandler(ctx context.Context, c *app.RequestContext) {
    ts := c.Param("ts")
    if !validTimestamp.MatchString(ts) {
        c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid report timestamp"})
        return
    }
    // ... s.EvalReporter.Report(ts) ...
}
```
- **Mirror `chunkHandler` posture** (`knowledge.go:121-154`): nil seam → 503; bad `:ts` → 400; not-found → 404; other err → 500 generic (W-3).

**File-header pattern** (mirror `knowledge.go:17-43`):
- One-paragraph file comment naming the Plan / SC, wire contract (status codes), and the security note.

---

### `internal/server/server.go` (MODIFY — config, request-response)

**Analog:** itself — `Deps` struct (lines 140-181) + `New()` route registration (lines 197-207)

**Deps field to add** (mirror `KnowledgeBuilder` field at `server.go:171`):
```go
type Deps struct {
    // ... existing fields ...
    Knowledge KnowledgeBuilder
    // Phase 6 — /api/eval/* handlers. nil = the handlers return 503
    // (mirrors Knowledge nil-posture at knowledge.go:76).
    EvalReporter EvalReporter
}
```

**Route registration extension** (`server.go:197-207`):
```go
func New(addr string, deps *Deps) *server.Hertz {
    h := server.Default(server.WithHostPorts(addr))
    // ... existing routes ...
    h.GET("/api/knowledge/graph", deps.graphHandler)
    h.GET("/api/knowledge/chunk/:id", deps.chunkHandler)
    // Phase 6 — eval dashboard + audit
    h.GET("/api/eval/reports", deps.evalIndexHandler)           // serves INDEX.json
    h.GET("/api/eval/reports/:ts", deps.evalReportHandler)      // serves one report
    h.GET("/api/eval/frozen", deps.evalFrozenHandler)           // frozen-set list + lock metadata
    h.GET("/api/audit", deps.auditHandler)                      // QueryAudit + retention projection
    return h
}
```

**Invariant:** the chat SSE wire (`POST /api/chat`) is NOT touched. The `/eval` route has its own (non-SSE) transport (D-04/D12 preserved by isolation — CONTEXT `<established_patterns>`).

---

### `internal/server/chat.go` (MODIFY — utility, transform)

**Analog:** itself — `computePromptHash` (lines 420-425)

**Current (weak) implementation:**
```go
// chat.go:420-425
func computePromptHash(skillName, modelVersion string) string {
    h := sha256.New()
    h.Write([]byte(skillName))
    h.Write([]byte(modelVersion))
    return hex.EncodeToString(h.Sum(nil))
}
```

**RESEARCH Q1 BLOCKING ( Pitfall 1):** the chat.go:416-419 comment explicitly defers the lift to "Phase 5" — Phase 5 shipped without doing it. The frozen lock's `prompt_hash` pin (SC-6 / D6-13) is meaningless until this is resolved. **Planner gate.**
- **Option (a) — STRENGTHEN (recommended):** add the rendered system-prompt body to the hash input. ~5-line change in chat.go + matching change in the lock writer. Accept that existing audit rows' `prompt_hash` values become stale (dev-scale, reproducibility key not lookup key — RESEARCH A3).
- **Option (b) — KEEP + DOCUMENT:** leave the hash semantics, add `eval_set_sha256` to the lock pinning the frozen JSONL content as the real drift signal.

**Whichever option is chosen, the invariant is:** the frozen `.lock.json`'s `prompt_hash` field MUST be byte-identical to whatever `computePromptHash` produces. `AssertPin` and `computePromptHash` move together.

---

### `internal/server/audit_test.go` + `internal/server/eval_test.go` (NEW — tests)

**Analog:** `internal/server/knowledge_test.go` (lines 73-115 + `invokeGraphHandler` lines 146-152)

**Handler-invocation pattern** (`knowledge_test.go:146-152` — exact helper to clone per handler):
```go
func invokeAuditHandler(t *testing.T, deps *Deps, q string) *ut.ResponseRecorder {
    t.Helper()
    h := server.Default(server.WithHostPorts(":0"))
    h.GET("/api/audit", deps.auditHandler)
    return ut.PerformRequest(h.Engine, http.MethodGet, "/api/audit?"+q, nil)
}
```
- Imports: `github.com/cloudwego/hertz/pkg/app/server` + `github.com/cloudwego/hertz/pkg/common/ut` + `github.com/stretchr/testify/{assert,require}`.

**Stub-double pattern** (`knowledge_test.go:52-71` — clone for `EvalReporter`):
```go
type stubEvalReporter struct {
    index     EvalIndex
    indexErr  error
    // ... per-method canned returns ...
}
// Compile-time check at file bottom:
var _ EvalReporter = (*stubEvalReporter)(nil)
```

**Test cases to pin (from RESEARCH Validation Architecture Wave 0):**
- `TestAuditHandler_NilStoreReturns503` — mirrors `TestGraphHandler_NilBuilderReturns503`
- `TestAuditHandler_Success` — non-admin, recent rows → raw; old rows → `[REDACTED:]`
- `TestRetentionRedact` — pure-fn test of `redact()` with canned timestamps
- `TestAdminToken` — `SKILLFORGE_ADMIN_TOKEN` set → raw; unset → redacted; wrong token (timing-safe) → redacted
- `TestAuditHandler_SQLi` — `user=' OR '1'='1` → returns no rows / survives (mirror the existing T-3-SQLI regression)
- `TestEvalReports_PathTraversal` — `:ts='../etc/passwd'` → 400 (regex gate)
- `TestEvalReports_NilReporterReturns503`

---

### `internal/server/chat_session_test.go` (MODIFY — test, batch concurrent)

**Analog:** itself — `TestSessionIsolationConcurrent` (lines 253-313) + `cannedChatModel` (lines 91-111)

**Pattern to scale (RESEARCH Pitfall 5 — load-bearing invariants):**
```go
func TestSessionIsolation50x5(t *testing.T) {
    // ... reuse newTestAgent(t, &cannedChatModel{}), newTestStore(t), newTestHertz(t, deps) ...
    const sessions, turns = 50, 5
    var wg sync.WaitGroup
    wg.Add(sessions)
    for i := 0; i < sessions; i++ {
        go func(idx int) {
            defer wg.Done()
            sessID := fmt.Sprintf("sess-50x5-%d", idx)
            query := fmt.Sprintf("query-session-%d-turn-unique", idx)  // UNIQUE per session
            for t := 0; t < turns; t++ {
                ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                resp, err := postChatRaw(t-ctx-placeholder, ctx, addr, sessID, query)
                _ = drainBody(t, resp)
                cancel()
            }
        }(i)
    }
    wg.Wait()
    // INVARIANTS (Pitfall 5):
    // (a) each GetSession(sessN) returns exactly 10 messages (5 user + 5 assistant)
    // (b) NONE of session N's messages contain any OTHER session's query text
    // (c) run under `go test -race`
}
```

**Invariants to preserve exactly** (Pitfall 5):
1. Each goroutine gets a UNIQUE `X-Session-Id`.
2. Each goroutine does 5 sequential POSTs.
3. After settle: `GetSession(sessN)` returns exactly 10 messages.
4. Cross-session leakage check: for each session, assert NONE of its messages contain any OTHER session's query text (the `assert.NotContains` pattern at `chat_session_test.go:305-312`).
5. Reuse `cannedChatModel` (refuses every request → exercises the refusal-path audit write too).

**Sampling rate (RESEARCH):** per-wave under `-race`, NOT per-commit (250 canned POSTs add overhead; the `WHERE session_id=?` invariant is unchanged from Phase 3).

---

### `internal/store/store.go` + `internal/store/sqlite/audit.go` (MODIFY — model + service, CRUD)

**Analog:** itself — `AuditFilter` (`store.go:96-109`) + `QueryAudit` (`audit.go:85-168`)

**`AuditFilter` extension** (`store.go:96-109`):
```go
type AuditFilter struct {
    // ... existing fields ...
    Limit int
    Offset int  // NEW — D6-05 LIMIT/OFFSET pagination
}
```

**QueryAudit SQL extension** (`audit.go:118-130`):
```go
limit := filter.Limit
if limit <= 0 { limit = 100 }

q := "SELECT " + queryAuditCols + " FROM audit"
if len(clauses) > 0 {
    q += " WHERE " + strings.Join(clauses, " AND ")
}
q += " ORDER BY ts DESC LIMIT ?"
args = append(args, limit)
// NEW (D6-05):
if filter.Offset > 0 {
    q += " OFFSET ?"
    args = append(args, filter.Offset)
}
```

**INVARIANT (T-3-AUDIT — load-bearing):** NO new UPDATE or DELETE SQL string is added to `internal/store/sqlite/audit.go`. Retention redaction is a READ-TIME server-layer projection (Pitfall 4). The store package remains INSERT + SELECT + the ONE `UpdateAuditMetrics` exception owned by the eval sidecar.

**INVARIANT (T-3-SQLI):** the new `OFFSET ?` placeholder follows the existing 100%-parameterized pattern. NEVER `fmt.Sprintf` with the offset value.

---

### `cmd/server/main.go` (MODIFY — config, request-response)

**Analog:** itself — Deps wiring (referenced from `server.go:140-181` comment block + `cmd/eval/main.go` parallel bootstrap)

**Wiring to add** (mirror how `KnowledgeBuilder` is constructed + assigned in production):
```go
evalReporter := eval.NewFilesystemReporter("eval/reports", "eval/frozen")  // new constructor
deps := &server.Deps{
    // ... existing ...
    Knowledge: knowledgeBuilder,
    EvalReporter: evalReporter,  // NEW
}
```
- Exact file-path constants mirror `cmd/eval/main.go:91-96` (`defaultReportPath` etc.).
- `cmd/server` is the parallel of `cmd/eval` for env wiring — reuse the same `envOr` / `envRequired` helpers.

---

### `skills/compliance/eval-gate.json` (NEW — config, file-I/O)

**Analog:** `skills/compliance/SKILL.md` (co-location D2-07 — same directory as the Skill it gates)

**RESEARCH Q2 BLOCKING — PATH RESOLUTION:**
- CONTEXT D6-10 says `.claude/skills/<skill>/eval-gate.json` — this is WRONG.
- The product Skill is at `skills/compliance/SKILL.md` (verified — only product Skill).
- `.claude/skills/` holds META skills (spike-findings, sketch-findings) — NOT the SkillForge product Skills.
- **RESOLUTION:** place at `skills/<skill>/eval-gate.json` (i.e. `skills/compliance/eval-gate.json`). Honors the D6-10 "co-located with the Skill it gates" specificity, matches the D6-09 path-filter `skills/*/SKILL.md`.

**File contents:**
```json
{
  "faithfulness_threshold": 0.9
}
```
- Phase 6 ships exactly ONE Skill → one threshold = 0.9 (D6-10).
- Default 0.9 for any future Skill that omits the file.

---

### `eval/frozen/compliance.jsonl` + `compliance.lock.json` (NEW — data + config, file-I/O)

**Analog:** `eval/golden/compliance.jsonl` (existing 50-case set) + `internal/eval/testdata/frozen_5.jsonl` (5-case seed)

**JSONL schema** — IDENTICAL to the golden set (`dataset.go:49-55` `Case`):
```jsonc
{"id":"compliance-001","category":"adversarial","query":"...","ground_truth":"...","expected_chunks":["c1"]}
```
- The frozen set is a pinned copy of the golden set (same `Case` schema, same `LoadGolden` reader).
- `cmd/eval --frozen=compliance` reads `eval/frozen/compliance.jsonl` instead of `eval/golden/compliance.jsonl`.

**`.lock.json` sidecar schema** (RESEARCH Example 2):
```json
{
  "model_ver": "glm-5.2",
  "prompt_hash": "<hex SHA-256 — MUST match computePromptHash output post-Q1>",
  "seed": 42,
  "eval_set_version": "compliance-2026-08-13-v1",
  "skill_name": "compliance"
}
```

---

### `eval/reports/INDEX.json` (NEW — data, file-I/O)

**NO existing analog** — greenfield format. Schema defined in RESEARCH Pattern 2. Created on first `cmd/eval --archive` run via `eval.WriteArchive`.

---

### `.github/workflows/eval-gate.yml` (NEW — config / CI, batch)

**NO existing analog** — confirmed no `.github/` directory exists. Greenfield GitHub Actions workflow.

**Standard pattern (RESEARCH §State of the Art — native `paths:` filter, no third-party action):**
```yaml
name: eval-gate
on:
  pull_request:
    paths:
      - 'skills/*/SKILL.md'
      - 'eval/golden/*'
      - 'eval/frozen/*'
      - 'internal/eval/**'
      - 'internal/agent/**'

jobs:
  eval-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
      - name: Run frozen eval
        env:
          GLM_API_KEY: ${{ secrets.GLM_API_KEY }}    # NEVER echo; NEVER on default branch via push
          BGE_M3_URL: ${{ secrets.BGE_M3_URL }}
          MILVUS_ADDRESS: ${{ secrets.MILVUS_ADDRESS }}
        run: go run ./cmd/eval --frozen=compliance --archive
      - name: Gate decision + PR comment
        run: |
          set -euo pipefail
          # Run the pure gateDecision helper; format markdown table; fail loud on regression.
          go run ./cmd/eval --gate-check=eval/report.json > gate-comment.md
          # (alternative: a dedicated cmd/evalgate binary)
      - name: Comment PR
        if: always()
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh pr comment "$PR_NUMBER" --body-file gate-comment.md
      - name: Upload artifacts
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: eval-reports
          path: |
            eval/report.json
            eval/calibration.json
```

**Invariants (D6-09 / D6-11 / D6-12 + ASVS V6 / Pitfall 3):**
- `pull_request` ONLY (NOT `push` to main) — secret exposure on default branch.
- `${{ secrets.GLM_API_KEY }}` — never logged, never in report.json/calibration.json (verified `internal/eval/chatmodel.go:121-131` has no env access by design).
- `gh pr comment` markdown table names the Skill + threshold + ❌/✅ (SC-2 requires naming the Skill — D6-11).
- Path-filter (D6-09) uses native `paths:` — NO third-party `dorny/paths-filter` action needed.
- GitHub default: secrets are NOT available to forked-PR workflows (the safe path).
- `[skip-eval]` override deferred to Phase 7 (D6-12).

---

### `web/src/components/eval/EvalDashboard.tsx` (NEW — component / route, request-response)

**Analog:** `web/src/App.tsx::LiveApp` (lines 54-168 — composition root pattern) + `web/src/components/graph/KnowledgeGraphBrowser.tsx` (data-fetching + empty/loading/error states)

**Composition pattern** (from `App.tsx:122-141`):
```tsx
return (
  <>
    <TopBar ... />                              // optional: admin-mode top bar
    <main data-testid="eval-dashboard">
      <SkillsTable />                           {/* reads /api/eval/reports via store slice */}
      <TrendPanel />                            {/* recharts line + dashed baseline */}
      <CrossEvalTab />                          {/* D6-14 */}
    </main>
    <ReloadToast />                             {/* reuse existing component */}
  </>
);
```

**State-loading pattern** (from `KnowledgeGraphBrowser.tsx:40-50`):
- Read state from `useAppStore` eval slice (NEW — see app-store.ts assignment).
- Branch on `status: 'loading' | 'error' | 'ready'` (mirror `KnowledgeGraphBrowser.tsx:52-108`).

**Polling pattern (D6-03 — manual Refresh + 30s poll):**
- `useEffect` with `setInterval(refetch, 30_000)` — mounted only on `/eval` route.
- Manual Refresh button dispatches the same `refetch`.

**Invariant (D6-01):** entering `/eval` hides the chat three-panel; the dashboard is its own cognitive surface. NO SSE transport on this route — plain JSON fetch.

---

### `web/src/components/eval/SkillsTable.tsx` (NEW) + `.test.tsx`

**Analog:** `web/src/components/graph/KnowledgeGraphBrowser.tsx` (data-driven panel) + `__tests__/KnowledgeGraphBrowser.test.tsx` (test pattern)

**Data source (D6-04 + CONTEXT `<canonical_refs>`):**
- Reads `/api/eval/reports` (INDEX.json) — one row per Skill.
- The Skills table also consumes `GET /api/skills` data for the Skill list (`item.manifest.*` per the BLOCKING CONSTRAINT — verified `app-store.ts:99-100`).
- Columns: `faithfulness`, `context_precision` (= "citation precision"), `answer_relevance`, `context_recall`, derived `refusal_accuracy`, latest-run `ts`, ❌/✅ vs threshold badge.

**refusal_accuracy derivation (RESEARCH Pattern 3):**
```ts
const refusalAccuracy = report.per_category?.['refusal-required']?.['context_recall'] ?? 0;
```

**Status-color tokens (sketch-findings SKILL.md — load-bearing):**
```tsx
<span className={cn('text-xs',
  faithfulness >= threshold ? 'text-status-compliant' : 'text-status-non-compliant')}>
  {faithfulness >= threshold ? '✅' : '❌'}
</span>
```
- `compliant` (green) / `needs-info` (yellow) / `non-compliant` (red) / `unanswered` (gray) — established in `sketch-findings-ai-agent` SKILL.md and already wired in `LeftSidebar.tsx:209-213`.

**Test pattern** (`__tests__/KnowledgeGraphBrowser.test.tsx:64-83` — exact helpers to clone):
```tsx
function mockFetchOk(body: unknown, status = 200): ReturnType<typeof vi.fn> { /* ... */ }
function mockFetchError(message: string): ReturnType<typeof vi.fn> { /* ... */ }

it('renders one row per Skill with faith/cp/ar/refusal_acc + ❌/✅ badge', async () => {
  mockFetchOk(SAMPLE_INDEX);
  render(<SkillsTable />);
  const rows = await screen.findAllByTestId('eval-skills-row');
  expect(rows).toHaveLength(1);
  expect(rows[0]).toHaveTextContent('0.92');             // faithfulness
  expect(rows[0]).toHaveAttribute('data-pass', 'true');  // ✅
});
```
- Imports: `@testing-library/react`, `vitest`, `@testing-library/jest-dom/vitest`.
- File path: `web/src/components/eval/SkillsTable.test.tsx` (co-located, matching Phase-5 `graph/` does NOT co-locate — but the planner picks; RESEARCH Validation Architecture writes `src/components/eval/SkillsTable.test.tsx`).

---

### `web/src/components/eval/TrendPanel.tsx` (NEW) + `.test.tsx`

**Analog:** `web/src/components/graph/*` (panel pattern) + the `dataviz` mental model from sketch-findings

**recharts pattern (RESEARCH Example 4):**
```tsx
import { LineChart, Line, ReferenceLine, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

<TrendPanel runs={runs} baseline={frozenBaseline} deltaThresholdPct={5} />

function TrendPanel({ runs, baseline, deltaThresholdPct }: Props) {
  const latest = runs.at(-1)?.faithfulness ?? 0;
  const driftPct = baseline ? Math.abs(latest - baseline) * 100 : 0;
  return (
    <div>
      {baseline && driftPct > deltaThresholdPct && (
        <div role="alert" data-testid="trend-drift-flag">
          ⚠ Live faithfulness drifted {driftPct.toFixed(1)}% from frozen baseline
        </div>
      )}
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={runs}>
          <XAxis dataKey="ts" /><YAxis domain={[0, 1]} />
          <Tooltip />
          <Line dataKey="faithfulness" stroke="var(--status-compliant)" dot />
          {baseline && (
            <ReferenceLine y={baseline} stroke="var(--text-muted)" strokeDasharray="4 4"
              label={{ value: 'frozen baseline', fill: 'var(--text-muted)' }} />
          )}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
```

**RESEARCH Open Question 3 (planner decision — recharts vs hand-rolled SVG):**
- recharts ~95KB gzipped — proportionate for a headline SC-1 feature.
- Hand-rolled SVG is the defensible anti-dep alternative (the project's D14 anti-dep stance — `web/package.json` shows NO chart lib today).
- Either choice MUST use the same status-color tokens (`--status-compliant` etc.) for visual consistency.

**Invariant (D6-16):** overlay the frozen-set baseline as a gray dashed reference line (unifies SC-1 trend + SC-6 reproducibility on one visual). The `>5%` drift flag (D6-14) is the SAME numeric threshold as cross-eval — Pitfall 8 made visible.

---

### `web/src/components/eval/CrossEvalTab.tsx` (NEW — component, transform)

**Analog:** `web/src/components/graph/GraphTree.tsx` (table/list rendering) + `internal/eval/crosseval.go` (the diff function it surfaces)

**Pattern:**
- Reads `/api/eval/reports` filtered by `judge_model` (per-judge reports live at `eval/reports/<judge>/<ts>.json`).
- Renders a table: one row per Skill, columns per judge model, ❌ flag on any row with `>5%` faithfulness delta.
- The diff computation lives in `internal/eval/crosseval.go` (pure fn); the tab is a thin view.

---

### `web/src/components/eval/AuditTable.tsx` (NEW) + `.test.tsx`

**Analog:** `web/src/components/graph/GraphTree.tsx` (data table pattern) + `LeftSidebar.tsx` status-color tokens + `KnowledgeGraphBrowser.tsx` filter-bar pattern (`GraphFilterInput`)

**Filter bar (D6-05 — `user`, `skill`, `since`, `until`, `refusal_reason`, `limit`, `offset`):**
- Mirror `GraphFilterInput` (the input + onChange controlled pattern).
- Map filter state to `/api/audit?...` query string.

**Redaction UX (D6-07):**
```tsx
{row.Query.startsWith('REDACTED:') ? (
  <span data-testid="audit-row-redacted"
    title={row.Query}  // hover detail: full REDACTED:<reason> string w/ ts + boundary
    className="inline-flex items-center gap-1 rounded-md bg-surface-3 px-2 py-0.5 text-xs text-text-muted">
    REDACTED (90d+)
  </span>
) : (
  <span className="text-xs text-text">{row.Query}</span>
)}
```
- **Invariant (D6-07):** raw content is NEVER expandable from the UI without the admin token. The badge is gray (`text-text-muted`); the hover tooltip shows timestamp + retention boundary.

**Test cases:**
- `renders rows from /api/audit` (mock INDEX-like audit rows)
- `renders [REDACTED] badge on rows past retention` (mock row with old `ts` + `Query='REDACTED:...'`)
- `filter bar narrows rows by skill` (mirror `KnowledgeGraphBrowser.test.tsx:129-151` filter test)

---

### `web/src/store/app-store.ts` (MODIFY — store, request-response)

**Analog:** itself — existing slices + actions (lines 116-264)

**Eval slice to add (D14 — NO `messages` field, load-bearing):**
```ts
// Phase 6 — eval dashboard slice. INVARIANT (D14 / file-header comment L7-12):
// NO `messages` field. useChat owns the chat stream; the eval slice holds
// reports / auditRows / filters / route state ONLY.
export type EvalRoute = 'dashboard' | 'audit';
export interface EvalState {
  route: 'chat' | 'eval';              // D6-01 admin-mode toggle
  evalSubroute: EvalRoute;             // /eval vs /eval/audit
  indexReports: IndexRun[];            // /api/eval/reports
  auditRows: AuditRow[];               // /api/audit
  auditFilter: AuditFilter;            // filter-bar state
  frozenBaseline?: FrozenSetInfo;      // /api/eval/frozen — for TrendPanel overlay
}
```

**Action pattern (mirror `setSkills`, `setLastRouting` at `app-store.ts:217-227`):**
```ts
setRoute: (route: 'chat' | 'eval') => set({ route }),
setEvalSubroute: (sub: EvalRoute) => set({ evalSubroute: sub }),
setIndexReports: (runs: IndexRun[]) => set({ indexReports: runs }),
setAuditRows: (rows: AuditRow[]) => set({ auditRows: rows }),
setAuditFilter: (f: AuditFilter) => set({ auditFilter: f }),
```

**DEFAULTS (mirror `app-store.ts:178-199`):**
```ts
route: 'chat',
evalSubroute: 'dashboard',
indexReports: [],
auditRows: [],
auditFilter: { limit: 100, offset: 0 },
frozenBaseline: undefined,
```

**BANNED (file-header comment lines 7-12 — load-bearing):** do NOT add a `messages` field. The eval slice and the chat `useChat` stream live on different routes; the chat wire (`POST /api/chat` SSE) is untouched.

---

### `web/src/App.tsx` (MODIFY — composition root, request-response)

**Analog:** itself — `LiveApp` (lines 54-168)

**Branching pattern (RESEARCH Open Question 4 — minimal in-store router recommended):**
```tsx
function LiveApp() {
  const route = useAppStore((s) => s.route);
  // ... existing hooks ...

  if (route === 'eval') {
    return <EvalDashboard />;   // /eval OR /eval/audit (subroute read inside)
  }
  return (
    <>
      <ThreePanel left={<LeftSidebar />} center={...} right={...} />
      <ReloadToast />
      {/* ... */}
    </>
  );
}
```

**popstate / hashchange listener (RESEARCH §Standard Stack — alternative to react-router):**
```tsx
useEffect(() => {
  function onPop() {
    const path = window.location.pathname;
    if (path === '/eval') useAppStore.getState().setRoute('eval');
    else if (path === '/eval/audit') { setRoute('eval'); setEvalSubroute('audit'); }
    else useAppStore.getState().setRoute('chat');
  }
  window.addEventListener('popstate', onPop);
  return () => window.removeEventListener('popstate', onPop);
}, []);
```
- ~30 LOC. Aligns with D14 (everything in app-store) + 5 phases of avoiding react-router.
- react-router is the heavier alternative (RESEARCH Open Question 4) — only if shareable URLs become a requirement.

**Invariant (D6-01):** entering `/eval` hides the chat three-panel; returning to `/` restores it. The two routes are mutually exclusive at the composition root.

---

### `web/src/components/layout/LeftSidebar.tsx` (MODIFY — component, event-driven)

**Analog:** itself — 4-section pattern (`新建会话` / `会话` / `文件` / `技能`) at lines 86-233

**Nav entry to add (D6-01):**
```tsx
{/* 0. Eval Dashboard nav entry — admin mode toggle (D6-01) */}
<section>
  <button
    type="button"
    data-testid="left-sidebar-eval-dashboard"
    onClick={() => useAppStore.getState().setRoute('eval')}
    className="block w-full text-left rounded-md px-2 py-1.5 text-xs text-text hover:bg-surface-2"
  >
    Eval Dashboard
  </button>
</section>
```
- Place as section 0 (top of sidebar) OR as a new section between sessions and files — planner picks.
- Subscribe to `route` so the active state highlights when `route === 'eval'`:
```ts
const route = useAppStore((s) => s.route);
```

**Icon convention** (LeftSidebar uses `lucide-react` — `MessageCircle`, `Folder`, `Wrench`): pick a lucide icon for the eval dashboard (`BarChart3` or `Activity` are conventional).

---

## Shared Patterns

### Authentication / Authorization (admin token — binary split)

**Source:** NEW — `internal/server/audit.go::isAdminRequest` (RESEARCH Pattern 1)
**Apply to:** `internal/server/audit.go` only (the `/api/audit` handler)

```go
// ASVS V6.2.1 — constant-time compare, NOT ==.
// Unset SKILLFORGE_ADMIN_TOKEN → isAdmin=false for ALL requests
// (demo works OOB with retention-redacted rows; raw access requires the env).
func isAdminRequest(c *app.RequestContext) bool {
    expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
    if expected == "" { return false }
    var got string
    c.Request.Header.VisitAll(func(k, v []byte) {
        if string(k) == "X-Admin-Token" { got = string(v) }
    })
    return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
```

**Threat model (RESEARCH §Security Domain):** timing attack on token compare (Spoofing) → mitigated by `subtle.ConstantTimeCompare`. IDOR on `/api/audit` (Information disclosure) → mitigated by binary admin token + retention redaction. Real RBAC is v2 SCALE-* (D6-08 deferred).

---

### Error Handling (handler envelope)

**Source:** `internal/server/knowledge.go:61-63` + lines 90-95
**Apply to:** all NEW `internal/server/*.go` handlers (`audit.go`, `eval.go`)

```go
type graphResponseError struct {
    Error string `json:"error"`
}
// Reuse this envelope OR define a sibling auditResponseError / evalResponseError.

// 503 posture when Deps seam is nil (mirrors graphHandler:76-81):
if s.Store == nil {
    c.JSON(http.StatusServiceUnavailable, graphResponseError{
        Error: "audit store not initialized (set Deps.Store at server construction)",
    })
    return
}

// 500 generic message — W-3 convention. NEVER err.Error() in the response body:
log.Printf("[WARN] server: audit query failed: %v", err)
c.JSON(http.StatusInternalServerError, graphResponseError{
    Error: "audit query failed — see server logs",
})
```

**Status-code convention (mirror `knowledge.go`):**
- nil seam → 503
- bad request (malformed body / bad `:id` / bad `:ts`) → 400
- not found → 404
- internal error → 500 generic (full err logged at `[WARN]` server-side)

---

### Validation (path traversal + SQLi)

**Source:** `internal/store/sqlite/audit.go:85-168` (SQLi-safe) + `internal/server/knowledge.go::chunkHandler` (`:id` int-parse)
**Apply to:** `/api/audit` (SQLi — reuse AuditFilter) + `/api/eval/reports/:ts` (path traversal — NEW regex gate)

```go
// /api/eval/reports/:ts — VALIDATE :ts BEFORE any os.ReadFile.
// A `..` or `/` in :ts is the path-traversal attack vector (V5 ASVS).
var validTimestamp = regexp.MustCompile(`^[0-9T:-]+Z$`)
if !validTimestamp.MatchString(ts) {
    c.JSON(http.StatusBadRequest, ...)
    return
}
```

```go
// /api/audit — NEVER build SQL in the handler. Reuse store.QueryAudit:
// 100% ?-parameterized (T-3-SQLI regression-tested; DROP TABLE survives).
f := store.AuditFilter{UserID: string(c.Query("user")), /* ... */}
rows, err := s.Store.QueryAudit(ctx, f)
// Also: integer-clamp limit/offset (reject negative/huge — V5 ASVS).
```

---

### Atomic File Writes (INDEX.json race)

**Source:** RESEARCH Pitfall 7
**Apply to:** `internal/eval/archive.go::WriteArchive`

```go
// Write to <path>.tmp then os.Rename — atomic on POSIX; Windows same-volume works.
tmp := indexPath + ".tmp"
if err := os.WriteFile(tmp, b, 0o644); err != nil { return err }
if err := os.Rename(tmp, indexPath); err != nil { return err }
// NOT safe under concurrent processes — document this. Git is the canonical history.
```

---

### Frontend Component Convention (data-driven panel + status-color tokens)

**Source:** `web/src/components/graph/KnowledgeGraphBrowser.tsx` + `web/src/components/layout/LeftSidebar.tsx:209-213`
**Apply to:** all NEW `web/src/components/eval/*.tsx` components

```tsx
// 1. Self-driving hook OR store-slice reader; branch on status (mirror KnowledgeGraphBrowser.tsx:40-108):
const { data, status, error, refetch } = useEvalReports();  // or useAppStore
if (status === 'error')   return <div data-testid="eval-error">...</div>;
if (status === 'loading') return <div data-testid="eval-loading">...</div>;
if (!data || data.length === 0) return <div data-testid="eval-empty">...</div>;

// 2. Status-color tokens (sketch-findings SKILL.md — established palette):
//    text-status-compliant (green) / text-status-non-compliant (red)
//    text-status-needs-info (yellow) / text-status-unanswered (gray)
<span className={cn('text-xs', pass ? 'text-status-compliant' : 'text-status-non-compliant')}>
  {pass ? '✅' : '❌'}
</span>

// 3. data-testid on every interactive node (mirror KnowledgeGraphBrowser.tsx:55,81,96,111):
<div data-testid="eval-skills-row" data-skill-id={s.skill} data-pass={pass}>...</div>
```

---

### Frontend Test Convention (mockFetch + Vitest)

**Source:** `web/src/__tests__/KnowledgeGraphBrowser.test.tsx:64-83, 86-170`
**Apply to:** all NEW `web/src/components/eval/*.test.tsx`

```tsx
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  useAppStore.setState({ /* reset relevant slice */ });
});

function mockFetchOk(body: unknown, status = 200) {
  const fn = vi.fn().mockImplementation(async () => ({
    ok: status >= 200 && status < 300, status,
    json: async () => body,
  } as unknown as Response));
  vi.stubGlobal('fetch', fn);
  return fn;
}
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.github/workflows/eval-gate.yml` | config (CI) | batch | Greenfield — confirmed no `.github/` directory exists. Use the standard GitHub Actions pattern in RESEARCH + the YAML skeleton in this file's Pattern Assignment. |
| `eval/reports/INDEX.json` (schema) | data | file-I/O | Greenfield format — schema defined in RESEARCH Pattern 2. No existing JSON-manifest-with-append-semantics to copy from. The atomic-write pattern comes from `internal/store/sqlite/audit.go`'s parameter-assembly discipline + RESEARCH Pitfall 7. |
| `internal/eval/crosseval.go` (the `>5%` diff logic) | utility | transform | No existing multi-judge diff exists in `internal/eval/`. The pure-fn shape mirrors `calibration.go::Calibrate`; the `>5%` threshold itself is documented in `eval/README.md` (Pitfall 8) — Phase 6 operationalizes the doc. |

---

## Metadata

**Analog search scope:**
- `internal/eval/*.go` (10 files — the shipped evaluator substrate)
- `internal/server/*.go` (16 files — the Hertz HTTP surface)
- `internal/store/**/*.go` (6 files — the Store interface + SQLite impl)
- `cmd/eval/main.go` + `cmd/server/main.go` (the two production CLIs)
- `web/src/components/{graph,inspector,layout,center,chat,verdict}/*` (the Phase-5 React substrate)
- `web/src/store/app-store.ts` + `web/src/App.tsx` (the Zustand + composition roots)
- `web/src/__tests__/*` (the Vitest pattern)
- `.claude/skills/{spike-findings,sketch-findings}-ai-agent/SKILL.md` (the design-language source)
- `skills/compliance/SKILL.md` (verified the SOLE product Skill — confirms eval-gate.json path)
- `eval/` (existing `golden/` + `README.md` + `NOTICE` — confirmed no `frozen/` or `reports/` yet)

**Files scanned:** 35 source + 2 SKILL.md + 4 planning docs (CONTEXT, RESEARCH, prior-phase carry-forwards)

**Pattern extraction date:** 2026-08-13

**Cross-cutting invariants the planner MUST preserve:**
1. **T-3-AUDIT INSERT-only** — no UPDATE/DELETE path is added to `internal/store/sqlite/audit.go`. Retention redaction is READ-TIME server-layer.
2. **T-3-SQLI parameterization** — `/api/audit` reuses `store.QueryAudit` 1:1; the new `OFFSET ?` placeholder follows the existing 100%-parameterized pattern.
3. **D14 no-`messages`** — the eval slice in `app-store.ts` holds `reports`/`auditRows`/`filters`/`route`, NEVER `messages`.
4. **D-04/D12 transport-swap** — the `/eval` route has its own (non-SSE) transport. The chat SSE wire is untouched.
5. **ASVS V6.2.1** — `X-Admin-Token` uses `subtle.ConstantTimeCompare`, NOT `==`.
6. **V5 ASVS path-traversal** — `/api/eval/reports/:ts` MUST regex-validate `:ts` (`^[0-9T:-]+Z$`) before `os.ReadFile`.
7. **D6-09 CI secret hygiene** — `GLM_API_KEY` only via `${{ secrets.GLM_API_KEY }}`; `pull_request` trigger only.
8. **`computePromptHash` ↔ `.lock.json` byte-identity** — RESEARCH Q1 BLOCKING; whichever option the planner picks, the hash function and the lock-pinning move together.
9. **`eval-gate.json` path** — RESEARCH Q2 BLOCKING; place at `skills/<skill>/eval-gate.json`, NOT `.claude/skills/<skill>/eval-gate.json`.
10. **`cmd/eval` does NOT exit non-zero on low faithfulness** — the gate decision lives in `gateDecision()` called from CI, not the eval CLI.
