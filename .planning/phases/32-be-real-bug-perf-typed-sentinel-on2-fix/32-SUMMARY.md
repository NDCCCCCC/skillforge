# Phase 32: BE Real Bug + Performance — Typed Sentinel + O(n²) Fix - Summary

**Closed:** 2026-09-09
**Status:** ✅ Complete

## Goal

Fix backend production bug (string-substring error matching) and a performance regression (O(n²) string concat in hot path).

## Scope

| REQ-ID | Description | Status |
|--------|-------------|--------|
| **REALBUG-03** | `isBackendUnavailable` 改 typed sentinel（`errors.Is` / `errors.As` / `net.Error.Timeout()`） | ✅ Fixed |
| **PERF-01** | `chat.go:joinStrings` O(n²) → `strings.Join(parts, "")` | ✅ Fixed |

## Commits

| Hash | Message |
|------|---------|
| `baf1ef0` | fix(32): isBackendUnavailable typed sentinel + joinStrings O(n²) → strings.Join |
| `0036f2b` | chore(phase-32): add verify-phase-32.sh hermetic gate |

## Changes

### internal/server/upload.go
- **Imports:** +`errors` +`net` +`syscall`
- **isBackendUnavailable** (line 445):
  - **Before:** 4× `strings.Contains(err.Error(), ...)` substring matches
  - **After:**
    - `errors.As` for `*net.DNSError` (DNS resolution failure)
    - `errors.Is` for `syscall.ECONNREFUSED` / `EHOSTUNREACH` / `ENETUNREACH` (connection-level errors)
    - `errors.Is` for `context.DeadlineExceeded` / `context.Canceled`
    - `errors.As` + `net.Error.Timeout()` for generic timeout (covers `*net.OpError` wrapping)

### internal/server/chat.go
- **joinStrings function** removed (lines 1042-1050)
- Call site (line 1039): `joinStrings(content)` → `strings.Join(content, "")` — same O(n), no behavior change

### internal/server/upload_test.go
- +7 unit tests covering nil / generic error / connection refused (raw + wrapped) / DNS / context deadline (raw + wrapped) / context canceled / net.Error timeout

### scripts/verify-phase-32.sh (new)
- 4-stage hermetic gate:
  1. `joinStrings` function/calls removed (PERF-01)
  2. `isBackendUnavailable` uses typed sentinels, no `strings.Contains` on `err.Error()`
  3. ~~err.Error() substring matching banned~~ (removed: out of Phase 32 scope — other valid uses exist)
  4. `go build ./internal/server/...` succeeds

## Verify Results

```
[INFO]  Stage 1: joinStrings function/calls removed (comments referencing it are fine)
[PASS]  Stage 2: isBackendUnavailable uses typed sentinels only
[INFO]  Stage 4: go build ./internal/server/...
[PASS]  go build ./internal/server/... succeeds
=== ALL CHECKS PASSED ===
```

## Trust Semantics Preserved

- `isBackendUnavailable` 行为不变（covered 4 原 cases + 新增 net.DNSError + net.Error.Timeout 兜底）
- `strings.Join(content, "")` 与原 `joinStrings` 字节级等价（顺序拼接无 separator）
- 无 audit / chunk 流改动

## Out of Scope

- 其他 `strings.Contains(err.Error(), ...)` 调用（`approval_rules_api.go:238` / `upload.go:319`）是业务逻辑 substring 检测，非网络错误，归 v1.4 ROBUST-06 范围
