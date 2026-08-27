// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Sparkline — Phase 17 D17-B1 pure SVG sparkline component.
// Zero external chart library dependencies (D17-B1 zero-dep rule).
// Renders a polyline + N circle points; each circle emits onPointHover on hover/click.

import { useMemo } from 'react';

export interface SparklinePoint {
  index: number;
  period: string;
  value: number;
}

export function Sparkline({ points, onPointHover }: {
  points: SparklinePoint[];
  onPointHover: (i: number) => void;
}) {
  const { polyline, circles } = useMemo(() => {
    const values = points.map((p) => p.value);
    const min = Math.min(...values);
    const max = Math.max(...values);
    const range = max - min || 1;
    const padding = range * 0.1; // ±10%
    const yMin = min - padding;
    const yMax = max + padding;
    const yRange = yMax - yMin;
    const w = 300, h = 32;
    const xStep = points.length > 1 ? w / (points.length - 1) : 0;
    const poly = points
      .map((p, i) => {
        const x = i * xStep;
        const y = h - ((p.value - yMin) / yRange) * h;
        return `${x},${y}`;
      })
      .join(' ');
    const cs = points.map((p, i) => {
      const x = i * xStep;
      const y = h - ((p.value - yMin) / yRange) * h;
      return { cx: x, cy: y, idx: i };
    });
    return { polyline: poly, circles: cs };
  }, [points]);

  if (points.length === 0) return null;

  return (
    <svg
      viewBox="0 0 300 32"
      width="100%"
      height="32"
      style={{ overflow: 'visible' }}
      data-testid="sparkline"
      role="img"
      aria-label={`sparkline with ${points.length} points`}
    >
      <polyline
        points={polyline}
        fill="none"
        stroke="var(--color-sparkline, var(--color-primary))"
        strokeWidth="1.5"
      />
      {circles.map((c) => (
        <circle
          key={c.idx}
          cx={c.cx}
          cy={c.cy}
          r="2.5"
          fill="var(--color-sparkline, var(--color-primary))"
          stroke="var(--color-surface-2)"
          strokeWidth="1"
          onMouseEnter={() => onPointHover(c.idx)}
          onClick={() => onPointHover(c.idx)}
          data-testid={`sparkline-point-${c.idx}`}
          style={{ cursor: 'pointer' }}
        />
      ))}
    </svg>
  );
}
