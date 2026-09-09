---
phase: 31-fe-real-bugs-hostname-config-evalstatus-ready
plan: "01"
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/lib/sanitize-url.ts
  - web/src/__tests__/sanitize-url.test.ts
  - web/.env.example
autonomous: true
requirements:
  - REALBUG-01
  - REALBUG-02
must_haves:
  truths:
    - "sanitize-url hostname allowlist is configurable via VITE_ALLOWED_HOSTNAMES env var"
    - "Runtime hostname allowlist can be updated via setHostnameAllowlist()"
    - "evalStatus transitions to 'ready' after successful /api/eval/reports fetch"
  artifacts:
    - path: web/src/lib/sanitize-url.ts
      provides: configurable hostname allowlist
      exports: getHostnameAllowlist, setHostnameAllowlist, resetHostnameAllowlist, sanitizeSourceUrl
    - path: web/src/hooks/useEvalReports.test.ts
      provides: evalStatus state transition coverage
      lines: 60+
    - path: scripts/verify-phase-31.sh
      provides: atomic verification gate
---

<objective>
Fix two production P0 frontend bugs: REALBUG-01 (hostname allowlist hardcoded) and REALBUG-02 (evalStatus never transitions to ready).
</objective>

<context>
@web/src/lib/sanitize-url.ts (current ALLOWED_HOSTNAMES const, lines 22-29)
@web/src/__tests__/sanitize-url.test.ts (existing 7-case suite)
@web/src/hooks/useEvalReports.ts (fetchOnce success path missing setEvalStatus('ready'), line 59)
@web/src/store/app-store.ts (setEvalStatus action at line 546, implementation at line 879)
@web/.env.example (add VITE_ALLOWED_HOSTNAMES)
</context>

<tasks>

<task type="auto">
  <name>Task 1: REALBUG-01 — Make sanitize-url hostname allowlist configurable</name>
  <files>web/src/lib/sanitize-url.ts, web/src/__tests__/sanitize-url.test.ts, web/.env.example</files>
  <action>
## sanitize-url.ts changes (web/src/lib/sanitize-url.ts)

Replace the module-level constant block (lines 22-29) with a mutable module variable plus accessor/mutator functions:

```typescript
const DEFAULT_HOSTNAMES = ['localhost', '127.0.0.1', '::1', 'spike.local', 'skillforge.local'] as const;

let _hostnameAllowlist = new Set([...DEFAULT_HOSTNAMES]);

export function getHostnameAllowlist(): Set<string> {
  return _hostnameAllowlist;
}

export function setHostnameAllowlist(hosts: string[]): void {
  _hostnameAllowlist = new Set(hosts);
}

export function resetHostnameAllowlist(): void {
  _hostnameAllowlist = new Set([...DEFAULT_HOSTNAMES]);
}

// Vitest-only test hook — not callable in production bundles.
export function __setHostnameAllowlistForTesting(hosts: string[]): void {
  if (import.meta.env.VITEST) {
    _hostnameAllowlist = new Set(hosts);
  }
}
```

At module init, read `VITE_ALLOWED_HOSTNAMES` from env and override the default if present:

```typescript
// At end of module (before any exported function), after the helper definitions:
// Expand default allowlist from env if set.
const _envHostnames = import.meta.env.VITE_ALLOWED_HOSTNAMES;
if (typeof _envHostnames === 'string' && _envHostnames.length > 0) {
  _hostnameAllowlist = new Set(_envHostnames.split(',').map((h: string) => h.trim()));
}
```

Update the two call sites inside `sanitizeSourceUrl` (lines 47 and 50) to use `getHostnameAllowlist()` instead of the old `ALLOWED_PROTOCOLS`/`ALLOWED_HOSTNAMES` constants:
- `if (!ALLOWED_PROTOCOLS.has(url.protocol))` → `if (!new Set(['https:', 'http:']).has(url.protocol))`
- `if (!ALLOWED_HOSTNAMES.has(url.hostname))` → `if (!getHostnameAllowlist().has(url.hostname))`

## sanitize-url.test.ts additions (web/src/__tests__/sanitize-url.test.ts)

Add three new `it` cases inside the existing `describe` block (after the existing 7 cases, before the closing `});`):

