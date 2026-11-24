// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 57-02 (ONT-02) -- the anchor line pins the three DISTINCT states
// (absent / degraded / no-entity / anchored) and the badge accessibility.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import KGAnchorLine from '../components/inspector/KGAnchorLine';

afterEach(cleanup);

describe('KGAnchorLine (57-02)', () => {
  it('renders nothing before the ontology_anchor event arrives', () => {
    const { container } = render(<KGAnchorLine />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders the degraded line + accessible badge when the KG is unavailable', () => {
    render(<KGAnchorLine kgAvailable={false} />);
    const line = screen.getByTestId('kg-anchor-line');
    expect(line).toHaveAttribute('data-kg-available', 'false');
    expect(line).toHaveTextContent('本体未接入（本次仅向量检索）');
    const badge = screen.getByTestId('kg-degraded-badge');
    expect(badge).toHaveTextContent('知识图谱不可用');
    expect(badge).toHaveAttribute('role', 'status');
    expect(badge).toHaveAttribute('aria-label', '知识图谱不可用');
  });

  it('distinguishes "worked but matched nothing" from "no KG"', () => {
    render(<KGAnchorLine kgAvailable entitiesMatched={0} />);
    const line = screen.getByTestId('kg-anchor-line');
    expect(line).toHaveAttribute('data-kg-available', 'true');
    expect(line).toHaveTextContent('本体锚定 · 未识别到实体');
    expect(screen.queryByTestId('kg-degraded-badge')).toBeNull();
  });

  it('renders the anchored labels verbatim', () => {
    render(<KGAnchorLine kgAvailable entitiesMatched={2} anchorEntities="差旅住宿费标准 / 年假制度" />);
    const line = screen.getByTestId('kg-anchor-line');
    expect(line).toHaveAttribute('data-entities-matched', '2');
    expect(line).toHaveTextContent('本体锚定 · 差旅住宿费标准 / 年假制度');
  });

  it('politely announces the line for screen readers', () => {
    render(<KGAnchorLine kgAvailable entitiesMatched={1} anchorEntities="差旅住宿费标准" />);
    expect(screen.getByTestId('kg-anchor-line')).toHaveAttribute('aria-live', 'polite');
  });
});
