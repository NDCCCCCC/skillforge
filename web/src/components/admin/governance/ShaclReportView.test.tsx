// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 46-04 — ShaclReportView component regression (46-UI-SPEC §Component 2/3/4):
//   - layer badges (tbox / entity / global dashed sentinel) + target_class chip
//   - zone-heading per-layer count chips (non-zero only, fixed order)
//   - dangling-edge column: 0 renders 0, missing renders — (Honesty red line 8)
//   - legacy reports without layer/target_class render badge-free without crashing
//   - detail/constraint render as React text nodes — HTML metacharacters are
//     escaped, no element injection (T-46-02 XSS mitigation)
//
// Note on the XSS innerHTML assertion: the detail <td> duplicates its content
// into a title="…" attribute, and HTML attribute serialization does not escape
// angle brackets — so a string-level innerHTML negative on the detail cell is a
// false positive by construction. The injection-proof assertion is therefore
// made at two levels: element queries (no b/i/img/script nodes) on the whole
// container, plus the literal innerHTML negative on the constraint cell (which
// has no title duplication). Rationale per the D-SPARQL-08 test-author policy.

import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ShaclReportView from './ShaclReportView';
import type { QuarantinedEntity, ShaclReport, ShaclViolation, ViolationLayer } from './types';

function makeViolation(overrides: Partial<ShaclViolation> = {}): ShaclViolation {
  return {
    severity: 'error',
    entity: 'http://skillforge.local/ontology#prod_1',
    constraint: 'sh:minCount',
    detail: 'property http://…rdfs:label has 0 value(s), minimum required is 1',
    ...overrides,
  };
}

function makeQuarantined(
  violations: ShaclViolation[],
  overrides: Partial<QuarantinedEntity> = {},
): QuarantinedEntity {
  return {
    entity: 'http://skillforge.local/ontology#prod_1',
    triple_count: 3,
    violations,
    ...overrides,
  };
}

function makeReport(overrides: Partial<ShaclReport> = {}): ShaclReport {
  return {
    health: 'error',
    error_count: 1,
    warning_count: 0,
    warnings: [],
    quarantined: [],
    triple_count: 176,
    shapes_path: 'data/shacl/shapes.ttl,data/shacl/entities.ttl',
    validated_at: '2026-10-08T00:00:00Z',
    trigger: 'boot',
    ...overrides,
  };
}

afterEach(cleanup);

