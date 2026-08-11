// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 04-02 Task 3 — CenterPanel tests.
//
// Pins D-02 invariants:
//   - default tab is `verdict`
//   - tabs component renders 3 tab buttons with labels
//     `审查结论`, `引用原文`, `Skill 定义`
//   - body switches on centerView.kind:
//     - `empty` + no currentVerdict → empty-state hero `等待合规审查结果`
//     - `verdict` + currentVerdict → <VerdictCard>
//     - `chunk-source` → <ChunkSourceView> with chunk-verbatim + TierBadge
//     - `skill-md` → <SkillMdPreview>
//   - tab clicks do NOT change centerView (read-only; Wave 3 wires
//     CitationCluster click → setCenterView)
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import CenterPanel from '../components/center/CenterPanel';
import { useAppStore } from '../store/app-store';
import type { Verdict } from '../types/verdict';

afterEach(cleanup);

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
    expect(screen.getByTestId('center-panel')).toHaveTextContent('等待合规审查结果');
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
    useAppStore.setState({ centerView: { kind: 'skill-md', skillId: 'compliance-review' } });
    render(<CenterPanel currentVerdict={null} />);
    expect(screen.getByTestId('skill-md-preview')).toBeInTheDocument();
    // Wave 3: SkillMdPreview renders all 7 fields from skills.json.
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('企业合规/制度审查');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('agent:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('reads:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('writes:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('temperature:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('steps:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('prohibitions:');
    expect(screen.getByTestId('skill-md-preview')).toHaveTextContent('required_refusal_phrases:');
  });

  it('renders three tab buttons with the correct Chinese labels', () => {
    render(<CenterPanel currentVerdict={null} />);
    const tablist = screen.getByRole('tablist');
    expect(tablist).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '审查结论' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '引用原文' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Skill 定义' })).toBeInTheDocument();
  });

  it('default tab is `verdict` (D-02) — clicking the second tab does NOT change centerView', () => {
    useAppStore.setState({ centerView: { kind: 'verdict', verdictId: 'v-001' } });
    render(<CenterPanel currentVerdict={compliantVerdict} />);

    const verdictTab = screen.getByRole('tab', { name: '审查结论' });
    expect(verdictTab).toHaveAttribute('aria-selected', 'true');

    // Clicking another tab is local UI; the store should remain at verdict.
    fireEvent.click(screen.getByRole('tab', { name: '引用原文' }));
    expect(useAppStore.getState().centerView.kind).toBe('verdict');
  });
});
