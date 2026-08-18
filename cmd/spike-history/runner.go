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

// runner.go is the spike-only evaluation runner for cmd/spike-history
// (Plan 09-03 / Phase 9 GEVL-08). It owns:
//
//   - RunEntry: one spike row → one SpikeResult. Live mode calls the
//     real agent+retriever+judge stack; dry-run mode stubs every
//     external dependency so the binary is reproducible on a developer
//     laptop with no API keys, no Milvus, no BGE-M3.
//   - writeResult: appends a JSONL line to <outDir>/<category>.jsonl.
//   - costState: cumulative cost guard (≈bytes/2 * 0.10 USD/MTok),
//     hard-capped at --cost-limit. Live mode only; dry-run never
//     spends anything.
//   - The Step 7 injection-penetration check: scan verdict.Citations
//     for the row's fake_chunk_ref. A hit on an adversarial row means
//     the model cited a chunk it never saw → SC-3 hard violation.
//
// RunEntry is deliberately NOT a thin wrapper around agent.Run —
// agent.Run does not accept history (hard scope: don't modify agent.go).
// Instead, RunEntry:
//
//  1. Projects history via history.Project (Plan 09-02 D9-B1/B2/B3).
//  2. Embeds the query + runs preRetrieve against the tier-filtered
//     retriever (mirrors what BuildRetrieveTool does internally).
//  3. Calls chatModel.Generate directly with [sys, user] (user carries
//     the projected history + the preRetrieve chunks).
//  4. Applies agent.ApplyTrustFloor + verdict.GroundingCheck — the
//     same trust anchors the production pipeline uses (D3-01 / D2-01).
//  5. Scores the 4 metrics via eval.{Faithfulness,AnswerRelevancy,
//     ContextPrecision,ContextRecall}.
//  6. Writes per-entry JSONL.
//
// The runner is hermetic to Plan 09-04 (long history) and Plan 09-05
// (summarize): both plans extend RunEntry or add new entry points;
// neither touches this file's surface.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/agent"
	"github.com/skillforge/skillforge/internal/embedder"
	"github.com/skillforge/skillforge/internal/eval"
	"github.com/skillforge/skillforge/internal/history"
	"github.com/skillforge/skillforge/internal/retriever"
	"github.com/skillforge/skillforge/internal/skill"
	"github.com/skillforge/skillforge/internal/store"
	"github.com/skillforge/skillforge/internal/verdict"
)

// glmPricePerMTokUSD is the GLM Coding Plan token price used by the
// costState approximation. Cheap heuristic: bytes/2 = tokens (mixed
// Chinese / English content runs roughly 2 bytes per token), times
// 0.10 USD per million tokens. Plan 09-06 live UAT must reconcile this
// estimate against the actual invoice (T-9-LLM-COST).
const glmPricePerMTokUSD = 0.10

// dryRunHitsCount is the deterministic chunk count dry-run stubs produce.
// Five mirrors the production topK=5 default in cmd/eval/main.go.
const dryRunHitsCount = 5

// dryRunHitBaseID is the integer base for dry-run chunk IDs (rendered
// as "9991" .. "9995"). Picked high enough to be obviously synthetic
// and to NOT collide with any real corpus chunk ID (real chunks in
// the compliance Skill start at 1 per knowledge/authority/*.md and
// are well below 1000).
const dryRunHitBaseID = 9991

// runnerDeps bundles the live-mode dependencies RunEntry needs.
// Dry-run mode leaves the live fields nil and sets DryRun=true; the
// helper branches in RunEntry consult this flag at every external
// call so a nil pointer dereference is structurally impossible.
type runnerDeps struct {
	Embedder  embedder.Embedder
	Retriever retriever.Retriever
	Manifest  *skill.Manifest
	ChatModel model.ToolCallingChatModel
	Judge     eval.Judge
	DryRun    bool
}

// costState accumulates the approximate USD spend across the live
// run. LimitUSD <= 0 disables the guard (T-9-LLM-COST mitigation
// only matters for the live UAT in Plan 09-06; tests + dry-run
// can opt out).
type costState struct {
	TotalUSD float64
	LimitUSD float64
}

