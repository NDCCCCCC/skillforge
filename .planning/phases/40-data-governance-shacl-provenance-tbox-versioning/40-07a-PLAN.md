---
phase: "40"
plan: "07a"
type: execute
wave: 3
depends_on:
  - "40-06"
files_modified:
  - web/src/components/admin/AdminDashboard.tsx
  - web/src/lib/app-store.ts
  - web/src/components/admin/governance/GovernanceTab.tsx
  - web/src/components/admin/governance/types.ts
  - web/src/components/admin/governance/governanceApi.ts
autonomous: false
requirements:
  - ONT-14
  - ONT-15
  - ONT-16
  - ONT-17

must_haves:
  truths:
    - "10th tab governance registered with ShieldCheck icon"
    - "Health banner shows red/warn/ok state matching SHACL report"
    - "Three-view switcher renders (校验报告 / 溯源图谱 / 版本演进)"
  artifacts:
    - path: "web/src/components/admin/governance/GovernanceTab.tsx"
      provides: "3-view switcher shell + HealthBanner + stub view renders"
    - path: "web/src/components/admin/governance/types.ts"
      provides: "ShaclReport, ProvenanceGraph, ChangelogEntry, GraphNode types (snake_case per 40-UI-SPEC)"
    - path: "web/src/components/admin/governance/governanceApi.ts"
      provides: "fetchShaclReport, fetchProvenanceGraph, fetchChangelog, rollbackTBox, runPipeline"
  key_links:
    - from: "web/src/components/admin/AdminDashboard.tsx"
      to: "web/src/components/admin/governance/GovernanceTab.tsx"
      via: "10th SubrouteTab entry"
---

<objective>
Wire the /admin/governance tab shell: register 10th tab, GovernanceTab with three-view switcher, HealthBanner, governance types, and governanceApi. This plan produces the infrastructure that 40-07b's views depend on.
</objective>

<context>
@.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md
@web/src/components/admin/AdminDashboard.tsx
@web/src/lib/app-store.ts
@web/src/components/admin/ontology/OntologyTab.tsx
</context>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<tasks>

<task type="auto">
  <name>Task 1: Register 10th tab + app-store union</name>
  <files>
    - web/src/components/admin/AdminDashboard.tsx
    - web/src/lib/app-store.ts
  </files>
  <read_first>
    - web/src/components/admin/AdminDashboard.tsx
    - web/src/lib/app-store.ts
    - web/src/__tests__/AdminFeedbackTabs.test.tsx
  </read_first>
  <behavior>
    - Test: AdminDashboard has 10 tabs with governance as 10th
    - Test: Tab order array extended to 10 entries
    - Test: Store union includes 'governance'
    - Test: GovernanceTab renders when adminSubroute === 'governance'
  </behavior>
  <action>
**AdminDashboard.tsx changes**:
1. Import `GovernanceTab` from `'./governance/GovernanceTab'`
2. Add to `handleSubroute` union: `'governance'`
3. Register 10th SubrouteTab:
```tsx
{ id: 'governance', label: '治理', icon: ShieldCheck }
```
4. Add view body branch:
```tsx
{adminSubroute === 'governance' && <GovernanceTab />}
```

**app-store.ts changes**:
1. Import `Subroute` type — find the existing type definition
2. Add `'governance'` to `adminSubroute` union type

**AdminFeedbackTabs test** (must extend to 10 tabs):
- Find the tab order array in `AdminFeedbackTabs.test.tsx`
- Extend from 9 to 10 entries: add `governance` to the array

**Tab icon**: `ShieldCheck` from `lucide-react`
**Tab label**: `治理`
**data-testid**: `admin-tab-governance`
  </action>
  <verify>
    <automated>cd /d/code/ClaudeCode/ai-agent/web && npx tsc --noEmit 2>&1 | head -20</automated>
  </verify>
  <done>10th tab registered with ShieldCheck icon; store union widened; GovernanceTab renders at correct route; test updated</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: GovernanceTab shell + HealthBanner + types + api</name>
  <files>
    - web/src/components/admin/governance/GovernanceTab.tsx
    - web/src/components/admin/governance/types.ts
    - web/src/components/admin/governance/governanceApi.ts
  </files>
  <read_first>
    - web/src/components/admin/ontology/OntologyTab.tsx (view-switcher idiom)
  </read_first>
  <action>
