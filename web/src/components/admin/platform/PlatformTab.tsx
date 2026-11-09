// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 41-05 — /admin/platform tab shell: three-view switcher
// (三元组查询 / MCP Inspector / Hybrid 调试). Replaces the
// placeholder from Plan 41-04.
//
// View-switcher idiom: GovernanceTab pattern (role="tablist" +
// aria-selected buttons, local state, no router). skillName state is
// lifted to tab level and passed to the triple-query and Hybrid views.
//
// 2610 redesign: tab switcher buttons migrated to the shared Button
// (variant swap for the active tab), a PageHeader added per the 2610
// admin shell. All data-testids and the skill-selector semantics
// (quick-260928-jxb BUG-2) are unchanged.
//
// 命名跟随后端改名：SPARQL Playground → 三元组查询（端点
// /api/sparql → /api/kg/triples）。Hybrid 视图保持原名 —— 它确实
// 是混合检索。

import { useEffect, useState } from 'react';
import { cn } from '../../../lib/cn';
import { useAppStore } from '../../../store/app-store';
import { PageHeader } from '../shared/Primitives';
import { Button } from '../../ui/button';
import KGTripleQueryView from './KGTripleQueryView';
import McpInspectorView from './McpInspectorView';
import HybridDebugView from './HybridDebugView';

type PlatformView = 'triples' | 'mcp' | 'hybrid';

const VIEWS: Array<{ id: PlatformView; label: string }> = [
  { id: 'triples', label: '三元组查询' },
  { id: 'mcp', label: 'MCP Inspector' },
  { id: 'hybrid', label: 'Hybrid 调试' },
];

export default function PlatformTab() {
  const [view, setView] = useState<PlatformView>('triples');
  const [skillName, setSkillName] = useState('');
  // quick-260928-jxb BUG-2: skillName was initialized '' with NO selector,
  // so 三元组查询/Hybrid always sent X-Skill-Name: '' → 401. Shared selector
  // below serves both views (MCP Inspector is server-scoped, unaffected).
  // useSkillReload already polls store.skills from App mount — no new fetch.
  const skills = useAppStore((s) => s.skills);
  const skillNames = skills.filter((s) => s.status === 'ok').map((s) => s.name);

  // Empty string = guaranteed 401; auto-default to the first ok skill.
  useEffect(() => {
    if (skillName === '' && skillNames.length > 0) {
      setSkillName(skillNames[0]);
    }
  }, [skillName, skillNames]);

  return (
    <div className="space-y-4" data-testid="platform-tab">
      <PageHeader
        title="平台调试"
        description="三元组模式查询、MCP 工具与 Hybrid 检索的联调工作台"
      />

      {/* View switcher */}
      <div className="flex items-center gap-1" role="tablist">
        {VIEWS.map((v) => (
          <Button
            key={v.id}
            type="button"
            role="tab"
            aria-selected={view === v.id}
            data-testid={`platform-view-${v.id}`}
            variant={view === v.id ? 'default' : 'ghost'}
            size="sm"
            onClick={() => setView(v.id)}
            className={cn(view !== v.id && 'text-text-muted')}
          >
            {v.label}
          </Button>
        ))}
      </div>

      {/* Shared Skill selector (三元组查询 + Hybrid both consume skillName) */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-text-muted">Skill 视角：</span>
        <select
          data-testid="platform-skill-select"
          value={skillName}
          onChange={(e) => setSkillName(e.target.value)}
          className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs"
        >
          <option value="" disabled>
            {skillNames.length === 0 ? '加载中…' : '选择 Skill'}
          </option>
          {skillNames.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </select>
      </div>

      {/* View bodies */}
      {view === 'triples' && (
        <KGTripleQueryView skillName={skillName} />
      )}
      {view === 'mcp' && <McpInspectorView />}
      {view === 'hybrid' && <HybridDebugView skillName={skillName} />}
    </div>
  );
}
