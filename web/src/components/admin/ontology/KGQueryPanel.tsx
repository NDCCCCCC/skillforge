// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — KG triple query panel (SC-5). Three inputs (s / p / o),
// empty = wildcard (`?x`). Results render as a monospace triple table;
// "Export JSON" downloads the raw response. The placeholder shows the
// canonical example `?x a skf:Skill` mapping: s=?x, p=rdf:type, o=Skill.
// Plan 40-08 — adds Skill selector (D-TIER-01 tier demo) and 来源 provenance
// column (D-PROV-03).
//
// 2610 redesign: Skill-list mount load migrated to the shared
// useAdminData hook; buttons migrated to ui/button (primary/secondary);
// header migrated to PageHeader; local cn copy replaced with lib/cn.
// The user-triggered pattern query stays an explicit mutation — it is
// not an initial load, so it keeps the imperative fetch flow.

import { useEffect, useState } from 'react';
import { Download, Search } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { Button } from '../../ui/button';
import { PageHeader } from '../shared/Primitives';
import { useAdminData } from '../shared/useAdminData';
import { fetchKGQuery } from './ontologyApi';
import type { KGQueryResponse, KGTriple } from './types';

const RDF_TYPE = 'http://www.w3.org/1999/02/22-rdf-syntax-ns#type';
const DEFAULT_NS = 'http://skillforge.local/ontology#';

/** Mount-time URL for the Skill selector options (?x a skf:Skill). */
const SKILL_LIST_URL = `/api/admin/ontology/kg/query?p=${encodeURIComponent(
  RDF_TYPE,
)}&o=${encodeURIComponent(`${DEFAULT_NS}Skill`)}`;

