// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SourceUrlPart — T-1-08 hard gate. Every source-url chunk is sanitized
// through `sanitizeSourceUrl()` BEFORE the href is assigned. Rejected
// URLs render a blocked-citation span + console.warn. Accepted URLs
// render an <a> with target="_blank" rel="noopener noreferrer" — the
// mandatory rel attributes that prevent reverse-tabnabbing.
//
// Modernized: pill-style chip with a globe icon and the document/host
// label, replacing the older dense inline badge.

import type { SourceUrlUIPart } from 'ai';
import { Globe, ShieldAlert } from 'lucide-react';
import { sanitizeSourceUrl } from '../../../lib/sanitize-url';

export function SourceUrlPart({ part }: { part: SourceUrlUIPart }) {
  const safe = sanitizeSourceUrl(part.url);

  if (!safe.ok) {
    // eslint-disable-next-line no-console
    console.warn(`[T-1-08] blocked citation URL host=${safe.hostname ?? '(unknown)'}`);
    return (
      <span className="citation-blocked" data-testid="citation-blocked">
        <ShieldAlert className="h-3 w-3" />
        <span>引用 URL 未通过安全白名单，已拦截</span>
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
      <Globe className="h-3 w-3" />
      <span>{label}</span>
    </a>
  );
}
