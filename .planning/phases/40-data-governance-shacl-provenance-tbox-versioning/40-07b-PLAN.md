---
phase: "40"
plan: "07b"
type: execute
wave: 3
depends_on:
  - "40-07a"
files_modified:
  - web/package.json
  - web/src/components/admin/governance/ShaclReportView.tsx
  - web/src/components/admin/governance/ProvenanceGraphView.tsx
  - web/src/components/admin/governance/TimelineView.tsx
autonomous: false
requirements:
  - ONT-14
  - ONT-15
  - ONT-16
  - ONT-17

must_haves:
  truths:
    - "ShaclReportView renders red quarantine table + yellow warnings table per D-SHACL-03"
    - "ProvenanceGraphView renders with Cytoscape.js breadthfirst layout"
    - "TimelineView renders version list with rollback buttons"
    - "cytoscape ^3.x pinned in package.json"
  artifacts:
    - path: "web/src/components/admin/governance/ProvenanceGraphView.tsx"
      provides: "Cytoscape.js wrapper with breadthfirst layout, search, detail panel"
    - path: "web/src/components/admin/governance/ShaclReportView.tsx"
      provides: "Quarantine + warnings tables with all 40-UI-SPEC columns"
    - path: "web/src/components/admin/governance/TimelineView.tsx"
      provides: "Version timeline with rollback lifecycle"
  key_links:
    - from: "web/src/components/admin/governance/ShaclReportView.tsx"
      to: "web/src/components/admin/governance/GovernanceTab.tsx"
      via: "HealthBanner imported from GovernanceTab"
    - from: "web/src/components/admin/governance/GovernanceTab.tsx"
      to: "web/src/components/admin/governance/ShaclReportView.tsx"
      via: "view === 'shacl' && <ShaclReportView />"
---

<objective>
Implement the three governance views (ShaclReportView / ProvenanceGraphView / TimelineView) and add cytoscape dependency. These complete the /admin/governance tab, replacing the stubs left by 40-07a.
</objective>

<context>
@.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md
@web/src/components/admin/governance/types.ts
@web/src/components/admin/governance/governanceApi.ts
@web/src/components/admin/ontology/SchemaProposalsTab.tsx
</context>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<tasks>

<task type="auto">
  <name>Task 1: Add cytoscape dependency to package.json</name>
  <files>
    - web/package.json
  </files>
  <read_first>
    - web/package.json
  </read_first>
  <behavior>
    - Test: `cytoscape` is listed in package.json dependencies
    - Test: Package version is ^3.x
    - Test: No React wrapper libs (react-cytoscape* packages)
  </behavior>
  <action>
Add to web/package.json dependencies:
```json
"cytoscape": "^3.30.0"
```

Use the latest stable 3.x version. No react-cytoscape wrapper libraries. No layout plugins (dagre/cose-bilkent).

Install with: `cd web && pnpm add cytoscape`
  </action>
  <verify>
    <automated>cd /d/code/ClaudeCode/ai-agent/web && cat package.json | grep cytoscape</automated>
  </verify>
  <done>cytoscape ^3.x added to package.json; no wrapper libs added</done>
</task>

<task type="auto">
  <name>Task 2: ShaclReportView — quarantine + warnings tables</name>
  <files>
    - web/src/components/admin/governance/ShaclReportView.tsx
  </files>
  <read_first>
    - .planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md (Component 4)
    - web/src/components/admin/ontology/SchemaProposalsTab.tsx
  </read_first>
  <behavior>
    - Test: Red zone (quarantine table) renders when quarantined.length > 0
    - Test: Yellow zone (warnings table) renders when warnings.length > 0
    - Test: provenance-missing rows show yellow badge
    - Test: ok state shows single muted line
    - Test: provenance-missing warning row shows Run ECL 回填 button
  </behavior>
  <action>
Implement ShaclReportView per 40-UI-SPEC §Component 4:

