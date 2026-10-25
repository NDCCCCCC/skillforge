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

// Package stubchat — extract.go implements the deterministic evidence
// extraction behind the stub LLM (Phase 49 / D-07). It parses the
// `[ref-N] tier=… source=…` evidence blocks that pipeline.buildPrompt
// renders into the user message, and renders a verdict.Verdict whose
// Answer and Citation.Span are VERBATIM substrings of the top chunk's
// body — the stub never paraphrases, never fabricates, and therefore
// passes the very same verdict.Validate / GroundingCheck / GroundingChecker
// chain a real model must pass (D-10: no exemptions).
package stubchat

import (
	"strings"
	"unicode/utf8"

	"github.com/skillforge/skillforge/internal/verdict"
)

// refBlock is one parsed `[ref-N]` evidence block from the user prompt.
// The framing format is owned by pipeline.buildPrompt:
//
//	[ref-%d] tier=%s source=%s\n%s\n---\n
type refBlock struct {
	ID      string
	Tier    string
	Source  string
	Content string
}

// blockHeaderLine reports whether line is a block header: the WHOLE line
// (no leading whitespace tolerated — chunk body text that merely contains
// a `[ref-` substring mid-line is content, not framing) matching
//
//	[ref-<digits>] tier=<non-space> source=<non-space>
//
// Line anchoring is deliberate (T-49-13): chunk bodies may contain
// markdown `---` separators; a non-greedy regex that stops at the first
// newline+`---` would truncate a block and could push a later span out of
// the extracted content, silently breaking grounding.
func blockHeaderLine(line string) (id, tier, source string, ok bool) {
	rest, found := strings.CutPrefix(line, "[ref-")
	if !found {
		return "", "", "", false
	}
	closeIdx := strings.Index(rest, "]")
	if closeIdx <= 0 {
		return "", "", "", false
	}
	id = rest[:closeIdx]
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", "", "", false
		}
	}
	fields := strings.Fields(rest[closeIdx+1:])
	if len(fields) != 2 {
		return "", "", "", false
	}
	tier, tierOK := strings.CutPrefix(fields[0], "tier=")
	source, srcOK := strings.CutPrefix(fields[1], "source=")
	if !tierOK || !srcOK || tier == "" || source == "" {
		return "", "", "", false
	}
	return id, tier, source, true
}

// extractRefBlocks scans user line-anchored for evidence blocks. A block's
// content is every line after its header up to the next header line (or
// end of input), with trailing blank lines and at most one trailing
// standalone `---` stripped — the `---` is buildPrompt's inter-block
// separator, not chunk content, but a `---` INSIDE a chunk body (before
// the next header) is preserved (T-49-13 ②).
func extractRefBlocks(user string) []refBlock {
	lines := strings.Split(user, "\n")
	var blocks []refBlock
	var cur *refBlock
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if id, tier, source, ok := blockHeaderLine(line); ok {
			blocks = append(blocks, refBlock{ID: id, Tier: tier, Source: source})
			cur = &blocks[len(blocks)-1]
			continue
		}
		if cur != nil {
			cur.Content += line + "\n"
		}
	}
	for i := range blocks {
		blocks[i].Content = trimBlockTail(blocks[i].Content)
	}
	return blocks
}

// trimBlockTail strips trailing blank lines and then at most one trailing
// standalone `---` separator line (plus any blank lines above it).
func trimBlockTail(s string) string {
	lines := strings.Split(s, "\n")
	// Drop trailing empty lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	// Drop ONE trailing standalone `---` (the framing separator).
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "---" {
		lines = lines[:len(lines)-1]
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
	}
	return strings.Join(lines, "\n")
}

// maxSpanRunes caps the extracted span at ~400 runes (an entire CJK
// paragraph fits comfortably; a 3000-byte SQL chunk yields its opening
// statement block instead of a single 22-rune line).
const maxSpanRunes = 400

