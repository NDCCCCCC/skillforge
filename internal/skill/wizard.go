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

package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Answers is the input shape for the wizard interview (D5-10). It
// carries the user's natural-language answers plus the optional
// temperature override. Field-by-field provenance is the 5 wizard
// prompts enumerated in the UI-SPEC §SC-3.
//
// The Temperature field is a pointer so the FE can express "user did
// not override" (nil = use type-default per D5-12 — compliance→0.0,
// QA→0.2, creative→0.7). The Commit path sanitizes the supplied value
// to [0.0, 1.0]; Validate enforces the [0.0, 1.0] range anyway, so the
// commit-side clamp is purely defensive.
type Answers struct {
	Purpose     string   `json:"purpose"`               // step 1: what business problem this Skill solves
	Reads       []string `json:"reads"`                 // step 2: which knowledge tiers (e.g. ["authority/**"])
	SkillType   string   `json:"skillType"`             // step 3: "compliance" | "qa" | "creative"
	Guardrails  string   `json:"guardrails"`            // step 4: free-form prohibited actions
	Temperature *float32 `json:"temperature,omitempty"` // step 5 (optional): user override of type-default
}

// Wizard drives the 5-step interview → SKILL.md flow (SC-3 / D5-10..12).
// It owns:
//   - the ChatModel used to emit a structured 7-field Manifest (reused
//     verbatim from internal/agent/critique.go:runCritiqueLLMCall).
//   - the skillsDir root the Commit path writes into (path-traversal
//     protected — see Commit's EvalSymlinks guard).
//
// The Wizard does NOT depend on the Loader or Registry: the wizard
// writes a file the Registry's fsnotify watcher picks up automatically
// (D5-11 closed loop). No explicit rescan is needed.
type Wizard struct {
	cm        model.ToolCallingChatModel
	skillsDir string
}

// NewWizard constructs a Wizard. cm is REQUIRED (Wizard.Generate would
// otherwise have nothing to call); skillsDir must be an absolute path
// (or one resolvable to absolute by Commit) — the path-traversal
// protection runs on the resolved absolute path, so passing a relative
// path that resolves across the filesystem is rejected at Commit time.
//
// Named NewWizard (not New) to avoid colliding with Registry's New in
// the same package — Registry.New takes a different argument shape
// (skillsDir + embedder) and a different return shape (*Registry,
// error). Mirrors NewRouter's naming convention.
func NewWizard(cm model.ToolCallingChatModel, skillsDir string) *Wizard {
	return &Wizard{cm: cm, skillsDir: skillsDir}
}

// wizardSystemPrompt instructs the LLM to emit a 7-field Manifest as
// JSON. The prompt deliberately references the SKILL_SCHEMA field
// numbers so the LLM keeps each field distinct (D5-11 closed-loop
// contract: the generated JSON MUST pass skill.Validate).
//
// Temperature recommendation per D5-12 is also baked into the prompt
// (compliance→0.0, QA→0.2, creative→0.7). The user can override in the
// 5th interview step; the prompt tells the model to prefer the
// recommendation unless the user supplied a value.
const wizardSystemPrompt = `You are a Skill manifest generator. The user has answered 5 interview questions about a Skill they want to create. Produce a JSON object with EXACTLY these 7 fields, conforming to the SKILL_SCHEMA.md v1.0 contract:

  1. "description" — non-empty intent-routing text (the 1-sentence summary used for semantic routing)
  2. "agent" — ONE of: ChatModelAgent, ReActAgent, SequentialAgent, ParallelAgent, LoopAgent, ConditionalAgent (default: ChatModelAgent for compliance/QA, ReActAgent for creative)
  3. "reads" — non-empty array of knowledge tier patterns (e.g. ["authority/**"]). NEVER use "*" wildcard.
  4. "writes" — array of write patterns; use [] for read-only Skills (default)
  5. "steps" — non-empty markdown ordered list (string)
  6. "guardrails" — object with "prohibitions" (MUST have ≥3 items — never cite clauses not retrieved; never paraphrase critical clauses; never modify authority tier content are safe defaults), "required_refusal_phrases" (array, may be empty), "tier_limits" (object, may be empty)
  7. "temperature" — float in [0.0, 1.0]. Recommended: compliance→0.0, QA→0.2, creative→0.7. If the user supplied a temperature, USE THE USER'S VALUE.

Return ONLY the JSON object. No prose, no markdown fence, no preamble.

Recommended temperature (use unless user supplied their own):
  compliance: 0.0
  qa: 0.2
  creative: 0.7`

