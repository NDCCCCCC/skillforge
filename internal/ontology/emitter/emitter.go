// Copyright 2026 SkillForge Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package emitter generates an RDFS/OWL TBox (Turtle) from SkillForge
// SKILL.md frontmatter and the KB directory tree.
//
// References:
//   - spike 005b (skillschema-rdfs-lift): reference algorithm.
//   - Phase 38 baseline: 6 Classes / 6 ObjectProperties / 8
//     DatatypeProperties, roundtrip lossless via tggo/goRDFlib.
//   - Phase 39 (D-TBox2-01): +NoteChunk / +TableSchema classes and
//     +belongsToNote / +referencesTable object properties for the ECL
//     pipeline ABox.
package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultNS is the default ontology IRI namespace prefix.
//
// All emitted URIs use `{ns}{localName}` form (e.g.,
// `http://skillforge.local/ontology#Skill`). Configurable via
// `SKILLFORGE_ONTOLOGY_NS` env.
const DefaultNS = "http://skillforge.local/ontology#"

// Skill describes a parsed SKILL.md frontmatter (the 7-field schema).
type Skill struct {
	Name           string   `yaml:"name"`
	Description    string   `yaml:"description"`
	Agent          string   `yaml:"agent"`
	Reads          []string `yaml:"reads"`
	Writes         []string `yaml:"writes"`
	Temperature    float64  `yaml:"temperature"`
	Prohibitions   []string `yaml:"prohibitions"`
	RefusalPhrases []string `yaml:"required_refusal_phrases"`
}

// KBCategory describes one tier directory under knowledge/.
type KBCategory struct {
	TierName      string
	SubCategories []string
}

// Emitter generates Turtle TBox content from SkillForge metadata.
type Emitter struct {
	SkillsRoot string
	KBRoot     string
	NS         string
}

// New constructs an Emitter with the given roots and namespace.
func New(skillsRoot, kbRoot, ns string) *Emitter {
	if ns == "" {
		ns = DefaultNS
	}
	return &Emitter{SkillsRoot: skillsRoot, KBRoot: kbRoot, NS: ns}
}

// Emit generates the full TBox Turtle content for the configured roots.
//
// The output:
//   - declares 8 owl:Class, 8 owl:ObjectProperty, 8 owl:DatatypeProperty,
//   - emits one individual per Skill (8 expected),
//   - emits one KBCategory per tier (5 expected) and one SubCategory per
//     sub-directory under sql-guides (7 expected).
//
// Byte-identical for identical inputs (deterministic ordering).
func (e *Emitter) Emit() (string, error) {
	skills, err := e.loadSkills()
	if err != nil {
		return "", fmt.Errorf("emitter: load skills: %w", err)
	}
	categories, err := e.loadKBCategories()
	if err != nil {
		return "", fmt.Errorf("emitter: load KB categories: %w", err)
	}
	return e.render(skills, categories), nil
}

