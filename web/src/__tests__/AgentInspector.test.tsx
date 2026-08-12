// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 — AgentInspector empty + loading state tests.
//
// Pins the SC-4 acceptance assertions for the placeholder:
//   1. Empty state renders 暂无 Agent 状态 + body copy verbatim.
//   2. Loading state renders 等待 Agent 首个事件... when streaming=true.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import AgentInspector from '../components/inspector/AgentInspector';

afterEach(cleanup);

describe('AgentInspector', () => {
  it('renders the empty state with the verbatim heading and body', () => {
    render(<AgentInspector />);
    const empty = screen.getByTestId('agent-inspector-empty');
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveTextContent('暂无 Agent 状态');
    expect(empty).toHaveTextContent('提出问题后，此处实时显示当前回合的工具调用、检索（含 chunk ID）与中间推理。');
  });

  it('renders the loading state when streaming=true', () => {
    render(<AgentInspector streaming />);
    const loading = screen.getByTestId('agent-inspector-loading');
    expect(loading).toBeInTheDocument();
    expect(loading).toHaveTextContent('等待 Agent 首个事件...');
  });

  it('does not render the empty hero when streaming=true', () => {
    render(<AgentInspector streaming />);
    expect(screen.queryByTestId('agent-inspector-empty')).toBeNull();
  });
});