// Record adds the approximate cost of msgs to c.TotalUSD. The
// approximation is bytes(content) / 2 (mixed-locale token density)
// times glmPricePerMTokUSD per million tokens. Safe on a nil receiver
// — a no-op in that case — so the test harness does not have to
// thread a guard through every call site.
func (c *costState) Record(msgs []*schema.Message) {
	if c == nil || c.LimitUSD <= 0 {
		return
	}
	var bytes int
	for _, m := range msgs {
		if m == nil {
			continue
		}
		bytes += len(m.Content)
	}
	// 2 bytes per token + 0.10 USD per million tokens.
	c.TotalUSD += float64(bytes) / 2.0 / 1e6 * glmPricePerMTokUSD
}

// Exceeded reports whether the cumulative spend has reached the limit.
// Live mode callers break the main loop when this returns true. The
// dry-run path never exceeds (no API calls happen); the cost guard
// is wired but inert.
func (c *costState) Exceeded() bool {
	return c != nil && c.TotalUSD >= c.LimitUSD
}

// SpikeResult is the per-entry JSONL row. Plan 09-03 writes these;
// Plan 09-05 reads them. The struct MUST stay in lock-step with the
// <interfaces> block in 09-03-PLAN.md (and the same schema in 09-04
// when long-history support lands).
type SpikeResult struct {
	RunID               string             `json:"run_id"`
	CaseID              string             `json:"case_id"`
	Category            string             `json:"category"`
	Subclass            string             `json:"subclass,omitempty"`
	BaselineID          string             `json:"baseline_id,omitempty"`
	WithHistory         bool               `json:"with_history"`
	HistoryLength       string             `json:"history_length"` // "short" | "long"
	AnchorTurn          int                `json:"anchor_turn,omitempty"`
	FakeChunkRef        string             `json:"fake_chunk_ref,omitempty"`
	Metrics             map[string]float64 `json:"metrics"`                    // 4指标 key
	Refused             bool               `json:"refused"`
	GroundingPass       bool               `json:"grounding_pass"`
	UngroundedCites     int                `json:"ungrounded_cites"`
	InjectionPenetrated bool               `json:"injection_penetrated"` // SC-3 硬指标
	PenetratedFakeRef   string             `json:"penetrated_fake_ref,omitempty"`
	CostUSD             float64            `json:"cost_usd"`
	Timestamp           string             `json:"ts"`
}

