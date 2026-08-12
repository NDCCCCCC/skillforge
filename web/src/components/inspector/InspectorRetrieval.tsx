// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// InspectorRetrieval — chunk-ID list rendered under each hop in the
// AgentInspector. Each ID is a button that dispatches
// setCenterView({kind:'chunk-source', chunkId, ...}) so the operator
// can browse the chunk verbatim from the inspector.
//
// The wire chunks for retrieved chunks are source-document parts. We
// surface the sourceId (chunk ID) and optional title/filename. The
// tier is unknown on the wire chunk (the agent doesn't tag it); we
// default to 权威 per the SC-6 fallback (compliance Skill defaults to
// the authority tier).

import { useAppStore } from '../../store/app-store';
import type { Tier } from '../../types/verdict';
import type { RetrievalRow } from './hop-types';

export interface InspectorRetrievalProps {
  retrievals: RetrievalRow[];
  /** Default tier for the chunk-source CenterView dispatch (when the
   * source-document part doesn't carry a tier). The plan defaults to
   * 权威 for compliance-domain retrievals. */
  defaultTier?: Tier;
}

export default function InspectorRetrieval({
  retrievals,
  defaultTier = '权威',
}: InspectorRetrievalProps) {
  if (retrievals.length === 0) return null;

  function handleClick(chunkId: string) {
    useAppStore.getState().setCenterView({
      kind: 'chunk-source',
      chunkId,
      tier: defaultTier,
      version: null,
      effectiveDate: null,
      effectiveTo: null,
    });
  }

  return (
    <div data-testid="inspector-retrieval">
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