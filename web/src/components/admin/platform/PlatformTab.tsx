// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — /admin/platform tab shell: three-view switcher
// (SPARQL Playground / MCP Inspector / Hybrid 调试). Replaces the
// placeholder from Plan 41-04.
//
// View-switcher idiom: GovernanceTab pattern (role="tablist" +
// aria-selected buttons, local state, no router). skillName state is
// lifted to tab level and passed to SPARQL Playground and Hybrid views.

import { useState } from 'react';
import { cn } from '../../../lib/cn';
import SparqlPlaygroundView from './SparqlPlaygroundView';
import McpInspectorView from './McpInspectorView';
import HybridDebugView from './HybridDebugView';

type PlatformView = 'sparql' | 'mcp' | 'hybrid';

const VIEWS: Array<{ id: PlatformView; label: string }> = [
  { id: 'sparql', label: 'SPARQL Playground' },
  { id: 'mcp', label: 'MCP Inspector' },
  { id: 'hybrid', label: 'Hybrid 调试' },
];

export default function PlatformTab() {
  const [view, setView] = useState<PlatformView>('sparql');
  const [skillName, setSkillName] = useState('');

  return (
    <div className="space-y-4" data-testid="platform-tab">
      {/* View switcher */}
      <div className="flex items-center gap-1" role="tablist">
        {VIEWS.map((v) => (
          <button
            key={v.id}
            type="button"
            role="tab"
            aria-selected={view === v.id}
            data-testid={`platform-view-${v.id}`}
            onClick={() => setView(v.id)}
            className={cn(
              'rounded-md px-3 py-1.5 text-xs font-semibold transition-colors',
              view === v.id ? 'bg-primary-dim text-primary' : 'text-text-muted hover:bg-surface-2 hover:text-text',
            )}
          >
            {v.label}
          </button>
        ))}
      </div>

      {/* View bodies */}
      {view === 'sparql' && (
        <SparqlPlaygroundView skillName={skillName} onSkillChange={setSkillName} />
      )}
      {view === 'mcp' && <McpInspectorView />}
      {view === 'hybrid' && <HybridDebugView skillName={skillName} />}
    </div>
  );
}
