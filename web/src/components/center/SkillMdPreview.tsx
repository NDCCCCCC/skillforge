// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// SkillMdPreview — center-panel SKILL.md preview placeholder.
//
// Wave 2 ships the placeholder body; Wave 3 fills it from the
// skills.json fixture via the skill picker.
export interface SkillMdPreviewProps {
  skillId: string;
}

export default function SkillMdPreview({ skillId }: SkillMdPreviewProps) {
  return (
    <pre
      data-testid="skill-md-preview"
      className="whitespace-pre-wrap font-mono text-xs text-text"
    >{`[SKILL.md for ${skillId} — populated when skill picker is wired in Wave 3]`}</pre>
  );
}
