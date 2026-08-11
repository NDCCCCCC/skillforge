// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// CenterPanel — D-02 three-tab center panel. Default tab is `verdict`
// (the protagonist). Both the tab body AND the tab indicator are driven
// by `centerView.kind` from the Zustand app store (Tabs is controlled
// via `value`). This keeps a single source of truth: programmatic
// `setCenterView` calls (SkillPicker → skill-md, CitationCluster →
// chunk-source, verdict mirror) move both the content and the active
// highlight together, so the indicator never disagrees with the body
// (04-UAT test 8). The tabs are display-only status indicators here —
// navigation flows through the SkillPicker / CitationCluster, not the
// tab clicks (D-02).
//
// Empty-state hero per Copywriting Contract: `等待合规审查结果` +
// `右侧提出合规问题，agent 推理与引用会显示在这里。`
import { FileSearch } from 'lucide-react';
import { Tabs } from '../ui/tabs';
import { useAppStore } from '../../store/app-store';
import type { CenterView } from '../../store/app-store';
import type { Tier, Verdict } from '../../types/verdict';
import ChunkSourceView from './ChunkSourceView';
import RefusalCard from './RefusalCard';
import SkillMdPreview from './SkillMdPreview';
import VerdictCard from './VerdictCard';

const FALLBACK_VERDICT: Verdict = {
  status: 'unanswered',
  confidence: 0,
  citations: [],
  review_required: false,
};

/**
 * Maps the store's `centerView.kind` to the active tab value, so the
 * Tabs indicator follows programmatic `setCenterView` calls. `empty`
 * and `verdict` both surface under the `审查结论` (verdict) tab — the
 * empty-state hero renders in place of the VerdictCard when there is no
 * verdict yet (see renderBody).
 */
function activeTabValue(kind: CenterView['kind']): string {
  switch (kind) {
    case 'chunk-source':
      return 'chunk-source';
    case 'skill-md':
      return 'skill-md';
    case 'empty':
    case 'verdict':
    default:
      return 'verdict';
  }
}

export interface CenterPanelProps {
  currentVerdict: Verdict | null;
}

export default function CenterPanel({ currentVerdict }: CenterPanelProps) {
  const centerView = useAppStore((s) => s.centerView);

  const items = [
    { value: 'verdict', label: '审查结论' },
    { value: 'chunk-source', label: '引用原文' },
    { value: 'skill-md', label: 'Skill 定义' },
  ];

  function renderBody() {
    if (centerView.kind === 'empty' && !currentVerdict) {
      return (
        <div className="center-panel-tab-content flex flex-col items-center justify-center text-center min-h-[60vh]">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary-dim mb-4">
            <FileSearch className="h-6 w-6 text-primary" />
          </div>
          <div className="text-2xl font-semibold text-text">
            等待合规审查结果
          </div>
          <p className="mt-2 text-base text-text-muted">
            右侧提出合规问题，agent 推理与引用会显示在这里。
          </p>
        </div>
      );
    }
    if (centerView.kind === 'chunk-source') {
      return (
        <div className="center-panel-tab-content">
          <ChunkSourceView
            chunkId={centerView.chunkId}
            tier={centerView.tier as Tier}
            version={centerView.version}
            effectiveDate={centerView.effectiveDate}
          />
        </div>
      );
    }
    if (centerView.kind === 'skill-md') {
      return (
        <div className="center-panel-tab-content">
          <SkillMdPreview skillId={centerView.skillId} />
        </div>
      );
    }
    const verdict = currentVerdict ?? FALLBACK_VERDICT;
    return (
      <div className="center-panel-tab-content">
        <VerdictCard verdict={verdict} />
      </div>
    );
  }

  return (
    <div
      data-testid="center-panel"
      className="overflow-y-auto border-r border-border bg-bg p-6"
    >
      <Tabs value={activeTabValue(centerView.kind)} items={items} />
      {renderBody()}
    </div>
  );
}

// Exposed for testing convenience — Wave 2 keeps the RefusalCard
// reference here so tree-shakers don't drop the import in case a
// future plan adds the refusal-routing path.
export { RefusalCard };
