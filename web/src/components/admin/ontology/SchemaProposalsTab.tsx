// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — schema proposals list + generate form (SC-6). Rows show
// kind / local name / status / time; clicking a pending row opens the
// ProposalDrawer. The generate box accepts free text (admin pastes a
// domain snippet; Phase 39's ECL pipeline will feed merged notes here
// automatically — D-Induct-06).

import { useCallback, useEffect, useState } from 'react';
import { Sparkles } from 'lucide-react';
import { cn } from '../../../lib/cn';
import { fetchProposals, generateProposal } from './ontologyApi';
import type { Proposal } from './types';
import ProposalDrawer from './ProposalDrawer';

const STATUS_BADGE: Record<string, string> = {
  pending: 'bg-surface-3 text-text',
  approved: 'bg-primary-dim text-primary',
  rejected: 'bg-danger/10 text-danger',
};

export default function SchemaProposalsTab() {
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [text, setText] = useState('');
  const [loading, setLoading] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Proposal | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await fetchProposals();
      setProposals(resp.proposals ?? []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function handleGenerate() {
    if (!text.trim()) return;
    setGenerating(true);
    setError(null);
    try {
      await generateProposal(text.trim());
      setText('');
      await reload();
    } catch (e) {
      setError(String(e));
    } finally {
      setGenerating(false);
    }
  }

  return (
    <section data-testid="schema-proposals" className="space-y-3">
      <h2 className="text-sm font-semibold">Schema Proposals</h2>

      {/* Generate box */}
      <div className="rounded-md border border-border bg-surface-2 p-3">
        <label className="mb-1 block text-xs text-text-muted" htmlFor="proposal-source-text">
          粘贴领域文本，LLM 提议一个新的 Class / Property（仅生成 pending 记录，需人工审批）
        </label>
        <div className="flex gap-2">
          <textarea
            id="proposal-source-text"
            data-testid="proposal-source-text"
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={2}
            className="flex-1 rounded-md border border-border bg-surface px-2 py-1.5 text-xs"
            placeholder="e.g. 财务对账流程中经常引用「银行流水」这一数据资产……"
          />
          <button
            type="button"
            data-testid="proposal-generate"
            onClick={handleGenerate}
            disabled={generating || !text.trim()}
            className="inline-flex h-fit items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-semibold text-primary-fg disabled:opacity-50"
          >
            <Sparkles className="h-3.5 w-3.5" />
            {generating ? '生成中…' : 'Generate'}
          </button>
        </div>
      </div>

      {error && <div className="text-xs text-danger">{error}</div>}

      {/* Proposal list */}
      <div className="overflow-hidden rounded-md border border-border">
        <table className="w-full text-left text-xs">
          <thead className="bg-surface-2 text-text-muted">
            <tr>
              <th className="px-3 py-2">ID</th>
              <th className="px-3 py-2">Kind</th>
              <th className="px-3 py-2">本地名</th>
              <th className="px-3 py-2">描述</th>
              <th className="px-3 py-2">状态</th>
              <th className="px-3 py-2">时间</th>
            </tr>
          </thead>
          <tbody data-testid="proposal-rows">
            {loading && (
              <tr>
                <td colSpan={6} className="px-3 py-3 text-center text-text-muted">
                  加载中…
                </td>
              </tr>
            )}
            {!loading &&
              proposals.map((p) => (
                <tr
                  key={p.id}
                  data-testid={`proposal-row-${p.id}`}
                  onClick={() => setSelected(p)}
                  className={cn(
                    'cursor-pointer border-t border-border hover:bg-surface-2',
                  )}
                >
                  <td className="px-3 py-1.5 font-mono">{p.id}</td>
                  <td className="px-3 py-1.5">{p.kind}</td>
                  <td className="px-3 py-1.5 font-mono">skf:{p.local_name}</td>
                  <td className="max-w-xs truncate px-3 py-1.5" title={p.description_zh}>
                    {p.description_zh}
                  </td>
                  <td className="px-3 py-1.5">
                    <span
                      className={cn(
                        'rounded px-1.5 py-0.5 text-[11px] font-semibold',
                        STATUS_BADGE[p.status] ?? 'bg-surface-3 text-text-muted',
                      )}
                    >
                      {p.status}
                    </span>
                  </td>
                  <td className="px-3 py-1.5 text-text-muted">
                    {new Date(p.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            {!loading && proposals.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-3 text-center text-text-muted">
                  暂无 proposal — 在上方粘贴文本生成第一个
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <ProposalDrawer
        proposal={selected}
        onClose={() => setSelected(null)}
        onDecided={() => void reload()}
      />
    </section>
  );
}