// isFenceLine reports whether the line opens/closes a markdown code
// fence (``` or ~~~). Fence lines must never appear INSIDE a span: the
// stub answer renders as markdown downstream, and an unbalanced fence
// (maxSpanRunes can cut before the closing fence) would swallow the
// rest of the answer into one giant code block.
func isFenceLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// firstSpan picks the answer/citation span from chunk content: the
// CONTIGUOUS run of lines starting at the first content line (skipping
// blank lines, `#` headings, standalone `---` separators, and fence
// lines), kept verbatim including internal newlines, until maxSpanRunes
// runes have accumulated (cut at a whole-line boundary; a single
// over-long first line is rune-truncated) or the next fence line is
// reached — stopping AT a fence keeps the span contiguous (a verbatim
// substring, the GroundingCheck precondition) instead of splicing fence
// lines out. The pre-260925 behavior returned only the FIRST qualifying
// line — fine for prose regulations, but a SQL chunk's first line is
// often just `update etl_payment_due_t`, gutting the stub answer.
// Returns "" when content is all whitespace — the caller must then
// take the refused branch. An empty span must NEVER be produced: an
// empty string is a substring of everything, so it would silently pass
// GroundingCheck (strings.Contains(any, "") == true) and defeat D-10.
//
// The return value is always a verbatim substring of content (only
// leading/trailing whitespace removed), which is exactly the precondition
// GroundingCheck needs: strings.Contains(Normalize(chunk), Normalize(span)).
func firstSpan(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" || isFenceLine(trimmed) {
			continue
		}
		start = i
		break
	}
	if start < 0 {
		return ""
	}
	// Single over-long opening line: verbatim rune-truncated prefix.
	if n := utf8.RuneCountInString(lines[start]); n > maxSpanRunes {
		return string([]rune(strings.TrimSpace(lines[start]))[:maxSpanRunes])
	}
	var b strings.Builder
	count := 0
	for i := start; i < len(lines); i++ {
		if i > start && isFenceLine(lines[i]) {
			break // stop AT the next fence — span stays contiguous
		}
		add := utf8.RuneCountInString(lines[i]) + 1 // +1 newline
		if count > 0 && count+add > maxSpanRunes {
			break // whole-line boundary — span stays a verbatim substring
		}
		if count > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
		count += add
		if count >= maxSpanRunes {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

// renderVerdict deterministically maps extracted blocks to a Verdict.
//
// Refused branch (no blocks, or top block has no spannable content):
// Status="refused", Confidence=0, Citations = NON-NIL empty slice
// (verdict.Validate rejects nil), ReviewRequired=true, Mode=stub.
// It must NOT return an error — the pipeline would burn its 4 backoff
// retries on a deterministic outcome.
//
// OK branch: take blocks[0] only (slice + first element — no map
// iteration order, no randomness, so concurrent Streams are byte-identical).
// Span and Answer are both firstSpan(blocks[0].Content) — a verbatim
// substring of the chunk. Confidence is pinned to 0.5 and ReviewRequired
// to true by the stub itself (D-10): pipeline.go has no mode branch, so
// the floor must live here.
func renderVerdict(blocks []refBlock) verdict.Verdict {
	if len(blocks) == 0 {
		return refusedVerdict()
	}
	span := firstSpan(blocks[0].Content)
	if span == "" {
		return refusedVerdict()
	}
	return verdict.Verdict{
		Status:         "ok",
		Confidence:     0.5,
		Citations:      []verdict.Citation{{ChunkID: blocks[0].ID, Span: span, Tier: blocks[0].Tier}},
		ReviewRequired: true,
		Answer:         span,
		Mode:           verdict.ModeStub,
	}
}

// refusedVerdict is the shared refused shape (non-nil empty Citations —
// Validate's "citations field missing" check is nil-only).
func refusedVerdict() verdict.Verdict {
	return verdict.Verdict{
		Status:         "refused",
		Confidence:     0,
		Citations:      []verdict.Citation{},
		ReviewRequired: true,
		Mode:           verdict.ModeStub,
	}
}
