// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 3 — DetailPanel tests (Open Design redesign B2:
// formerly CenterPanel). Pins D-02 invariants:
//   - default tab is `verdict`
//   - tabs component renders 5 tab buttons with labels
//     `结论`, `引用原文`, `Skill 定义`, `Agent 状态`, `知识图谱`
//   - body switches on centerView.kind:
//     - `empty` + no currentVerdict → empty-state hero `等待回答结果`
//     - `verdict` + currentVerdict → <VerdictCard>
//     - `chunk-source` → <ChunkSourceView> with chunk-verbatim + TierBadge
//     - `skill-md` → <SkillMdPreview>
//   - tab clicks dispatch setCenterView (Phase-5 bidirectional nav)
//   - B2: the dp-head close button collapses the panel via
//     setDetailPanelOpen(false)
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import DetailPanel from '../components/layout/DetailPanel';
import { useAppStore } from '../store/app-store';
import type { Verdict } from '../types/verdict';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

const compliantVerdict: Verdict = {
  status: 'ok',
  confidence: 0.93,
  citations: [{ chunk_id: 'c-001', span: '条款', tier: '权威' }],
  review_required: false,
  compliance_status: 'compliant',
};

beforeEach(() => {
  useAppStore.setState({ centerView: { kind: 'empty' } });
});