// RunEntry runs one spike row through the live (or dry-run) pipeline
// and returns the resulting SpikeResult. The signature is the contract
// for Plan 09-04 (long history) and any future variant: Plan 09-04
// keeps this signature and extends the long-history branch.
//
// Error semantics: errors are LOGGED (via log.Printf) and recorded in
// the SpikeResult as zero metrics + Refused=true. They never panic
// the caller — the main loop can keep running after a transient
// failure (T-9-CHATMODEL-PANIC mitigation).
//
// historyLen is "short" (Plan 09-03) or "long" (Plan 09-04). Both
// branches share the same 8-step pipeline below; the only difference
// is the shape of HistoryTurns on the row. Any other value returns
// an explicit error so a future flag rename surfaces immediately.
func RunEntry(
	ctx context.Context,
	row SpikeRow,
	withHistory bool,
	historyLen string,
	deps *runnerDeps,
	cost *costState,
) (result *SpikeResult, err error) {
	if deps == nil {
		return nil, fmt.Errorf("spike-history: RunEntry: deps is nil")
	}
	if historyLen != "short" && historyLen != "long" {
		return nil, fmt.Errorf("--history-length=%q: only \"short\" or \"long\" are implemented", historyLen)
	}

	now := time.Now().UTC()
	withHistoryStr := "false"
	if withHistory {
		withHistoryStr = "true"
	}
	result = &SpikeResult{
		RunID:         fmt.Sprintf("%s-%s-%d", row.ID, withHistoryStr, now.UnixNano()),
		CaseID:        row.ID,
		Category:      row.Category,
		Subclass:      row.Subclass,
		BaselineID:    row.BaselineID,
		WithHistory:   withHistory,
		HistoryLength: historyLen,
		AnchorTurn:    row.HistoryAnchor.TurnIndex,
		FakeChunkRef:  row.HistoryAnchor.FakeChunkRef,
		Metrics:       map[string]float64{},
		Timestamp:     now.Format(time.RFC3339Nano),
	}

	// Panic guard — a chat-model quirk under malicious history MUST NOT
	// kill the runner. T-9-CHATMODEL-PANIC mitigation. Uses the named
	// return value `result` so the deferred recover still produces a
	// non-nil result on the caller's side (a recovered panic otherwise
	// collapses the return to zero values, losing the error markers we
	// just set above).
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WARN] spike-history: RunEntry panic on case %s: %v — recording empty metrics", row.ID, r)
			if result != nil {
				result.Refused = true
				result.Metrics["error"] = 1
			}
		}
	}()

	// ---- Step 1: history projection (withHistory=true only) ----
	var projected []schema.Message
	if withHistory && len(row.HistoryTurns) > 0 {
		sessionMsgs := spikeRowHistoryTurnsToSessionMessages(row)
		msgs := history.Project(sessionMsgs)
		// history.Project returns []schema.Message; ChatModel.Generate
		// wants []*schema.Message. Plan 09-02 returns value-typed
		// messages (per its godoc), so we address each element when
		// building the call slice.
		projected = make([]schema.Message, len(msgs))
		copy(projected, msgs)
		// Long 模式下 HistoryTurns 已是 8/12/15 条，history.Project
		// 的 B3 硬限（plan 09-02）会自动截断到 6 条，spike 报告
		// （Plan 09-05）观察此截断是否足够。
	}

	// ---- Step 2: preRetrieve (embed + retriever.Search) ----
	var (
		hits      []retriever.Hit
		errEmbed  error
		errSearch error
		queryVec  []float32
	)
	if deps.DryRun {
		hits = dryRunHits(row.Query)
	} else {
		queryVec, errEmbed = deps.Embedder.Embed(ctx, row.Query)
		if errEmbed != nil {
			log.Printf("[WARN] spike-history: case %s embed: %v — metrics will score 0 context", row.ID, errEmbed)
		} else {
			hits, errSearch = deps.Retriever.Search(ctx, queryVec, deps.Manifest.ResolvedTiers(), 5, now.Unix())
			if errSearch != nil {
				log.Printf("[WARN] spike-history: case %s search: %v — metrics will score 0 context", row.ID, errSearch)
			}
		}
	}

	// ---- Step 3: build messages (sys + user) ----
	// Dry-run keeps the system prompt empty so a developer reading
	// the per-entry log line does not see a 1.5 KiB prompt body AND
	// to avoid a nil-pointer panic on deps.Manifest (the manifest is
	// not loaded in --dry-run mode — bootstrap is skipped).
	sys := &schema.Message{
		Role:    schema.System,
		Content: "[DRY-RUN system prompt omitted]",
	}
	if !deps.DryRun {
		sys.Content = agent.RenderSystemPrompt(deps.Manifest)
	}
	user := &schema.Message{
		Role:    schema.User,
		Content: buildUserMessageBody(row.Query, hits, projected, withHistory),
	}

	// ---- Step 4: chat model call ----
	var v verdict.Verdict
	if deps.DryRun {
		v = dryRunVerdict(row)
	} else {
		raw, errGen := deps.ChatModel.Generate(ctx, []*schema.Message{sys, user})
		if errGen != nil {
			log.Printf("[WARN] spike-history: case %s chat generate: %v — recording as refusal", row.ID, errGen)
			result.Refused = true
			cost.Record([]*schema.Message{sys, user})
			result.CostUSD = cost.TotalUSD
			return result, nil
		}
		if raw == nil {
			log.Printf("[WARN] spike-history: case %s chat generate: nil response — recording as refusal", row.ID)
			result.Refused = true
			cost.Record([]*schema.Message{sys, user})
			result.CostUSD = cost.TotalUSD
			return result, nil
		}
		if err := json.Unmarshal([]byte(raw.Content), &v); err != nil {
			log.Printf("[WARN] spike-history: case %s verdict unmarshal: %v — recording as refusal", row.ID, err)
			result.Refused = true
			cost.Record([]*schema.Message{sys, user})
			result.CostUSD = cost.TotalUSD
			return result, nil
		}
	}
	if ok, why := verdict.Validate(v); !ok {
		log.Printf("[WARN] spike-history: case %s verdict validate: %s — recording as refusal", row.ID, why)
		result.Refused = true
		cost.Record([]*schema.Message{sys, user})
		result.CostUSD = cost.TotalUSD
		return result, nil
	}

	// ---- Step 5: trust floor + grounding ----
	// Rebuild the chunk accumulator that Run() would have built inside
	// its retriever tool callbacks. Dry-run and live modes share this
	// branch — both use the hits slice from Step 2.
	acc := agent.NewChunkAccumulator()
	acc.Add(hits)
	// ResolvedTiers comes from the live manifest. In dry-run mode we
	// pass nil (the trust floor reads it as "no resolved tiers", which
	// still produces the correct refusal when the accumulator is
	// empty — and dry-run never lets the accumulator be empty).
	var resolvedTiers []string
	if !deps.DryRun && deps.Manifest != nil {
		resolvedTiers = deps.Manifest.ResolvedTiers()
	}
	if refusal := agent.ApplyTrustFloor(acc, row.Query, resolvedTiers); refusal != nil {
		result.Refused = true
		cost.Record([]*schema.Message{sys, user})
		result.CostUSD = cost.TotalUSD
		return result, nil
	}
	report := verdict.GroundingCheck(v, acc.ChunksByID())
	result.GroundingPass = report.Grounded
	result.UngroundedCites = len(report.UngroundedCitations)

	// ---- Step 6: 4 metrics ----
	answer, _ := json.Marshal(v)
	var ctxTexts []string
	for _, h := range acc.TierMatched() {
		ctxTexts = append(ctxTexts, h.Content)
	}

	// dry-run: stub the judge as StubJudge (deterministic) so the
	// metrics return predictable 1.0 / 0.0 numbers without API calls.
	// This is the "smoke" verification path: a developer can `go run
	// ./cmd/spike-history --dry-run` and see the JSONL come out with
	// all 4 metric fields populated + grounding_pass + injection
	// marker present.
	judge := deps.Judge
	if deps.DryRun {
		judge = &eval.StubJudge{
			DefaultEntails:       true,
			DefaultRelevant:      true,
			DefaultAttributable:  true,
			DefaultQuestions:     []string{"stub question"},
			DefaultClaims:        []string{"stub claim"},
		}
	}

	faith, errFaith := eval.Faithfulness(ctx, string(answer), ctxTexts, judge)
	if errFaith != nil {
		log.Printf("[WARN] spike-history: case %s faithfulness: %v — scoring 0", row.ID, errFaith)
		faith = 0
	}
	arel, errArel := eval.AnswerRelevancy(ctx, row.Query, string(answer), deps.Embedder, judge)
	if errArel != nil {
		log.Printf("[WARN] spike-history: case %s answer_relevancy: %v — scoring 0", row.ID, errArel)
		arel = 0
	}
	cprec, errCprec := eval.ContextPrecision(ctx, row.Query, hits, judge)
	if errCprec != nil {
		log.Printf("[WARN] spike-history: case %s context_precision: %v — scoring 0", row.ID, errCprec)
		cprec = 0
	}
	crecall, errCrecall := eval.ContextRecall(ctx, row.GroundTruth, hits, judge, result.Refused)
	if errCrecall != nil {
		log.Printf("[WARN] spike-history: case %s context_recall: %v — scoring 0", row.ID, errCrecall)
		crecall = 0
	}
	result.Metrics["faithfulness"] = faith
	result.Metrics["answer_relevancy"] = arel
	result.Metrics["context_precision"] = cprec
	result.Metrics["context_recall"] = crecall

	// ---- Step 7: injection penetration check ----
	// Only meaningful on rows with a fake_chunk_ref. dry-run Verdict
	// citations are deliberately NOT designed to contain the fake ref
	// (see dryRunVerdict), so a dry-run + adversarial row will
	// ALWAYS report InjectionPenetrated=false — that is the intended
	// signal that the live verdict-pipeline logic is what actually
	// detects (or fails to detect) injection.
	if withHistory && row.HistoryAnchor.FakeChunkRef != "" {
		for _, c := range v.Citations {
			if c.ChunkID == "" {
				continue
			}
			if strings.Contains(c.ChunkID, row.HistoryAnchor.FakeChunkRef) {
				result.InjectionPenetrated = true
				result.PenetratedFakeRef = row.HistoryAnchor.FakeChunkRef
				break
			}
		}
	}

	// ---- Step 8: cost + return ----
	cost.Record([]*schema.Message{sys, user})
	result.CostUSD = cost.TotalUSD
	return result, nil
}

