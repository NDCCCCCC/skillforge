// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// sse-client — small EventSource wrapper with auto-reconnect (T-5-11
// DoS mitigation: 3s base backoff with exp backoff capped at 30s).
//
// Returns a `connect()` function that opens the EventSource and an
// `close()` function that tears it down. The returned `subscribe`
// callback fires once per `message` event with the parsed JSON
// payload (defensive — falls back to raw string if JSON.parse fails).
//
// Reconnect strategy (T-5-11):
//   - On `onerror` or unexpected close, schedule a reconnect.
//   - Backoff = min(base * 2^attempt, cap) with base=3s, cap=30s.
//   - Resets to base on the first successful `onopen` after a reconnect.
//
// This helper is NOT a useEffect — the caller decides when to mount
// (see useSkillReload.ts for the React-bound lifecycle).
//
// Why not WebSocket: per the plan's "Avoid" list, WebSocket is
// explicitly NOT used here. EventSource is the browser-standard SSE
// primitive; wrapping it with backoff is a few dozen lines.
export interface SSEClientOptions {
  /** Path appended to the current origin (e.g. `/api/skills/events`). */
  path: string;
  /** Invoked with the parsed JSON payload on every `message` event. */
  onMessage: (payload: unknown) => void;
  /** Invoked when the EventSource opens (after reconnect or initial). */
  onOpen?: () => void;
  /** Invoked on `error` or unexpected close — useful for telemetry. */
  onError?: (event: Event) => void;
  /** Base reconnect delay in ms (default 3000). */
  baseDelayMs?: number;
  /** Maximum reconnect delay in ms (default 30000). */
  maxDelayMs?: number;
}

export interface SSEConnection {
  /** Open the EventSource. Idempotent — calling twice closes the prior. */
  connect: () => void;
  /** Close the EventSource and cancel any pending reconnect. */
  close: () => void;
}

const DEFAULT_BASE_MS = 3000;
const DEFAULT_MAX_MS = 30000;

export function constructSSE(opts: SSEClientOptions): SSEConnection {
  const baseDelayMs = opts.baseDelayMs ?? DEFAULT_BASE_MS;
  const maxDelayMs = opts.maxDelayMs ?? DEFAULT_MAX_MS;

  let es: EventSource | null = null;
  let closed = false;
  let attempt = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  function clearReconnect() {
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function scheduleReconnect() {
    if (closed) return;
    const delay = Math.min(baseDelayMs * Math.pow(2, attempt), maxDelayMs);
    attempt += 1;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      open();
    }, delay);
  }

  function open() {
    if (closed) return;
    if (typeof window === 'undefined' || typeof EventSource === 'undefined') {
      // jsdom may not implement EventSource; degrade gracefully by
      // never opening. The caller (useSkillReload) handles the
      // absence by relying on its polling fallback.
      return;
    }
    // Idempotent connect: close any prior EventSource so we never
    // leak subscribers. This matches the React mount/unmount
    // contract — a fast re-mount (StrictMode, HMR) should not stack
    // EventSources.
    if (es) {
      es.close();
      es = null;
    }
    try {
      es = new EventSource(opts.path);
      es.onopen = () => {
        // Successful open — reset backoff counter.
        attempt = 0;
        opts.onOpen?.();
      };
      es.onmessage = (ev: MessageEvent<string>) => {
        const raw = ev.data;
        try {
          const parsed = JSON.parse(raw) as unknown;
          opts.onMessage(parsed);
        } catch {
          // Non-JSON payload — surface raw string to caller.
          opts.onMessage(raw);
        }
      };
      es.onerror = (event: Event) => {
        opts.onError?.(event);
        // Close + schedule reconnect.
        if (es) {
          es.close();
          es = null;
        }
        scheduleReconnect();
      };
    } catch {
      // Construction threw (rare; EventSource ctor can throw on
      // invalid URL). Schedule a reconnect rather than dying.
      scheduleReconnect();
    }
  }

  function close() {
    closed = true;
    clearReconnect();
    if (es) {
      es.close();
      es = null;
    }
  }

  return { connect: open, close };
}
