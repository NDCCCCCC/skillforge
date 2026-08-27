// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Sparkline } from './Sparkline';

describe('Sparkline', () => {
  it('TestSparklinePureSVG', () => {
    const onPointHover = vi.fn();
    render(
      <Sparkline
        points={[
          { index: 0, period: '2026-05', value: 4231 },
          { index: 1, period: '2026-06', value: 4198 },
          { index: 2, period: '2026-07', value: 4150 },
        ]}
        onPointHover={onPointHover}
      />,
    );

    const svg = screen.getByTestId('sparkline');
    expect(svg).toBeInTheDocument();
    expect(svg.tagName).toBe('svg');

    const polyline = svg.querySelector('polyline');
    expect(polyline).toBeInTheDocument();
    expect(polyline).toHaveAttribute('fill', 'none');

    const circles = svg.querySelectorAll('circle');
    expect(circles).toHaveLength(3);

    expect(screen.getByTestId('sparkline-point-0')).toBeInTheDocument();
    expect(screen.getByTestId('sparkline-point-2')).toBeInTheDocument();
  });

  it('TestSparklineHoverPoint', () => {
    const onPointHover = vi.fn();
    render(
      <Sparkline
        points={[
          { index: 0, period: '2026-05', value: 4231 },
          { index: 1, period: '2026-06', value: 4198 },
        ]}
        onPointHover={onPointHover}
      />,
    );

    fireEvent.mouseEnter(screen.getByTestId('sparkline-point-1'));
    expect(onPointHover).toHaveBeenCalledWith(1);

    fireEvent.click(screen.getByTestId('sparkline-point-0'));
    expect(onPointHover).toHaveBeenCalledWith(0);
  });

  it('TestSparklineAutoscale', () => {
    const onPointHover = vi.fn();
    render(
      <Sparkline
        points={[
          { index: 0, period: '2026-05', value: 100 },
          { index: 1, period: '2026-06', value: 200 },
        ]}
        onPointHover={onPointHover}
      />,
    );

    const svg = screen.getByTestId('sparkline');
    const polyline = svg.querySelector('polyline');
    const points = polyline!.getAttribute('points')!;
    // Points with known range: 100 to 200 (range=100, padding=10 => 90 to 210)
    // First point (100) maps to y ≈ 32 (bottom), second (200) maps to y ≈ 0 (top)
    const coords = points.split(' ').map((c) => c.split(','));
    expect(Number(coords[0][1])).toBeGreaterThan(Number(coords[1][1]));
  });

  it('TestSparklineNoDeps', async () => {
    const { readFileSync } = await import('fs');
    const content = readFileSync(
      'D:/code/ClaudeCode/ai-agent/web/src/components/chat/parts/Sparkline.tsx',
      'utf-8',
    );
    const recharts = /recharts/.test(content);
    const sparklines = /react-sparklines/.test(content);
    const chartjs = /chart\.js/.test(content);
    expect({ recharts, sparklines, chartjs }).toEqual({
      recharts: false,
      sparklines: false,
      chartjs: false,
    });
  });
});
