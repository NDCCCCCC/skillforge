// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// 49-02 (D-08): tests for the stub-mode annotation on the Verdict type —
// VERDICT_MODES / VerdictMode / Verdict.mode / isStubVerdict. Mirrors the
// Go-side internal/verdict verdict.Mode + ModeStub.
import { describe, expect, it } from 'vitest';
import {
  COMPLIANCE_STATUSES,
  VERDICT_MODES,
  isRefusal,
  isStubVerdict,
  isVerdict,
  type Verdict,
} from './verdict';

const realVerdict: Verdict = {
  status: 'ok',
  confidence: 0.97,
  citations: [{ chunk_id: '7', span: '由部门负责人审批', tier: 'authority' }],
  review_required: false,
  answer: '由部门负责人审批',
};

const stubVerdict: Verdict = {
  status: 'ok',
  confidence: 0.5,
  citations: [{ chunk_id: '7', span: '由部门负责人审批', tier: 'authority' }],
  review_required: true,
  answer: '由部门负责人审批',
  mode: 'stub',
};

describe('verdict stub mode (49-02 D-08)', () => {
  it('VERDICT_MODES contains only "stub" and Verdict.mode accepts it', () => {
    expect(VERDICT_MODES).toEqual(['stub']);
    expect(stubVerdict.mode).toBe('stub');
  });

  it('real-model verdicts have no mode field (wire backward-compat)', () => {
    expect(realVerdict.mode).toBeUndefined();
    expect('mode' in realVerdict).toBe(false);
  });

  it('isStubVerdict is true only for mode:"stub" verdicts', () => {
    expect(isStubVerdict(stubVerdict)).toBe(true);
    expect(isStubVerdict(realVerdict)).toBe(false);
  });

  it('isVerdict still accepts verdicts with and without mode (guard unchanged)', () => {
    expect(isVerdict(stubVerdict)).toBe(true);
    expect(isVerdict(realVerdict)).toBe(true);
    // Refusal payloads are still not verdicts.
    expect(isVerdict({ reason: 'out_of_scope', can_user_rephrase: true })).toBe(false);
    expect(isRefusal({ reason: 'out_of_scope', can_user_rephrase: true })).toBe(true);
  });

  it('isStubVerdict rejects non-verdict garbage', () => {
    expect(isStubVerdict(null)).toBe(false);
    expect(isStubVerdict({ mode: 'stub' })).toBe(false); // no status ⇒ not a verdict
    expect(isStubVerdict({ status: 'ok', mode: 'real' })).toBe(false); // mode not in VERDICT_MODES semantics
  });

  it('COMPLIANCE_STATUSES untouched (existing enum intact)', () => {
    expect(COMPLIANCE_STATUSES).toEqual(['compliant', 'non_compliant', 'not_applicable']);
  });
});
