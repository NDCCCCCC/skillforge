---
phase: quick-260928-jxb
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/components/admin/governance/governanceApi.ts
  - web/src/components/admin/governance/governanceApi.test.ts
  - web/src/components/admin/platform/PlatformTab.tsx
  - web/src/components/admin/platform/PlatformTab.test.tsx
  - cmd/server/main.go
  - .planning/quick/sf44-start.sh
autonomous: true
must_haves:
  truths:
    - "治理 tab renders the SHACL report (quarantine/warnings zones) instead of white-screening when BE sends warnings:null or edges:null"
    - "SPARQL Playground Run Query sends a real skill name in X-Skill-Name — no 401 'missing X-Skill-Name header'"
    - "Hybrid 调试 submits with the same selected skill (one selector serves both views)"
    - "GET /api/admin/platform/mcp/status no longer 503s when MCP is enabled at boot"
    - "sf44-start.sh boots the server against GLM_* while LOCAL endpoint is down"
  artifacts:
    - path: "web/src/components/admin/governance/governanceApi.ts"
      provides: "null→[] normalization for warnings/quarantined/nodes/edges/entries"
      contains: "?? []"
    - path: "web/src/components/admin/governance/governanceApi.test.ts"
      provides: "regression pin for null-array normalization"
    - path: "web/src/components/admin/platform/PlatformTab.tsx"
      provides: "shared Skill selector above view bodies, auto-defaults to first skill"
      contains: "platform-skill-select"
    - path: "web/src/components/admin/platform/PlatformTab.test.tsx"
      provides: "regression pin: selector renders + auto-defaults non-empty"
    - path: "cmd/server/main.go"
      provides: "deps literal includes MCPServer: mcpSrv (hoisted var)"
      contains: "MCPServer: mcpSrv"
    - path: ".planning/quick/sf44-start.sh"
      provides: "TEMP GLM fallback via unset LOCAL_* (untracked, NOT committed)"
      contains: "unset LOCAL_BASE_URL"
  key_links:
    - from: "web/src/components/admin/platform/PlatformTab.tsx (selector onChange)"
      to: "platformApi.ts skillHeaders → X-Skill-Name header"
      via: "lifted skillName state passed to SparqlPlaygroundView + HybridDebugView"
      pattern: "setSkillName|X-Skill-Name"
    - from: "cmd/server/main.go deps literal"
      to: "internal/server/server.go:487 Deps.MCPServer"
      via: "MCPServer: mcpSrv in the &server.Deps{...} literal"
      pattern: "MCPServer:\\s+mcpSrv"
---

<objective>
Fix the 3 bugs found in the 2026-09-28 operator admin-console walkthrough, plus one env-only start-script change:

1. **BUG-1 (P0)**: 治理 tab white-screens — BE Go nil slices serialize to `"warnings":null` / `"edges":null`; FE consumption sites call `.length`/`.some`/`.map` on null with no error boundary → whole AdminDashboard dies.
2. **BUG-2 (P1)**: SPARQL Playground + Hybrid 调试 always 401 — `PlatformTab` holds `skillName` state initialized `''` but renders NO selector, so `X-Skill-Name: ''` is sent.
3. **BUG-3 (P1)**: MCP admin API 503 — `deps.MCPServer` never wired (Phase 43-01 forward-declaration leftover: the `if deps != nil` patch at main.go:993-995 runs before `deps` is assigned at 1059, and the literal omits the field).
4. **TASK-4 (env-only)**: `sf44-start.sh` sources `.env` whose `LOCAL_*` vars (endpoint 10.62.10.36:8888, DOWN) override valid `GLM_*`; unset `LOCAL_*` after sourcing.

Purpose: restore the admin console's 治理 + 平台 tabs to functional; unblock operator live verification.
Output: 3 atomic commits (FE null-guard + test, FE selector + test, BE wiring). Start-script edit stays untracked.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md

<interfaces>
<!-- Verified contracts — executor needs no further codebase exploration. -->

From web/src/components/admin/governance/types.ts:
```typescript
export interface ShaclReport {
  health: ShaclHealth; error_count: number; warning_count: number;
  warnings: ShaclViolation[];          // wire delivers null when empty
  quarantined: QuarantinedEntity[];    // wire delivers [] (verified live)
  triple_count: number; shapes_path: string; validated_at: string;
  trigger: 'boot' | 'rollback';
}
export interface ProvenanceGraph {
  nodes: GraphNode[];                  // verify live showed edges:null
  edges: GraphEdge[];                  // null when empty
}
```

