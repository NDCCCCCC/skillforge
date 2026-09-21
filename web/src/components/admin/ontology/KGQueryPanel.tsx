// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — KG triple query panel (SC-5). Three inputs (s / p / o),
// empty = wildcard (`?x`). Results render as a monospace triple table;
// "Export JSON" downloads the raw response. The placeholder shows the
// canonical example `?x a skf:Skill` mapping: s=?x, p=rdf:type, o=Skill.

import { useState } from 'react';
import { Download, Search } from 'lucide-react';
import { fetchKGQuery } from './ontologyApi';
import type { KGTriple } from './types';

const RDF_TYPE = 'http://www.w3.org/1999/02/22-rdf-syntax-ns#type';
const DEFAULT_NS = 'http://skillforge.local/ontology#';

export default function KGQueryPanel() {
  const [s, setS] = useState('');
  const [p, setP] = useState(RDF_TYPE);
  const [o, setO] = useState(`${DEFAULT_NS}Skill`);
  const [matches, setMatches] = useState<KGTriple[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function runQuery() {
    setLoading(true);
    setError(null);
    try {
      const resp = await fetchKGQuery(s.trim(), p.trim(), o.trim());
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
      <h2 className="text-sm font-semibold">KG 三元组查询</h2>

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
        <button
          type="button"
          data-testid="kg-query-run"
          onClick={runQuery}
          disabled={loading}
          className="mt-5 inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg disabled:opacity-50"
        >
          <Search className="h-3.5 w-3.5" />
          Query
        </button>
        <button
          type="button"
          data-testid="kg-query-export"
          onClick={exportJSON}
          disabled={!matches}
          className="mt-5 inline-flex items-center gap-1 rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-semibold hover:bg-surface-3 disabled:opacity-50"
        >
          <Download className="h-3.5 w-3.5" />
          Export JSON
        </button>
      </div>

      {error && <div className="text-xs text-danger">查询失败: {error}</div>}

      <div className="max-h-72 overflow-y-auto rounded-md border border-border">
        <table className="w-full text-left font-mono text-[11px]">
          <thead className="sticky top-0 bg-surface-2 text-text-muted">
            <tr>
              <th className="px-2 py-1.5">subject</th>
              <th className="px-2 py-1.5">predicate</th>
              <th className="px-2 py-1.5">object</th>
            </tr>
          </thead>
          <tbody data-testid="kg-query-results">
            {(matches ?? []).map((t, i) => (
              <tr key={i} className="border-t border-border">
                <td className="break-all px-2 py-1">{t.s}</td>
                <td className="break-all px-2 py-1 text-text-muted">{t.p}</td>
                <td className="break-all px-2 py-1">{t.o}</td>
              </tr>
            ))}
            {matches !== null && matches.length === 0 && (
              <tr>
                <td colSpan={3} className="px-2 py-3 text-center text-text-muted">
                  无匹配三元组
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {matches !== null && (
        <div className="font-mono text-[11px] text-text-muted">
          {matches.length} match(es)
        </div>
      )}
    </section>
  );
}
