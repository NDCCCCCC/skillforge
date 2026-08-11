---
name: spike-001-test
description: Spike 001 test skill — verifies temperature passthrough
agent: ChatModelAgent
reads: []
writes: []
temperature: 0
---

# Spike 001 Test Skill

Minimal SKILL.md with only the fields needed for H1 verification.
- name: spike-001-test
- description: placeholder
- agent: ChatModelAgent
- reads: []
- writes: []
- temperature: 0  ← this is the field under test

guardrails:
  prohibitions:
    - "placeholder"