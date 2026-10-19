// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 47-04 — TimelineView rollback-completion callback contract (REL-02 /
// D-02b). The governance banner only learns about the post-rollback SHACL
// report because TimelineView notifies its parent after a successful
// rollback; that hand-off is the half that can silently regress.
//
// Pinned here:
//   - fired exactly once on a successful rollback, after the unchanged
//     success toast
//   - never fired when the rollback itself fails
//   - a throwing parent handler cannot convert a successful rollback into the
//     failure toast (T-47-14 — the user must never be told a rollback failed
//     when it succeeded)
//   - a FAILING changelog refetch after a successful rollback cannot do the
//     same, and does not suppress the REL-02 callback (MEDIUM-1, 47-REVIEW.md)
//
// No new data-testid values were added to the component; the suite drives the
// pre-existing `governance-rollback-${version}` hooks.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import TimelineView from './TimelineView';
import type { ChangelogEntry } from './types';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function makeEntry(version: number): ChangelogEntry {
  return {
    version,
    who: 'admin',
    when: '2026-10-08T00:00:00Z',
    what: { added: ['http://example.org/tbl_x'], removed: [] },
    why: `v${version}`,
  };
}

const CHANGELOG = { current_version: 3, entries: [makeEntry(3), makeEntry(2)] };

/** Fetch router: rollback ok/fail is toggled by `rollbackOk`. */
function stubFetch(rollbackOk: boolean) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('/api/admin/ontology/rollback')) {
      if (!rollbackOk) return Promise.resolve(jsonResponse({ error: 'boom' }, 500));
      return Promise.resolve(jsonResponse({ version: 4, new_violations: 2 }));
    }
    if (url.includes('/api/admin/ontology/changelog')) {
      return Promise.resolve(jsonResponse(CHANGELOG));
    }
    throw new Error(`unexpected fetch: ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

/**
 * MEDIUM-1 (47-REVIEW.md): the mount changelog GET answers normally so the
 * component renders its timeline, and only the POST-rollback refetch fails with
 * a 500. This is the case the first three cases could not reach — stubFetch
 * answered the changelog URL successfully every time.
 */
function stubFetchChangelogRefetchFails() {
  let changelogCalls = 0;
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes('/api/admin/ontology/rollback')) {
      return Promise.resolve(jsonResponse({ version: 4, new_violations: 2 }));
    }
    if (url.includes('/api/admin/ontology/changelog')) {
      changelogCalls += 1;
      if (changelogCalls === 1) return Promise.resolve(jsonResponse(CHANGELOG));
      return Promise.resolve(jsonResponse({ error: 'changelog handler exploded' }, 500));
    }
    throw new Error(`unexpected fetch: ${url}`);
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  document.body.innerHTML = '';
});

describe('TimelineView — onRollbackComplete (plan 47-04)', () => {
  it('fires the callback exactly once after a successful rollback', async () => {
    stubFetch(true);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const spy = vi.fn();

    render(<TimelineView onRollbackComplete={spy} />);
    const button = await screen.findByTestId('governance-rollback-2');
    fireEvent.click(button);

    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));
    // Success toast text is unchanged (D-02e) and agrees with the response.
    expect(document.body.textContent).toContain(
      '已回滚至 v2（当前 v4）· 新增 2 条 violation',
    );
    expect(document.body.querySelector('.sf-toast-error')).toBeNull();
  });

  it('never fires the callback when the rollback fails', async () => {
    stubFetch(false);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const spy = vi.fn();

    render(<TimelineView onRollbackComplete={spy} />);
    const button = await screen.findByTestId('governance-rollback-2');
    fireEvent.click(button);

    await waitFor(() => expect(document.body.querySelector('.sf-toast-error')).not.toBeNull());
    expect(document.body.textContent).toContain('回滚失败');
    expect(spy).not.toHaveBeenCalled();
  });

  it('a throwing parent handler does not turn a successful rollback into the failure toast', async () => {
    stubFetch(true);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const spy = vi.fn(() => {
      throw new Error('refetch blew up');
    });

    render(<TimelineView onRollbackComplete={spy} />);
    const button = await screen.findByTestId('governance-rollback-2');
    fireEvent.click(button);

    await waitFor(() =>
      expect(document.body.textContent).toContain('已回滚至 v2（当前 v4）'),
    );
    expect(document.body.querySelector('.sf-toast-error')).toBeNull();
    // finally block still ran — the button is no longer disabled
    await waitFor(() =>
      expect(screen.getByTestId('governance-rollback-2')).not.toBeDisabled(),
    );
  });

  it('a changelog refetch failure after a successful rollback is not reported as a rollback failure', async () => {
    // MEDIUM-1: rollback POST resolves 200, the follow-up changelog GET 500s.
    const fetchMock = stubFetchChangelogRefetchFails();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const spy = vi.fn();

    render(<TimelineView onRollbackComplete={spy} />);
    const button = await screen.findByTestId('governance-rollback-2');
    fireEvent.click(button);

    // REL-02 must not depend on the changelog refetch: the parent is notified
    // regardless, so the post-rollback SHACL report still reaches the banner.
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1));

    // The refetch really did happen and really did fail — otherwise this case
    // would pass vacuously.
    await waitFor(() => {
      const changelogCalls = fetchMock.mock.calls.filter((c) =>
        String(c[0]).includes('/api/admin/ontology/changelog'),
      );
      expect(changelogCalls.length).toBe(2);
    });

    // The rollback succeeded, so its success toast stands and nothing may
    // claim it failed (T-47-14 — repudiation).
    await waitFor(() =>
      expect(document.body.textContent).toContain('已回滚至 v2（当前 v4）'),
    );
    expect(document.body.textContent).not.toContain('回滚失败');
    expect(document.body.querySelector('.sf-toast-error')).toBeNull();

    // finally still ran.
    await waitFor(() =>
      expect(screen.getByTestId('governance-rollback-2')).not.toBeDisabled(),
    );
  });
});
