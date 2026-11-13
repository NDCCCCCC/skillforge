// SPDX-License-Identifier: Apache-2.0
// scripts/repin/main.go — re-author a frozen eval lock against the CURRENT
// rendered Skill prompt.
//
// Why a tool and not a hand edit: the lock's prompt_hash is only meaningful
// if it is produced by the SAME function that verifies it
// (eval.PromptHashFor / reprohash.PromptHash). A hand-typed hash is exactly
// the drift this machinery exists to prevent — and a stale hash is what
// internal/eval's TestShippedFrozenLockIsInSync is designed to catch.
//
// What it does (one Skill, one lock):
//   1. load the Skill manifest from <skills-dir> (the SHIPPED template —
//      examples/skills, not the repo's live skills/ dir);
//   2. render the agent system prompt exactly like the eval harness does
//      (agent.RenderSystemPrompt);
//   3. compute the prompt hash through eval.PromptHashFor;
//   4. rewrite <lock>.lock.json with the new prompt_hash + eval_set_version;
//   5. print the new file's sha256 so the verify scripts' byte-identity
//      pins (EXPECTED_COMPLIANCE_SHA) can be updated to match.
//
// It does NOT run the eval — the scores must come from a real eval run
// against the same prompt (cmd/eval). This tool only re-authors the pin;
// the ORDER matters: run the eval FIRST, then re-pin, so the archived
// report and the lock describe the same prompt.
//
// Usage:
//   go run ./scripts/repin \
//     --skill compliance \
//     --skills-dir examples/skills \
//     --lock eval/frozen/compliance.lock.json \
//     --version compliance-2026-11-13-v2
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/kb"
	"github.com/skillforge/skillforge/internal/skill"
)

// lockFile mirrors eval.LockFile's JSON shape (model_ver / prompt_hash /
// seed / eval_set_version / skill_name). Declared locally so this tool can
// rewrite the file without dragging the whole eval report machinery in.
type lockFile struct {
	ModelVersion    string `json:"model_ver"`
	PromptHash      string `json:"prompt_hash"`
	Seed            int    `json:"seed"`
	EvalSetVersion  string `json:"eval_set_version"`
	SkillName       string `json:"skill_name"`
}

// tierRegistry mirrors the four knowledge-base tiers the shipped compliance
// Skill reads (internal/eval/frozen_test.go's testKBReg). The registry only
// affects tier VALIDATION during skill loading — the rendered prompt is the
// same for any registry that accepts the Skill's declared tiers.
func tierRegistry() kb.Registry {
	return kb.New([]kb.Entry{
		{Slug: "authority", DisplayName: "Authority", Role: kb.RoleAuthority, RetrievalEnabled: true},
		{Slug: "historical", DisplayName: "Historical", Role: kb.RoleHistorical, RetrievalEnabled: true},
		{Slug: "personal", DisplayName: "Personal", Role: kb.RolePersonal, RetrievalEnabled: true},
		{Slug: "draft", DisplayName: "Draft", Role: kb.RoleDraft, RetrievalEnabled: true},
	})
}

func main() {
	skillName := flag.String("skill", "compliance", "Skill name to re-pin")
	skillsDir := flag.String("skills-dir", "examples/skills", "directory holding <skill>/SKILL.md")
	lockPath := flag.String("lock", "eval/frozen/compliance.lock.json", "lock file to rewrite")
	version := flag.String("version", "", "new eval_set_version (required)")
	dryRun := flag.Bool("dry-run", false, "print the computed hash without writing the lock")
	flag.Parse()

	if strings.TrimSpace(*version) == "" {
		fmt.Fprintln(os.Stderr, "[repin] --version is required (the new eval_set_version)")
		os.Exit(2)
	}

	loader := skill.NewLoader(*skillsDir, tierRegistry())
	if _, err := loader.Scan(); err != nil {
		fmt.Fprintf(os.Stderr, "[repin] scan %s: %v\n", *skillsDir, err)
		os.Exit(1)
	}
	var manifest *skill.Manifest
	for _, m := range loader.Skills() {
		if m.Name == *skillName {
			manifest = m
		}
	}
	if manifest == nil {
		fmt.Fprintf(os.Stderr, "[repin] skill %q not found under %s\n", *skillName, *skillsDir)
		os.Exit(1)
	}

	body := agent.RenderSystemPrompt(manifest)
	newHash := eval.PromptHashFor(*skillName, eval.DefaultJudgeModel, body)

	raw, err := os.ReadFile(*lockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[repin] read lock: %v\n", err)
		os.Exit(1)
	}
	var lf lockFile
	if err := json.Unmarshal(raw, &lf); err != nil {
		fmt.Fprintf(os.Stderr, "[repin] parse lock: %v\n", err)
		os.Exit(1)
	}
	if lf.SkillName != *skillName {
		fmt.Fprintf(os.Stderr, "[repin] lock pins skill %q, asked for %q\n", lf.SkillName, *skillName)
		os.Exit(1)
	}

	fmt.Printf("[repin] skill      = %s\n", *skillName)
	fmt.Printf("[repin] model_ver  = %s\n", lf.ModelVersion)
	fmt.Printf("[repin] old hash   = %s\n", lf.PromptHash)
	fmt.Printf("[repin] new hash   = %s\n", newHash)
	fmt.Printf("[repin] prompt len = %d bytes\n", len(body))

	if *dryRun {
		fmt.Println("[repin] dry-run — lock untouched")
		return
	}

	lf.PromptHash = newHash
	lf.EvalSetVersion = *version
	out, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[repin] marshal lock: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')
	if err := os.WriteFile(*lockPath, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[repin] write lock: %v\n", err)
		os.Exit(1)
	}

	sum := sha256.Sum256(out)
	fmt.Printf("[repin] wrote %s\n", *lockPath)
	fmt.Printf("[repin] NEW_SHA256=%s\n", hex.EncodeToString(sum[:]))
	fmt.Println("[repin] update EXPECTED_COMPLIANCE_SHA in the verify scripts to the value above.")
	_ = filepath.Base(*lockPath)
}