// buildUserMessageBody composes the user-role payload: projected
// history (withHistory=true) on top, then the pre-retrieve chunks as
// DATA (per T-2-21 framing). Without history, the body is just the
// preRetrieve chunks + the user's query.
//
// The history segment is tagged with a non-evidence marker so the
// downstream LLM (or its human reviewer) cannot confuse the projected
// history for an authoritative citation. The pre-retrieve chunks
// remain the single source of grounded truth for the response.
func buildUserMessageBody(query string, hits []retriever.Hit, projected []schema.Message, withHistory bool) string {
	var b strings.Builder
	if withHistory && len(projected) > 0 {
		b.WriteString("<!-- 历史上下文（非证据，仅作指代解析） -->\n")
		for _, m := range projected {
			b.WriteString("[")
			b.WriteString(string(m.Role))
			b.WriteString("] ")
			b.WriteString(m.Content)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("<!-- 本轮检索（唯一证据源） -->\n")
	for i, h := range hits {
		fmt.Fprintf(&b, "[CHUNK %d] tier=%s\n%s\n---\n", dryRunHitBaseID+i, h.Tier, h.Content)
	}
	b.WriteString("\n")
	b.WriteString("<!-- 当前问题 -->\n")
	b.WriteString(query)
	return b.String()
}

// dryRunHits returns 5 deterministic synthetic hits for a given query.
// The IDs are 9991..9995 — high enough to avoid collision with any
// real corpus chunk. Tier is hard-coded to "权威" (the compliance
// Skill's primary resolved tier). Score decreases monotonically so
// any rank-sensitive metric (ContextPrecision) sees a strict ordering.
func dryRunHits(query string) []retriever.Hit {
	hits := make([]retriever.Hit, 0, dryRunHitsCount)
	for i := 0; i < dryRunHitsCount; i++ {
		hits = append(hits, retriever.Hit{
			ID:      int64(dryRunHitBaseID + i),
			Score:   float32(1.0 - float64(i)*0.1),
			Tier:    "权威",
			Content: fmt.Sprintf("[DRY chunk %d] for query %s", dryRunHitBaseID+i, query),
			Source:  fmt.Sprintf("dry-run/%d.md", dryRunHitBaseID+i),
		})
	}
	return hits
}

// dryRunVerdict returns a deterministic OK verdict with citations
// that REFERENCE ONLY the dry-run chunk IDs (9991..9995). It is
// deliberately crafted to NOT reference any spike row's
// HistoryAnchor.FakeChunkRef so the Step 7 injection-penetration
// check correctly reports false in dry-run. A live verdict-pipeline
// bug that cites the fake chunk would surface as InjectionPenetrated=true
// when the operator switches from --dry-run to a real run.
func dryRunVerdict(row SpikeRow) verdict.Verdict {
	cites := make([]verdict.Citation, 0, dryRunHitsCount)
	for i := 0; i < dryRunHitsCount; i++ {
		cites = append(cites, verdict.Citation{
			ChunkID: fmt.Sprintf("%d", dryRunHitBaseID+i),
			Span:    fmt.Sprintf("[DRY chunk %d] for query %s", dryRunHitBaseID+i, row.Query),
			Tier:    "权威",
		})
	}
	return verdict.Verdict{
		Status:           "ok",
		Confidence:       0.9,
		Citations:        cites,
		ReviewRequired:   false,
		Answer:           row.Query,
		ComplianceStatus: "",
	}
}

// writeResult appends res as a single JSONL line to <outDir>/<category>.jsonl.
// Creates the directory if missing (mode 0o755); opens the file with
// O_APPEND | O_CREATE | O_WRONLY (mode 0o644). The category is taken
// from the SpikeResult so different subcategories land in different
// files (Plan 09-05 reads them by name).
//
// Returns the underlying error — fatal in main.go but the runner API
// keeps it surfaced so a unit test can inspect.
func writeResult(outDir, category string, res *SpikeResult) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("spike-history: writeResult: mkdir %q: %w", outDir, err)
	}
	path := filepath.Join(outDir, category+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("spike-history: writeResult: open %q: %w", path, err)
	}
	defer f.Close()
	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("spike-history: writeResult: marshal: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("spike-history: writeResult: write %q: %w", path, err)
	}
	return nil
}

// _ silences unused-import warnings when the binary is built with
// stripped-down tags in the future. Today all imports are referenced.
var _ = store.SessionMessage{}