describe('DetailPanel', () => {
  it('renders empty-state hero when centerView.kind === "empty" and no currentVerdict', () => {
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByTestId('detail-panel')).toHaveTextContent('等待回答结果');
  });

  it('renders VerdictCard when centerView.kind === "verdict" and a currentVerdict is provided', () => {
    useAppStore.setState({ centerView: { kind: 'verdict', verdictId: 'v-001' } });
    render(<DetailPanel currentVerdict={compliantVerdict} />);
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    expect(screen.getByTestId('verdict-card')).toHaveAttribute('data-verdict-status', 'ok');
    expect(screen.getByTestId('verdict-card')).toHaveAttribute(
      'data-compliance-status',
      'compliant',
    );
  });

  it('renders ChunkSourceView with real chunk verbatim when centerView.kind === "chunk-source"', () => {
    useAppStore.setState({
      centerView: {
        kind: 'chunk-source',
        chunkId: 'c-001',
        tier: '权威',
        version: 'v3.2',
        effectiveDate: '2026-01-15',
        effectiveTo: null,
      },
    });
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByTestId('chunk-verbatim')).toBeInTheDocument();
    // Wave 3: ChunkSourceView pulls the verbatim text from the
    // chunks.json fixture via the chunk-lookup helper.
    expect(screen.getByTestId('chunk-verbatim')).toHaveTextContent('员工出差住宿费超过标准额度');
    expect(screen.getByTestId('tier-badge')).toHaveAttribute('data-tier', '权威');
    expect(screen.getByText('版本: v3.2')).toBeInTheDocument();
    expect(screen.getByText('生效日期: 2026-01-15')).toBeInTheDocument();
  });

  it('renders SkillMdPreview with 7-field SKILL.md content when centerView.kind === "skill-md"', () => {
    useAppStore.setState({ centerView: { kind: 'skill-md', skillId: 'simple-qa' } });
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByTestId('skill-md-preview')).toBeInTheDocument();
    // Wave 3: SkillMdPreview renders all 7 fields from skills.json.
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('通用问答');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('agent:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('reads:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('writes:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('temperature:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('steps:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('prohibitions:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('required_refusal_phrases:');
  });

  it('renders five tab buttons with the correct Chinese labels', () => {
    render(<DetailPanel currentVerdict={null} />);
    const tablist = screen.getByRole('tablist');
    expect(tablist).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '结论' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '引用原文' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Skill 定义' })).toBeInTheDocument();
    // Phase 5 — SC-4 inspector + SC-5 graph browser
    expect(screen.getByRole('tab', { name: 'Agent 状态' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '知识图谱' })).toBeInTheDocument();
  });

  it('default tab is `verdict` — clicking the Agent 状态 tab dispatches setCenterView (Phase-5 bidirectional nav)', () => {
    useAppStore.setState({ centerView: { kind: 'verdict', verdictId: 'v-001' } });
    render(<DetailPanel currentVerdict={compliantVerdict} />);

    const verdictTab = screen.getByRole('tab', { name: '结论' });
    expect(verdictTab).toHaveAttribute('aria-selected', 'true');

    // Phase 5 — tabs are clickable; clicking Agent 状态 dispatches the
    // agent-inspector CenterView kind (D-02 restriction lifted).
    fireEvent.click(screen.getByRole('tab', { name: 'Agent 状态' }));
    expect(useAppStore.getState().centerView.kind).toBe('agent-inspector');
  });

  it('clicking 知识图谱 tab dispatches setCenterView({kind:"knowledge-graph"})', () => {
    useAppStore.setState({ centerView: { kind: 'empty' } });
    render(<DetailPanel currentVerdict={null} />);
    fireEvent.click(screen.getByRole('tab', { name: '知识图谱' }));
    expect(useAppStore.getState().centerView.kind).toBe('knowledge-graph');
  });

  it('tab indicator follows programmatic setCenterView (04-UAT test 8)', () => {
    // SkillPicker path: selecting a skill sets kind='skill-md' — the
    // Skill 定义 tab must be the active indicator (not stuck on 结论).
    useAppStore.setState({ centerView: { kind: 'skill-md', skillId: 'simple-qa' } });
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByRole('tab', { name: 'Skill 定义' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: '结论' })).toHaveAttribute('aria-selected', 'false');

    // Citation-click path: clicking a citation sets kind='chunk-source'
    // — the 引用原文 tab must be the active indicator.
    cleanup();
    useAppStore.setState({
      centerView: {
        kind: 'chunk-source',
        chunkId: 'c-001',
        tier: '权威',
        version: 'v3.2',
        effectiveDate: '2026-01-15',
        effectiveTo: null,
      },
    });
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByRole('tab', { name: '引用原文' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Skill 定义' })).toHaveAttribute('aria-selected', 'false');
  });

  it('agent-inspector kind mounts the empty-state placeholder (SC-4)', () => {
    useAppStore.setState({ centerView: { kind: 'agent-inspector' } });
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByTestId('agent-inspector-empty')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Agent 状态' })).toHaveAttribute('aria-selected', 'true');
  });

  it('B2 — the dp-head close button dispatches setDetailPanelOpen(false)', () => {
    useAppStore.setState({ centerView: { kind: 'empty' }, detailPanelOpen: true });
    render(<DetailPanel currentVerdict={null} />);
    fireEvent.click(screen.getByTestId('detail-panel-close'));
    expect(useAppStore.getState().detailPanelOpen).toBe(false);
  });

  it('knowledge-graph kind mounts the empty-state hero when BE returns an empty corpus (SC-5)', async () => {
    // Plan 05-05 replaced the placeholder with a real fetch-driven
    // implementation. The browser reaches 'idle' on mount, fires a
    // fetch, and renders 知识库为空 when the BE returns a corpus with
    // 0 docs across all 4 tiers.
    //
    // Root cause of D-46-DEF-01: since quick-260929-g7x the browser
    // defaults to 本体/ontology mode (`useState<GraphViewMode>('ontology')`
    // in KnowledgeGraphBrowser.tsx), and the document-corpus empty-state
    // hero is rendered ONLY under `mode === 'document'`. The old
    // assertion relied on an implicit default that has since changed, so
    // it observed `ontology-graph-empty` / 本体暂无数据 instead. The SC-5
    // contract is still implemented — it now lives in 文档 mode, so the
    // test must select that mode (same shape as KnowledgeGraphBrowser's
    // own `renders the empty-state hero in 文档 mode` case) before asserting.
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async () => ({
        ok: true,
        status: 200,
        statusText: 'OK',
        json: async () => ({
          tiers: [
            { tier: '权威', docCount: 0, chunkCount: 0, docs: [] },
            { tier: '历史', docCount: 0, chunkCount: 0, docs: [] },
            { tier: '个人', docCount: 0, chunkCount: 0, docs: [] },
            { tier: '草稿', docCount: 0, chunkCount: 0, docs: [] },
          ],
        }),
      } as unknown as Response)),
    );

    useAppStore.setState({ centerView: { kind: 'knowledge-graph' } });
    render(<DetailPanel currentVerdict={null} />);
    expect(screen.getByRole('tab', { name: '知识图谱' })).toHaveAttribute('aria-selected', 'true');

    // Select 文档 mode — the empty-state hero is a document-mode concern
    // (KnowledgeGraphBrowser.tsx: `mode === 'document' && (!graph || totalDocs === 0)`).
    fireEvent.click(screen.getByTestId('graph-mode-document'));

    await waitFor(() => {
      expect(screen.getByTestId('knowledge-graph-empty')).toBeInTheDocument();
    });
    expect(screen.getByTestId('knowledge-graph-empty')).toHaveTextContent('知识库为空');
  });

  // Plan 12-07 Task 3 — 规划轨迹 tab between 审计轨迹 + 引用原文.
  it('renders seven tab buttons including 规划轨迹 (Plan 12-07)', () => {
    render(<DetailPanel currentVerdict={null} />);
    // The new 规划轨迹 tab exists alongside the prior 6.
    expect(screen.getByRole('tab', { name: '规划轨迹' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '审计轨迹' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '引用原文' })).toBeInTheDocument();
  });

  it('clicking 规划轨迹 tab dispatches setCenterView({kind:"plan-replay"}) and renders PlanReplayView', async () => {
    // Stub the BE /api/run/:id/plan endpoint with an empty payload so
    // PlanReplayView renders the empty state.
    useAppStore.setState({
      centerView: { kind: 'empty' },
      activeSessionId: 'run-tab-test',
    });
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/api/run/run-tab-test/plan')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ runId: 'run-tab-test', todos: [], generatedAt: 0, count: 0 }),
        } as Response;
      }
      return { ok: false, status: 404 } as Response;
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<DetailPanel currentVerdict={null} />);
    fireEvent.click(screen.getByRole('tab', { name: '规划轨迹' }));
    expect(useAppStore.getState().centerView.kind).toBe('plan-replay');

    // PlanReplayView's empty state surfaces (the BE returned 0 todos).
    await waitFor(() => {
      expect(screen.getByTestId('plan-replay-empty')).toBeInTheDocument();
    });
  });
});