// loadSkills walks {skillsRoot}/*/SKILL.md and parses the frontmatter.
//
// Skills are returned sorted by name for deterministic output.
func (e *Emitter) loadSkills() ([]Skill, error) {
	entries, err := os.ReadDir(e.SkillsRoot)
	if err != nil {
		return nil, fmt.Errorf("read skills root: %w", err)
	}
	var out []Skill
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		path := filepath.Join(e.SkillsRoot, ent.Name(), "SKILL.md")
		fm, err := parseFrontmatter(path)
		if err != nil {
			// Skip skills without SKILL.md or unparseable — not fatal.
			continue
		}
		if fm.Name == "" {
			fm.Name = ent.Name()
		}
		out = append(out, fm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// loadKBCategories walks {kbRoot}/* and records sub-directories of sql-guides.
func (e *Emitter) loadKBCategories() ([]KBCategory, error) {
	entries, err := os.ReadDir(e.KBRoot)
	if err != nil {
		return nil, fmt.Errorf("read KB root: %w", err)
	}
	var out []KBCategory
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		cat := KBCategory{TierName: ent.Name()}
		tierPath := filepath.Join(e.KBRoot, ent.Name())
		if ent.Name() == "sql-guides" {
			subEntries, err := os.ReadDir(tierPath)
			if err == nil {
				for _, sub := range subEntries {
					if sub.IsDir() {
						cat.SubCategories = append(cat.SubCategories, sub.Name())
					}
				}
				sort.Strings(cat.SubCategories)
			}
		}
		out = append(out, cat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TierName < out[j].TierName })
	return out, nil
}

// parseFrontmatter reads path and extracts the YAML block between leading/trailing `---` markers.
func parseFrontmatter(path string) (Skill, error) {
	var s Skill
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return s, fmt.Errorf("no frontmatter delimiter")
	}
	body := strings.TrimPrefix(text, "---")
	idx := strings.Index(body, "\n---")
	if idx < 0 {
		return s, fmt.Errorf("no closing frontmatter delimiter")
	}
	yamlBody := body[:idx]
	if err := yaml.Unmarshal([]byte(yamlBody), &s); err != nil {
		return s, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return s, nil
}

// render assembles the full Turtle document.
//
// Structure mirrors spike 005b's skillforge_tbox.ttl so the emitted TBox is
// directly comparable to the validated spike artifact (215 triples):
//
//   - 8 owl:Class (type + label + comment)
//   - 8 owl:ObjectProperty (type + domain + range + label)
//   - 8 owl:DatatypeProperty (type + domain + range + label)
//   - Tier individuals (skf:{tier}), KBCategory individuals (skf:cat_{tier})
//     linked via belongsToTier, SubCategory individuals (skf:sub_{tier}_{name})
//   - SkillAgent individuals per distinct agent type
//   - Skill individuals with hasReadAccess/hasWriteAccess tier-glob bindings
//   - RefusalPhrase individuals + skill→phrase requiresRefusal bindings
func (e *Emitter) render(skills []Skill, cats []KBCategory) string {
	var b strings.Builder

	// Prefixes.
	b.WriteString("@prefix rdf:  <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n")
	b.WriteString("@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .\n")
	b.WriteString("@prefix owl:  <http://www.w3.org/2002/07/owl#> .\n")
	b.WriteString("@prefix xsd:  <http://www.w3.org/2001/XMLSchema#> .\n")
	fmt.Fprintf(&b, "@prefix skf:  <%s> .\n\n", e.NS)

	// 8 owl:Class declarations (type + label + comment).
	classes := []struct{ Name, Label, Comment string }{
		{"Tier", "Tier", "Trust tier for KB content (权威/历史/个人/sql-guides/draft)."},
		{"KBCategory", "KB Category", "A top-level category within a Tier."},
		{"SubCategory", "SubCategory", "A sub-directory within a KB Category (e.g. 取数口径 / 操作手册 / 汽车险)."},
		{"Skill", "Skill", "A SkillForge skill — a self-contained agent workflow declared by SKILL.md."},
		{"SkillAgent", "Skill Agent Type", "The agent type that powers a Skill (ChatModelAgent, ReActAgent, Sequential, Parallel, Loop)."},
		{"RefusalPhrase", "Refusal Phrase", "A refusal phrase a Skill is required to emit under specified conditions."},
		// Phase 39 (D-TBox2-01): ECL pipeline ABox schema.
		{"NoteChunk", "Note Chunk", "A merged logical note produced by the ECL pipeline from __secNN source fragments."},
		{"TableSchema", "Table Schema", "An ETL table whose DML appears in the SQL of a NoteChunk."},
	}
	for _, c := range classes {
		fmt.Fprintf(&b, "skf:%s a owl:Class ;\n    rdfs:label \"%s\" ;\n    rdfs:comment \"%s\" .\n\n", c.Name, c.Label, c.Comment)
	}

	// 8 owl:ObjectProperty declarations (type + domain + range + label).
	objProps := []struct{ Name, Label, Domain, Range string }{
		{"belongsToTier", "category belongs to tier", "KBCategory", "Tier"},
		{"hasSubCategory", "category has sub-category", "KBCategory", "SubCategory"},
		{"hasReadAccess", "skill reads from category", "Skill", "KBCategory"},
		{"hasWriteAccess", "skill writes to category", "Skill", "KBCategory"},
		{"poweredBy", "skill powered by agent type", "Skill", "SkillAgent"},
		{"requiresRefusal", "skill requires refusal phrase", "Skill", "RefusalPhrase"},
		// Phase 39 (D-TBox2-01): note↔table relations. belongsToNote is
		// declared-only this phase (zero ABox instances — open-question
		// resolution #1; reserved for section-level ABox, Phase 40+).
		{"belongsToNote", "section chunk belongs to note", "NoteChunk", "NoteChunk"},
		{"referencesTable", "note references table", "NoteChunk", "TableSchema"},
	}
	for _, p := range objProps {
		fmt.Fprintf(&b, "skf:%s a owl:ObjectProperty ;\n    rdfs:domain skf:%s ;\n    rdfs:range skf:%s ;\n    rdfs:label \"%s\" .\n\n", p.Name, p.Domain, p.Range, p.Label)
	}

	// 8 owl:DatatypeProperty declarations (type + domain + range + label).
	dataProps := []struct{ Name, Label, Domain, Range string }{
		{"skillName", "skill name", "Skill", "xsd:string"},
		{"skillDescription", "skill description", "Skill", "xsd:string"},
		{"skillTemperature", "skill LLM temperature", "Skill", "xsd:decimal"},
		{"categoryName", "category name", "KBCategory", "xsd:string"},
		{"subCategoryName", "sub-category name", "SubCategory", "xsd:string"},
		{"tierName", "tier name", "Tier", "xsd:string"},
		{"agentTypeName", "agent type name", "SkillAgent", "xsd:string"},
		{"refusalPhraseText", "refusal phrase text", "RefusalPhrase", "xsd:string"},
	}
	for _, p := range dataProps {
		fmt.Fprintf(&b, "skf:%s a owl:DatatypeProperty ;\n    rdfs:domain skf:%s ;\n    rdfs:range %s ;\n    rdfs:label \"%s\" .\n\n", p.Name, p.Domain, p.Range, p.Label)
	}

	// ABox: Tier individuals.
	for _, c := range cats {
		fmt.Fprintf(&b, "skf:%s a skf:Tier ;\n    skf:tierName \"%s\" .\n\n", c.TierName, c.TierName)
	}

	// ABox: KBCategory individuals + SubCategory individuals.
	for _, c := range cats {
		fmt.Fprintf(&b, "skf:cat_%s a skf:KBCategory ;\n    skf:categoryName \"%s\" ;\n    skf:belongsToTier skf:%s .\n\n", c.TierName, c.TierName, c.TierName)
		for _, sub := range c.SubCategories {
			fmt.Fprintf(&b, "skf:sub_%s_%s a skf:SubCategory ;\n    skf:subCategoryName \"%s\" ;\n    skf:belongsToTier skf:cat_%s .\n\n", c.TierName, sub, sub, c.TierName)
		}
	}

	// ABox: SkillAgent individuals (one per distinct agent type).
	agents := map[string]bool{}
	for _, s := range skills {
		if s.Agent != "" {
			agents[s.Agent] = true
		}
	}
	agentNames := make([]string, 0, len(agents))
	for a := range agents {
		agentNames = append(agentNames, a)
	}
	sort.Strings(agentNames)
	for _, a := range agentNames {
		fmt.Fprintf(&b, "skf:agent_%s a skf:SkillAgent ;\n    skf:agentTypeName \"%s\" .\n\n", a, a)
	}

	// ABox: Skill individuals (grouped bindings, spike-style).
	for _, s := range skills {
		fmt.Fprintf(&b, "skf:skill_%s a skf:Skill ;\n    skf:skillName \"%s\" ;\n    skf:skillDescription \"%s\" ;\n    skf:skillTemperature \"%.2f\"^^xsd:decimal ;\n    skf:poweredBy skf:agent_%s .\n\n", s.Name, escape(s.Name), escape(s.Description), s.Temperature, s.Agent)
		for _, r := range s.Reads {
			tier := tierFromGlob(r)
			if tier != "" {
				fmt.Fprintf(&b, "skf:skill_%s skf:hasReadAccess skf:cat_%s .\n", s.Name, tier)
			}
		}
		for _, w := range s.Writes {
			tier := tierFromGlob(w)
			if tier != "" {
				fmt.Fprintf(&b, "skf:skill_%s skf:hasWriteAccess skf:cat_%s .\n", s.Name, tier)
			}
		}
		for _, p := range s.RefusalPhrases {
			fmt.Fprintf(&b, "skf:skill_%s skf:requiresRefusal skf:phrase_%s .\n", s.Name, sanitizeLocal(p))
		}
	}

	// ABox: RefusalPhrase individuals (deduplicated across skills).
	phrases := map[string]bool{}
	for _, s := range skills {
		for _, p := range s.RefusalPhrases {
			phrases[p] = true
		}
	}
	phraseNames := make([]string, 0, len(phrases))
	for p := range phrases {
		phraseNames = append(phraseNames, p)
	}
	sort.Strings(phraseNames)
	for _, p := range phraseNames {
		fmt.Fprintf(&b, "skf:phrase_%s a skf:RefusalPhrase ;\n    skf:refusalPhraseText \"%s\" .\n\n", sanitizeLocal(p), escape(p))
	}

	return b.String()
}

// tierFromGlob extracts the tier name from a glob like "authority/**" → "authority".
// Returns "" if not a tier glob.
func tierFromGlob(pattern string) string {
	if !strings.HasSuffix(pattern, "/**") {
		return ""
	}
	return strings.TrimSuffix(pattern, "/**")
}

// escape minimal Turtle string escaping.
func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// sanitizeLocal turns a refusal phrase into a safe Turtle local name.
func sanitizeLocal(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
