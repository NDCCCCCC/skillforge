// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// CenterPanel — the center-panel tab strip + body. Phase 4 shipped 3
// tabs (verdict / chunk-source / skill-md); Phase 5 extends to 5 tabs
// (adds Agent 状态 + 知识图谱) and LIFTS the D-02 "tabs are display-only"
// restriction — Phase 5 tabs are clickable and dispatch setCenterView
// (Phase-4-deferred bidirectional tab nav, resolved per UI-SPEC §Layout).
//
// The tab indicator stays bound to `centerView.kind` via the controlled
// `Tabs` `value` prop so programmatic setCenterView calls (SkillPicker
// → skill-md, CitationCluster → chunk-source, etc.) keep working
// alongside user clicks.
//
// Empty-state hero per Copywriting Contract: `等待合规审查结果` +
// `右侧提出合规问题，agent 推理与引用会显示在这里。`
import { FileSearch } from 'lucide-react';
import type { UIMessage } from 'ai';
import { Tabs } from '../ui/tabs';
import { useAppStore } from '../../store/app-store';
import type { CenterView } from '../../store/app-store';
import type { Tier, Verdict } from '../../types/verdict';
import AgentInspector from '../inspector/AgentInspector';
import ChunkSourceView from './ChunkSourceView';
import DocViewer from './DocViewer';
import KnowledgeGraphBrowser from '../graph/KnowledgeGraphBrowser';
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
 * Tabs indicator follows programmatic `setCenterView` calls. Phase 5
 * adds the two new tab values (Agent 状态, 知识图谱).
 */
function activeTabValue(kind: CenterView['kind']): string {
  switch (kind) {
    case 'chunk-source':
    // doc-view shares the 引用原文 tab (both are source-text views).
    case 'doc-view':
      return 'chunk-source';
    case 'skill-md':
      return 'skill-md';
    case 'agent-inspector':
      return 'agent-inspector';
    case 'knowledge-graph':
      return 'knowledge-graph';
    case 'empty':
    case 'verdict':
    default:
      return 'verdict';
  }
}

/**
 * Maps a tab value (emitted by Tabs.onValueChange on click) to the
 * matching CenterView kind. Phase 5 wires the 4th + 5th tabs through
 * to the agent-inspector + knowledge-graph kinds (their full content
 * ships in 05-05; the empty-state placeholders are mounted now).
 */
function tabValueToCenterView(value: string): CenterView | null {
  switch (value) {
    case 'verdict':
      return { kind: 'empty' };
    case 'chunk-source':
      // Phase 4 had no entry-point into chunk-source from tab click
      // because chunk-source requires a prior CitationCluster click.
      // Keep that behavior: a bare tab click without context stays
      // at the empty verdict tab. The inherited 04-UI-SPEC empty-state
      // copy is `点击聊天中的引用以查看权威条款原文。`.
      return null;
    case 'skill-md':
      return { kind: 'skill-md', skillId: 'compliance-review' };
    case 'agent-inspector':
      return { kind: 'agent-inspector' };
    case 'knowledge-graph':
      return { kind: 'knowledge-graph' };
    default:
      return null;
  }
}

export interface CenterPanelProps {
  currentVerdict: Verdict | null;
  /**
   * Plan 05-05 / SC-4 (AGNT-06) — useChat.messages streamed down from
   * <App>. NOT Zustand state (D14). Only the inspector consumes this;
   * the verdict + chunk-source + skill-md panels read from props +
   * store as before.
   */
  messages?: UIMessage[];
  /** useChat.status — used by the inspector to detect active hop. */
  chatStatus?: 'submitted' | 'streaming' | 'ready' | 'error';
}

export default function CenterPanel({
  currentVerdict,
  messages = [],
  chatStatus = 'ready',
}: CenterPanelProps) {
  const centerView = useAppStore((s) => s.centerView);
  const setCenterView = useAppStore((s) => s.setCenterView);

  // Phase 5 — 5 tabs in document order. The horizontal-scroll fallback
  // for narrow widths is inherited from the .tabs CSS (overflow-x: auto
  // on the parent tablist). No "more" dropdown (UI-SPEC §Layout).
  const items = [
    { value: 'verdict', label: '审查结论' },
    { value: 'chunk-source', label: '引用原文' },
    { value: 'skill-md', label: 'Skill 定义' },
    { value: 'agent-inspector', label: 'Agent 状态' },
    { value: 'knowledge-graph', label: '知识图谱' },
  ];

  function handleTabChange(value: string) {
    const next = tabValueToCenterView(value);
    if (next) setCenterView(next);
  }

  function renderBody() {
    // Tab-4: Agent inspector (SC-4).
    if (centerView.kind === 'agent-inspector') {
      return (
        <div className="center-panel-tab-content">
          <AgentInspector messages={messages} status={chatStatus} />
        </div>
      );
    }
    // Tab-5: Knowledge graph browser (SC-5).
    if (centerView.kind === 'knowledge-graph') {
      return (
        <div className="center-panel-tab-content">
          <KnowledgeGraphBrowser />
        </div>
      );
    }
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
            effectiveTo={centerView.effectiveTo}
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
    // Doc viewer — full corpus-doc source (left FILES click / graph
    // doc-node click entry points).
    if (centerView.kind === 'doc-view') {
      return (
        <div className="center-panel-tab-content">
          <DocViewer tier={centerView.tier} file={centerView.file} />
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
      <Tabs value={activeTabValue(centerView.kind)} items={items} onValueChange={handleTabChange} />
      {renderBody()}
    </div>
  );
}

// Exposed for testing convenience — Wave 2 keeps the RefusalCard
// reference here so tree-shakers don't drop the import in case a
// future plan adds the refusal-routing path.
export { RefusalCard };
