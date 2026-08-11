// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SourceUrlPart — T-1-08 hard gate. Every source-url chunk is sanitized
// through `sanitizeSourceUrl()` BEFORE the href is assigned. Rejected
// URLs render a blocked-citation span + console.warn. Accepted URLs
// render an <a> with target="_blank" rel="noopener noreferrer" — the
// mandatory rel attributes that prevent reverse-tabnabbing.
//
// Per 04-UI-SPEC.md §FEUI-03 the citation click also signals the center
// panel to load the chunk verbatim (D-01). Wave 3 wires the full click
// handler; Wave 1 emits the clickable element with the right safety
// posture so the spike stub's XSS vector is eliminated.

import type { SourceUrlUIPart } from 'ai';
import { sanitizeSourceUrl } from '../../../lib/sanitize-url';

export function SourceUrlPart({ part }: { part: SourceUrlUIPart }) {
  const safe = sanitizeSourceUrl(part.url);

  if (!safe.ok) {
    // eslint-disable-next-line no-console
    console.warn(`[T-1-08] blocked citation URL host=${safe.hostname ?? '(unknown)'}`);
    return (
      <span className="citation-blocked" data-testid="citation-blocked">
        引用 URL 未通过安全白名单，已拦截：
        {safe.hostname ?? safe.reason}
      </span>
    );
  }

  const label = part.title ?? new URL(part.url).hostname;

  return (
    <a
      className="citation-chip"
      href={safe.href}
      target="_blank"
      rel="noopener noreferrer"
      data-testid="citation-link"
    >
      {label}
    </a>
  );
}
