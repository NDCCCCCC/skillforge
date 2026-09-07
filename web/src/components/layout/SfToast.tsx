// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SfToast — Plan 29-07 Task 1 (FBK-04/TKT-03 chat side) — the global
// 'sf-toast' toast viewport. Before this component the window-level
// 'sf-toast' CustomEvent had dispatchers but NO listener anywhere in the
// app: SessionDrawer's rename-failure toast (and FeedbackRow's B6-era
// placeholder toasts) vanished into the void. SfToast owns BOTH halves —
//
//   1. LISTENER: window.addEventListener('sf-toast') + removal on
//      unmount, mirroring the 'wizard-success' precedent (App.tsx:301-309).
//      Because the component owns the listener itself, it can be mounted
//      per route-branch: App.tsx mounts it in the chat + eval branches,
//      and 29-08 mounts a second instance inside AdminDashboard's root
//      div (the /admin branch early-returns before App's own markup).
//
//   2. VIEWPORT: single slot, latest-wins — a new message replaces the
//      old one and resets its dismiss timer. Markup mirrors the existing
//      wizard-success-toast (fixed bottom-4 right-4 z-40 rounded-md
//      border bg-surface-2 px-4 py-3 shadow-lg, × close at right-1 top-1).
//
// Event contract: CustomEvent('sf-toast', { detail: { kind, message } })
// with LENIENT kind mapping for legacy dispatchers (UI-SPEC §Surface 4):
//   'error'                          → error   (SessionDrawer.tsx:170 verbatim)
//   'success' | 'compliant'          → success ('compliant' = the B6-era
//                                       FeedbackRow placeholder that
//                                       dispatched raw Vote values as kind)
//   anything else / missing / unknown → info
//
// Auto-dismiss: 4000ms (success/info), 6000ms (error). No auto-retry, no
// queue — one slot keeps the toast non-blocking per D-29-04.
//
// Zero new dependencies, zero new design tokens: every class below is
// already used by the wizard-success toast / ReloadToast precedents, and
// the × glyph is the same text node the wizard toast renders (no new icon).
import { useCallback, useEffect, useRef, useState } from 'react';
import { cn } from '../../lib/cn';

type ToastKind = 'success' | 'error' | 'info';

const AUTO_DISMISS_MS: Record<ToastKind, number> = {
  success: 4000,
  info: 4000,
  error: 6000,
};

/** Lenient kind mapping — never crash on a legacy/unknown dispatcher. */
function normalizeKind(raw: unknown): ToastKind {
  if (raw === 'error') return 'error';
  if (raw === 'success' || raw === 'compliant') return 'success';
  return 'info';
}

interface ToastState {
  /** Monotonic slot key — bumps per message so a REPLACEMENT (even with
   *  an identical payload) re-runs the timer effect (latest-wins). */
  key: number;
  kind: ToastKind;
  message: string;
}

export default function SfToast() {
  const [toast, setToast] = useState<ToastState | null>(null);
  const keyRef = useRef(0);

  // Listener half — mounted for this component's lifetime; removed on
  // unmount so a route switch (chat ↔ eval ↔ admin) never leaks handlers.
  useEffect(() => {
    function onToast(e: Event) {
      const detail = (e as CustomEvent<{ kind?: unknown; message?: unknown }>).detail;
      const message = typeof detail?.message === 'string' ? detail.message : '';
      if (message === '') return; // nothing to show — ignore malformed events
      keyRef.current += 1;
      setToast({ key: keyRef.current, kind: normalizeKind(detail?.kind), message });
    }
    window.addEventListener('sf-toast', onToast);
    return () => window.removeEventListener('sf-toast', onToast);
  }, []);

  // Timer half — keyed on the toast object so every new message RESETS
  // the dismiss timer (latest-wins single slot).
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), AUTO_DISMISS_MS[toast.kind]);
    return () => clearTimeout(timer);
  }, [toast]);

  const close = useCallback(() => setToast(null), []);

  if (!toast) return null;

  return (
    <div
      data-testid="sf-toast"
      data-kind={toast.kind}
      role="status"
      aria-live="polite"
      className={cn(
        'fixed bottom-4 right-4 z-40 rounded-md border bg-surface-2 px-4 py-3 shadow-lg',
        toast.kind === 'success' && 'border-status-compliant/40 text-status-compliant',
        toast.kind === 'error' && 'border-status-non-compliant/40 text-status-non-compliant',
        toast.kind === 'info' && 'border-border text-text',
      )}
    >
      <div className="flex items-center gap-2 text-xs font-semibold">{toast.message}</div>
      <button
        type="button"
        data-testid="sf-toast-close"
        onClick={close}
        className="absolute right-1 top-1 inline-flex h-6 w-6 items-center justify-center rounded-md text-text-muted hover:bg-surface-3"
        aria-label="关闭"
      >
        ×
      </button>
    </div>
  );
}