describe('ShaclReportView — layered rendering (plan 46-04)', () => {
  it('legacy report without layer/target_class renders badge-free constraint rows (no crash, no chips)', () => {
    const report = makeReport({
      quarantined: [
        makeQuarantined([makeViolation()], { dangling_edge_count: 0 }),
      ],
    });
    render(<ShaclReportView report={report} />);
    const row = screen.getByTestId('quarantine-row-0');
    expect(row).toHaveTextContent('sh:minCount');
    expect(row).not.toHaveTextContent('TBox');
    expect(row).not.toHaveTextContent('实体');
    expect(row).not.toHaveTextContent('跨实体');
    // unstamped violations → all-zero layer counts → chips span not rendered
    expect(screen.queryByTestId('governance-quarantine-layer-counts')).toBeNull();
  });

  it('entity row renders 实体 badge + target_class chip', () => {
    const report = makeReport({
      quarantined: [
        makeQuarantined([
          makeViolation({
            layer: 'entity',
            target_class: 'http://skillforge.local/ontology#InsuranceProduct',
          }),
        ]),
      ],
    });
    render(<ShaclReportView report={report} />);
    const row = screen.getByTestId('quarantine-row-0');
    expect(row).toHaveTextContent('实体');
    expect(row).toHaveTextContent('http://skillforge.local/ontology#InsuranceProduct');
  });

  it('global row renders 跨实体 dashed badge and no target_class chip (sentinel)', () => {
    const report = makeReport({
      quarantined: [
        makeQuarantined([makeViolation({ entity: '(global)', layer: 'global' })], {
          entity: '(global)',
        }),
      ],
    });
    render(<ShaclReportView report={report} />);
    const row = screen.getByTestId('quarantine-row-0');
    expect(row).toHaveTextContent('跨实体');
    // sentinel: no single NodeShape class → no chip text anywhere in the row
    expect(row).not.toHaveTextContent('InsuranceProduct');
  });

  it('unknown layer value renders the raw string (console honesty — 不吞不猜)', () => {
    const report = makeReport({
      quarantined: [makeQuarantined([makeViolation({ layer: 'future' as ViolationLayer })])],
    });
    render(<ShaclReportView report={report} />);
    expect(screen.getByTestId('quarantine-row-0')).toHaveTextContent('future');
  });

  it('dangling-edge column renders 0 as 0 (not —), missing as — (Honesty red line 8)', () => {
    const report = makeReport({
      quarantined: [
        makeQuarantined([makeViolation({})], { entity: 'http://example.org/p1', dangling_edge_count: 0 }),
        makeQuarantined([makeViolation({})], { entity: 'http://example.org/o1' }),
      ],
    });
    render(<ShaclReportView report={report} />);
    const header = screen.getByTitle('指向该隔离实体的入边数量（在 KG 查询对象位仍可见）');
    expect(header).toHaveTextContent('悬挂边');
    // column order: 实体(0) · 三元组(1) · 悬挂边(2)
    const row0 = within(screen.getByTestId('quarantine-row-0')).getAllByRole('cell');
    expect(row0[2].textContent).toBe('0');
    const row1 = within(screen.getByTestId('quarantine-row-1')).getAllByRole('cell');
    expect(row1[2].textContent).toBe('—');
  });

  it('zone headings show per-layer counts — non-zero only, fixed order, red + yellow zones', () => {
    const report = makeReport({
      quarantined: [
        makeQuarantined([makeViolation({ layer: 'tbox' })], { entity: 'http://example.org/p1' }),
        makeQuarantined(
          [
            makeViolation({ constraint: 'sh:class', layer: 'tbox' }),
            makeViolation({
              constraint: 'sh:class',
              layer: 'entity',
              target_class: 'http://skillforge.local/ontology#InsuranceProduct',
            }),
          ],
          { entity: 'http://example.org/p2' },
        ),
      ],
      warnings: [
        { ...makeViolation({ severity: 'warning', layer: 'entity' }) },
        { ...makeViolation({ severity: 'warning', layer: 'entity', constraint: 'sh:class' }) },
      ],
      health: 'error',
    });
    render(<ShaclReportView report={report} />);
    const red = screen.getByTestId('governance-quarantine-layer-counts');
    expect(red).toHaveTextContent('TBox 2');
    expect(red).toHaveTextContent('实体 1');
    expect(red).not.toHaveTextContent('跨实体');
    const yellow = screen.getByTestId('governance-warnings-layer-counts');
    expect(yellow).toHaveTextContent('实体 2');
  });

  it('preserves the provenance-missing special badge in the yellow zone (Task 2 non-regression)', () => {
    const report = makeReport({
      health: 'warn',
      warnings: [
        {
          severity: 'warning',
          entity: 'http://example.org/tbl_x',
          constraint: 'provenance-missing',
          detail: 'no provenance',
        },
      ],
    });
    render(<ShaclReportView report={report} />);
    expect(screen.getByTestId('warning-row-0')).toHaveTextContent('provenance 缺失（存量数据）');
  });

  it('renders HTML metacharacters in detail/constraint as text — no element injection (T-46-02)', () => {
    const report = makeReport({
      quarantined: [
        makeQuarantined([
          makeViolation({
            detail: '<b>x</b><img src=x onerror=alert(1)>',
            constraint: '<i>y</i>',
          }),
        ]),
      ],
    });
    const { container } = render(<ShaclReportView report={report} />);
    // rendered verbatim as text (React default escaping)
    expect(screen.getByTestId('quarantine-row-0').textContent).toContain('<b>x</b>');
    expect(screen.getByTestId('quarantine-row-0').textContent).toContain('<i>y</i>');
    // no element ever materialized — the actual XSS proof
    expect(container.querySelectorAll('b, i, img, script, iframe')).toHaveLength(0);
    // string-level negative on the constraint cell (no title-attr duplication there)
    expect(container.innerHTML).not.toContain('<i>');
  });

  it('hint carries the subject-position semantics + dangling-edge guidance (46-UI-SPEC §Copywriting)', () => {
    const report = makeReport({
      quarantined: [makeQuarantined([makeViolation()], { dangling_edge_count: 0 })],
    });
    render(<ShaclReportView report={report} />);
    expect(screen.getByTestId('governance-quarantine-zone').textContent).toContain(
      '查询 subject 位置不可见',
    );
    expect(screen.getByTestId('governance-quarantine-zone').textContent).toContain('悬挂边列');
    expect(screen.getByTestId('governance-quarantine-zone').textContent).toContain('Run ECL 重建');
  });
});
