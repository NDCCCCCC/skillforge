// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 38-05 — the /admin?tab=ontology view body. Three internal views:
// TBox Browser (类层次) / KG Query (三元组查询) / Schema Proposals.
// Simple button switching (the tab strip inside the ontology tab — no
// router state, purely local).

import { useState } from 'react';
import { cn } from '../../../lib/cn';
import TBoxBrowser from './TBoxBrowser';
import KGQueryPanel from './KGQueryPanel';
import SchemaProposalsTab from './SchemaProposalsTab';

type OntologyView = 'tbox' | 'kg' | 'proposals';

const VIEWS: Array<{ id: OntologyView; label: string }> = [
  { id: 'tbox', label: 'TBox 浏览' },
  { id: 'kg', label: 'KG 查询' },
  { id: 'proposals', label: 'Schema Proposals' },
];

export default function OntologyTab() {
  const [view, setView] = useState<OntologyView>('tbox');

  return (
    <div data-testid="ontology-tab" className="space-y-4">
      <div className="flex items-center gap-1" role="tablist">
        {VIEWS.map((v) => (
          <button
            key={v.id}
            type="button"
            role="tab"
            aria-selected={view === v.id}
            data-testid={`ontology-view-${v.id}`}
            onClick={() => setView(v.id)}
            className={cn(
              'rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
              view === v.id
                ? 'bg-primary-dim text-primary'
                : 'text-text-muted hover:bg-surface-2 hover:text-text',
            )}
          >
            {v.label}
          </button>
        ))}
      </div>

      {view === 'tbox' && <TBoxBrowser />}
      {view === 'kg' && <KGQueryPanel />}
      {view === 'proposals' && <SchemaProposalsTab />}
    </div>
  );
}
