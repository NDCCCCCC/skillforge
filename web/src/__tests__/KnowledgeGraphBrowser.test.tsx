// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-03 Task 2 — KnowledgeGraphBrowser empty + loading state tests.
//
// Pins the SC-5 acceptance assertions for the placeholder:
//   1. Empty state renders 知识库为空 + body copy verbatim.
//   2. Loading state renders 正在加载知识层级... when loading=true.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import KnowledgeGraphBrowser from '../components/graph/KnowledgeGraphBrowser';

afterEach(cleanup);

describe('KnowledgeGraphBrowser', () => {
  it('renders the empty state with the verbatim heading and body', () => {
    render(<KnowledgeGraphBrowser />);
    const empty = screen.getByTestId('knowledge-graph-empty');
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveTextContent('知识库为空');
    expect(empty).toHaveTextContent('cmd/spiked index');
  });

  it('renders the loading state when loading=true', () => {
    render(<KnowledgeGraphBrowser loading />);
    const loading = screen.getByTestId('knowledge-graph-loading');
    expect(loading).toBeInTheDocument();
    expect(loading).toHaveTextContent('正在加载知识层级...');
  });

  it('does not render the empty hero when loading=true', () => {
    render(<KnowledgeGraphBrowser loading />);
    expect(screen.queryByTestId('knowledge-graph-empty')).toBeNull();
  });
});
