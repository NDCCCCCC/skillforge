// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 1 — RoutingTracePart tests.
//
// Pins the SC-1 acceptance assertions:
//   1. Renders the section header 路由分析 + one row per SkillScore
//      with the `{name} · cos {score}` shape.
//   2. The selected row uses `--color-primary` (text-primary class).
//   3. When `ambiguous=true`, renders SkillPicker in ambiguity mode.
//   4. Defensive empty state `路由信息缺失。` when scores are empty.
//   5. Silently drops non-routed decisions (`retrieve`, `answer`, etc.).
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import type { UIMessage } from 'ai';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import RoutingTracePart from '../components/chat/parts/RoutingTracePart';
import { useAppStore } from '../store/app-store';

afterEach(cleanup);

function part(data: unknown): UIMessage['parts'][number] {
  return { type: 'data-decision', id: 'd1', data } as unknown as UIMessage['parts'][number];
}

describe('RoutingTracePart', () => {
  it('renders 路由分析 + one row per SkillScore with cos score', () => {
    render(
      <RoutingTracePart
        part={part({
          step: 0,
          decision: 'route',
          skillScores: [
            { skillId: 'compliance-review', skillName: '企业合规/制度审查', score: 0.87, selected: true },
            { skillId: 'general-qa', skillName: '通用问答', score: 0.42, selected: false },
          ],
          selected: 'compliance-review',
          ambiguous: false,
        })}
      />,
    );

    expect(screen.getByTestId('routing-trace')).toBeInTheDocument();
    expect(screen.getByTestId('routing-trace')).toHaveTextContent('路由分析');

    const rows = screen.getAllByTestId('routing-trace-row');
    expect(rows).toHaveLength(2);

    const compliance = rows[0];
    expect(compliance).toHaveAttribute('data-skill-id', 'compliance-review');
    expect(compliance).toHaveAttribute('data-selected', 'true');
    expect(compliance).toHaveTextContent('企业合规/制度审查');
    expect(compliance).toHaveTextContent('cos 0.87');
    // Phase 4 modernization: the selected row uses the
    // .routing-trace-modern-row[data-selected="true"] selector for its
    // primary highlight (background pill + brighter text). We assert
    // both the class hook and the data-selected attribute.
    expect(compliance).toHaveClass('routing-trace-modern-row');

    const general = rows[1];
    expect(general).toHaveAttribute('data-selected', 'false');
    expect(general).toHaveTextContent('cos 0.42');
  });

  it('renders SkillPicker ambiguity mode when ambiguous=true', () => {
    const onPick = vi.fn();
    const onDismiss = vi.fn();

    render(
      <RoutingTracePart
        part={part({
          step: 0,
          decision: 'route',
          skillScores: [
            { skillId: 'a', skillName: 'Skill A', score: 0.55, selected: false },
            { skillId: 'b', skillName: 'Skill B', score: 0.51, selected: false },
          ],
          ambiguous: true,
        })}
        onPickAmbiguous={onPick}
        onDismissAmbiguous={onDismiss}
      />,
    );

    expect(screen.getByTestId('routing-trace-ambiguity')).toBeInTheDocument();
    // Open the picker dropdown
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    const menu = screen.getByTestId('skill-picker-menu');
    expect(menu).toBeInTheDocument();
    // The picker renders one item per candidate with cos score
    const items = within(menu).getAllByTestId('skill-picker-item');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent('cos 0.55');
    expect(items[1]).toHaveTextContent('cos 0.51');
    // Per-candidate CTA present
    const ctas = within(menu).getAllByTestId('skill-picker-cta');
    expect(ctas).toHaveLength(2);
    // Dismiss CTA present
    expect(within(menu).getByTestId('skill-picker-dismiss')).toHaveTextContent('使用默认 Skill');
  });

  it('renders the defensive empty state when skillScores is empty', () => {
    render(
      <RoutingTracePart
        part={part({
          step: 0,
          decision: 'route',
          skillScores: [],
          ambiguous: false,
        })}
      />,
    );
    expect(screen.getByTestId('routing-trace-empty')).toBeInTheDocument();
    expect(screen.getByTestId('routing-trace-empty')).toHaveTextContent('路由信息缺失。');
  });

  it('silently drops non-routed decisions (retrieve / answer / refuse)', () => {
    const { container } = render(
      <RoutingTracePart
        part={part({
          step: 1,
          decision: 'retrieve',
          toolCallId: 'tc1',
        })}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('routes counter_evidence decision through the same renderer', () => {
    render(
      <RoutingTracePart
        part={part({
          step: 2,
          decision: 'counter_evidence',
          skillScores: [
            { skillId: 'a', skillName: 'Skill A', score: 0.91, selected: true },
          ],
        })}
      />,
    );
    expect(screen.getByTestId('routing-trace')).toHaveAttribute('data-decision', 'counter_evidence');
    expect(screen.getAllByTestId('routing-trace-row')).toHaveLength(1);
  });

  it('ambiguity picker per-candidate CTA invokes onPickAmbiguous with the chosen skillId', () => {
    const onPick = vi.fn();
    render(
      <RoutingTracePart
        part={part({
          step: 0,
          decision: 'route',
          skillScores: [
            { skillId: 'a', skillName: 'Skill A', score: 0.55, selected: false },
            { skillId: 'b', skillName: 'Skill B', score: 0.51, selected: false },
          ],
          ambiguous: true,
        })}
        onPickAmbiguous={onPick}
      />,
    );
    fireEvent.click(screen.getByTestId('skill-picker-trigger'));
    const ctas = screen.getAllByTestId('skill-picker-cta');
    fireEvent.click(ctas[0]);
    expect(onPick).toHaveBeenCalledWith('a');
  });

  it('mirror updates store.lastRouting on every route decision', () => {
    // Side-effect: the MessageBubble dispatcher mirrors DecisionPayload
    // into the Zustand store. The store helper is verified here via a
    // direct dispatch (the bubble-side mirror is exercised in the
    // MessageBubble.test.tsx suite after this PR).
    useAppStore.setState({ lastRouting: undefined });
    useAppStore.getState().setLastRouting({
      skillScores: [
        { skillId: 'a', skillName: 'A', score: 0.7, selected: true },
      ],
      ambiguous: false,
      selected: 'a',
      ts: Date.now(),
    });
    const routing = useAppStore.getState().lastRouting;
    expect(routing).toBeDefined();
    expect(routing?.selected).toBe('a');
    expect(routing?.skillScores).toHaveLength(1);
  });
});