From web/src/store/app-store.ts (lines 308-317, 442, 634):
```typescript
export interface SkillSummary {
  id: string; name: string; description: string;
  version?: number; status: 'ok' | 'failed'; error?: string;
}
// AppState: skills: SkillSummary[]  (line 442)
// export const useAppStore = create<...>(...)  (line 634)
// useSkillReload polls GET /api/skills continuously from App mount —
// store.skills is already populated by the time /admin is opened. NO new fetch needed.
```

From web/src/components/admin/platform/platformApi.ts (lines 25-27):
```typescript
function skillHeaders(skill: string): HeadersInit {
  return { 'X-Admin-Token': localStorage.getItem(ADMIN_TOKEN_KEY) ?? '', 'X-Skill-Name': skill };
}
```

From cmd/server/main.go:
```go
// line 135
"github.com/skillforge/skillforge/internal/server/platform/mcp"
// line 965:  var deps *server.Deps
// line 971-996: mcpServiceSkill check → mcpEnabled if/else; mcpSrv := mcp.NewServer(ontologyKG, mcpServiceSkill) INSIDE else branch (line 980)
// line 993-995: dead `if deps != nil { deps.MCPServer = mcpSrv }` (deps is nil here — assigned at 1059)
// line 1059-1167: deps = &server.Deps{...} literal; last field ECL: eclManager, (line 1166)
```

From internal/server/server.go (line 487): `MCPServer *mcp.Server`

From cmd/server/main.go (lines 302-304) — env precedence LOCAL_* > GLM_* > default:
```go
glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))
glmBase := envOr("LOCAL_BASE_URL", envOr("GLM_BASE_URL", defaultGLMBaseURL))
serverModel := envOr("LOCAL_MODEL", envOr("GLM_MODEL", "glm-5.3-flash"))
```

Pattern to copy for the selector (web/src/components/admin/ontology/KGQueryPanel.tsx:82-99):
"Skill 视角：" label + `<select>` with skill options — but there the list comes from KG enumeration; here use store.skills (simpler, already polled).
</interfaces>

## Source audit (all 4 items covered)

