// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// quick-260928-jxb Task 1 — regression pins for Go nil-slice null guards.
// The 2026-09-28 operator walkthrough white-screened the 治理 tab because
// Go nil slices serialize to JSON null while the FE types claim arrays
// (ShaclReportView.tsx `.length` on null → whole AdminDashboard dies).
// These tests pin the normalization at the fetch-wrapper choke point.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchChangelog, fetchProvenanceGraph, fetchShaclReport } from './governanceApi';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

beforeEach(() => {
  localStorage.setItem('sf_admin_token', 'tok-abc');
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('governanceApi — null-array normalization (quick-260928-jxb BUG-1)', () => {
  it('fetchShaclReport normalizes warnings:null to [] (live payload that white-screened the 治理 tab)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          jsonResponse({
            health: 'warn',
            error_count: 0,
            warning_count: 1,
            warnings: null,
            quarantined: [
              { entity: 'http://example.org/note_001', triple_count: 2, violations: [] },
            ],
            triple_count: 176,
            shapes_path: 'shapes/skillforge.ttl',
            validated_at: '2026-09-28T00:00:00Z',
            trigger: 'boot',
          }),
        ),
      ),
    );

    const report = await fetchShaclReport();
    expect(report.warnings).toEqual([]);
    expect(report.quarantined).toHaveLength(1);
  });

  it('fetchProvenanceGraph normalizes edges:null to [] (live payload that crashed the graph view)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          jsonResponse({
            nodes: [{ id: 'notes/a.md', kind: 'file', label: 'a.md' }],
            edges: null,
          }),
        ),
      ),
    );

    const graph = await fetchProvenanceGraph();
    expect(graph.edges).toEqual([]);
    expect(graph.nodes).toHaveLength(1);
  });

  it('fetchChangelog normalizes entries:null to [] (same Go nil-slice class)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(jsonResponse({ current_version: 1, entries: null }))),
    );

    const changelog = await fetchChangelog();
    expect(changelog.current_version).toBe(1);
    expect(changelog.entries).toEqual([]);
  });

  it('passes non-null bodies through unchanged (arrays keep their contents)', async () => {
    const warnings = [
      {
        severity: 'warning' as const,
        entity: 'http://example.org/tbl_x',
        constraint: 'provenance-missing',
        detail: 'no provenance',
      },
    ];
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          jsonResponse({
            health: 'warn',
            error_count: 0,
            warning_count: 1,
            warnings,
            quarantined: [],
            triple_count: 176,
            shapes_path: 'shapes/skillforge.ttl',
            validated_at: '2026-09-28T00:00:00Z',
            trigger: 'boot',
          }),
        ),
      ),
    );

    const report = await fetchShaclReport();
    expect(report.warnings).toEqual(warnings);
    expect(report.quarantined).toEqual([]);
  });
});
