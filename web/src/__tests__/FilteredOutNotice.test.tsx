// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-05 Task 2 — FilteredOutNotice tests (SC-8 / CMPL-08).
//
// Pins the banner copy per UI-SPEC §SC-8:
//   - Renders ONLY when the chunk is superseded (effectiveTo < now).
//   - Banner copy includes 此片段已失效 + {effectiveTo} + 不会出现在检索结果中。仅可浏览。
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import FilteredOutNotice from '../components/center/FilteredOutNotice';

afterEach(cleanup);

describe('FilteredOutNotice', () => {
  it('renders the documented banner copy when effectiveTo is provided (epoch seconds)', () => {
    // 2023-03-01 UTC = 1677628800
    render(<FilteredOutNotice effectiveTo={1677628800} />);
    const banner = screen.getByTestId('filtered-out-notice');
    expect(banner).toBeInTheDocument();
    expect(banner.textContent).toContain('此片段已失效');
    expect(banner.textContent).toContain('2023-03-01');
    expect(banner.textContent).toContain('不会出现在检索结果中');
    expect(banner.textContent).toContain('仅可浏览');
  });

  it('does NOT render when effectiveTo is null (chunk is currently active)', () => {
    const { container } = render(<FilteredOutNotice effectiveTo={null} />);
    expect(container.querySelector('[data-testid="filtered-out-notice"]')).toBeNull();
  });
});