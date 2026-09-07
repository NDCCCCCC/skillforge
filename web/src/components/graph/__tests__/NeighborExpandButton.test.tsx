// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 28-05 Task 4 — NeighborExpandButton component tests.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import NeighborExpandButton from '../NeighborExpandButton';

afterEach(() => {
  cleanup();
});

describe('NeighborExpandButton', () => {
  it('renders with the + symbol when not loading', () => {
    render(<NeighborExpandButton chunkId="7" onExpand={vi.fn()} />);
    const btn = screen.getByTestId('neighbor-expand-btn');
    expect(btn).toHaveTextContent('+');
    expect(btn).not.toBeDisabled();
  });

  it('renders a spinner when loading=true', () => {
    render(<NeighborExpandButton chunkId="7" onExpand={vi.fn()} loading />);
    const btn = screen.getByTestId('neighbor-expand-btn');
    expect(btn.querySelector('.animate-spin')).not.toBeNull();
    expect(btn).toBeDisabled();
  });

  it('is disabled when disabled=true even if not loading', () => {
    render(<NeighborExpandButton chunkId="7" onExpand={vi.fn()} disabled />);
    expect(screen.getByTestId('neighbor-expand-btn')).toBeDisabled();
  });

  it('calls onExpand with the correct chunkId on click', () => {
    const onExpand = vi.fn();
    render(<NeighborExpandButton chunkId="7" onExpand={onExpand} />);
    fireEvent.click(screen.getByTestId('neighbor-expand-btn'));
    expect(onExpand).toHaveBeenCalledTimes(1);
    expect(onExpand).toHaveBeenCalledWith('7');
  });

  it('does not call onExpand when disabled', () => {
    const onExpand = vi.fn();
    render(<NeighborExpandButton chunkId="7" onExpand={onExpand} disabled />);
    fireEvent.click(screen.getByTestId('neighbor-expand-btn'));
    expect(onExpand).not.toHaveBeenCalled();
  });

  it('does not call onExpand when loading', () => {
    const onExpand = vi.fn();
    render(<NeighborExpandButton chunkId="7" onExpand={onExpand} loading />);
    fireEvent.click(screen.getByTestId('neighbor-expand-btn'));
    expect(onExpand).not.toHaveBeenCalled();
  });

  it('sets data-chunk-id attribute to the chunkId prop', () => {
    render(<NeighborExpandButton chunkId="42" onExpand={vi.fn()} />);
    expect(screen.getByTestId('neighbor-expand-btn')).toHaveAttribute('data-chunk-id', '42');
  });

  it('has the correct aria-label', () => {
    render(<NeighborExpandButton chunkId="7" onExpand={vi.fn()} />);
    expect(screen.getByTestId('neighbor-expand-btn')).toHaveAttribute('aria-label', '查看 7 的关系邻居');
  });
});
