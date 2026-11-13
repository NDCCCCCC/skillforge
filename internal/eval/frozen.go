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

package eval

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/skillforge/skillforge/internal/llmconfig"
	"github.com/skillforge/skillforge/internal/reprohash"
)

// The two-layer reproducibility tolerance (D6-15 / SC-6).
//
// SC-6 asks for "numerically identical scores within tolerance", and
// there is no single honest number for that: a hermetic pipeline and a
// live LLM API have different noise floors. So the claim is split into
// two independently-gated layers.
const (
	// StrictReproducibilityTolerance gates the hermetic layer
	// (StubJudge + deterministic embedder). Any drift above this is a
	// determinism BUG in our pipeline — an unseeded rand, a map
	// iteration order, a time.Now.
	//
	// Strengthened from the Phase 3 value of 1e-9, which sat below the
	// float64 noise floor of these metrics (means over sums of cosine
	// similarities) and therefore pinned an accidental property of the
	// current operation order rather than determinism itself.
	StrictReproducibilityTolerance = 1e-6

	// LiveReproducibilityTolerance gates the JITTER LAYER ONLY: the same
	// judge scoring the SAME fixed inputs twice. Its gate test
	// (TestRunnerLiveReproducibility) builds samples via
	// buildFrozenSamples(), which synthesizes the answers outright —
	// Answer = "claim-1. claim-2.", Contexts = [frozen-context-a,
	// frozen-context-b]. No agent runs, no real answer is generated.
	//
	// For THAT semantic, 1e-3 is honest: a judge re-scoring identical
	// text barely moves.
	//
	// What it does NOT cover, and must never be quoted as covering, is
	// end-to-end reproducibility — running the agent again to regenerate
	// the answer, then judging that new answer. Measured on adv-001 and
	// adv-002 with an identical prompt_hash / model_ver / seed, the
	// end-to-end faithfulness for the same case came out 0.500 once and
	// 1.000 the next run: ±0.5, i.e. 500x this tolerance. The dominant
	// noise source is the generator, not the judge.
	//
	// So: a single --frozen score is one SAMPLE, not a measurement.
	// Aggregate over repeats before treating a delta as a regression.
	LiveReproducibilityTolerance = 1e-3
)

// LockFile is the sidecar that pins a frozen eval set (D6-13 / SC-6).
// It lives next to the set it pins: eval/frozen/<skill>.jsonl is pinned
// by eval/frozen/<skill>.lock.json.
//
// What the pin buys: it lets the runner PROVE the model and the prompt
// did not move underneath the scores. Without that proof, two numbers
// from two runs are not comparable at all.
//
// Careful with the wording here. SC-6 is often paraphrased as
// "re-running a frozen set returns numerically identical scores", but
// that sentence conflates two different claims:
//   · judge-layer stability (same inputs, judge twice) -> 1e-3, gated
//     by TestRunnerLiveReproducibility against synthesized answers
//   · end-to-end stability (agent regenerates the answer, then judge)
//     -> NOT gated here, and measured at ±0.5 on this corpus
// AssertPin proves the pin, not the score's repeatability.
//
// Fields:
//   - ModelVersion — the LLM id the frozen scores were produced with.
//   - PromptHash — reprohash.PromptHash(skill_name, model_ver,
//     rendered system-prompt body). Post-Q1 this covers the prompt
//     BODY, so editing SKILL.md moves the hash (see reprohash docs).
//   - Seed — the deterministic seed of the strict (StubJudge) layer.
//   - EvalSetVersion — human-readable version of the .jsonl content.
//   - SkillName — the Skill the set evaluates; also the first input to
//     PromptHash.
type LockFile struct {
	ModelVersion   string `json:"model_ver"`
	PromptHash     string `json:"prompt_hash"`
	Seed           int64  `json:"seed"`
	EvalSetVersion string `json:"eval_set_version"`
	SkillName      string `json:"skill_name"`
}

