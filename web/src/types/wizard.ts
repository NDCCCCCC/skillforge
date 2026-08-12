// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Plan 05-04 Task 2 — wizard type module. Mirrors the Go
// internal/skill/wizard.go Answers struct (T-2-30/V5 ASVS: drift between
// the Go JSON tags + the TS wire types is the canonical source of FE↔BE
// bugs; this file is the single TS mirror that the wizard components
// import from).
//
// SkillType is the discriminator for the D5-12 temperature recommendation
// (compliance -> 0.0, qa -> 0.2, creative -> 0.7). User-supplied
// Temperature overrides the recommendation when non-null.

export interface WizardAnswers {
  /** First-sentence purpose; the BE slugifies the first sentence to
   * derive the Skill directory name (T-5-16 path-traversal gate). */
  purpose: string;
  /** Read tiers / glob paths the Skill needs (authority / history /
   * personal / drafts). */
  reads: string[];
  /** Compliance | qa | creative (drives the type-default temperature). */
  skillType: 'compliance' | 'qa' | 'creative' | string;
  /** Free-form additional prohibitions / guardrails text. */
  guardrails: string;
  /** Optional user-supplied temperature override; null = use type-default
   * (D5-12). Range is sanitized to [0.0, 2.0] BE-side on commit. */
  temperature?: number | null;
}

// D5-12 — type-default temperature. Single source of truth; useWizard
// consults this when the user does not supply an explicit value.
export const DEFAULT_TEMPERATURE_BY_TYPE: Record<string, number> = {
  compliance: 0.0,
  qa: 0.2,
  creative: 0.7,
};

/** A single interview step. The order in WIZARD_STEPS defines the order
 * the user walks through them; prompts are copy-pasted verbatim from
 * 05-UI-SPEC §SC-3. */
export interface WizardStep {
  /** Stable id used in data-* attributes for tests. */
  id: 'purpose' | 'reads' | 'type' | 'guardrails' | 'temperature';
  /** Verbatim prompt copy (Chinese; do not edit without UI-SPEC approval). */
  prompt: string;
  /** Whether the step is skippable. Today all steps are required. */
  isOptional: boolean;
}

// Order is load-bearing — useWizard indexes WIZARD_STEPS[currentStep]
// to render the next prompt.
export const WIZARD_STEPS: readonly WizardStep[] = [
  {
    id: 'purpose',
    prompt: '请描述这个 Skill 要解决的业务问题（一句话）',
    isOptional: false,
  },
  {
    id: 'reads',
    prompt: '它需要读取哪些知识层？可选：权威/历史/个人/草稿',
    isOptional: false,
  },
  {
    id: 'type',
    prompt: '这是合规审查类、通用问答类、还是创意类 Skill？',
    isOptional: false,
  },
  {
    id: 'guardrails',
    prompt: '需要哪些禁止事项？',
    isOptional: false,
  },
  {
    id: 'temperature',
    prompt: '确认温度（按回车使用类型默认值，合规 0.0 / QA 0.2 / 创意 0.7）',
    isOptional: true,
  },
] as const;

/** BE response shape — mirrors internal/server/skills.go wizardResponse. */
export interface WizardPreviewResponse {
  /** The generated 7-field Manifest (already passed Validate BE-side). */
  preview?: {
    description: string;
    agent: string;
    reads: string[];
    writes: string[];
    steps: string;
    guardrails: {
      prohibitions: string[];
      required_refusal_phrases: string[];
      tier_limits?: Record<string, string[]>;
    };
    temperature?: number;
  };
  /** Slugified directory name (BE-derived from the purpose's first sentence). */
  name: string;
  /** Non-empty on any failure (validation failed twice, LLM error, name
   * extraction error, etc.). FE renders this inline per UI-SPEC §SC-3. */
  error?: string;
}

/** BE response shape for POST /api/skills/wizard/commit. */
export interface WizardCommitResponse {
  /** True when the atomic write succeeded (the file is on disk). */
  ok: boolean;
  /** Absolute path of the written file. */
  path: string;
  /**
   * True when the new Skill is also confirmed registered in the Loader's
   * rescanned cache — i.e. GET /api/skills now lists it WITHOUT a server
   * restart. The FE MUST gate its "已注册并热加载" success toast on this
   * flag (NOT on `ok`) so the toast text is truthful by construction
   * (UAT Test 9 regression).
   */
  registered?: boolean;
  /** Post-commit Skill count (the FE may use it for telemetry / subtitles). */
  skillsTotal?: number;
  /** Non-empty on any failure (write error, validation, registration not confirmed). */
  error?: string;
}