**Layout**:
- Red zone: only when `report.health === 'error'` and `report.quarantined.length > 0`
- Yellow zone: only when `report.warnings.length > 0`
- ok state: `report.health === 'ok'` with single line

**Quarantine table** (red zone):
Columns: 实体 | 三元组 | 约束 | 违规详情 | 来源 | 行号
- 实体: `entity` URI, `mono break-all`
- 三元组: `triple_count`, mono
- 约束: first violation constraint + "+{n}" if more
- 违规详情: `detail`, `max-w-xs truncate` + `title={detail}`
- 来源: `source_path`, mono truncate
- 行号: `L{start}–L{end}` or 「—」

Below table: hint line (12px muted): 「隔离实体不进入主索引；可修复数据后通过 Run ECL 重建，或待下次启动重新校验。」

**Warnings table** (yellow zone):
Columns: 实体 | 约束 | 违规详情 | 来源 | 行号
(no 三元组 column)
- `provenance-missing` constraint rows show yellow badge: 「provenance 缺失（存量数据）」

**HealthBanner** is imported from GovernanceTab (exported in 07a). The yellow-zone backfill CTA is in the banner already.

**Note**: ShaclReportView receives the report prop from GovernanceTab. If report is null, render skeleton.
  </action>
  <verify>
    <automated>cd /d/code/ClaudeCode/ai-agent/web && npx tsc --noEmit 2>&1 | grep -i shacl | head -10</automated>
  </verify>
  <done>Quarantine table renders red zone; warnings table renders yellow zone; provenance-missing badge shows</done>
</task>

<task type="auto">
  <name>Task 3: ProvenanceGraphView — Cytoscape.js wrapper</name>
  <files>
    - web/src/components/admin/governance/ProvenanceGraphView.tsx
  </files>
  <read_first>
    - .planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md (Component 5)
  </read_first>
  <behavior>
    - Test: Graph renders with breadthfirst layout
    - Test: Node click shows detail panel
    - Test: Search highlights matching node and dims others
    - Test: Quarantined nodes have dashed red border
    - Test: Empty state shows when no nodes
  </behavior>
  <action>
Implement ProvenanceGraphView per 40-UI-SPEC §Component 5:

