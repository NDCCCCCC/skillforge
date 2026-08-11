// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// CenterPanel — D-02 three-tab center panel. Default tab is `verdict`
// (the protagonist). Tab body switches on `centerView.kind` from the
// Zustand app store. The component is read-only here — Wave 3's
// CitationCluster click changes the store; Wave 2 doesn't wire the
// click-through yet but CenterPanel already reacts to store changes.
//
// Empty-state hero per Copywriting Contract: `等待合规审查结果` +
// `右侧提出合规问题，agent 推理与引用会显示在这里。`
import { FileSearch } from 'lucide-react';
import { Tabs } from '../ui/tabs';
import { useAppStore } from '../../store/app-store';
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
      <Tabs defaultValue="verdict" items={items} />
      {renderBody()}
    </div>
  );
}

// Exposed for testing convenience — Wave 2 keeps the RefusalCard
// reference here so tree-shakers don't drop the import in case a
// future plan adds the refusal-routing path.
export { RefusalCard };
