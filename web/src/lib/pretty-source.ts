// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// pretty-source — display-layer beautification for corpus source paths
// (quick 260929-d5w2). Milvus source strings and graph doc filenames
// carry the full disk name, e.g.
//
//   sql-guides/取数口径/032_保单号查支付方式与开户行__sec02_SQL_正文.md
//
// which is unreadable in narrow rails and SVG node labels. prettySourcePath
// renders the reader-facing short form while the RAW path stays available
// for hover (title attributes) and machine reads (data-* attributes):
//
//   ① basename — drop directory prefixes (skill slug / tier dirs)
//   ② strip the trailing ".md"
//   ③ split the `__secNN_<section>` section suffix: the base becomes the
//      label and the section name is appended after " · " with underscores
//      dropped ("SQL_正文" → "SQL正文")
//
//   → "032_保单号查支付方式与开户行 · SQL正文"
//
// Pure function — no fixture / endpoint coupling.

/** The " · " separator between the doc base name and its section label. */
const SECTION_SEPARATOR = ' · ';

export function prettySourcePath(path: string): string {
  // ① basename — drop any directory prefix (sql-guides/tier/... forms).
  const slash = path.lastIndexOf('/');
  let base = slash >= 0 ? path.slice(slash + 1) : path;
  // Strip the markdown extension when present (inputs may already lack it).
  base = base.replace(/\.md$/i, '');
  // ② __secNN_<section> suffix → "<base> · <section>" (underscores dropped).
  const m = base.match(/^(.*?)__sec(\d{2})_(.+)$/);
  if (m) {
    return `${m[1]}${SECTION_SEPARATOR}${m[3].replace(/_/g, '')}`;
  }
  return base;
}