// Generate builds the wizard prompt from answers, calls the LLM with
// response_format=json_object, parses the response into a Manifest,
// and validates the result. On validation failure the call is retried
// once with the error appended to the prompt; on a second failure the
// error is surfaced to the FE for inline display per UI-SPEC §SC-3.
//
// The retry is bounded to ONE additional call (T-5-18) — beyond that,
// the wizard UI surfaces the validator's error to the user.
func (w *Wizard) Generate(ctx context.Context, ans Answers) (*Manifest, error) {
	if w.cm == nil {
		return nil, fmt.Errorf("wizard: ChatModel is nil (construct via skill.New(cm, skillsDir))")
	}

	userPrompt := buildWizardUserPrompt(ans, nil)
	msgs := []*schema.Message{
		schema.SystemMessage(wizardSystemPrompt),
		schema.UserMessage(userPrompt),
	}

	rawMsg, err := w.cm.Generate(ctx, msgs, model.WithTemperature(0.0))
	if err != nil {
		return nil, fmt.Errorf("wizard: LLM call: %w", err)
	}
	raw := ""
	if rawMsg != nil {
		raw = rawMsg.Content
	}

	m, err := parseWizardManifest(raw)
	if err != nil {
		return nil, err
	}

	if verr := Validate(m); verr != nil {
		// One retry with the validator's error in the prompt (T-5-18).
		retryPrompt := buildWizardUserPrompt(ans, verr)
		retryMsgs := []*schema.Message{
			schema.SystemMessage(wizardSystemPrompt),
			schema.UserMessage(retryPrompt),
		}
		retryMsg, rerr := w.cm.Generate(ctx, retryMsgs, model.WithTemperature(0.0))
		if rerr != nil {
			return nil, fmt.Errorf("wizard: retry LLM call: %w", rerr)
		}
		retryRaw := ""
		if retryMsg != nil {
			retryRaw = retryMsg.Content
		}
		m2, perr := parseWizardManifest(retryRaw)
		if perr != nil {
			return nil, perr
		}
		if verr2 := Validate(m2); verr2 != nil {
			return nil, fmt.Errorf("wizard: validation failed twice (first: %v; second: %v)", verr, verr2)
		}
		return m2, nil
	}

	return m, nil
}

// buildWizardUserPrompt renders the 5 interview answers + optional
// validation error (on retry) into the LLM's user-side prompt. The
// format mirrors the UI-SPEC §SC-3 prompts so the model has the same
// shape regardless of how the FE chose to phrase the answers.
func buildWizardUserPrompt(ans Answers, verr error) string {
	var b strings.Builder
	fmt.Fprintf(&b, "User's answers:\n")
	fmt.Fprintf(&b, "  Purpose: %s\n", strings.TrimSpace(ans.Purpose))
	fmt.Fprintf(&b, "  Reads: %s\n", strings.Join(ans.Reads, ", "))
	fmt.Fprintf(&b, "  Skill type: %s\n", strings.TrimSpace(ans.SkillType))
	fmt.Fprintf(&b, "  Guardrails (additional prohibitions beyond the 3 defaults): %s\n", strings.TrimSpace(ans.Guardrails))
	if ans.Temperature != nil {
		fmt.Fprintf(&b, "  User-supplied temperature override: %g (USE THIS — do NOT use the type-default)\n", *ans.Temperature)
	} else {
		fmt.Fprintf(&b, "  No user temperature override; use the type-default recommendation.\n")
	}
	if verr != nil {
		fmt.Fprintf(&b, "\nYour previous output failed validation: %v\n", verr)
		fmt.Fprintf(&b, "Fix the failing field(s) and re-emit the JSON object.\n")
	}
	return b.String()
}

