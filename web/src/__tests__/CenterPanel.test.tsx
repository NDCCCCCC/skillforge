// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 3 — CenterPanel tests.
//
// Pins D-02 invariants:
//   - default tab is `verdict`
//   - tabs component renders 3 tab buttons with labels
//     `结论`, `引用原文`, `Skill 定义`
//   - body switches on centerView.kind:
//     - `empty` + no currentVerdict → empty-state hero `等待回答结果`
//     - `verdict` + currentVerdict → <VerdictCard>
//     - `chunk-source` → <ChunkSourceView> with chunk-verbatim + TierBadge
//     - `skill-md` → <SkillMdPreview>
//   - tab clicks do NOT change centerView (read-only; Wave 3 wires
//     CitationCluster click → setCenterView)
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import CenterPanel from '../components/center/CenterPanel';
import { useAppStore } from '../store/app-store';
import type { Verdict } from '../types/verdict';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

const compliantVerdict: Verdict = {
  status: '合规',
  confidence: 0.93,
  citations: [{ chunk_id: 'c-001', span: '条款', tier: '权威' }],
  review_required: false,
};

beforeEach(() => {
  useAppStore.setState({ centerView: { kind: 'empty' } });
});

describe('CenterPanel', () => {
  it('renders empty-state hero when centerView.kind === "empty" and no currentVerdict', () => {
    render(<CenterPanel currentVerdict={null} />);
    expect(screen.getByTestId('center-panel')).toHaveTextContent('等待回答结果');
  });

  it('renders VerdictCard when centerView.kind === "verdict" and a currentVerdict is provided', () => {
    useAppStore.setState({ centerView: { kind: 'verdict', verdictId: 'v-001' } });
    render(<CenterPanel currentVerdict={compliantVerdict} />);
    expect(screen.getByTestId('verdict-card')).toBeInTheDocument();
    expect(screen.getByTestId('verdict-card')).toHaveAttribute('data-verdict-status', '合规');
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
    render(<CenterPanel currentVerdict={null} />);
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
    render(<CenterPanel currentVerdict={null} />);
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
    render(<CenterPanel currentVerdict={null} />);
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
    render(<CenterPanel currentVerdict={compliantVerdict} />);

    const verdictTab = screen.getByRole('tab', { name: '结论' });
    expect(verdictTab).toHaveAttribute('aria-selected', 'true');

    // Phase 5 — tabs are clickable; clicking Agent 状态 dispatches the
    // agent-inspector CenterView kind (D-02 restriction lifted).
    fireEvent.click(screen.getByRole('tab', { name: 'Agent 状态' }));
    expect(useAppStore.getState().centerView.kind).toBe('agent-inspector');
  });

  it('clicking 知识图谱 tab dispatches setCenterView({kind:"knowledge-graph"})', () => {
    useAppStore.setState({ centerView: { kind: 'empty' } });
    render(<CenterPanel currentVerdict={null} />);
    fireEvent.click(screen.getByRole('tab', { name: '知识图谱' }));
    expect(useAppStore.getState().centerView.kind).toBe('knowledge-graph');
  });

  it('tab indicator follows programmatic setCenterView (04-UAT test 8)', () => {
    // SkillPicker path: selecting a skill sets kind='skill-md' — the
    // Skill 定义 tab must be the active indicator (not stuck on 结论).
    useAppStore.setState({ centerView: { kind: 'skill-md', skillId: 'simple-qa' } });
    render(<CenterPanel currentVerdict={null} />);
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
    render(<CenterPanel currentVerdict={null} />);
    expect(screen.getByRole('tab', { name: '引用原文' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Skill 定义' })).toHaveAttribute('aria-selected', 'false');
  });

  it('agent-inspector kind mounts the empty-state placeholder (SC-4)', () => {
    useAppStore.setState({ centerView: { kind: 'agent-inspector' } });
    render(<CenterPanel currentVerdict={null} />);
    expect(screen.getByTestId('agent-inspector-empty')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Agent 状态' })).toHaveAttribute('aria-selected', 'true');
  });

  it('knowledge-graph kind mounts the empty-state hero when BE returns an empty corpus (SC-5)', async () => {
    // Plan 05-05 replaced the placeholder with a real fetch-driven
    // implementation. The browser reaches 'idle' on mount, fires a
    // fetch, and renders 知识库为空 when the BE returns a corpus with
    // 0 docs across all 4 tiers.
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
    render(<CenterPanel currentVerdict={null} />);
    expect(screen.getByRole('tab', { name: '知识图谱' })).toHaveAttribute('aria-selected', 'true');

    await waitFor(() => {
      expect(screen.getByTestId('knowledge-graph-empty')).toBeInTheDocument();
    });
  });
});