**governance/types.ts** (new file — per 40-UI-SPEC §Component 2):
```typescript
export type ShaclHealth = 'ok' | 'warn' | 'error';
export type ViolationSeverity = 'error' | 'warning';
export interface ShaclViolation {
  severity: ViolationSeverity;
  entity: string;
  constraint: string;
  detail: string;
  source_path?: string;
  line_start?: number;
  line_end?: number;
}
export interface QuarantinedEntity {
  entity: string;
  triple_count: number;
  violations: ShaclViolation[];
}
export interface ShaclReport {
  health: ShaclHealth;
  error_count: number;
  warning_count: number;
  warnings: ShaclViolation[];
  quarantined: QuarantinedEntity[];
  triple_count: number;
  shapes_path: string;
  validated_at: string;
  trigger: 'boot' | 'rollback';
}
export type ProvenanceKind = 'file' | 'emitter' | 'induction';
export interface ProvenanceInfo {
  source_kind: ProvenanceKind;
  source_path: string;
  line_start?: number;
  line_end?: number;
  extracted_at: string;
}
export interface GraphNode {
  id: string;
  kind: 'file' | 'note' | 'table' | 'skill' | 'skillmd' | 'quarantined';
  label: string;
  provenance?: ProvenanceInfo;
  tier?: string;
}
export interface GraphEdge { source: string; target: string; }
export interface ProvenanceGraph { nodes: GraphNode[]; edges: GraphEdge[]; }
export type ChangelogWho = 'admin' | 'induction:llm' | 'system:rollback';
export interface ChangelogEntry {
  version: number;
  who: ChangelogWho;
  when: string;
  what: { added: string[]; removed: string[] };
  why: string;
}
```

**governanceApi.ts** (new file):
```typescript
const ADMIN_TOKEN_KEY = 'sf_admin_token';

function adminHeaders(): HeadersInit {
  return { 'X-Admin-Token': localStorage.getItem(ADMIN_TOKEN_KEY) ?? '' };
}

export class GovernanceApiError extends Error {
  constructor(public status: number, public message: string) { super(message); }
}

export async function fetchShaclReport(): Promise<ShaclReport> {
  const r = await fetch('/api/admin/governance/shacl', { headers: adminHeaders() });
  if (!r.ok) {
    const body = await r.json().catch(() => ({ message: '网络错误' }));
    throw new GovernanceApiError(r.status, body.message ?? '网络错误');
  }
  return r.json();
}

export async function fetchProvenanceGraph(): Promise<ProvenanceGraph> {
  const r = await fetch('/api/admin/governance/provenance', { headers: adminHeaders() });
  if (!r.ok) {
    const body = await r.json().catch(() => ({ message: '网络错误' }));
    throw new GovernanceApiError(r.status, body.message ?? '网络错误');
  }
  return r.json();
}

export async function fetchChangelog(): Promise<{ current_version: number; entries: ChangelogEntry[] }> {
  const r = await fetch('/api/admin/ontology/changelog', { headers: adminHeaders() });
  if (!r.ok) {
    const body = await r.json().catch(() => ({ message: '网络错误' }));
    throw new GovernanceApiError(r.status, body.message ?? '网络错误');
  }
  return r.json();
}

export async function rollbackTBox(targetVersion: number, reason?: string): Promise<{ version: number; new_violations: number }> {
  const r = await fetch('/api/admin/ontology/rollback', {
    method: 'POST',
    headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ target_version: targetVersion, reason }),
  });
  if (!r.ok) {
    const body = await r.json().catch(() => ({ message: '网络错误' }));
    throw new GovernanceApiError(r.status, body.message ?? '网络错误');
  }
  return r.json();
}

export async function runPipeline(): Promise<void> {
  const r = await fetch('/api/admin/pipeline/run', {
    method: 'POST',
    headers: adminHeaders(),
  });
  if (!r.ok) {
    const body = await r.json().catch(() => ({ message: '网络错误' }));
    throw new GovernanceApiError(r.status, body.message ?? '网络错误');
  }
}
```

