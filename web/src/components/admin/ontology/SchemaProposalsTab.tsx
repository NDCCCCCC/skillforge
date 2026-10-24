// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — schema proposals list + generate form (SC-6). Rows show
// kind / local name / status / time; clicking a pending row opens the
// ProposalDrawer. The generate box accepts free text (admin pastes a
// domain snippet; Phase 39's ECL pipeline will feed merged notes here
// automatically — D-Induct-06).
//
// 2610 redesign: initial proposals load migrated to the shared
// useAdminData hook (reload reused after generate); the per-component
// STATUS_BADGE hex-class copy replaced with the shared StatusBadge;
// header migrated to PageHeader; loading / empty states migrated to the
// shared States primitives. The table keeps its custom markup because
// rows are interactive (click-to-open) and carry per-row testids,
// which DataTable's column contract does not express.

import { useState } from 'react';
import { Sparkles } from 'lucide-react';
import { Button } from '../../ui/button';
import StatusBadge from '../shared/StatusBadge';
import { EmptyState, LoadingSkeleton } from '../shared/States';
import { PageHeader } from '../shared/Primitives';
import { useAdminData } from '../shared/useAdminData';
import { generateProposal } from './ontologyApi';
import type { Proposal } from './types';
import ProposalDrawer from './ProposalDrawer';

export default function SchemaProposalsTab() {
  const { data, loading, error, reload } = useAdminData<{ proposals: Proposal[] }>(
    '/api/admin/ontology/proposals',
  );
  const proposals: Proposal[] = data?.proposals ?? [];
  const [text, setText] = useState('');
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Proposal | null>(null);

  async function handleGenerate() {
    if (!text.trim()) return;
    setGenerating(true);
    setGenError(null);
    try {
      await generateProposal(text.trim());
      setText('');
      reload();
    } catch (e) {
      setGenError(String(e));
    } finally {
      setGenerating(false);
    }
  }

  return (
    <section data-testid="schema-proposals" className="space-y-3">
      <PageHeader
        title="Schema Proposals"
        description="LLM 从领域文本归纳的新 Class / Property，需人工审批后写入 TBox"
      />

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
          <Button
            type="button"
            data-testid="proposal-generate"
            onClick={() => void handleGenerate()}
            disabled={generating || !text.trim()}
            size="sm"
            className="h-fit"
          >
            <Sparkles className="h-3.5 w-3.5" />
            {generating ? '生成中…' : 'Generate'}
          </Button>
        </div>
      </div>

      {error && <div className="text-xs text-danger">加载失败: {error}</div>}
      {genError && <div className="text-xs text-danger">{genError}</div>}

      {/* Proposal list */}
      {loading ? (
        <LoadingSkeleton testId="proposal-loading" />
      ) : proposals.length === 0 ? (
        <EmptyState
          title="暂无 proposal"
          hint="在上方粘贴文本生成第一个"
          testId="proposal-empty"
        />
      ) : (
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
              {proposals.map((p) => (
                <tr
                  key={p.id}
                  data-testid={`proposal-row-${p.id}`}
                  onClick={() => setSelected(p)}
                  className="cursor-pointer border-t border-border hover:bg-surface-2"
                >
                  <td className="px-3 py-1.5 font-mono">{p.id}</td>
                  <td className="px-3 py-1.5">{p.kind}</td>
                  <td className="px-3 py-1.5 font-mono">skf:{p.local_name}</td>
                  <td className="max-w-xs truncate px-3 py-1.5" title={p.description_zh}>
                    {p.description_zh}
                  </td>
                  <td className="px-3 py-1.5">
                    <StatusBadge status={p.status} />
                  </td>
                  <td className="px-3 py-1.5 text-text-muted">
                    {new Date(p.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ProposalDrawer
        proposal={selected}
        onClose={() => setSelected(null)}
        onDecided={() => reload()}
      />
    </section>
  );
}
