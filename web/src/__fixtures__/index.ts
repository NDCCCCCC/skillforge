// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Fixture registry — single entry point for all UIMessageChunk fixtures
// used by the MockChatTransport in dev/test. Task 2 populates this with
// 10 fixtures (4 verdicts + 3 refusals + multi-part + sub-progress +
// citation-tiered); Task 1 stub ships an empty registry so the wire
// contract test compiles while the fixtures are still being authored.
//
// The registry is keyed by MockFixtureName (string union) so tests can
// pick a specific fixture set. The MockChatTransport defaults to
// 'multi-part' for the richest D-05 coverage.
import type { SkillForgeChunk } from '../types/wire-types';
import compliantFixture from './verdict-compliant.json';
import needsInfoFixture from './verdict-needs-info.json';
import nonCompliantFixture from './verdict-non-compliant.json';
import unansweredFixture from './verdict-unanswered.json';
import refusalOutOfScopeFixture from './refusal-out-of-scope.json';
import refusalNoSourceFixture from './refusal-no-source.json';
import refusalCitationMismatchFixture from './refusal-citation-mismatch.json';
import multiPartFixture from './multi-part.json';
import subProgressFixture from './sub-progress.json';
import citationTieredFixture from './citation-tiered.json';
import skillsFixture from './skills.json';
import chunksFixture from './chunks.json';

export interface SkillFixture {
  id: string;
  name: string;
  description: string;
  reads: string[];
}

export interface ChunkFixtureEntry {
  tier: '权威' | '历史' | '个人' | '草稿';
  version: string;
  effective_date: string;
  text: string;
}

export type ChunksFixtureMap = Record<string, ChunkFixtureEntry>;

export type MockFixtureName =
  | 'verdict-compliant'
  | 'verdict-needs-info'
  | 'verdict-non-compliant'
  | 'verdict-unanswered'
  | 'refusal-out-of-scope'
  | 'refusal-no-source'
  | 'refusal-citation-mismatch'
  | 'multi-part'
  | 'sub-progress'
  | 'citation-tiered';

export const FIXTURE_REGISTRY: Record<MockFixtureName, SkillForgeChunk[]> = {
  'verdict-compliant': compliantFixture as SkillForgeChunk[],
  'verdict-needs-info': needsInfoFixture as SkillForgeChunk[],
  'verdict-non-compliant': nonCompliantFixture as SkillForgeChunk[],
  'verdict-unanswered': unansweredFixture as SkillForgeChunk[],
  'refusal-out-of-scope': refusalOutOfScopeFixture as SkillForgeChunk[],
  'refusal-no-source': refusalNoSourceFixture as SkillForgeChunk[],
  'refusal-citation-mismatch': refusalCitationMismatchFixture as SkillForgeChunk[],
  'multi-part': multiPartFixture as SkillForgeChunk[],
  'sub-progress': subProgressFixture as SkillForgeChunk[],
  'citation-tiered': citationTieredFixture as SkillForgeChunk[],
};

/** Default fixture for dev mode — covers D-05 diversity. */
export const DEFAULT_FIXTURE: MockFixtureName = 'multi-part';

/** Skill list fixture for Wave 3's skill picker (FEUI-04). */
export const SKILLS_FIXTURE: SkillFixture[] = skillsFixture as SkillFixture[];

/** Chunk verbatim map for Wave 3's ChunkSourceView. */
export const CHUNKS_FIXTURE: ChunksFixtureMap = chunksFixture as ChunksFixtureMap;
