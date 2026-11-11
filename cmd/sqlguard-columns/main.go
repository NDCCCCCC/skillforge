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

// Command sqlguard-columns 扫描取数语料，统计顶层谓词里出现过的列，生成
// 「可调槽位白名单」的候选报告（Markdown）。
//
// 它不修改任何代码。白名单 `sqlguard.AdjustableColumns` 是一次授权面的扩大，
// 必须人工审核（PR 审核）后手工写进代码 —— 本命令只把语料事实摆出来，避免
// 靠记忆维护常量（DESIGN §8.1「白名单来源化」）。
//
// 用法：
//
//	go run ./cmd/sqlguard-columns -dir knowledge/sql-guides -out .planning/v2/adjustable-columns.md
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/skillforge/skillforge/internal/sqlguard"
)

type columnStat struct {
	column       string
	predicates   int
	literalCount int
	valueKinds   map[string]bool
	samples      map[string]bool
	adjustable   bool
}

func baseColumn(col string) string {
	if i := strings.LastIndex(col, "."); i >= 0 {
		return col[i+1:]
	}
	return col
}

func main() {
	dir := flag.String("dir", "knowledge/sql-guides", "corpus directory to scan")
	out := flag.String("out", "", "write the markdown report here (default: stdout)")
	flag.Parse()

	stats := map[string]*columnStat{}
	files, parsed, skipped := 0, 0, 0

	err := filepath.WalkDir(*dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		files++
		for _, sql := range sqlguard.ExtractTemplates(string(raw)) {
			sh := sqlguard.Extract(sql)
			if !sh.Ok {
				skipped++
				continue
			}
			parsed++
			for _, p := range sh.Predicates {
				base := baseColumn(p.Column)
				if base == "" {
					continue
				}
				st, ok := stats[base]
				if !ok {
					st = &columnStat{
						column:     base,
						valueKinds: map[string]bool{},
						samples:    map[string]bool{},
						adjustable: sqlguard.AdjustableColumns[strings.ToLower(base)],
					}
					stats[base] = st
				}
				st.predicates++
				// 只有「右侧是字面量」的谓词才是用户可能改的槽位；
				// 表达式谓词（列=列）不是。
				if p.ValueKind != "" && p.ValueKind != "expr" && p.Value != "" {
					st.literalCount++
					st.valueKinds[p.ValueKind] = true
					if len(st.samples) < 3 {
						// 语料里的值常常粘连到下一句 SQL（伪换行、未闭合引号），
						// 报告只需要「这个槽位长什么样」，所以折行压平 + 截断。
						sample := strings.Join(strings.Fields(p.Value), " ")
						if r := []rune(sample); len(r) > 48 {
							sample = string(r[:48]) + "…"
						}
						if sample != "" {
							st.samples[sample] = true
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sqlguard-columns: walk %s: %v\n", *dir, err)
		os.Exit(1)
	}

	list := make([]*columnStat, 0, len(stats))
	for _, st := range stats {
		list = append(list, st)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].literalCount != list[j].literalCount {
			return list[i].literalCount > list[j].literalCount
		}
		return list[i].predicates > list[j].predicates
	})

	var b strings.Builder
	b.WriteString("# 可调槽位白名单候选（语料统计）\n\n")
	b.WriteString("> 本文件由 `go run ./cmd/sqlguard-columns` 生成，是**候选清单**，不是授权。\n")
	b.WriteString("> 把某一列加进 `internal/sqlguard.AdjustableColumns` 属于扩大授权面，\n")
	b.WriteString("> 必须人工确认（PR 审核）——见 `.planning/v2/DESIGN.md` §8.1。\n\n")
	fmt.Fprintf(&b, "- 扫描目录：`%s`\n", *dir)
	fmt.Fprintf(&b, "- Markdown 文件：%d；可解析 SQL 块：%d；解析失败跳过：%d\n", files, parsed, skipped)
	fmt.Fprintf(&b, "- 不同谓词列：%d（其中已在白名单：%d）\n\n", len(list), countAdjustable(list))

	b.WriteString("| 列 | 谓词数 | 带字面量 | 值类型 | 已在白名单 | 语料里的值样例 |\n")
	b.WriteString("|---|---:|---:|---|---|---|\n")
	for _, st := range list {
		flagMark := "—"
		if st.adjustable {
			flagMark = "✅"
		}
		fmt.Fprintf(&b, "| `%s` | %d | %d | %s | %s | %s |\n",
			st.column, st.predicates, st.literalCount,
			joinSet(st.valueKinds), flagMark, joinSamples(st.samples))
	}

	b.WriteString("\n## 怎么用\n\n")
	b.WriteString("1. 挑出「带字面量」且尚未在白名单里的列，判断它是否属于 机构/部门/人员、险种、时间 这三类业务槽位。\n")
	b.WriteString("2. 是 → 在 `internal/sqlguard/verify.go` 的 `AdjustableColumns` 里加一行，并在 commit message 说明语料依据。\n")
	b.WriteString("3. 不是（例如 `status`、`internal_flag` 这种口径字段）→ 保持不在白名单，R4 会把它记为告警。\n")

	// 反向检查：白名单里写着但语料里从没出现过的列。它们可能来自人工判断，
	// 也可能是陈旧条目 —— 摆出来让人决定，不自动清理（授权面只能手工收窄）。
	missing := make([]string, 0, len(sqlguard.AdjustableColumns))
	for col := range sqlguard.AdjustableColumns {
		if _, ok := stats[strings.ToLower(col)]; !ok {
			missing = append(missing, col)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		b.WriteString("\n## 白名单里有、但本次语料里没出现过的列\n\n")
		b.WriteString("可能来自人工判断或别的语料来源；保留还是清理需要人工确认。\n\n")
		for _, col := range missing {
			b.WriteString("- `" + col + "`\n")
		}
	}

	report := b.String()
	if *out == "" {
		fmt.Print(report)
		return
	}
	if err := os.WriteFile(*out, []byte(report), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "sqlguard-columns: write %s: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d columns, %d files, %d parsed statements)\n", *out, len(list), files, parsed)
}

func countAdjustable(list []*columnStat) int {
	n := 0
	for _, st := range list {
		if st.adjustable {
			n++
		}
	}
	return n
}

func joinSet(set map[string]bool) string {
	if len(set) == 0 {
		return "—"
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := keys[0]
	for _, k := range keys[1:] {
		out += ", " + k
	}
	return out
}

func joinSamples(set map[string]bool) string {
	if len(set) == 0 {
		return "—"
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += " / "
		}
		out += "`" + k + "`"
	}
	return out
}