**Cytoscape integration** (no React wrapper lib):
```tsx
import cytoscape from 'cytoscape';
import { useEffect, useRef, useState } from 'react';
import { fetchProvenanceGraph } from './governanceApi';
import type { GraphNode, ProvenanceGraph } from './types';

export default function ProvenanceGraphView() {
  const containerRef = useRef<HTMLDivElement>(null);
  const [cy, setCy] = useState<cytoscape.Core | null>(null);
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);
  const [graphData, setGraphData] = useState<ProvenanceGraph | null>(null);
  const [graphError, setGraphError] = useState<string | null>(null);
  const [searchQ, setSearchQ] = useState('');

  // Fetch graph data on mount
  useEffect(() => {
    fetchProvenanceGraph().then(setGraphData).catch(e => setGraphError(e.message));
  }, []);

  // Initialize Cytoscape
  useEffect(() => {
    if (!containerRef.current || !graphData) return;
    const c = cytoscape({
      container: containerRef.current,
      elements: [
        ...graphData.nodes.map(n => ({ data: { id: n.id, label: n.label, kind: n.kind } })),
        ...graphData.edges.map(e => ({ data: { source: e.source, target: e.target } })),
      ],
      layout: { name: 'breadthfirst', directed: true, roots: ['#file', '#skill', '#skillmd'], levelSep: 48, nodeSep: 16, padding: 16 },
      wheelSensitivity: 0.2,
      userZoomingEnabled: true,
      userPanningEnabled: true,
      autoungrabify: true,
      autounselectify: false,
      style: [
        { selector: 'node', style: {
          label: 'data(label)',
          'font-family': 'var(--font-mono)',
          'font-size': '11px',
          shape: 'round-rectangle',
          'background-color': 'var(--color-primary-dim)',
          'border-color': 'var(--color-primary)',
          'border-width': '2px',
          width: 30, height: 20,
        }},
        { selector: 'edge', style: {
          'target-arrow-shape': 'triangle',
          'target-arrow-color': 'var(--color-border-muted)',
          'line-color': 'var(--color-border-muted)',
          width: 1,
        }},
        { selector: '.dim', style: { opacity: 0.15 } },
        { selector: '.hit', style: { 'border-color': 'var(--color-primary)', 'border-width': '3px' } },
        { selector: '[kind="file"]', style: { shape: 'rectangle', 'background-color': 'var(--color-surface-3)', 'border-color': 'var(--color-border-muted)' }},
        { selector: '[kind="table"]', style: { 'background-color': 'var(--color-ok-tint)', 'border-color': 'var(--color-status-compliant)' }},
        { selector: '[kind="quarantined"]', style: { shape: 'round-rectangle', 'border-style': 'dashed', 'border-color': 'var(--color-status-non-compliant)' }},
      ],
    });
    c.on('tap', 'node', (e) => {
      const nodeId = e.target.id();
      const node = graphData.nodes.find(n => n.id === nodeId);
      setSelectedNode(node ?? null);
    });
    setCy(c);
    return () => c.destroy();
  }, [graphData]);

  // Search highlight
  const handleSearch = (q: string) => {
    setSearchQ(q);
    if (!cy || !graphData) return;
    cy.elements().removeClass('hit dim');
    if (!q) return;
    const match = graphData.nodes.find(n => n.id.includes(q) || n.label.includes(q));
    if (match) {
      cy.$(`#${match.id}`).addClass('hit');
      cy.$(`#${match.id}`).closedNeighborhood().addClass('hit');
      cy.elements().not('.hit').addClass('dim');
    }
  };

  const detailPanel = selectedNode ? (
    <div className="w-72 space-y-2 text-xs">
      <div><span className="text-text-muted">IRI</span><br/><span className="mono break-all">{selectedNode.id}</span></div>
      <div><span className="text-text-muted">来源</span><br/>{selectedNode.provenance?.source_kind ?? '—'}</div>
      {selectedNode.provenance?.source_path && (
        <div><span className="text-text-muted">路径</span><br/><span className="mono break-all">{selectedNode.provenance.source_path}</span></div>
      )}
      {selectedNode.provenance?.line_start && (
        <div><span className="text-text-muted">行号</span><br/><span className="mono">L{selectedNode.provenance.line_start}–L{selectedNode.provenance.line_end}</span></div>
      )}
      {selectedNode.provenance?.extracted_at && (
        <div><span className="text-text-muted">时间</span><br/><span className="mono">{new Date(selectedNode.provenance.extracted_at).toLocaleString()}</span></div>
      )}
      {selectedNode.tier && <div><span className="text-text-muted">tier</span><br/><span className="mono">{selectedNode.tier}</span></div>}
    </div>
  ) : (
    <div className="w-72 text-text-muted text-xs">点击节点查看溯源明细</div>
  );

  if (graphError) return <div className="text-danger text-sm">溯源图谱加载失败（点击重试）</div>;
  if (!graphData) return <div className="h-96 bg-surface animate-pulse rounded" />;
  if (graphData.nodes.length === 0) return <div className="py-12 text-text-muted text-sm">暂无溯源数据 — 运行 ECL 后生成溯源链</div>;

  return (
    <div className="grid gap-4 lg:grid-cols-[1fr_288px]">
      <div>
        <div className="flex gap-2 mb-2">
          <input data-testid="provenance-search" value={searchQ} onChange={e => handleSearch(e.target.value)}
            placeholder="搜索实体（IRI 或名称）" className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs" />
          <button onClick={() => { setSearchQ(''); cy?.elements().removeClass('hit dim'); }}
            className="border border-border bg-surface-2 px-2 py-1.5 text-xs rounded hover:bg-surface-3"
            disabled={!searchQ}>清除高亮</button>
        </div>
        <div ref={containerRef} className="min-h-96 overflow-hidden rounded-md border border-border bg-surface"
          role="img" aria-label={`溯源图谱：${graphData.nodes.length} 个实体、${graphData.edges.length} 条关系`} />
      </div>
      <div className="border border-border rounded p-3">{detailPanel}</div>
    </div>
  );
}
```

**Cytoscape styles via CSS variables** — use getComputedStyle at initialization time, NOT hardcoded hex values.

**Search**: case-insensitive substring match on node `id` and `label`. No match shows inline muted 「未找到实体 {q}」.

**Empty state**: when graphData.nodes.length === 0 → 「暂无溯源数据 — 运行 ECL 后生成溯源链」
  </action>
  <verify>
    <automated>cd /d/code/ClaudeCode/ai-agent/web && npx tsc --noEmit 2>&1 | grep -i provenance | head -10</automated>
  </verify>
  <done>Graph renders with Cytoscape breadthfirst; node click shows detail panel; search highlights neighborhood; empty/error states render correctly</done>
</task>

<task type="auto">
  <name>Task 4: TimelineView — version演进 with rollback</name>
  <files>
    - web/src/components/admin/governance/TimelineView.tsx
  </files>
  <read_first>
    - .planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md (Component 6)
  </read_first>
  <behavior>
    - Test: Timeline shows entries newest-first
    - Test: who badge renders admin/induction:llm/system:rollback correctly
    - Test: Current version has 「当前」 badge
    - Test: 回滚到此版本 button on non-current entries
    - Test: Rollback confirm dialog shows
    - Test: Success toast with new violation count
  </behavior>
  <action>
Implement TimelineView per 40-UI-SPEC §Component 6:

```tsx
import { useEffect, useState } from 'react';
import { cn } from '../../../lib/cn';
import { fetchChangelog, rollbackTBox } from './governanceApi';