// LoadLock reads a frozen-set .lock.json sidecar. Mirrors LoadGolden's
// posture (os.Open + defer Close + json decode + explicit field
// validation) — a lock missing a field is a caller/authoring bug, and
// silently defaulting it to "" would produce a pin that always passes.
//
// Returns an error naming the missing field so the operator can fix the
// file without reading this source.
func LoadLock(path string) (LockFile, error) {
	var lock LockFile
	f, err := os.Open(path)
	if err != nil {
		return LockFile{}, fmt.Errorf("eval.LoadLock: open %q: %w", path, err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&lock); err != nil {
		return LockFile{}, fmt.Errorf("eval.LoadLock: decode %q: %w", path, err)
	}
	// Explicit required-field validation. Seed is NOT validated: 0 is a
	// legitimate seed value, so its zero value is indistinguishable from
	// "absent" and we do not guess.
	for _, req := range []struct {
		field string
		value string
	}{
		{"model_ver", lock.ModelVersion},
		{"prompt_hash", lock.PromptHash},
		{"eval_set_version", lock.EvalSetVersion},
		{"skill_name", lock.SkillName},
	} {
		if req.value == "" {
			return LockFile{}, fmt.Errorf("eval.LoadLock: %q missing required field %q", path, req.field)
		}
	}
	return lock, nil
}

// AssertPin verifies that the LIVE configuration matches the frozen
// pin. It is the SC-6 gate: cmd/eval --frozen calls it BEFORE running
// any metric, so a drifted run fails loud instead of publishing scores
// that are silently incomparable to the baseline.
//
// The prompt check re-derives the hash through reprohash.PromptHash —
// the SAME function internal/server/chat.go::computePromptHash calls.
// That byte-identity is the load-bearing invariant (PATTERNS invariant
// #8): if the two ever diverge, the pin degrades from "proof" to
// "decoration".
//
// Checks run in most-actionable-first order:
//  1. model_ver drift — the operator changed JUDGE_MODEL/the model id.
//  2. skill_name drift — the frozen set is being run against a
//     different Skill (reported before the hash, because a skill
//     mismatch would ALSO surface as a hash drift and that error would
//     point the operator at the wrong cause).
//  3. prompt_hash drift — the rendered prompt body (or another hash
//     input) moved. This is the check Q1 made possible.
//
// Returns nil when the live configuration is pin-identical.
func AssertPin(lock LockFile, liveModel, liveSkillName, livePromptBody string) error {
	if lock.ModelVersion != liveModel {
		return fmt.Errorf("eval.AssertPin: model_ver drift (lock=%q, live=%q) — the frozen scores were produced with a different model; re-freeze the set or restore the pinned model",
			lock.ModelVersion, liveModel)
	}
	if lock.SkillName != liveSkillName {
		return fmt.Errorf("eval.AssertPin: skill_name drift (lock=%q, live=%q) — this frozen set pins a different Skill",
			lock.SkillName, liveSkillName)
	}
	liveHash := reprohash.PromptHash(liveSkillName, liveModel, livePromptBody)
	if lock.PromptHash != liveHash {
		return fmt.Errorf("eval.AssertPin: prompt_hash drift (lock=%q, live=%q) — the rendered system prompt changed since the set was frozen; re-freeze the set to publish a new baseline",
			lock.PromptHash, liveHash)
	}
	return nil
}

// PromptHashFor is the lock-writer's counterpart to AssertPin: it
// produces the value that belongs in a .lock.json's prompt_hash field.
//
// Exposed so the ONLY way to author a lock is through the same function
// that verifies it — a hand-computed hash is exactly the kind of drift
// this machinery exists to prevent.
func PromptHashFor(skillName, modelVersion, renderedPromptBody string) string {
	return reprohash.PromptHash(skillName, modelVersion, renderedPromptBody)
}

// ActiveModelVersion reports the model id the current process would use
// for the judge (and, in cmd/eval, the agent — both resolve through
// llmconfig.Effective()). This is the "live" value AssertPin compares
// against the lock's model_ver.
//
// 261113-obm: it reads llmconfig.Effective() — the SAME source
// ResolveLLM dials — so the pin compares against the model the judge
// actually uses. It used to read env only (LOCAL_MODEL > JUDGE_MODEL >
// DefaultJudgeModel), which meant a model changed on the admin panel
// updated the dial target but not the pin comparison: two sources for
// one question, and the page could drift from the pin unnoticed.
//
// On the env-preset path (no panel config) a deployment relying on
// LOCAL_BASE_URL/LOCAL_API_KEY (self-hosted endpoint) MUST still set
// LOCAL_MODEL — otherwise the pin/report records the GLM default name
// while actually calling the local server. The panel-config path has no
// such gap: the model comes from the same file that names the endpoint.
//
// Effective errors when a panel config exists but its key cannot be
// resolved; the model id is then unknown, so fall back to
// DefaultJudgeModel (the same floor ResolveLLM's no-key failure
// resolves to).
func ActiveModelVersion() string {
	_, _, m, err := llmconfig.Effective()
	if err != nil || m == "" {
		return DefaultJudgeModel
	}
	return m
}
