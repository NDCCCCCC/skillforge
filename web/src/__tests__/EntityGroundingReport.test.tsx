// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Phase 57-03 (ONT-03) -- the report surface pins. The honesty distinction
// between "did not run" and "ran and found nothing" is the whole subject.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import EntityGroundingReport from '../components/verdict/EntityGroundingReport';

afterEach(cleanup);

describe('EntityGroundingReport (57-03)', () => {
  it('renders NOTHING when entities is absent (the check did not run)', () => {
    const { container } = render(<EntityGroundingReport />);
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByTestId('entity-grounding-report')).toBeNull();
  });

  it('renders the unverified block with the locked copy and every entity', () => {
    render(
      <EntityGroundingReport
        entities={{ verified: ['差旅住宿费标准'], unverified: ['虚构的审批委员会', '不存在的部门'] }}
      />,
    );
    const report = screen.getByTestId('entity-grounding-report');
    expect(report).toHaveAttribute('data-verdict', 'unverified');
    expect(report).toHaveTextContent('实体校验未通过');
    expect(report).toHaveTextContent('这些实体在本 Skill 可读语料中不存在，回答已降级为需复核');
    expect(screen.getAllByTestId('entity-unverified-item')).toHaveLength(2);
    expect(screen.getByText('虚构的审批委员会')).toBeInTheDocument();
  });

  it('renders a collapsed pass line when everything verified', () => {
    render(<EntityGroundingReport entities={{ verified: ['差旅住宿费标准', '年假制度'], unverified: [] }} />);
    const report = screen.getByTestId('entity-grounding-report');
    expect(report).toHaveAttribute('data-verdict', 'verified');
    expect(report.tagName).toBe('DETAILS');
    expect(report).toHaveTextContent('实体校验通过（2 个）');
    // Collapsed by default: the verified list body is hidden behind <summary>.
    expect(report).not.toHaveAttribute('open');
  });

  it('renders the explicit empty-copy when the check ran and found no entity names', () => {
    render(<EntityGroundingReport entities={{ verified: [], unverified: [] }} />);
    const report = screen.getByTestId('entity-grounding-report');
    expect(report).toHaveAttribute('data-verdict', 'empty');
    expect(report).toHaveTextContent('实体校验：未识别到实体名称');
  });

  it('is READ-ONLY: zero buttons and zero click handlers', () => {
    const { container } = render(
      <EntityGroundingReport entities={{ verified: [], unverified: ['x实体'] }} />,
    );
    expect(container.querySelectorAll('button')).toHaveLength(0);
    expect(container.querySelectorAll('[onclick]')).toHaveLength(0);
  });
});
