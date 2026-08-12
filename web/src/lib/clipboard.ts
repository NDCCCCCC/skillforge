// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-06 Task 2 — SC-7 RedlineCard CTA helper.
//
// copyToClipboard writes text to the system clipboard via navigator.clipboard
// and resolves with true on success / false on failure. The caller manages
// toast state (the existing wizard-success-toast pattern in App.tsx is the
// project's toast shape — adding sonner just for two CTAs would be a heavy
// new dep, so we keep the surface minimal and let RedlineCard render its
// own inline 已复制到剪贴板 toast).
//
// Browser support: navigator.clipboard.writeText is available in all
// evergreen browsers since 2022 (Chromium 66+, Firefox 63+, Safari 13.1+).
// In jsdom (vitest) the API is absent; tests mock navigator.clipboard via
// Object.defineProperty before rendering.
//
// Security note: navigator.clipboard.writeText requires a user-gesture
// context (a click). The RedlineCard CTAs are <button> elements, so the
// gesture is preserved end-to-end. We do NOT attempt to write to the
// clipboard outside a click handler (the browser will reject it).
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
      return false;
    }
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Permission denied / non-secure context / clipboard API unavailable.
    // The caller renders an error toast; never throw out of this helper.
    return false;
  }
}

// downloadMarkdown writes a markdown-shaped Blob to a <a download> element
// and triggers a click. Used by RedlineCard's 下载为 .md CTA so the
// operator can stash the diff alongside the source policy doc. Like
// copyToClipboard, this MUST be called from a user-gesture handler.
export function downloadMarkdown(filename: string, body: string): void {
  if (typeof document === 'undefined') return;
  const blob = new Blob([body], { type: 'text/markdown;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  // Revoke on the next tick so the click has time to dispatch.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
