// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// StepTree -- 56-04 Task 4 (PANEL-02) Langfuse-style trace tree surface.
//
// Rendered INSIDE the existing 「Agent 状态」 tab (no 8th tab: the
// 7-tab IA is locked by 56-UI-SPEC section 4). The tree sits above the
// per-hop accordion so the operator first sees the whole run at a
// glance (检索 -> 工具 -> 规划 -> 决策), then drills into one hop.
//
// Execution-vs-completed visual: an `active` node shows a spinner
// (Loader2 + animate-spin) and a 「进行中」 badge; a `done` node shows
// CheckCircle2 + a 「完成」 badge. The status is derived in
// buildStepTree() from useChat.status + the plan todo statuses -- this
// component renders, it does not infer.
//
// Phase 58 extension slot: node.channels exists on the type but is
// deliberately NOT read here (PROV-01/02 owns that rendering).
import { CheckCircle2, Loader2, ListTree } from 'lucide-react';
import { cn } from '../../lib/cn';
import type { StepNodeStatus, StepTreeNode } from './step-tree';

function NodeStatusIcon({ status }: { status: StepNodeStatus }) {
  if (status === 'active') {
    return (
      <Loader2
        data-testid="step-tree-status-icon"
        data-status="active"
        className="h-3.5 w-3.5 shrink-0 animate-spin text-primary"
      />
    );
  }
  return (
    <CheckCircle2
      data-testid="step-tree-status-icon"
      data-status="done"
      className="h-3.5 w-3.5 shrink-0 text-status-compliant"
    />
  );
}

function NodeBadge({ status }: { status: StepNodeStatus }) {
  return (
    <span
      data-testid="step-tree-status-badge"
      data-status={status}
      className={cn(
        'ml-auto inline-flex shrink-0 items-center rounded-md border px-1.5 py-0.5 text-[11px] font-semibold',
        status === 'active'
          ? 'border-primary/40 bg-primary/10 text-primary'
          : 'border-status-compliant/40 bg-status-compliant/10 text-status-compliant',
      )}
    >
      {status === 'active' ? '进行中' : '完成'}
    </span>
  );
}

function NodeRow({ node, depth }: { node: StepTreeNode; depth: number }) {
  return (
    <li
      data-testid="step-tree-node"
      data-node-id={node.id}
      data-kind={node.kind}
      data-status={node.status}
      data-depth={depth}
      className={cn(
        'rounded-md border border-border bg-surface px-2 py-1.5',
        node.status === 'active' ? 'border-l-4 border-l-primary' : 'border-l-4 border-l-transparent',
      )}
    >
      <div className="flex items-center gap-2">
        <NodeStatusIcon status={node.status} />
        <span className="truncate text-xs font-semibold text-text">{node.label}</span>
        {node.detail && (
          <span
            data-testid="step-tree-node-detail"
            className="truncate font-mono text-[11px] text-text-muted"
          >
            {node.detail}
          </span>
        )}
        <NodeBadge status={node.status} />
      </div>
      {node.children && node.children.length > 0 && (
        <ul data-testid="step-tree-children" className="mt-1.5 space-y-1 pl-5">
          {node.children.map((child) => (
            <NodeRow key={child.id} node={child} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  );
}

export interface StepTreeProps {
  nodes: StepTreeNode[];
  /** True while the answer is still streaming -- selects the empty copy. */
  isStreaming: boolean;
}

export default function StepTree({ nodes, isStreaming }: StepTreeProps) {
  if (nodes.length === 0) {
    return (
      <div
        data-testid="step-tree-empty"
        className="rounded-md border border-border bg-surface px-3 py-2 text-xs text-text-muted"
      >
        {isStreaming
          ? '等待第一个执行步骤…'
          : '本回合没有可聚合的步骤（普通问答不产生检索/工具/规划节点）。'}
      </div>
    );
  }

  const activeCount = nodes.filter((n) => n.status === 'active').length;

  return (
    <section data-testid="step-tree" data-node-count={nodes.length} className="flex flex-col gap-2">
      <header className="flex items-center gap-2 text-xs text-text-muted">
        <ListTree className="h-3.5 w-3.5" />
        <span className="font-semibold text-text">执行步骤</span>
        <span data-testid="step-tree-summary">
          共 {nodes.length} 步{activeCount > 0 ? ` · ${activeCount} 进行中` : ''}
        </span>
      </header>
      <ul className="space-y-1.5">
        {nodes.map((node) => (
          <NodeRow key={node.id} node={node} depth={0} />
        ))}
      </ul>
    </section>
  );
}