export default function KGQueryPanel() {
  const [s, setS] = useState('');
  const [p, setP] = useState(RDF_TYPE);
  const [o, setO] = useState(`${DEFAULT_NS}Skill`);
  const [matches, setMatches] = useState<KGTriple[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedSkill, setSelectedSkill] = useState('');
  const { data: skillList } = useAdminData<KGQueryResponse>(SKILL_LIST_URL);

  const skills: string[] = (skillList?.matches ?? [])
    .map((m) => {
      const prefix = `${DEFAULT_NS}skill_`;
      return m.s.startsWith(prefix) ? m.s.slice(prefix.length) : null;
    })
    .filter((n): n is string => n !== null);

  async function runQuery() {
    setLoading(true);
    setError(null);
    try {
      const resp = await fetchKGQuery(s.trim(), p.trim(), o.trim(), selectedSkill || undefined);
      setMatches(resp.matches);
    } catch (e) {
      setError(String(e));
      setMatches(null);
    } finally {
      setLoading(false);
    }
  }

  function exportJSON() {
    if (!matches) return;
    const blob = new Blob([JSON.stringify({ matches }, null, 2)], {
      type: 'application/json',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'kg-query-result.json';
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <section data-testid="kg-query-panel" className="space-y-3">
      <PageHeader
        title="KG 三元组查询"
        description="按 s/p/o 模式查询知识图谱，留空即通配（?x）"
      />

      {/* Skill selector row */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-text-muted">Skill 视角：</span>
        <select
          data-testid="kg-query-skill-select"
          value={selectedSkill}
          onChange={(e) => {
            setSelectedSkill(e.target.value);
            runQuery();
          }}
          className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs"
        >
          <option value="">不过滤（admin 基线）</option>
          {skills.map((sk) => (
            <option key={sk} value={sk}>
              {sk}
            </option>
          ))}
        </select>
      </div>

      <div className="grid gap-2 md:grid-cols-[1fr_1fr_1fr_auto_auto]">
        <label className="space-y-1">
          <span className="font-mono text-[11px] text-text-muted">s (subject)</span>
          <input
            data-testid="kg-query-s"
            value={s}
            onChange={(e) => setS(e.target.value)}
            placeholder="?x（留空通配）"
            className="w-full rounded-md border border-border bg-surface-2 px-2 py-1.5 font-mono text-xs"
          />
        </label>
        <label className="space-y-1">
          <span className="font-mono text-[11px] text-text-muted">p (predicate)</span>
          <input
            data-testid="kg-query-p"
            value={p}
            onChange={(e) => setP(e.target.value)}
            placeholder="rdf:type"
            className="w-full rounded-md border border-border bg-surface-2 px-2 py-1.5 font-mono text-xs"
          />
        </label>
        <label className="space-y-1">
          <span className="font-mono text-[11px] text-text-muted">o (object)</span>
          <input
            data-testid="kg-query-o"
            value={o}
            onChange={(e) => setO(e.target.value)}
            placeholder="skf:Skill"
            className="w-full rounded-md border border-border bg-surface-2 px-2 py-1.5 font-mono text-xs"
          />
        </label>
        <Button
          type="button"
          data-testid="kg-query-run"
          onClick={() => void runQuery()}
          disabled={loading}
          size="sm"
          className="mt-5"
        >
          <Search className="h-3.5 w-3.5" />
          Query
        </Button>
        <Button
          type="button"
          data-testid="kg-query-export"
          onClick={exportJSON}
          disabled={!matches}
          variant="secondary"
          size="sm"
          className="mt-5"
        >
          <Download className="h-3.5 w-3.5" />
          Export JSON
        </Button>
      </div>

      {error && <div className="text-xs text-danger">查询失败: {error}</div>}

      <div className="max-h-72 overflow-y-auto rounded-md border border-border">
        <table className="w-full text-left font-mono text-[11px]">
          <thead className="sticky top-0 bg-surface-2 text-text-muted">
            <tr>
              <th className="px-2 py-1.5">subject</th>
              <th className="px-2 py-1.5">predicate</th>
              <th className="px-2 py-1.5">object</th>
              <th className="px-2 py-1.5">来源</th>
            </tr>
          </thead>
          <tbody data-testid="kg-query-results">
            {(matches ?? []).map((t, i) => (
              <tr key={i} className="border-t border-border">
                <td className="break-all px-2 py-1">{t.s}</td>
                <td className="break-all px-2 py-1 text-text-muted">{t.p}</td>
                <td className="break-all px-2 py-1">{t.o}</td>
                <td className="px-2 py-1">
                  {t.provenance ? (
                    <span className="flex flex-col gap-0.5">
                      <StatusToneBadge kind={t.provenance.source_kind} />
                      {t.provenance.source_path && (
                        <span
                          className="text-[11px] truncate max-w-xs"
                          title={t.provenance.source_path}
                        >
                          {t.provenance.source_path}
                        </span>
                      )}
                      {t.provenance.line_start != null && t.provenance.line_start > 0 && (
                        <span className="text-[11px] text-text-muted">
                          L{t.provenance.line_start}
                          {t.provenance.line_end != null && t.provenance.line_end > 0
                            ? `–L${t.provenance.line_end}`
                            : ''}
                        </span>
                      )}
                    </span>
                  ) : (
                    <span className="text-text-muted">—</span>
                  )}
                </td>
              </tr>
            ))}
            {matches !== null && matches.length === 0 && (
              <tr>
                <td colSpan={4} className="px-2 py-3 text-center text-text-muted">
                  无匹配三元组
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {matches !== null && (
        <div className="font-mono text-[11px] text-text-muted">
          Skill 视角：{selectedSkill || '不过滤（admin 基线）'} · 匹配 {matches.length} 条
        </div>
      )}
    </section>
  );
}

/** Provenance source-kind chip — status tokens only (no hex / raw
 * color classes; 2610 redesign). */
function StatusToneBadge({ kind }: { kind: string }) {
  const label = kind === 'file' ? '文件' : kind === 'emitter' ? '生成' : '归纳';
  return (
    <span
      className={cn(
        'text-[11px] w-fit font-semibold px-1 rounded',
        kind === 'file'
          ? 'bg-status-compliant/10 text-status-compliant'
          : kind === 'emitter'
            ? 'bg-surface-3 text-text-muted'
            : 'bg-primary-dim text-primary',
      )}
    >
      {label}
    </span>
  );
}
