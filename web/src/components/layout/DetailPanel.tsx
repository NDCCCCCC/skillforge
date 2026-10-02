// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// DetailPanel — the chat-first layout's collapsible right side panel
// (Open Design redesign B2; formerly the three-panel layout's
// CenterPanel). Phase 4 shipped 3 tabs (verdict / chunk-source /
// skill-md); Phase 5 extended to 5 tabs (adds Agent 状态 + 知识图谱)
// and lifted the D-02 "tabs are display-only" restriction — tabs are
// clickable and dispatch setCenterView.
//
// The tab indicator stays bound to `centerView.kind` via the controlled
// `Tabs` `value` prop so programmatic setCenterView calls (SkillPicker
// → skill-md, CitationCluster → chunk-source, etc.) keep working
// alongside user clicks. B2 switches the tab strip to the underline
// variant and adds the dp-head (title + close → setDetailPanelOpen).
import { X } from 'lucide-react';
import { FileSearch } from 'lucide-react';
import { useMemo } from 'react';
import type { UIMessage } from 'ai';
import { extractGraphFocus, hasAnsweredTurn } from '../../lib/graph-focus';
import { Tabs } from '../ui/tabs';
import { useAppStore } from '../../store/app-store';
import type { CenterView } from '../../store/app-store';
import type { Tier, Verdict } from '../../types/verdict';
import AgentInspector from '../inspector/AgentInspector';
import AuditTimelineTab from '../AuditTimelineTab';
import ChunkSourceView from '../center/ChunkSourceView';
import DocViewer from '../center/DocViewer';
import KnowledgeGraphBrowser from '../graph/KnowledgeGraphBrowser';
import PlanReplayView from '../PlanReplayView';
import RefusalCard from '../center/RefusalCard';
import SkillMdPreview from '../center/SkillMdPreview';
import VerdictCard from '../center/VerdictCard';

const FALLBACK_VERDICT: Verdict = {
  status: 'refused',
  confidence: 0,
  citations: [],
  review_required: false,
};

/**
 * Maps the store's `centerView.kind` to the active tab value, so the
 * Tabs indicator follows programmatic `setCenterView` calls.
 */
function activeTabValue(kind: CenterView['kind']): string {
  switch (kind) {
    case 'audit-timeline':
      return 'audit-timeline';
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
    case 'plan-replay':
      return 'plan-replay';
    case 'empty':
    case 'verdict':
    default:
      return 'verdict';
  }
}

/**
 * Maps a tab value (emitted by Tabs.onValueChange on click) to the
 * matching CenterView kind.
 */
function tabValueToCenterView(value: string): CenterView | null {
  switch (value) {
    case 'verdict':
      return { kind: 'empty' };
    case 'chunk-source':
      // chunk-source requires a prior CitationCluster click; a bare tab
      // click without context stays at the empty verdict tab.
      return null;
    case 'skill-md':
      return { kind: 'skill-md', skillId: 'simple-qa' };
    case 'audit-timeline':
      return { kind: 'audit-timeline' };
    case 'agent-inspector':
      return { kind: 'agent-inspector' };
    case 'knowledge-graph':
      return { kind: 'knowledge-graph' };
    case 'plan-replay':
      return { kind: 'plan-replay' };
    default:
      return null;
  }
}

export interface DetailPanelProps {
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

export default function DetailPanel({
  currentVerdict,
  messages = [],
  chatStatus = 'ready',
}: DetailPanelProps) {
  const centerView = useAppStore((s) => s.centerView);
  const setCenterView = useAppStore((s) => s.setCenterView);
  const setDetailPanelOpen = useAppStore((s) => s.setDetailPanelOpen);
  // Plan 12-07 — PlanReplayView needs a runId. Prefer the plan slice's
  // currentRunId (the run that emitted data-plan-trace last); fall back
  // to the active session id (chatId === runId for v1).
  const activeSessionId = useAppStore((s) => s.activeSessionId);

  // Quick 260929-d5w3 Task 1 — question focus for the knowledge graph.
  // Derived READ-ONLY from the streamed messages: the latest assistant
  // verdict's citations become the graph focus; an answered turn WITHOUT
  // citations (refusal) becomes the honest empty-focus state.
  const graphFocus = useMemo(() => extractGraphFocus(messages), [messages]);
  const answeredTurn = useMemo(() => hasAnsweredTurn(messages), [messages]);

  // 7 tabs in document order (Plan 11-07: 审计轨迹 slots between
  // 结论 and 引用原文 per UI-SPEC §Position 3; Plan 12-07: 规划轨迹
  // sits between 审计轨迹 and 引用原文 per PLAN-05 logical flow).
  const items = [
    { value: 'verdict', label: '结论' },
    { value: 'audit-timeline', label: '审计轨迹' },
    { value: 'plan-replay', label: '规划轨迹' },
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
    // Plan 11-07 — HITL-04 audit timeline tab body (read-only).
    if (centerView.kind === 'audit-timeline') {
      return (
        <div className="center-panel-tab-content">
          <AuditTimelineTab />
        </div>
      );
    }
    // Tab-4: Agent inspector (SC-4).
    if (centerView.kind === 'agent-inspector') {
      return (
        <div className="center-panel-tab-content">
          <AgentInspector messages={messages} status={chatStatus} />
        </div>
      );
    }
    // Tab-5: Knowledge graph browser (SC-5). 260929-d5w3: the graph
    // receives the current answer's citation focus (仅当前问题 mode).
    if (centerView.kind === 'knowledge-graph') {
      return (
        <div className="center-panel-tab-content">
          <KnowledgeGraphBrowser focus={graphFocus} hasAnswer={answeredTurn} />
        </div>
      );
    }
    // Plan 12-07 — PLAN-05 规划轨迹回放 tab body. Reads runId from
    // the plan slice (the most-recent data-plan-trace runId — the same
    // identity that owns the live PlanTracePart). If no run has emitted
    // a trace yet, falls back to activeSessionId (chatId === runId for
    // the v1 single-session-per-run model).
    if (centerView.kind === 'plan-replay') {
      const activeRunId = useAppStore.getState().plan.currentRunId ?? activeSessionId;
      return (
        <div className="center-panel-tab-content">
          <PlanReplayView runId={activeRunId} />
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
            等待回答结果
          </div>
          <p className="mt-2 text-base text-text-muted">
            提问后，agent 的结论与引用会显示在这里。
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
    // Doc viewer — full corpus-doc source (knowledge modal / graph
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
      data-testid="detail-panel"
      className="detail-panel flex h-full min-w-0 flex-col"
    >
      <div className="dp-head flex h-11 flex-none items-center justify-between border-b border-border px-4">
        <span className="text-sm font-semibold tracking-wide text-text">详情</span>
        <button
          type="button"
          data-testid="detail-panel-close"
          onClick={() => setDetailPanelOpen(false)}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-2 hover:text-text"
          aria-label="收起详情面板"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="flex-none border-b border-border px-4 pt-2">
        <Tabs
          variant="underline"
          value={activeTabValue(centerView.kind)}
          items={items}
          onValueChange={handleTabChange}
        />
      </div>
      <div className="detail-panel-body min-h-0 flex-1 overflow-y-auto">{renderBody()}</div>
    </div>
  );
}

// Exposed for testing convenience — Wave 2 keeps the RefusalCard
// reference here so tree-shakers don't drop the import in case a
// future plan adds the refusal-routing path.
export { RefusalCard };
