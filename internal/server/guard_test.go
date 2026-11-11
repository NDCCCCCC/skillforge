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

package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/skillforge/skillforge/internal/retriever"
)

const guardNoteSQL = `select a.policy_no 保单号, a.department_group_name 部门组, a.product_name 险种名称
from wuh_dw.etl_nonauto_policy_t a
where a.agent_code in ('431','100AG2')
and a.department_group_name = '黄冈中心支公司'
and a.inception_date between DATE('2025-01-01') and date('2025-12-31')`

func guardNote() []retriever.Hit {
	return []retriever.Hit{{
		ID:      1,
		Tier:    "personal",
		Source:  "sql-guides/030_承保清单.md",
		Content: "**涉及表/视图**: wuh_dw.etl_nonauto_policy_t\n\n```sql\n" + guardNoteSQL + "\n```",
	}}
}

func answerWithSQL(sql string) string {
	return "**结论**\n\n模板调整如下：\n\n```sql\n" + sql + "\n```\n\n**改动清单**\n- 机构槽位：黄冈 → 襄阳\n"
}

// 授权范围内的改动必须放行。这是最重要的回归：guard 上线后不能把正常
// 的取数请求挡掉。
func TestGuardAllowsSlotOnlyChange(t *testing.T) {
	gen := strings.ReplaceAll(guardNoteSQL, "'黄冈中心支公司'", "'襄阳中心支公司'")
	gen = strings.ReplaceAll(gen, "2025-01-01", "2024-01-01")
	gen = strings.ReplaceAll(gen, "2025-12-31", "2024-12-31")

	rejected, replacement := guardOperationalSQL(answerWithSQL(gen), guardNote())
	assert.False(t, rejected, "slot-only change must pass")
	assert.Empty(t, replacement)
}

// 用户的核心诉求：模型改了字段，必须拒绝输出。
func TestGuardRejectsChangedColumn(t *testing.T) {
	gen := strings.ReplaceAll(guardNoteSQL, "a.department_group_name = '黄冈中心支公司'", "a.department_name = '黄冈中心支公司'")

	rejected, replacement := guardOperationalSQL(answerWithSQL(gen), guardNote())
	require.True(t, rejected)
	assert.Contains(t, replacement, "未通过模板校验")
	assert.Contains(t, replacement, "未做任何适配")
	assert.Contains(t, replacement, guardNoteSQL, "the original template must be handed back")
}

func TestGuardRejectsExtraTable(t *testing.T) {
	gen := strings.ReplaceAll(guardNoteSQL, "from wuh_dw.etl_nonauto_policy_t a",
		"from wuh_dw.etl_nonauto_policy_t a\nleft join wuh_dw.secret_salary_t s on s.policy_no = a.policy_no")

	rejected, _ := guardOperationalSQL(answerWithSQL(gen), guardNote())
	assert.True(t, rejected)
}

// 非 SQL 答案（拒答、说明）不该被这个闸门处理。
func TestGuardIgnoresNonSQLAnswer(t *testing.T) {
	rejected, _ := guardOperationalSQL("未在取数手册中找到匹配模板，请补充机构与时间范围。", guardNote())
	assert.False(t, rejected)
}

// 语料本身有非法样本；模板不可解析时 fail-open，不能把正常回答挡掉。
func TestGuardFailsOpenOnUnparseableTemplate(t *testing.T) {
	chunks := []retriever.Hit{{Content: "```sql\nto_date('2026-06-31')\n```"}}
	rejected, _ := guardOperationalSQL(answerWithSQL(guardNoteSQL), chunks)
	assert.False(t, rejected, "unparseable template must not block the answer")
}

// 没有任何 chunk 时同样不拦。
func TestGuardNoChunks(t *testing.T) {
	rejected, _ := guardOperationalSQL(answerWithSQL(guardNoteSQL), nil)
	assert.False(t, rejected)
}

