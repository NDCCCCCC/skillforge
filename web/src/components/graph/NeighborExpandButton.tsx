// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 28-05 Task 4 — NeighborExpandButton component.
//
// A small "+" button that appears on each doc/chunk node in the graph.
// Clicking it triggers the neighbor expansion flow: the parent component
// owns the fetch lifecycle (via useGraphNeighbors), and this button is
// the pure click target.
//
// Visual: border + rounded + text-xs, consistent with existing shadcn/Tailwind
// UI conventions. Shows a spinner while loading.
import type { FC } from 'react';

export interface NeighborExpandButtonProps {
  /** The chunk ID to pass to the neighbors API. */
  chunkId: string;
  /** Called when the user clicks the button. */
  onExpand: (chunkId: string) => void;
  /** Disable the button while a fetch is in flight. */
  disabled?: boolean;
  /** Show a spinner inside the button. */
  loading?: boolean;
}

/** Small button rendered next to a doc/chunk node. Clicking it expands
 * the node's neighbor subgraph via the onExpand callback. */
const NeighborExpandButton: FC<NeighborExpandButtonProps> = ({
  chunkId,
  onExpand,
  disabled,
  loading,
}) => {
  return (
    <button
      type="button"
      data-testid="neighbor-expand-btn"
      data-chunk-id={chunkId}
      disabled={disabled || loading}
      onClick={() => onExpand(chunkId)}
      className="rounded border border-border bg-surface-2 px-1.5 py-0.5 text-xs text-text-muted hover:bg-surface-3 disabled:opacity-50"
      title="查看关系邻居"
      aria-label={`查看 ${chunkId} 的关系邻居`}
    >
      {loading ? (
        <span className="inline-block h-3 w-3 animate-spin rounded-full border border-text-muted border-t-transparent" />
      ) : (
        '+'
      )}
    </button>
  );
};

export default NeighborExpandButton;
