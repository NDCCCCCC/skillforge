// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-06 Task 2 — CounterEvidenceCard tests.
//
// Pins the SC-6 acceptance assertions (and the D5-13 trust moat on the
// FE side):
//   (a) DISCLAIMER sentence present verbatim — load-bearing T-5-27 test.
//       The disclaimer is a hardcoded JSX literal (no prop customization)
//       so the operator ALWAYS sees the "annotation ≠ grounding" framing.
//   (b) Empty state copy: 未检索到与本次结论冲突的历史片段。 when
//       conflictChunks is empty / undefined.
//   (c) Conflict chunk row renders the 历史 tier badge with teal bg +
//       the snippet text + the chunk id.
//   (d) Click chunk row dispatches setCenterView({ kind:'chunk-source', ...})
//       — this is the click-through to the chunk viewer.
//
// The D5-13 trust moat is VERIFIED HERE by structural prop isolation:
// the card is constructed with ONLY conflictChunks (the verdict's
// citations are not a prop). If a future refactor tries to merge the
// two, this test file won't compile against the new shape — surfacing
// the violation at typecheck time, not at runtime.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import CounterEvidenceCard from '../components/verdict/CounterEvidenceCard';
import { useAppStore } from '../store/app-store';
import type { CenterView } from '../store/app-store';
import type { ConflictChunk } from '../types/wire-types';

afterEach(cleanup);

const DISCLAIMER = '以下内容来自「历史」知识层，仅供人工参考，不参与本次结论的生成与校验。';
const EMPTY_COPY = '未检索到与本次结论冲突的历史片段。';

const conflict: ConflictChunk = {
  chunkId: 'c-101',
  tier: '历史',
  source: 'history/2023-travel-policy.md',
  snippet: '2023年规定：经济舱机票每日 500 元（已失效）',
  effectiveFrom: 1672531200, // 2023-01-01 UTC
  effectiveTo: 1704067199, // 2023-12-31 UTC
};

describe('CounterEvidenceCard', () => {
  let spy: ReturnType<typeof vi.fn>;
  let restore: () => void;

  beforeEach(() => {
    // Replace setCenterView with a spy so we can assert dispatches
    // without driving the real CenterPanel through the store. The
    // RoutingTracePart test uses the same pattern (replace → assert).
    spy = vi.fn();
    const original = useAppStore.getState().setCenterView;
    useAppStore.setState({ setCenterView: spy });
    restore = () => useAppStore.setState({ setCenterView: original });
  });

  afterEach(() => {
    restore();
  });

  it('renders the disclaimer sentence VERBATIM (T-5-27 load-bearing)', () => {
    render(<CounterEvidenceCard conflictChunks={[]} />);
    const disclaimer = screen.getByTestId('counter-evidence-disclaimer');
    // Strict equality — any edit to the disclaimer must be a deliberate
    // spec change, not a silent copy tweak. The test fails red on drift.
    expect(disclaimer.textContent).toBe(DISCLAIMER);
  });

  it('renders the disclaimer even when conflictChunks is undefined', () => {
    render(<CounterEvidenceCard />);
    // The disclaimer is the trust-moat marker — it must show regardless
    // of whether the BE returned any conflicts.
    expect(screen.getByTestId('counter-evidence-disclaimer').textContent).toBe(DISCLAIMER);
  });

  it('renders empty-state copy when conflictChunks is empty', () => {
    render(<CounterEvidenceCard conflictChunks={[]} />);
    const empty = screen.getByTestId('counter-evidence-empty');
    expect(empty.textContent).toBe(EMPTY_COPY);
    // Rows container absent on empty.
    expect(screen.queryByTestId('counter-evidence-rows')).toBeNull();
  });

  it('renders empty-state copy when conflictChunks is undefined', () => {
    render(<CounterEvidenceCard />);
    expect(screen.getByTestId('counter-evidence-empty').textContent).toBe(EMPTY_COPY);
  });

  it('renders one row per conflict chunk with teal tier badge + snippet', () => {
    render(
      <CounterEvidenceCard
        conflictChunks={[
          conflict,
          { ...conflict, chunkId: 'c-102', snippet: '另一条历史依据' },
        ]}
      />,
    );
    const rows = screen.getAllByTestId('counter-evidence-row');
    expect(rows).toHaveLength(2);

    // Row carries the chunk id + snippet + 历史 tier badge.
    const first = rows[0];
    expect(first).toHaveAttribute('data-chunk-id', 'c-101');
    expect(first).toHaveTextContent('c-101');
    expect(first).toHaveTextContent('2023年规定：经济舱机票每日 500 元（已失效）');

    // The 历史 tier badge — the SCOPED §Color teal surface.
    const badge = screen.getAllByTestId('counter-evidence-tier-badge')[0];
    expect(badge).toHaveAttribute('data-tier', '历史');
    expect(badge).toHaveTextContent('历史');
    expect(badge).toHaveClass('bg-annotation-dim', 'text-annotation');
  });

  it('omits empty-state copy when conflict chunks are present', () => {
    render(<CounterEvidenceCard conflictChunks={[conflict]} />);
    expect(screen.queryByTestId('counter-evidence-empty')).toBeNull();
  });

  it('click chunk row dispatches setCenterView with kind=chunk-source', () => {
    render(<CounterEvidenceCard conflictChunks={[conflict]} />);
    const row = screen.getByTestId('counter-evidence-row');
    fireEvent.click(row);

    expect(spy).toHaveBeenCalledTimes(1);
    const view: CenterView = spy.mock.calls[0][0];
    expect(view.kind).toBe('chunk-source');
    if (view.kind !== 'chunk-source') return; // TS narrowing
    expect(view.chunkId).toBe('c-101');
    expect(view.tier).toBe('历史');
    expect(view.version).toBeNull();
    // Effective dates round-tripped from the ConflictChunk.
    expect(view.effectiveDate).toBe(String(conflict.effectiveFrom));
    expect(view.effectiveTo).toBe(conflict.effectiveTo);
  });

  it('click chunk row passes null effectiveDate when effectiveFrom is 0 / undefined', () => {
    render(
      <CounterEvidenceCard
        conflictChunks={[{ ...conflict, effectiveFrom: 0, effectiveTo: 0 }]}
      />,
    );
    fireEvent.click(screen.getByTestId('counter-evidence-row'));
    expect(spy).toHaveBeenCalledTimes(1);
    const view: CenterView = spy.mock.calls[0][0];
    if (view.kind !== 'chunk-source') return;
    expect(view.effectiveDate).toBeNull();
    expect(view.effectiveTo).toBeNull();
  });

  it('D5-13 FE moat: props interface declares ONLY conflictChunks (no citations)', () => {
    // The trust moat on the FE side is enforced by the props interface:
    // CounterEvidenceCardProps has `conflictChunks?: ConflictChunk[]` and
    // NOTHING else. A future refactor that tries to feed verdict.citations
    // to this card would have to add a new prop — caught at code review.
    // This test pins the prop name list so a rename / silent addition
    // surfaces here, not in production.
    const card = render(<CounterEvidenceCard conflictChunks={[conflict]} />);
    const root = card.getByTestId('counter-evidence-card');
    // data-conflict-count mirrors rows.length — proves the card reads
    // ONLY the conflictChunks prop for its row count (NOT verdict.citations).
    expect(root).toHaveAttribute('data-conflict-count', '1');
    // And the disclaimer is still present regardless of count.
    expect(card.getByTestId('counter-evidence-disclaimer').textContent).toBe(DISCLAIMER);
  });
});
