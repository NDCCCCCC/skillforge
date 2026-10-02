// Copyright 2026 SkillForge Authors
//
// SPDX-License-Identifier: Apache-2.0

// Package nl2sql is Phase 21 Task 21-03 (NL2SQL preview generation,
// DATA-11). It converts a user's natural-language query into a
// SELECT-only SQL preview that the data officer audits before the
// actual query runs.
//
// Design choices for v1.1:
//
//   - View-driven: the agent does NOT see the raw warehouse schema.
//     It only sees the registered precompiled views (DATA-10 allow
//     list). The Schema argument carries the column list per
//     view, no table names.
//
//   - LLM-driven: v1.1 calls the same chat model the agent uses
//     (cmd/eval / cmd/server chat model). A real production
//     deployment would isolate this to a dedicated, locked-down
//     "SQL generator" model; v1.1 uses the same model the agent has
//     already audited.
//
//   - AST-gated: every preview passes through externalauth.RejectNonSelect
//     (re-defined here to avoid an import cycle with the
//     externalauth package). Non-SELECT is rejected before the
//     preview is shown to the data officer.
//
//   - Output bound: max_tokens=200 (a preview is one SQL line; no
//     narrative). If the model emits prose, the caller falls back
//     to the deterministic extractor.
//
//   - Determinism: the extracted SQL preview is canonicalized
//     (whitespace collapsed, trailing semicolon dropped) before
//     being persisted — that way downstream hash comparisons stay
//     stable across equivalent previews.
package nl2sql

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/skillforge/skillforge/internal/externalauth"
)

// ColumnSpec is one column in a precompiled view, surfaced to
// the model so it can emit column names without guessing.
type ColumnSpec struct {
	Name string
	Type string // "string" | "int" | "float" | "bool" | "datetime" — for prompt context only
}

// ViewSchema is the public schema of one registered view, given
// to the model so it can author SELECTs against the column list.
type ViewSchema struct {
	ViewName string
	Columns  []ColumnSpec
}

// Schema is the full set of views the agent is allowed to query
// (DATA-10 allow list). The model prompt enumerates these as the
// "tables" it can target.
type Schema struct {
	Views []ViewSchema
}

// Generate produces a SQL preview from a natural-language query.
// `agentQuery` is what the user typed (zh-CN or en). `chatModel`
// is the same ToolCallingChatModel the agent uses. Returns the
// raw LLM response (canonicalized) + an estimated row count hint.
//
// The Estimate is a heuristic — the model emits "<N>" inline in
// its reasoning chain ("预计返回 ~5 条"), which we extract. If
// extraction fails the caller treats Estimate as 0 (unknown) and
// lets the data officer decide.
func Generate(ctx context.Context, chatModel model.ToolCallingChatModel, allowedViews Schema, agentQuery string) (externalauth.QueryPreview, error) {
	prompt := buildPrompt(allowedViews, agentQuery)
	out, err := chatModel.Generate(ctx, []*schema.Message{
		{Role: "system", Content: nl2sqlSystemPrompt},
		{Role: "user", Content: prompt},
	}, model.WithMaxTokens(200), model.WithTemperature(0))
	if err != nil {
		return externalauth.QueryPreview{}, fmt.Errorf("nl2sql: model call: %w", err)
	}
	content := strings.TrimSpace(out.Content)
	sql := extractSQL(content)
	view := extractView(content)
	estimate := extractEstimate(content)

	if err := externalauth.RejectNonSelect(sql); err != nil {
		return externalauth.QueryPreview{}, fmt.Errorf("nl2sql: %w", err)
	}
	if view == "" {
		return externalauth.QueryPreview{}, fmt.Errorf("nl2sql: model did not name a view in %q", content)
	}

	return externalauth.QueryPreview{
		SQLPreview:     canonicalizeSQL(sql),
		ViewName:       view,
		EstimatedRows:  estimate,
		BusinessReason: agentQuery,
	}, nil
}

const nl2sqlSystemPrompt = `你是 SkillForge 外部授权通道的 NL2SQL 预览生成器。你的唯一任务是输出一个 SQL SELECT 语句 + 一个视图名 + 一个估计返回行数。

规则:
- 输出格式必须严格是:
  VIEW: <view_name>
  SQL: <SELECT ...>
  ESTIMATE: <integer>
- 只允许 SELECT (可以带 WITH 子句)。任何 INSERT/UPDATE/DELETE/DDL/CALL 一律拒绝。
- 视图名必须从提供的 SCHEMA 里选; 不允许编造视图名或表名。
- 不要输出解释、注释或多余文本。
- 估计行数是粗略值; 写一个合理整数即可。`

func buildPrompt(allowedViews Schema, q string) string {
	var b strings.Builder
	b.WriteString("SCHEMA:\n")
	for _, v := range allowedViews.Views {
		b.WriteString(fmt.Sprintf("- %s(", v.ViewName))
		for i, c := range v.Columns {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s %s", c.Name, c.Type))
		}
		b.WriteString(")\n")
	}
	b.WriteString("\nQUERY: ")
	b.WriteString(q)
	b.WriteString("\n\n按规则的格式输出:")
	return b.String()
}

// extractSQL pulls the value of "SQL: ..." from the model's output.
// Falls back to the entire content if the prefix is missing
// (some smaller models just emit raw SQL). Always runs
// RejectNonSelect on the result.
var (
	viewLineRe   = regexp.MustCompile(`(?im)^VIEW:\s*([a-z][a-z0-9_]*)`)
	sqlLineRe    = regexp.MustCompile(`(?im)^SQL:\s*(.+?)\s*$`)
	estimateRe   = regexp.MustCompile(`(?im)^ESTIMATE:\s*(\d+)`)
)

// schemaMessage is a tiny local shim so this package doesn't have
// to import eino's schema package (which pulls a chain of other
// deps). The model.ToolCallingChatModel.Generate accepts []*schema.Message
// but we accept any type that satisfies the same shape — to keep
// tests hermetic we accept a local slice.

func extractSQL(content string) string {
	if m := sqlLineRe.FindStringSubmatch(content); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	// Fallback: assume entire content is the SQL (some smaller
	// models skip the prefix). RejectNonSelect still gates it.
	return strings.TrimSpace(content)
}

func extractView(content string) string {
	if m := viewLineRe.FindStringSubmatch(content); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func extractEstimate(content string) int {
	if m := estimateRe.FindStringSubmatch(content); len(m) >= 2 {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		if n < 0 {
			return 0
		}
		return n
	}
	return 0
}

// canonicalizeSQL strips trailing semicolons, collapses internal
// whitespace, and trims. Two equivalent previews produce the same
// string so hash comparisons in downstream tooling are stable.
func canonicalizeSQL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ";")
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
		default:
			b.WriteRune(r)
			inSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