// parseWizardManifest extracts a JSON object from the LLM's response
// and unmarshals it into a Manifest. Strips a leading/trailing
// markdown fence (```json ... ```) if the model added one despite the
// instruction — defense-in-depth so a stray fence does not break the
// parse.
func parseWizardManifest(raw string) (*Manifest, error) {
	s := strings.TrimSpace(raw)
	// Strip leading/trailing markdown fence if present.
	if strings.HasPrefix(s, "```") {
		// Find first newline after the opening fence.
		nl := strings.Index(s, "\n")
		if nl >= 0 {
			s = s[nl+1:]
		}
		if strings.HasSuffix(s, "```") {
			s = strings.TrimSuffix(s, "```")
		}
		s = strings.TrimSpace(s)
	}
	// Must start with { and end with }.
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("wizard: LLM output is not a JSON object (first/last chars: %q/%q)", firstN(s, 40), lastN(s, 40))
	}
	var m Manifest
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("wizard: parse LLM JSON: %w (raw first 200: %q)", err, firstN(s, 200))
	}
	return &m, nil
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// skillNamePattern enforces the Skill name contract: lowercase ASCII
// alphanumerics + dash. The directory layout contract requires
// skills/<name>/SKILL.md, and the V12 ASVS path-traversal protection
// relies on the name being a single safe token (no separators, no ..).
var skillNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// ErrSkillAlreadyExists is returned by Commit when the target
// skills/<name>/SKILL.md already exists. W-1 (Phase 5 review): the
// wizard commit endpoint must NOT silently overwrite an existing Skill
// — the trust root for compliance verdicts is the canonical
// skills/<name>/SKILL.md file, and a careless re-commit (or a
// network-reachable instance per CLAUDE.md's open-source deployment
// posture) must not be able to replace it without an explicit signal.
// The HTTP layer detects this via errors.Is and returns a 409-style
// {ok:false, error:"..."} envelope so the FE can surface "pick a
// different name" inline.
var ErrSkillAlreadyExists = errors.New("skill already exists")