**GovernanceTab.tsx** (new file — per 40-UI-SPEC §Component 1 + Layout):
Three internal views (OntologyTab switcher idiom) + shared health banner:
```tsx
import { useEffect, useState } from 'react';
import { cn } from '../../../lib/cn';
import { fetchShaclReport, runPipeline } from './governanceApi';
import type { ShaclReport } from './types';

// HealthBanner sub-component (also referenced by ShaclReportView for yellow-zone backfill)
function HealthBanner({ report }: { report: ShaclReport }) {
  const dot = <span className={cn('h-2 w-2 rounded-full',
    report.health === 'error' ? 'bg-status-non-compliant' :
    report.health === 'warn' ? 'bg-status-needs-info' : 'bg-status-compliant')} />;
  const label = report.health === 'error' ? `发现 error 级违规 — ${report.quarantined.length} 个实体已隔离` :
    report.health === 'warn' ? '存在 warning 级警告 — 数据已放行' :
    `校验通过 — 0 违规`;
  const hasBackfill = report.health === 'warn' && report.warnings.some(w => w.constraint === 'provenance-missing');
  return (
    <div role="status" aria-live="polite" className="rounded-md border bg-surface-2 p-3 flex items-center gap-3">
      {dot}
      <span className="text-sm font-semibold">{label}</span>
      <span className="text-xs mono">error {report.error_count}</span>
      <span className="text-xs mono">warning {report.warning_count}</span>
      {hasBackfill && (
        <button onClick={() => {
          if (window.confirm('运行 ECL 管线？将重建 sql-guides 的切块、实体抽取与索引（幂等操作，可重复执行），并同时回填 provenance 与 belongsToTier。')) {
            runPipeline().catch(console.error);
          }
        }} data-testid="governance-backfill-btn"
          className="ml-auto bg-primary text-primary-fg px-3 py-1 rounded text-xs font-semibold">
          Run ECL 回填
        </button>
      )}
      <div className="text-xs text-text-muted">
        校验时点 {new Date(report.validated_at).toLocaleString()}（{report.trigger === 'boot' ? '启动时' : '回滚后'}）· shapes {report.shapes_path}
      </div>
    </div>
  );
}

type GovernanceView = 'shacl' | 'provenance' | 'timeline';

const VIEWS: Array<{ id: GovernanceView; label: string }> = [
  { id: 'shacl', label: '校验报告' },
  { id: 'provenance', label: '溯源图谱' },
  { id: 'timeline', label: '版本演进' },
];

export default function GovernanceTab() {
  const [view, setView] = useState<GovernanceView>('shacl');
  const [shaclReport, setShaclReport] = useState<ShaclReport | null>(null);
  const [shaclError, setShaclError] = useState<string | null>(null);

  useEffect(() => {
    fetchShaclReport().then(setShaclReport).catch(e => setShaclError(e.message));
  }, []);

  const healthBanner = shaclReport ? (
    <HealthBanner report={shaclReport} />
  ) : shaclError ? (
    <div className="text-danger text-sm">治理状态加载失败（点击重试）</div>
  ) : (
    <div className="h-10 bg-surface-2 animate-pulse rounded" />
  );

  return (
    <div className="space-y-4" data-testid="governance-tab">
      {/* Health banner */}
      <div data-testid="governance-health-banner">{healthBanner}</div>

      {/* View switcher */}
      <div className="flex items-center gap-1" role="tablist">
        {VIEWS.map(v => (
          <button key={v.id} role="tab" aria-selected={view === v.id}
            data-testid={`governance-view-${v.id}`}
            onClick={() => setView(v.id)}
            className={cn('rounded-md px-3 py-1.5 text-xs font-semibold',
              view === v.id ? 'bg-primary-dim text-primary' : 'text-text-muted hover:bg-surface-2')}>
            {v.label}
          </button>
        ))}
      </div>

      {/* View body — stubs for 07b; real implementations in 40-07b */}
      {view === 'shacl' && <div className="text-text-muted text-sm">校验报告加载中...</div>}
      {view === 'provenance' && <div className="text-text-muted text-sm">溯源图谱加载中...</div>}
      {view === 'timeline' && <div className="text-text-muted text-sm">版本演进加载中...</div>}
    </div>
  );
}

export { HealthBanner };
```
  </action>
  <verify>
    <how-to-verify>
      1. Open browser DevTools Network tab
      2. Navigate to /admin?tab=governance
      3. Health banner renders with correct color (red/warn/ok) — test all three states by inspecting the ShaclReport shape
      4. Three view tabs switch correctly
      5. No console errors
    </how-to-verify>
    <resume-signal>
      Type "approved" or describe issues
    </resume-signal>
  </verify>
  <done>Health banner visible with correct red/warn/ok state; three views switch; governanceApi calls correct endpoints; HealthBanner exported for ShaclReportView</done>
</task>

</tasks>

<output>
Create `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-07a-SUMMARY.md` when done
</output>
