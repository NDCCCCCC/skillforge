# eval/frozen — Frozen Baseline Locks

This directory holds the **frozen baselines** the acceptance gates
verify byte-identity against. The lock files are SACRED — do not edit
them in place; a drift in the lock file's sha256 breaks the
acceptance gate and forces an explicit rebaseline decision at the
milestone close.

## Files

| File | Phase | Purpose |
|------|-------|---------|
| `compliance.lock.json` | Phase 8 (SC-5) | Compliance golden set lock — model_ver + prompt_hash + seed for the 75-case `eval/golden/compliance.jsonl` |
| `hitl.lock.json` | Phase 11 (11-08) | HITL golden set lock — 75-case `eval/golden/hitl.jsonl` sha256 + per-category counts + acceptance-gate thresholds |

## Byte-Identity Verification

The acceptance gate (`scripts/verify-phase-11.sh` Stage 11) verifies
BOTH locks are present + readable:

```bash
sha256sum eval/frozen/compliance.lock.json eval/frozen/hitl.lock.json
```

The expected sha256 values are recorded in `.planning/STATE.md` at
the phase closeout; a drift between the on-disk sha256 and the
recorded pin means either (a) the lock file was edited in place —
FORBIDDEN — or (b) the phase is being rebaselined (operator-driven,
requires a milestone-close waiver).

## compliance.lock.json (Phase 8)

Schema (frozen at Phase 8 closeout; do not extend):

```json
{
  "model_ver": "glm-5.2",
  "prompt_hash": "<sha256 of rendered system prompt>",
  "seed": 42,
  "eval_set_version": "compliance-2026-08-13-v1",
  "skill_name": "compliance"
}
```

The Phase 11 acceptance gate (`cmd/spike-history --frozen --archive`)
reads this lock and computes its sha256 at run-time; the value is
recorded into `reports/frozen_lock_<date>.sha256` as ship evidence.

## hitl.lock.json (Phase 11)

Schema (v1, Phase 11 introduction):

```json
{
  "schema_version": "v1",
  "created_at": "2026-08-20",
  "locked_by": "Phase 11 acceptance gate (11-08)",
  "golden_sha256": "<sha256 of eval/golden/hitl.jsonl bytes>",
  "model_ver": "hitl-frozen-v1",
  "prompt_hash": "<sha256 of testdata/skills/hitl-smoke/SKILL.md>",
  "case_count": 75,
  "category_counts": { "should-interrupt": 20, "...": "..." },
  "gate_thresholds": { "...": "..." }
}
```

The `golden_sha256` field is the byte-identity anchor for
`eval/golden/hitl.jsonl`. A change to the golden set requires
updating `golden_sha256` AND `created_at` + `locked_by` in the same
commit — a silent golden edit surfaces as a lock mismatch in the
acceptance gate.

## Rebaseline Cadence

- **Phase 8 pin (`compliance.lock.json`)**: rebaselined only at v0.8
  milestone close. Phase 11–13 behavior changes must NOT touch this
  file.
- **Phase 11 pin (`hitl.lock.json`)**: rebaselined at Phase 11
  acceptance sign-off. Subsequent HITL changes (Phase 12 plan_approve,
  etc.) rebaseline at their own phase close.