// Commit atomically writes m to <skillsDir>/<m.Name>/SKILL.md and
// returns the absolute path written. Four guards run before any disk
// write:
//
//  1. Skill name must match ^[a-z0-9-]+$ — first gate against path
//     traversal (T-5-16). Names like "../../etc/passwd" or "Foo Bar"
//     are rejected before any directory operation.
//  2. skillsDir is resolved to an absolute path; the resolved target
//     path is checked to live inside that absolute root — second gate
//     (V12 ASVS reuse from internal/skill/loader.go:158-178).
//  3. The Manifest re-runs Validate — third gate (D5-11 closed-loop
//     contract; an invalid manifest NEVER reaches disk).
//  4. The target skills/<name>/SKILL.md must NOT already exist — fourth
//     gate (W-1). Returns ErrSkillAlreadyExists so the HTTP layer can
//     surface a 409-style "pick a different name" envelope. Default-deny
//     per CLAUDE.md: the wizard never silently overwrites the trust root.
//
// The atomic write is write-temp + rename so the fsnotify watcher
// (Plan 05-01) observes exactly one CREATE+WRITE+CHMOD burst followed
// by the rename — the reload path picks up the validated file. If the
// Manifest's Name is empty the path is rejected with a clear error.
func (w *Wizard) Commit(ctx context.Context, m *Manifest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("wizard: Commit: manifest is nil")
	}
	if m.Name == "" {
		// Manifest.Name is NOT a YAML field — the loader derives it from
		// the parent directory. Wizard callers set it explicitly (the
		// HTTP layer extracts it from the preview payload). An empty
		// name here means the caller forgot to set it.
		return "", fmt.Errorf("wizard: Commit: manifest.Name is empty (caller must set it from the preview payload before committing)")
	}
	if !skillNamePattern.MatchString(m.Name) {
		return "", fmt.Errorf(
			"wizard: Commit: Skill name %q does not match ^[a-z0-9-]+$ (T-5-16 path-traversal protection)",
			m.Name,
		)
	}

	// Second gate — resolve skillsDir to an absolute root and confirm
	// the resolved target lives inside it. Mirrors loader.go:153-178
	// (the V12 ASVS symlink-escape guard). We do NOT use EvalSymlinks
	// for the root itself (the skills root is owned by us) — we only
	// verify the resolved target path.
	skillsRoot, err := filepath.Abs(w.skillsDir)
	if err != nil {
		return "", fmt.Errorf("wizard: resolve skillsDir: %w", err)
	}
	skillsRoot, err = filepath.EvalSymlinks(skillsRoot)
	if err != nil {
		// Fall back to Abs if the root has not been created yet — the
		// wizard may be writing the very first Skill into an empty dir.
		skillsRoot, err = filepath.Abs(w.skillsDir)
		if err != nil {
			return "", fmt.Errorf("wizard: resolve skillsDir (fallback): %w", err)
		}
	}

	// Third gate — Validate must pass before any disk write.
	if err := Validate(m); err != nil {
		return "", fmt.Errorf("wizard: Commit: validation failed: %w", err)
	}

	skillDir := filepath.Join(skillsRoot, m.Name)
	// Belt-and-braces: even after the regex gate, confirm the resolved
	// path still lives inside skillsRoot. filepath.Join does NOT defend
	// against ".." embedded in m.Name (the regex forbids it), but the
	// additional HasPrefix check catches future pattern changes.
	if !strings.HasPrefix(skillDir, skillsRoot+string(filepath.Separator)) && skillDir != skillsRoot {
		return "", fmt.Errorf("wizard: Commit: resolved path %q escapes skills root %q", skillDir, skillsRoot)
	}

	// Fourth gate (W-1) — default-deny: refuse to overwrite an existing
	// skills/<name>/SKILL.md. The fsnotify watcher reloads the file the
	// instant it is replaced, so a silent overwrite would let a careless
	// operator (or a network-reachable instance per CLAUDE.md's
	// open-source deployment posture) mutate the trust root for the next
	// verdict without any signal. The HTTP layer detects
	// ErrSkillAlreadyExists via errors.Is and surfaces a 409-style
	// envelope so the operator can pick a different name or remove the
	// existing directory explicitly.
	//
	// We check the FILE (not the directory): an orphan
	// skills/<name>/ directory left by a previously-failed commit is
	// not a signal that the Skill exists — only the SKILL.md file is.
	// MkdirAll below is idempotent and will reuse the existing dir.
	finalPath := filepath.Join(skillDir, "SKILL.md")
	if _, statErr := os.Stat(finalPath); statErr == nil {
		return "", fmt.Errorf(
			"wizard: Commit: skill %q already exists — pick a different name or remove the existing directory (%w)",
			m.Name, ErrSkillAlreadyExists,
		)
	}

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", fmt.Errorf("wizard: mkdir %s: %w", skillDir, err)
	}

	body := renderSKILLMD(m)

	// Atomic write: write to a temp file in the SAME directory, then
	// rename. Same-directory rename is atomic on POSIX; on Windows the
	// rename is atomic within NTFS for files (Go's os.Rename uses
	// MoveFileExW with MOVEFILE_REPLACE_EXISTING). The fsnotify watcher
	// sees a single CREATE+WRITE+CHMOD burst on the temp file followed
	// by a CREATE on the target path.
	tmp, err := os.CreateTemp(skillDir, ".SKILL.md.tmp.*")
	if err != nil {
		return "", fmt.Errorf("wizard: create temp file in %s: %w", skillDir, err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if rename fails.
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write([]byte(body)); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("wizard: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("wizard: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, finalPath); err != nil {
		return "", fmt.Errorf("wizard: atomic rename %s -> %s: %w", tmpName, finalPath, err)
	}

	return finalPath, nil
}

// renderSKILLMD serializes a Manifest to the on-disk SKILL.md form:
// YAML frontmatter between `---` markers + a markdown body. The body
// is a single-section header that explains the Skill in plain text so
// the operator reviewing the file sees something meaningful.
//
// The shape mirrors the canonical skills/compliance/SKILL.md (see
// .planning/SKILL_SCHEMA.md §Example). Frontmatter is rendered via
// yaml.Marshal — same library the loader uses, guaranteeing round-trip
// parse equivalence.
func renderSKILLMD(m *Manifest) string {
	// Manual YAML frontmatter (avoids a yaml.Marshal import in this
	// file — keeps wizard.go dependency-light). Field-by-field ordering
	// matches SKILL_SCHEMA.md v1.0 so the on-disk file reads naturally.
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", yamlQuoteIfNeeded(m.Description))
	fmt.Fprintf(&b, "agent: %s\n", m.Agent)
	b.WriteString("reads:\n")
	for _, r := range m.Reads {
		fmt.Fprintf(&b, "  - %s\n", yamlQuoteIfNeeded(r))
	}
	b.WriteString("writes:\n")
	if len(m.Writes) == 0 {
		b.WriteString("  []\n")
	} else {
		for _, w := range m.Writes {
			fmt.Fprintf(&b, "  - %s\n", yamlQuoteIfNeeded(w))
		}
	}
	b.WriteString("steps: |\n")
	for _, line := range strings.Split(strings.TrimRight(m.Steps, "\n"), "\n") {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	b.WriteString("guardrails:\n")
	b.WriteString("  prohibitions:\n")
	for _, p := range m.Guardrails.Prohibitions {
		fmt.Fprintf(&b, "    - %s\n", yamlQuoteIfNeeded(p))
	}
	b.WriteString("  required_refusal_phrases:\n")
	if len(m.Guardrails.RequiredRefusalPhrases) == 0 {
		b.WriteString("    []\n")
	} else {
		for _, p := range m.Guardrails.RequiredRefusalPhrases {
			fmt.Fprintf(&b, "    - %s\n", yamlQuoteIfNeeded(p))
		}
	}
	b.WriteString("  tier_limits:\n")
	if len(m.Guardrails.TierLimits) == 0 {
		b.WriteString("    {}\n")
	} else {
		// Stable order via sorted keys (deterministic on-disk output).
		keys := make([]string, 0, len(m.Guardrails.TierLimits))
		for k := range m.Guardrails.TierLimits {
			keys = append(keys, k)
		}
		// simple insertion sort — short slice, no need for sort package.
		for i := 1; i < len(keys); i++ {
			for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
				keys[j-1], keys[j] = keys[j], keys[j-1]
			}
		}
		for _, k := range keys {
			fmt.Fprintf(&b, "    %s: [%s]\n", k, joinQuoted(m.Guardrails.TierLimits[k]))
		}
	}
	if m.Temperature != nil {
		fmt.Fprintf(&b, "temperature: %s\n", formatFloat(*m.Temperature))
	}
	b.WriteString("---\n\n# ")
	b.WriteString(m.Description)
	b.WriteString("\n\n通过 SkillForge 向导创建。\n")
	return b.String()
}

// yamlQuoteIfNeeded wraps a string in double quotes when it contains
// characters YAML treats as special (colons, hashes, leading dashes,
// or any character that would change the field's parse type). The
// wizard's user-input strings (description, prohibition text) are
// almost always prose and need quoting; the structural fields
// (agent, tier pattern) are safe as bare tokens.
func yamlQuoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	needsQuote := false
	for _, r := range s {
		if r == ':' || r == '#' || r == '"' || r == '\n' || r == '\\' {
			needsQuote = true
			break
		}
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "*") || strings.HasPrefix(s, "&") || strings.HasPrefix(s, "!") {
		needsQuote = true
	}
	if !needsQuote {
		return s
	}
	// Escape backslashes + double quotes; wrap.
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ")
	return `"` + r.Replace(s) + `"`
}

func joinQuoted(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = yamlQuoteIfNeeded(s)
	}
	return strings.Join(parts, ", ")
}

func formatFloat(f float32) string {
	// Render with one decimal place — matches the canonical SKILL.md
	// (0.0 / 0.2 / 0.7). Avoids "0" (which YAML parses as int).
	return fmt.Sprintf("%.1f", f)
}
