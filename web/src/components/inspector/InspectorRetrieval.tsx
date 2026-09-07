// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// InspectorRetrieval — chunk-ID list rendered under each hop in the
// AgentInspector. Each ID is a button that dispatches
// setCenterView({kind:'chunk-source', chunkId, ...}) so the operator
// can browse the chunk verbatim from the inspector.
//
// The dispatch needs a tier for the ChunkSourceView metadata row, but the
// wire chunk carries none (the agent doesn't tag it). 26-08 residual sweep:
// the default is the REGISTRY's first authority-role slug (the citable
// role) resolved from the public projection — the hardcoded CN default was
// deleted because, after the KB migration, that literal is not a slug and
// the dispatch would target a knowledge base that cannot exist. Until the
// projection resolves, the placeholder is the EMPTY STRING and TierBadge
// renders nothing for it (an honest "tier unknown", never a fabricated
// tier).
import { useEffect, useState } from 'react';
import { firstSlugByRole } from '../../lib/kb-bases';
import { useAppStore } from '../../store/app-store';
import type { RetrievalRow } from './hop-types';

export interface InspectorRetrievalProps {
  retrievals: RetrievalRow[];
  /** Optional tier override for the chunk-source dispatch. When omitted,
   * the first authority-role slug from the KB projection is used ('' until
   * the projection resolves — no tier literal substitute). */
  defaultTier?: string;
}

export default function InspectorRetrieval({
  retrievals,
  defaultTier,
}: InspectorRetrievalProps) {
  const [registryDefault, setRegistryDefault] = useState('');

  useEffect(() => {
    if (defaultTier !== undefined) return;
    let cancelled = false;
    void firstSlugByRole('authority').then((slugValue) => {
      if (!cancelled) setRegistryDefault(slugValue);
    });
    return () => {
      cancelled = true;
    };
  }, [defaultTier]);

  if (retrievals.length === 0) return null;

  function handleClick(chunkId: string) {
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier: defaultTier ?? registryDefault,
      version: null,
      effectiveDate: null,
      effectiveTo: null,
    });
  }

  return (
    <div data-testid="inspector-retrieval" data-default-tier={defaultTier ?? registryDefault}>
      <div className="text-xs font-semibold text-text-muted">
        检索结果（{retrievals.length} 条）
      </div>
      <ul className="mt-1 flex flex-col gap-1">
        {retrievals.map((r) => (
          <li key={r.chunkId} className="flex items-center gap-2 text-xs">
            <button
              type="button"
              data-testid="inspector-retrieval-row"
              data-chunk-id={r.chunkId}
              onClick={() => handleClick(r.chunkId)}
              className="rounded-md border border-border bg-surface-2 px-2 py-1 font-mono text-text hover:bg-surface-3"
              title={r.title ?? r.chunkId}
            >
              {r.chunkId}
              {r.filename && (
                <span className="ml-2 text-text-muted">{r.filename}</span>
              )}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