export default function TimelineView() {
  const [changelog, setChangelog] = useState<{ current_version: number; entries: ChangelogEntry[] } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [rollingBack, setRollingBack] = useState<number | null>(null);

  useEffect(() => {
    fetchChangelog().then(setChangelog).catch(e => setError(e.message));
  }, []);

  const handleRollback = async (version: number) => {
    if (!window.confirm(`回滚到 v${version}？将写入新版本 v${(changelog?.current_version ?? 0) + 1}、重建 KG 并自动重跑 SHACL 校验；失效引用实体将进入隔离区。`)) return;
    setRollingBack(version);
    try {
      const result = await rollbackTBox(version);
      const toast = document.createElement('div');
      toast.className = 'sf-toast-success';
      toast.textContent = `已回滚至 v${version}（当前 v${result.version}）· 新增 ${result.new_violations} 条 violation`;
      document.body.appendChild(toast);
      setTimeout(() => toast.remove(), 5000);
      fetchChangelog().then(setChangelog);
    } catch (e: any) {
      const toast = document.createElement('div');
      toast.className = 'sf-toast-error';
      toast.textContent = `回滚失败：${e.message}`;
      document.body.appendChild(toast);
      setTimeout(() => toast.remove(), 5000);
    } finally {
      setRollingBack(null);
    }
  };

  if (error) return <div className="text-danger text-sm">版本日志加载失败（点击重试）</div>;
  if (!changelog) return <div className="space-y-2">{[1,2,3].map(i => <div key={i} className="h-16 bg-surface-2 animate-pulse rounded" />)}</div>;
  if (changelog.entries.length === 0) return (
    <div className="text-text-muted text-sm">暂无演进记录 — 当前为初始版本 v{changelog.current_version}</div>
  );

  return (
    <div className="space-y-4">
      <div className="text-sm font-semibold">当前版本 <span className="text-primary mono">v{changelog.current_version}</span></div>
      <div className="space-y-3">
        {changelog.entries.map(entry => (
          <div key={entry.version} data-testid={`governance-timeline-entry-${entry.version}`}
            className="flex gap-3 items-start">
            <div className="mt-1.5 h-2 w-2 rounded-full bg-primary shrink-0" />
            <div className="flex-1 space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-xs font-mono font-semibold">v{entry.version}</span>
                <span className={cn('text-xs px-1.5 py-0.5 rounded',
                  entry.who === 'admin' ? 'bg-surface-3 text-text-muted' :
                  entry.who === 'induction:llm' ? 'bg-primary-dim text-primary' :
                  'bg-status-needs-info/10 text-status-needs-info')}>
                  {entry.who === 'admin' ? '管理员' : entry.who === 'induction:llm' ? 'LLM 归纳' : '系统回滚'}
                </span>
                {entry.version === changelog.current_version && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-status-compliant/10 text-status-compliant">当前</span>
                )}
                <span className="text-xs text-text-muted mono">{new Date(entry.when).toLocaleString()}</span>
              </div>
              <div className="text-xs text-text-muted">
                +{entry.what.added.length} / -{entry.what.removed.length}
                {entry.what.added.length > 0 && (
                  <details className="inline">
                    <summary className="cursor-pointer text-xs">详情</summary>
                    <pre className="text-xs mt-1 p-2 bg-surface-2 rounded overflow-auto max-w-xs break-all">{entry.what.added.join('\n')}</pre>
                  </details>
                )}
              </div>
              <div className="text-xs text-text-muted line-clamp-2" title={entry.why}>{entry.why}</div>
              {entry.version !== changelog.current_version && (
                <button
                  data-testid={`governance-rollback-${entry.version}`}
                  onClick={() => handleRollback(entry.version)}
                  disabled={rollingBack !== null}
                  className="mt-1 border border-border bg-surface-2 px-2 py-1 rounded text-xs hover:bg-surface-3 disabled:opacity-50"
                  aria-label={`回滚到版本 v${entry.version}`}>
                  {rollingBack === entry.version ? '回滚中...' : '回滚到此版本'}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```
  </action>
  <verify>
    <automated>cd /d/code/ClaudeCode/ai-agent/web && npx tsc --noEmit 2>&1 | grep -i timeline | head -10</automated>
  </verify>
  <done>Timeline renders entries newest-first; who badges correct; 回滚到此版本 buttons on non-current; rollback lifecycle (confirm -> POST -> toast) works</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 5: Update GovernanceTab to use real views</name>
  <files>
    - web/src/components/admin/governance/GovernanceTab.tsx
  </files>
  <action>
Replace the stub view bodies in GovernanceTab (from 07a) with real view imports:

In GovernanceTab.tsx:
1. Add imports:
```tsx
import ShaclReportView from './ShaclReportView';
import ProvenanceGraphView from './ProvenanceGraphView';
import TimelineView from './TimelineView';
```

2. Replace the three stub divs with:
```tsx
{view === 'shacl' && <ShaclReportView report={shaclReport} />}
{view === 'provenance' && <ProvenanceGraphView />}
{view === 'timeline' && <TimelineView />}
```

3. Remove the stub divs and the "加载中..." placeholder text.

The HealthBanner component should remain exported from GovernanceTab (used by ShaclReportView).

**What to verify:**
1. Navigate to /admin?tab=governance
2. Verify health banner renders
3. Click through all three views — each should show the real component
4. No TypeScript errors
  </action>
  <verify>
    <how-to-verify>
      1. cd web && npx tsc --noEmit — verify no errors
      2. Browser: navigate to /admin?tab=governance, click each view tab
    </how-to-verify>
    <resume-signal>
      Type "approved" or describe issues
    </resume-signal>
  </verify>
  <done>GovernanceTab renders real ShaclReportView / ProvenanceGraphView / TimelineView; health banner still exported</done>
</task>

</tasks>

<output>
Create `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-07b-SUMMARY.md` when done
</output>