| Item | Plan coverage |
|------|---------------|
| BUG-1 null crash | Task 1 |
| BUG-2 missing skill selector | Task 2 |
| BUG-3 deps.MCPServer never wired | Task 3 |
| TASK-4 start script GLM switch | Task 3 (env-only, untracked) |
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Normalize null arrays in governance API wrappers (BUG-1)</name>
  <files>web/src/components/admin/governance/governanceApi.ts, web/src/components/admin/governance/governanceApi.test.ts</files>
  <behavior>
    - fetchShaclReport with BE body `{"health":"warn","warnings":null,"quarantined":[...],...}` resolves with `warnings: []` (never null) — this exact payload white-screened ShaclReportView.tsx:156
    - fetchProvenanceGraph with `{"nodes":[...],"edges":null}` resolves with `edges: []` — this exact payload crashed ProvenanceGraphView.tsx:147
    - fetchChangelog with `{"current_version":1,"entries":null}` resolves with `entries: []` (same Go nil-slice class)
    - Normal (non-null) bodies pass through unchanged — arrays keep their contents
  </behavior>
  <action>
    Normalize at the single choke point — the fetch wrappers in governanceApi.ts (NOT at consumption sites; per walkthrough recommendation, keep ShaclReportView/GovernanceTab/ProvenanceGraphView as-is). Go nil slices serialize to JSON `null`, and the FE types claim arrays; this file already owns response normalization (error envelope unwrapping), so array normalization belongs here too.

    In `fetchShaclReport`: parse the JSON body, then return `{ ...data, warnings: data.warnings ?? [], quarantined: data.quarantined ?? [] }` (quarantined delivered `[]` today, but same one-line guard costs nothing and HealthBanner:45 reads `.length` on it).

    In `fetchProvenanceGraph`: return `{ ...data, nodes: data.nodes ?? [], edges: data.edges ?? [] }`.

    In `fetchChangelog`: normalize `entries: data.entries ?? []` the same way.

    Then write `governanceApi.test.ts` (new file — none exists today) with vitest + `vi.stubGlobal('fetch', ...)` mocking `Response.json` bodies per the behavior cases above; 4 tests: shacl-null-warnings, provenance-null-edges, changelog-null-entries, shacl-normal-passthrough. Copy the `jsonResponse` helper pattern from AdminDashboard.test.tsx:15-20. IMPORTANT: this is a NEW untracked file — it MUST be explicitly `git add`ed (untracked files silently missing from commits has bitten this repo 5+ times).
  </action>
  <verify>
    <automated>cd web && npx tsc --noEmit && npx vitest run src/components/admin/governance/governanceApi.test.ts</automated>
  </verify>
  <done>All 4 normalization tests pass; tsc clean; consumption sites untouched (git diff shows only governanceApi.ts + new test).</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Add shared Skill selector to PlatformTab (BUG-2)</name>
  <files>web/src/components/admin/platform/PlatformTab.tsx, web/src/components/admin/platform/PlatformTab.test.tsx</files>
  <behavior>
    - PlatformTab renders `<select data-testid="platform-skill-select">` with one option per store skill (status === 'ok' only — failed skills are absent from the KG and the BE would 401 on them)
    - When store.skills loads and skillName is still '', it auto-defaults to the first ok skill's name (empty string = guaranteed 401; must never persist)
    - Selecting a different skill updates the shared state — visible to BOTH SparqlPlaygroundView and HybridDebugView (they already receive the lifted skillName prop)
    - Empty store.skills → selector shows a 加载中 placeholder option, no crash
  </behavior>
  <action>
    In PlatformTab.tsx: import `useAppStore` from '../../../store/app-store'; read `const skills = useAppStore((s) => s.skills);` and derive `const skillNames = skills.filter((s) => s.status === 'ok').map((s) => s.name);`. Render a selector row BETWEEN the view switcher and the view bodies — shared by SPARQL + Hybrid views since both consume skillName (MCP Inspector is server-scoped, unaffected). Copy the KGQueryPanel.tsx:82-99 idiom: `<span className="text-xs text-text-muted">Skill 视角：</span>` + `<select data-testid="platform-skill-select" value={skillName} onChange={(e) => setSkillName(e.target.value)} className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs">`. Include a placeholder `<option value="" disabled>{skillNames.length === 0 ? '加载中…' : '选择 Skill'}</option>` so the never-empty invariant is visible, then map skillNames to options. Add `useEffect` that auto-selects the first skill when `skillName === '' && skillNames.length > 0` (deps: [skillName, skillNames]). Do NOT touch SparqlPlaygroundView/HybridDebugView props or internals — the existing lifted-state plumbing and the Ctrl+Enter deps re-attach keep working unchanged.

    Then write `PlatformTab.test.tsx` (new file): render `<PlatformTab />` after `useAppStore.setState({ skills: [{id:'1', name:'compliance', description:'', status:'ok'}, {id:'2', name:'calculator', description:'', status:'ok'}, {id:'3', name:'broken', description:'', status:'failed'}] })` (import useAppStore like AdminDashboard.test.tsx:13 does); stub global fetch to resolve 404 for everything (the child views degrade, that's fine). Assert: `screen.getByTestId('platform-skill-select')` exists, its value auto-defaulted to `'compliance'` (waitFor), option list contains compliance + calculator but NOT broken, and fireEvent.change to `'calculator'` updates the value. IMPORTANT: new untracked file — MUST be explicitly `git add`ed.
  </action>
  <verify>
    <automated>cd web && npx tsc --noEmit && npx vitest run src/components/admin/platform/PlatformTab.test.tsx</automated>
  </verify>
  <done>Selector renders once above the views, auto-defaults to a real skill name (never ''), one selector drives both SPARQL + Hybrid; tsc clean; test passes.</done>
</task>

<task type="auto">
  <name>Task 3: Wire deps.MCPServer at boot (BUG-3) + temp GLM switch in start script (TASK-4)</name>
  <files>cmd/server/main.go, .planning/quick/sf44-start.sh</files>
  <action>
    **main.go (BUG-3)** — the `if deps != nil { deps.MCPServer = mcpSrv }` block at lines 993-995 is dead: `deps` is a nil `*server.Deps` (line 965) until the literal at 1059, and `mcpSrv` is scoped inside the else branch (line 980) so the literal can't reference it either. Fix by hoisting the variable:
    1. Immediately BEFORE the `mcpEnabled := os.Getenv("SKILLFORGE_MCP_ENABLED")` line (975), declare `var mcpSrv *mcp.Server`.
    2. In the else branch, change `mcpSrv := mcp.NewServer(ontologyKG, mcpServiceSkill)` to the assignment form `mcpSrv = mcp.NewServer(ontologyKG, mcpServiceSkill)`.
    3. DELETE the dead block: the stale comment at 989-992 ("Store server reference for admin API (Task 3) … wired later via a follow-up patch below") plus the `if deps != nil { deps.MCPServer = mcpSrv }` lines. Dead code removal preferred over keeping it (the patch it promised never landed).
    4. In the `deps = &server.Deps{...}` literal, add `MCPServer: mcpSrv,` as the last field right after `ECL: eclManager,` (line 1166), with a comment in the file's established style: nil when SKILLFORGE_MCP_ENABLED disables MCP → /api/admin/platform/mcp/* answer 503 (same nil-posture as ECL/Ontology). This satisfies the admin MCP status/tools/invoke handlers which currently always see nil.
    Do not reorder or reformat surrounding lines — minimal diff.

    **sf44-start.sh (TASK-4)** — env-only helper, untracked and NOT gitignored (verified: `git check-ignore` exit 1, `git log` empty): edit in place, do NOT commit it. After the `set +a` line (line 10), add:
    a comment `# TEMP (2026-09-28): LOCAL endpoint 10.62.10.36:8888 (Unsloth) is DOWN — connection refused.` plus `# Unset LOCAL_* so the server falls back to GLM_* (precedence LOCAL_* > GLM_* > default, main.go:302-304).` and the line `unset LOCAL_BASE_URL LOCAL_API_KEY LOCAL_MODEL`.

    **Commit hygiene:** commit ONLY cmd/server/main.go in this task's commit — explicitly `git add cmd/server/main.go` (the script stays untracked on purpose).
  </action>
  <verify>
    <automated>wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go build ./cmd/server && CGO_ENABLED=1 go test ./internal/server/... -count=1'</automated>
  </verify>
  <done>`go build` succeeds and the full internal/server CGO test suite passes (WSL, per env-wsl-backend-runbook — Windows cgo.exe is broken). `grep -n "MCPServer: mcpSrv" cmd/server/main.go` finds the literal field. sf44-start.sh contains the unset line with TEMP comment and remains untracked (`git status --porcelain .planning/quick/sf44-start.sh` shows `??`, not `M`).</done>
</task>

</tasks>

<verification>
1. `cd web && npx tsc --noEmit` — clean after Tasks 1+2.
2. `cd web && npx vitest run src/components/admin/governance/governanceApi.test.ts src/components/admin/platform/PlatformTab.test.tsx` — all new tests green.
3. `cd web && npx vitest run src/components/admin` — closest existing regression net (AdminDashboard.test.tsx, OntologyTab.test.tsx) still green.
4. WSL build + `CGO_ENABLED=1 go test ./internal/server/... -count=1` — green after Task 3.
5. All commits made; worktree has no modified tracked files remaining; new test files ARE in the commits (`git status` shows no `??` under web/src).
</verification>

<success_criteria>
- BUG-1: governance API wrappers can no longer deliver null arrays to view components; the exact live payloads observed in the walkthrough (`warnings:null`, `edges:null`) are pinned by tests.
- BUG-2: SPARQL Run Query and Hybrid 搜索 submit a real skill name via X-Skill-Name; selector auto-defaults, one control serves both views.
- BUG-3: deps literal wires MCPServer; dead if-block removed; CGO build + server test suite green.
- TASK-4: start script boots on GLM while LOCAL is down; script intentionally untracked.
- Orchestrator runtime smoke (post-execution): GET /api/admin/platform/mcp/status ≠ 503; 治理 tab renders quarantine table; SPARQL query returns rows.
</success_criteria>

<output>
Create `.planning/quick/260928-jxb-admin-3-bug-null-guard-sparql-skill-sele/260928-jxb-SUMMARY.md` when done.
</output>
