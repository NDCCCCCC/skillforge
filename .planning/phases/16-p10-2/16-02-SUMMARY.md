---
phase: 16-p10-2
plan: 02
status: done
commit_task1: 22bb646
commit_task2: 41332d3
requirements: [DATA-07, DATA-08]
---

# Plan 16-02 Summary: reads channel vocabulary + XOR enforcement + data+deep rejection

## Tasks Completed

### Task 1: manifest.go channel vocabulary + HasDataChannel() + tier.go ResolveTiers defense branch

**Commit:** `22bb646`

**Files changed:**
- `internal/skill/manifest.go` -- Added `ReadDataAggregatedOnly` constant, `ChannelKind` type with `ChannelKindTier`/`ChannelKindData`, `HasDataChannel()` method (broad "data:" prefix detection), `Channel()` method
- `internal/skill/tier.go` -- Added defense-in-depth guard at ResolveTiers entry: `strings.HasPrefix(r, "data:")` returns error citing D16-C2
- `internal/skill/manifest_test.go` -- Added `TestManifestChannel_DataChannel`, `TestManifestChannel_DataChannel_DetectsAnyDataPrefix`, `TestManifestChannel_TierChannel`, `TestManifestChannel_EmptyReads`, `TestResolveTiers_DataPrefixDefenseInDepth`, `TestResolveTiers_ExistingTierBehavior_Unchanged`

**Verification:** `go test ./internal/skill/ -run 'TestManifestChannel|TestResolveTiers' -count=1 -v` -- PASS (all cases)

### Task 2: validate.go XOR enforcement + data+deep rejection + TestChannelGuardXOR

**Commit:** `41332d3`

**Files changed:**
- `internal/skill/validate.go` -- Inserted channel-kind branching in Validate() (check 3a): (a) XOR: len(reads)>1 with data channel rejects, naming both conflict values; (b) unknown data:* mode rejection (only ReadDataAggregatedOnly allowed); (c) deep+data rejection (deep_agent.go:171-174 cited); (d) ResolveTiers skipped for data channel. Validation lives in Validate() body (not Scan) so reloadOne auto-covers hot-reload window (Pitfall 6).
- `internal/skill/validate_test.go` -- Added `TestChannelGuardXOR` (6 table-driven sub-cases: mixed data+tier, pure data passes, unknown data mode, deep+data, reverse mixed, existing tier regression), `TestChannelGuardXOR_PureDataChannel_SkipsResolveTiers`, `TestChannelGuardXOR_ReloadOne_HotReloadWindow`

**Verification:** `go test ./internal/skill/ -count=1 -v` -- PASS (all cases including reloadOne)

## Acceptance Criteria Verification

| Criterion | Status | Evidence |
|-----------|--------|----------|
| `manifest.go` contains `ReadDataAggregatedOnly = "data:aggregated_only"` | PASS | Line 142 |
| `manifest.go` contains `func (m *Manifest) HasDataChannel()` | PASS | Line 173 |
| `manifest.go` contains `ChannelKindData` | PASS | Line 157 |
| `tier.go` ResolveTiers has `strings.HasPrefix "data:"` defense branch | PASS | Lines 130-136 |
| ResolveTiers rejects `"data:aggregated_only"` input | PASS | TestResolveTiers_DataPrefixDefenseInDepth |
| `data:external:authorized` not in any allowlist | PASS | validate.go line 100 rejects unknown data: prefixes |
| validate.go XOR branch uses `HasDataChannel()` | PASS | Line 107 |
| validate.go contains deep+data rejection citing deep_agent.go | PASS | Lines 120-125 |
| String literal `data:aggregated_only` in validate.go only in error messages (decisions via constant) | PASS | `grep -c` returns 0 raw literals; all references use `ReadDataAggregatedOnly` |
| TestChannelGuardXOR 7 sub-cases (6 table + SkipsResolveTiers + ReloadOne) | PASS | All pass |
| TestChannelGuardXOR case 1 names both conflict values | PASS | Substrings checked: `ReadDataAggregatedOnly`, `"authority/**"`, `"mutually exclusive"` |
| TestChannelGuardXOR case 6 (reloadOne) uses real Registry | PASS | Creates temp dir + skill dir, loads via New(), modifies file, calls Reload() |
| `go test ./internal/skill/ -count=1` full suite exit 0 | PASS | 7.594s |
| `go test ./internal/agent/ -count=1` downstream no regression | PASS | 4.871s |
| Existing TestResolveTiers cases zero regression | PASS | All existing tier paths byte-identical |

## Threat Model Coverage

| Threat ID | Mitigation | Test Lock |
|-----------|------------|-----------|
| T-16-02-01 (Elevation: cross-channel mixing) | Validate() XOR rejects both directions | TestChannelGuardXOR cases 1+5 |
| T-16-02-02 (Elevation: hot-reload bypass) | Validation in Validate() body, reloadOne re-runs parseAndValidate | TestChannelGuardXOR_ReloadOne_HotReloadWindow |
| T-16-02-03 (Tampering: unknown data:* mode) | Exact-match allowlist (only ReadDataAggregatedOnly); others rejected with v1.0 hint | TestChannelGuardXOR case 3 |
| T-16-02-04 (Tampering: data+deep prompt/tool mismatch) | Fail-closed rejection citing deep_agent.go:171-174 | TestChannelGuardXOR case 4 |
| T-16-02-SC (Supply chain) | Zero new dependencies | N/A |

## CGO Constraint

`internal/kpi` remains CGO-free (verified: no import of `internal/duckdb` or `go-duckdb` in production files; only comment references).