// 2026-11-11 线上回归：笔记里有另一篇模板（承保清单）与本次生成共享事实表
// 但 join 了另一张人员表。它不是这次生成的依据，表集合也不同 —— 必须
// 判定为「模板不可用作基准」而不是拿它硬比（当时 R1+R3+R5 同时误报）。
func TestGuardIgnoresLowOverlapTemplate(t *testing.T) {
	otherNote := []retriever.Hit{{
		Content: "```sql\n" +
			"select a.policy_no, c.wardname from wuh_dw.etl_nonauto_policy_t a\n" +
			"left join ids_wuh.nonauto_people_t c on a.policy_no = c.policy_no\n" +
			"where a.department_group_name = '襄阳中心支公司' and a.inception_year = '2025'\n" +
			"```",
	}}
	rejected, replacement := guardOperationalSQL(answerWithSQL(guardNoteSQL), otherNote)
	assert.False(t, rejected, "an incomplete/different recipe must not be used as the reference")
	assert.Empty(t, replacement)
}

// 反过来：生成结果**多出**模板里没有的表，不存在「模板残缺」的解释，必须拦。
func TestGuardRejectsAddedTable(t *testing.T) {
	gen := strings.ReplaceAll(guardNoteSQL, "from wuh_dw.etl_nonauto_policy_t a",
		"from wuh_dw.etl_nonauto_policy_t a\nleft join wuh_dw.secret_salary_t s on s.policy_no = a.policy_no")
	rejected, replacement := guardOperationalSQL(answerWithSQL(gen), guardNote())
	require.True(t, rejected)
	assert.Contains(t, replacement, "未通过模板校验")
	assert.Contains(t, replacement, "secret_salary_t")
}

// 跨 chunk 模板重组（DESIGN §8.4）：4KB 分片会把一份取数模板切成两段，
// 单段抽不出完整 SQL 时 guard 只能 skip —— 等于这条请求没有本体校验。
// 同一来源、id 相邻的分片必须能接回去，让违规改动仍然被拦下。
func TestGuardVerifiesAcrossAdjacentChunkFragments(t *testing.T) {
	// 切在 FROM 之前：前半只有 select 列表（无表），后半没有 SELECT 关键字，
	// 两段单独都抽不出可用模板。
	cut := strings.Index(guardNoteSQL, "\nfrom ")
	require.Positive(t, cut)
	first := "**涉及表/视图**: wuh_dw.etl_nonauto_policy_t\n\n```sql\n" + guardNoteSQL[:cut] + "\n"
	second := guardNoteSQL[cut+1:] + "\n```"

	fragments := []retriever.Hit{
		{ID: 21, Tier: "personal", Source: "sql-guides/030_承保清单.md", Content: first},
		{ID: 22, Tier: "personal", Source: "sql-guides/030_承保清单.md", Content: second},
	}

	// 违规：把谓词字段换掉。只有模板被拼回来，R3 才会报。
	gen := strings.ReplaceAll(guardNoteSQL,
		"a.department_group_name = '黄冈中心支公司'",
		"a.department_name = '黄冈中心支公司'")

	rejected, replacement := guardOperationalSQL(answerWithSQL(gen), fragments)
	require.True(t, rejected,
		"adjacent fragments must be merged so the predicate change is caught")
	assert.Contains(t, replacement, "未通过模板校验")
}

// 对照组：id 不相邻说明中间那段没进 prompt，不能硬拼 —— 拼了就是把两份
// 不同的模板缝在一起。此时仍应 fail-open（跳过校验、放行）。
func TestGuardDoesNotMergeNonAdjacentFragments(t *testing.T) {
	cut := strings.Index(guardNoteSQL, "\nfrom ")
	require.Positive(t, cut)
	first := "**涉及表/视图**: wuh_dw.etl_nonauto_policy_t\n\n```sql\n" + guardNoteSQL[:cut] + "\n"
	second := guardNoteSQL[cut+1:] + "\n```"

	fragments := []retriever.Hit{
		{ID: 21, Tier: "personal", Source: "sql-guides/030_承保清单.md", Content: first},
		{ID: 23, Tier: "personal", Source: "sql-guides/030_承保清单.md", Content: second},
	}

	gen := strings.ReplaceAll(guardNoteSQL,
		"a.department_group_name = '黄冈中心支公司'",
		"a.department_name = '黄冈中心支公司'")

	rejected, _ := guardOperationalSQL(answerWithSQL(gen), fragments)
	assert.False(t, rejected,
		"non-adjacent fragments must not be stitched together; the guard has to fail open")
}
