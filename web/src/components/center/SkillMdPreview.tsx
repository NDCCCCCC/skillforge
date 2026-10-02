// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SkillMdPreview — center-panel SKILL.md preview. Wave 3 reads the
// selected Skill from skills.json (via Vite JSON import) and renders
// the 7-field metadata block per .planning/SKILL_SCHEMA.md.
//
// When `skillId` is not in the fixture, render the defensive empty-
// state copy `当前 Skill 没有 SKILL.md 文件。` per 04-UI-SPEC
// §Copywriting Contract (Defensive copy for malformed fixture).
import skillsData from '../../__fixtures__/skills.json';

interface Guardrails {
  prohibitions: string[];
  required_refusal_phrases: string[];
}

interface SkillFixtureEntry {
  id: string;
  name: string;
  description: string;
  agent: string;
  reads: string[];
  writes: string[];
  temperature: number;
  steps: string[];
  guardrails: Guardrails;
}

const SKILLS: Record<string, SkillFixtureEntry> = (skillsData as SkillFixtureEntry[]).reduce(
  (acc, skill) => {
    acc[skill.id] = skill;
    return acc;
  },
  {} as Record<string, SkillFixtureEntry>,
);

export interface SkillMdPreviewProps {
  skillId: string;
}

export default function SkillMdPreview({ skillId }: SkillMdPreviewProps) {
  const skill = SKILLS[skillId];

  if (!skill) {
    return (
      <div
        data-testid="skill-md-preview"
        data-skill-id={skillId}
        className="text-text-muted text-xs"
      >
        当前 Skill 没有 SKILL.md 文件。
      </div>
    );
  }

  const prohibitionList = skill.guardrails.prohibitions.join(', ');
  const refusalPhrases = skill.guardrails.required_refusal_phrases.join(', ');

  return (
    <div data-testid="skill-md-preview" data-skill-id={skillId} className="text-text">
      <div className="mb-3">
        <h3 data-testid="skill-md-name" className="text-lg font-semibold">
          {skill.name}
        </h3>
        <p className="text-xs text-text-muted mt-1">{skill.description}</p>
      </div>

      <div className="mt-2 text-xs">
        <span className="text-text-dim">agent:</span>{' '}
        <span className="font-mono text-text">{skill.agent}</span>
      </div>

      <div className="mt-2 text-xs">
        <span className="text-text-dim">reads:</span>{' '}
        <span className="font-mono text-text">[{skill.reads.join(', ')}]</span>
      </div>

      <div className="mt-2 text-xs">
        <span className="text-text-dim">writes:</span>{' '}
        <span className="font-mono text-text">
          {skill.writes.length === 0 ? '[]' : `[${skill.writes.join(', ')}]`}
        </span>
      </div>

      <div className="mt-2 text-xs">
        <span className="text-text-dim">temperature:</span>{' '}
        <span className="font-mono text-text">{skill.temperature}</span>
      </div>

      <div className="mt-2 text-xs">
        <div className="text-text-dim">steps:</div>
        <ol className="list-decimal list-inside mt-1 space-y-1">
          {skill.steps.map((step, i) => (
            <li key={i} className="text-xs text-text">
              {step}
            </li>
          ))}
        </ol>
      </div>

      <div className="mt-2 text-xs">
        <span className="text-text-dim">prohibitions:</span>{' '}
        <span className="font-mono text-text">[{prohibitionList}]</span>
      </div>

      <div className="mt-2 text-xs">
        <span className="text-text-dim">required_refusal_phrases:</span>{' '}
        <span className="font-mono text-text">[{refusalPhrases}]</span>
      </div>
    </div>
  );
}
