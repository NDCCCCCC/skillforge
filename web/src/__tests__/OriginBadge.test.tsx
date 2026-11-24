// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 58-02 (PROV-01) -- the origin pill pins the single wire-default
// decision point and the unknown-render-raw rule.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import OriginBadge, { resolveCitationOrigin } from '../components/chat/parts/OriginBadge';

afterEach(cleanup);

describe('resolveCitationOrigin (D-58-02 single decision point)', () => {
  it('maps an absent origin to vector (wire semantics, not an FE guess)', () => {
    expect(resolveCitationOrigin({ chunk_id: '1', span: 's', tier: 'authority' })).toBe('vector');
  });

  it('passes every explicit origin through verbatim', () => {
    expect(resolveCitationOrigin({ chunk_id: '1', span: 's', tier: 'authority', origin: 'vector' })).toBe('vector');
    expect(resolveCitationOrigin({ chunk_id: '1', span: 's', tier: 'authority', origin: 'graph' })).toBe('graph');
    expect(resolveCitationOrigin({ chunk_id: '1', span: 's', tier: 'authority', origin: 'hybrid' })).toBe('hybrid');
  });
});

describe('OriginBadge', () => {
  it('renders the vector pill with its glyph and locked label', () => {
    render(<OriginBadge origin="vector" />);
    const badge = screen.getByTestId('origin-badge');
    expect(badge).toHaveAttribute('data-origin', 'vector');
    expect(badge).toHaveTextContent('语义');
  });

  it('renders the graph pill with the本体 label and the graph glyph', () => {
    render(<OriginBadge origin="graph" />);
    const badge = screen.getByTestId('origin-badge');
    expect(badge).toHaveAttribute('data-origin', 'graph');
    expect(badge).toHaveTextContent('本体');
  });

  it('renders the hybrid pill with BOTH glyphs and the双通道 label', () => {
    render(<OriginBadge origin="hybrid" />);
    const badge = screen.getByTestId('origin-badge');
    expect(badge).toHaveTextContent('双通道');
    const { container } = render(<OriginBadge origin="hybrid" />);
    const glyphs = container.querySelectorAll('[data-glyph]');
    expect(glyphs).toHaveLength(2);
    expect(Array.from(glyphs).map((g) => g.getAttribute('data-glyph'))).toEqual(['vector', 'graph']);
  });

  it('renders an UNKNOWN origin as its raw key, with no glyph (never blank)', () => {
    const { container } = render(<OriginBadge origin="mystery_channel" />);
    const badge = screen.getByTestId('origin-badge');
    expect(badge).toHaveTextContent('mystery_channel');
    expect(container.querySelectorAll('[data-glyph]')).toHaveLength(0);
    expect(badge.className).toContain('text-text-muted');
  });
});
