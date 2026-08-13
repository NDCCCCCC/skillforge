// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// stripFsPath — shared utility for stripping absolute filesystem paths
// from BE error strings before they are rendered into the UI. Lifted
// from ReloadToast.tsx in Plan 05-08 (WR-01 closure) so LeftSidebar +
// any other consumer can apply the same defense-in-depth against
// T-5-IA-04 (path/secret leakage in error text).
//
// Semantics (verbatim from the original inline implementation):
//   - Input is a string or undefined (passthrough).
//   - Last occurrence of '/' or '\' wins (basename).
//   - If no separator is found, the input is returned unchanged.

/** Strip an absolute filesystem path to its basename. Defensive — the
 * BE LoadError hook may include a path; we never want to leak server
 * filesystem structure into the UI. */
export function stripFsPath(s: string | undefined): string | undefined {
  if (!s) return s;
  // Last `/` or `\` wins.
  const idx = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'));
  if (idx < 0) return s;
  return s.slice(idx + 1);
}