```typescript
  it('uses the default allowlist when VITE_ALLOWED_HOSTNAMES is not set', async () => {
    // Ensure default list is active (default module state after import).
    const result = sanitizeSourceUrl('http://skillforge.local/foo');
    expect(result.ok).toBe(true);
  });

  it('overrides the allowlist when VITE_ALLOWED_HOSTNAMES env is set', async () => {
    // Simulate env override by calling the test hook.
    const { __setHostnameAllowlistForTesting } = await import('../lib/sanitize-url');
    __setHostnameAllowlistForTesting(['custom.company.com']);

    const result = sanitizeSourceUrl('http://custom.company.com/foo');
    expect(result.ok).toBe(true);

    const blocked = sanitizeSourceUrl('http://skillforge.local/foo');
    expect(blocked.ok).toBe(false);
    expect(blocked.reason).toBe('blocked_hostname');

    // Restore default.
    const { resetHostnameAllowlist } = await import('../lib/sanitize-url');
    resetHostnameAllowlist();
  });

  it('setHostnameAllowlist and resetHostnameAllowlist work at runtime', async () => {
    const { setHostnameAllowlist, resetHostnameAllowlist, sanitizeSourceUrl: sanitize } = await import('../lib/sanitize-url');

    setHostnameAllowlist(['ephemeral.local']);
    expect(sanitize('http://ephemeral.local/foo').ok).toBe(true);
    expect(sanitize('http://skillforge.local/foo').ok).toBe(false);

    resetHostnameAllowlist();
    expect(sanitize('http://skillforge.local/foo').ok).toBe(true);
  });
```

## .env.example addition (web/.env.example)

Add at end of file (before or after the VITE_REAL_BACKEND block):

```bash
# ---------------------------------------------------------------------------
# Source-URL sanitization allowlist (T-1-08)
# ---------------------------------------------------------------------------
# Comma-separated list of allowed hostnames for citation chip hrefs.
# Defaults to localhost/127.0.0.1/::1/spike.local/skillforge.local if unset.
# Production deployments must set this to match their backend domain.
# Example: VITE_ALLOWED_HOSTNAMES=localhost,127.0.0.1,::1,yourapp.company.com
# VITE_ALLOWED_HOSTNAMES=
```
</action>
  <verify>
    <automated>cd web && pnpm vitest run src/__tests__/sanitize-url.test.ts --reporter=verbose 2>&1 | tail -20</automated>
  </verify>
  <done>
sanitize-url.ts exports getHostnameAllowlist/setHostnameAllowlist/resetHostnameAllowlist/__setHostnameAllowlistForTesting; VITE_ALLOWED_HOSTNAMES overrides defaults on init; all 10 test cases pass (7 original + 3 new); .env.example documents VITE_ALLOWED_HOSTNAMES; no `ALLOWED_HOSTNAMES = new Set(` hardcoded re-introduction
  </done>
</task>

<task type="auto">
  <name>Task 2: REALBUG-02 — Add setEvalStatus('ready') to useEvalReports success path</name>
  <files>web/src/hooks/useEvalReports.ts, web/src/hooks/useEvalReports.test.ts</files>
  <action>
## useEvalReports.ts change (web/src/hooks/useEvalReports.ts)

In the `fetchOnce` function (around line 57-59), after `setIndexReports(data.runs ?? [])` and `setEvalError(null)`, insert:

```typescript
setEvalStatus('ready');
```

Specifically, the success block (lines 56-59 currently) should read:

```typescript
const data = (await resp.json()) as EvalIndex;
setIndexReports(data.runs ?? []);
setEvalError(null);
setEvalStatus('ready');
```

Do NOT modify `EvalDashboard.tsx` or any UI component — the functional fix is complete in the hook. The three-state UI (loading/error/ready) will be addressed in a future phase.

## New test file (web/src/hooks/useEvalReports.test.ts)

Create this file with full coverage of evalStatus transitions:

```typescript
// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useEvalReports } from '../hooks/useEvalReports';
import * as appStore from '../store/app-store';

// Mock the store to isolate the hook's status transitions.
vi.mock('../store/app-store', () => ({
  useAppStore: vi.fn(),
}));

const mockSetEvalStatus = vi.fn();
const mockSetEvalError = vi.fn();
const mockSetIndexReports = vi.fn();

function setupMockStore(initialStatus = 'idle') {
  (appStore.useAppStore as ReturnType<typeof vi.fn>).mockImplementation(
    (selector: (s: Record<string, unknown>) => unknown) => {
      const state: Record<string, unknown> = {
        setEvalStatus: mockSetEvalStatus,
        setEvalError: mockSetEvalError,
        setIndexReports: mockSetIndexReports,
        evalStatus: initialStatus,
        evalError: null,
      };
      return selector(state);
    }
  );
}

describe('useEvalReports — evalStatus transitions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupMockStore('idle');
    // Mock fetch globally.
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('sets evalStatus to ready after successful /api/eval/reports response', async () => {
    const mockData = { runs: [{ id: 'run-1', skill: 'compliance', createdAt: '2026-09-01', scores: {} }] };
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockData,
    } as Response);

    const { result } = renderHook(() => useEvalReports(30_000));

    // Wait for the initial fetch to resolve.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    expect(mockSetEvalStatus).toHaveBeenCalledWith('loading');
    expect(mockSetEvalStatus).toHaveBeenCalledWith('ready');
    expect(mockSetIndexReports).toHaveBeenCalledWith(mockData.runs);
    expect(mockSetEvalError).not.toHaveBeenCalled();
  });

  it('does NOT set evalStatus to ready on 503 response', async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => ({ error: 'eval not initialized' }),
    } as Response);

    renderHook(() => useEvalReports(30_000));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    const readyCalls = mockSetEvalStatus.mock.calls.filter(([s]) => s === 'ready');
    expect(readyCalls).toHaveLength(0);
    expect(mockSetEvalError).toHaveBeenCalled();
  });

  it('does NOT set evalStatus to ready when fetch throws', async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('network error'));

    renderHook(() => useEvalReports(30_000));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });

    const readyCalls = mockSetEvalStatus.mock.calls.filter(([s]) => s === 'ready');
    expect(readyCalls).toHaveLength(0);
    expect(mockSetEvalError).toHaveBeenCalled();
  });
});
```

Note: The test uses `@testing-library/react` for `renderHook`. If that import is not yet in the web package dependencies, add it via `pnpm add -D @testing-library/react` before running.
</action>
  <verify>
    <automated>cd web && pnpm vitest run src/hooks/useEvalReports.test.ts --reporter=verbose 2>&1 | tail -20</automated>
  </verify>
  <done>
useEvalReports.ts line 59+ calls setEvalStatus('ready') after successful JSON parse; useEvalReports.test.ts exists with 3 passing cases (success→ready, 503→no-ready, throw→no-ready); EvalDashboard.tsx unchanged
  </done>
</task>

</tasks>

<verification>
## scripts/verify-phase-31.sh

Create `scripts/verify-phase-31.sh` (executable, bash shebang) with these four checks:

```bash
#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/.."

echo "=== Phase 31 verification ==="

echo "[1/4] vitest — sanitize-url tests"
cd web && pnpm vitest run src/lib/sanitize-url.test.ts src/hooks/useEvalReports.test.ts --reporter=verbose

echo "[2/4] tsc --noEmit"
pnpm tsc --noEmit

echo "[3/4] grep guard — no hardcoded ALLOWED_HOSTNAMES re-introduction"
COUNT=$(grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" web/src | grep -v node_modules | wc -l)
if [ "$COUNT" -ne 0 ]; then
  echo "FAIL: found $COUNT hardcoded ALLOWED_HOSTNAMES = new Set[ occurrences"
  grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" web/src | grep -v node_modules
  exit 1
fi
echo "PASS: no hardcoded ALLOWED_HOSTNAMES"

echo "[4/4] grep guard — setEvalStatus('ready') is present"
COUNT=$(grep -rn "setEvalStatus('ready')" web/src | grep -v node_modules | wc -l)
if [ "$COUNT" -lt 1 ]; then
  echo "FAIL: setEvalStatus('ready') not found"
  exit 1
fi
echo "PASS: setEvalStatus('ready') found in $COUNT location(s)"

echo "=== ALL CHECKS PASSED ==="
```

Make it executable: `chmod +x scripts/verify-phase-31.sh`
</verification>

<success_criteria>
- Task 1 commit: sanitize-url.ts configurable allowlist + 3 new test cases + .env.example entry
- Task 2 commit: useEvalReports.ts setEvalStatus('ready') + new useEvalReports.test.ts with 3 cases
- Both test files pass: `pnpm vitest run src/lib/sanitize-url.test.ts src/hooks/useEvalReports.test.ts`
- `pnpm tsc --noEmit` passes with zero errors
- `grep -rn "ALLOWED_HOSTNAMES\s*=\s*new Set\[" web/src | wc -l` == 0
- `grep -rn "setEvalStatus('ready')" web/src | wc -l` >= 1
</success_criteria>

<output>
Create `.planning/phases/31-fe-real-bugs-hostname-config-evalstatus-ready/31-01-SUMMARY.md` when done.
</output>
