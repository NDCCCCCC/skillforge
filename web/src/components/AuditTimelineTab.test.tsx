// SPDX-License-Identifier: Apache-2.0

// 56-04 Task 3 PANEL-03 -- AuditTimelineTab empty-state guidance asserts
// (the empty copy now distinguishes "nothing yet" from the guidance to
// ask a question, per 56-UI-SPEC section 4).

import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AuditTimelineTab from './AuditTimelineTab';
import { useAppStore } from '../store/app-store';

beforeEach(() => {
  cleanup();
  vi.stubGlobal('fetch', vi.fn(async () => ({
    ok: true,
    status: 200,
    json: async () => ({ rows: [] }),
  })));
  useAppStore.setState((s) => ({ activeSessionId: 'sess-a' }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('AuditTimelineTab empty state (56-04)', () => {
  it('shows the guidance copy, not just a bare empty line', async () => {
    render(<AuditTimelineTab />);
    await waitFor(() =>
      expect(screen.getByTestId('audit-timeline-empty')).toBeTruthy(),
    );
    expect(
      screen.getByText(/发起一次提问后/),
    ).toBeTruthy();
  });
});
