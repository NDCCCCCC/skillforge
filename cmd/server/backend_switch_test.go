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

package main

// backend_switch_test.go —— 启动装配的**源码级回归守卫**（Phase 49 / plan
// 49-01 Task 3）。
//
// 本文件不替代 internal/retriever/embedded 的行为测试；它守的是三件靠行为
// 测试难以覆盖的事：
//
//  1. 装配分支的**存在性**。一个缺失的 `case "embedded":` 在编译期完全合法，
//     在单元测试里完全不可见，只在真实启动时爆炸。
//  2. 既有分支的**零漂移**。none / mongo / milvus 三路的源码语义必须逐字不变
//     —— 这是"新增一条并行路径而不是改写既有路径"的机械证明。
//  3. 地雷的**归属交接**。glmKey 那一行属于 49-02 改造，本 plan 只 lock 现状，
//     让下一个 plan 有明确的凭据知道"这里要动、且动完要同步改本测试"。
//
// 手法沿用 cmd/server/wiring_lock_test.go：读 main.go 文本 + 嵌套感知取块 +
// strings.Contains。零网络、零 env、不做真实启动。

import (
	"strings"
	"testing"
)

// caseBlock returns the source text of a `switch` case branch: from the
// `case <label>:` line up to (but excluding) the next sibling `case`/`default`
// at the same indentation, or the closing brace of the switch.
//
// 取块必须"到下一个同级 case 为止"而不是"到下一个 case 字样为止"——分支体
// 内部可能有嵌套的 switch/select 以及字符串里的 "case " 字样。这里用行首
// 缩进（\n\tcase / \n\tdefault / \n\t}）做锚，足够稳且不引入 go/parser 的
// 体积与版本耦合。
func caseBlock(src, label string) (string, bool) {
	needle := "\n\tcase " + label + ":"
	i := strings.Index(src, needle)
	if i < 0 {
		return "", false
	}
	rest := src[i+1:] // 去掉前导换行，让分支体从 "case ..." 开始
	return rest[:firstStop(rest)], true
}

// firstStop returns the earliest sibling-terminator offset (or len if none).
func firstStop(s string) int {
	end := len(s)
	for _, stop := range []string{"\n\tcase ", "\n\tdefault:", "\n\t}"} {
		if j := strings.Index(s, stop); j >= 0 && j < end {
			end = j
		}
	}
	return end
}

// switchBlock returns the source of the whole `switch ... { ... }` statement
// that contains the given anchor (nesting-aware, via the shared structLiteral
// brace matcher's logic).
func switchBlock(src, anchor string) (string, bool) {
	i := strings.Index(src, anchor)
	if i < 0 {
		return "", false
	}
	open := strings.Index(src[i:], "{")
	if open < 0 {
		return "", false
	}
	start := i + open
	depth := 0
	for j := start; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1], true
			}
		}
	}
	return "", false
}

// TestBackendSwitchEmbeddedHasExplicitCase is the 地雷 1 lock
// (main.go indexerBackend switch): a `case "embedded":` must exist AND must not
// perform the milvus type assertion. Falling into `default:` is a RUNTIME
// PANIC — `r.(*milvus.MilvusClient)` on an *embedded.Client — not a compile
// error, so no other test in the repo can catch it before a real boot.
func TestBackendSwitchEmbeddedHasExplicitCase(t *testing.T) {
	src := mainGoSource(t)

	sw, ok := switchBlock(src, "var indexerBackend retriever.IndexerBackend")
	if !ok {
		t.Fatal("main.go no longer declares `var indexerBackend retriever.IndexerBackend` — if the upload wiring moved, update this lock to the new site")
	}
	if !strings.Contains(sw, `case "embedded":`) {
		t.Fatal(`indexerBackend switch has no explicit case "embedded": — an embedded client falls into default:, which does r.(*milvus.MilvusClient) and PANICS at runtime (49-RESEARCH Pitfall 1)`)
	}

	emb, ok := caseBlock(sw, `"embedded"`)
	if !ok {
		t.Fatal(`could not isolate the case "embedded": branch in the indexerBackend switch`)
	}
	if !strings.Contains(emb, "embedded.Client") {
		t.Fatalf("the indexerBackend embedded branch does not reference embedded.Client; got:\n%s", emb)
	}
	if strings.Contains(emb, "milvus.") {
		t.Fatalf("the indexerBackend embedded branch performs a milvus type assertion (runtime panic); got:\n%s", emb)
	}

	// 既有 milvus 路径必须逐字仍在 default: 上（新增分支不等于搬走旧分支）。
	if !strings.Contains(sw, "milvus.NewIndexerBackend(r.(*milvus.MilvusClient))") {
		t.Fatal("the indexerBackend default: branch no longer does milvus.NewIndexerBackend(r.(*milvus.MilvusClient)) — the milvus upload path drifted")
	}
}

// TestBuildRetrieverEmbeddedCaseNeedsNoExternalDB is the D-16 lock: the
// embedded assembly must construct + ensure + seed the in-process store and
// must NOT require any external database. Requiring MONGO_URI / MILVUS_ADDRESS
// would defeat the whole point of a hermetic demo, and an envRequired(...) in
// this branch would re-introduce a fatal dependency at an eager evaluation
// point (49-RESEARCH Pitfall 2).
func TestBuildRetrieverEmbeddedCaseNeedsNoExternalDB(t *testing.T) {
	src := mainGoSource(t)

	fn, ok := structLiteral(src, "func buildRetriever(")
	if !ok {
		t.Fatal("main.go no longer defines buildRetriever — if the backend factory moved, update this lock")
	}
	emb, ok := caseBlock(fn, `"embedded"`)
	if !ok {
		t.Fatal(`buildRetriever has no case "embedded": branch — RETRIEVER_BACKEND=embedded falls into default: and demands MILVUS_ADDRESS`)
	}

	for _, want := range []string{
		"embedded.New(",
		"SKILLFORGE_VECTOR_CACHE_DIR",
		"SeedFromKnowledgeDir(",
		".Ensure(",
	} {
		if !strings.Contains(emb, want) {
			t.Fatalf("buildRetriever's embedded branch does not contain %q; got:\n%s", want, emb)
		}
	}
	for _, forbidden := range []string{"MONGO_URI", "MILVUS_ADDRESS", "envRequired("} {
		if strings.Contains(emb, forbidden) {
			t.Fatalf("buildRetriever's embedded branch contains %q — the embedded backend must boot without any external database or eager env requirement (D-16); got:\n%s", forbidden, emb)
		}
	}
}

// TestBuildRetrieverNoneBranchUnchanged is the D-06 lock: adding a fourth
// backend must not drift the three existing ones. none stays a degraded
// default-deny, mongo stays keyed on MONGO_URI, and default: stays keyed on
// MILVUS_ADDRESS.
func TestBuildRetrieverNoneBranchUnchanged(t *testing.T) {
	src := mainGoSource(t)

	fn, ok := structLiteral(src, "func buildRetriever(")
	if !ok {
		t.Fatal("main.go no longer defines buildRetriever")
	}

	none, ok := caseBlock(fn, `"none"`)
	if !ok {
		t.Fatal(`buildRetriever has no case "none": branch — the degraded default-deny posture was removed`)
	}
	if !strings.Contains(none, "none(degraded default-deny)") {
		t.Fatalf("case \"none\" no longer reports the degraded default-deny backend string; got:\n%s", none)
	}
	if !strings.Contains(none, "WARN") {
		t.Fatalf("case \"none\" no longer logs a WARN — a silent degraded backend is worse than a loud one; got:\n%s", none)
	}

	mongoCase, ok := caseBlock(fn, `"mongo"`)
	if !ok {
		t.Fatal(`buildRetriever has no case "mongo": branch`)
	}
	if !strings.Contains(mongoCase, "MONGO_URI") {
		t.Fatalf("case \"mongo\" no longer reads MONGO_URI; got:\n%s", mongoCase)
	}

	// default: 分支（milvus）必须仍读 MILVUS_ADDRESS。Go 语法里 default 不带
	// label，所以这里不能复用 caseBlock，直接定位 default: 到 block 结尾。
	i := strings.Index(fn, "\n\tdefault:")
	if i < 0 {
		t.Fatal("buildRetriever has no default: branch (the milvus path)")
	}
	// 注意 tail 从 i+1 起（跳过前导换行）：否则第一个 terminator 就是
	// tail 自身的 "\n\tdefault:"，firstStop 会返回 0 并把分支体截成空串。
	tail := fn[i+1:]
	tail = tail[:firstStop(tail)]
	if !strings.Contains(tail, "MILVUS_ADDRESS") {
		t.Fatalf("buildRetriever's default: (milvus) branch no longer reads MILVUS_ADDRESS; got:\n%s", tail)
	}
}

// TestGlmKeyLineStillEagerFor49_02 is the 地雷 2 hand-off credential.
//
// **这一行归 49-02 改造（D-11），本 plan 只 lock 现状。** 49-02 把
// `glmKey := envOr("LOCAL_API_KEY", envRequired("GLM_API_KEY"))` 改成惰性求值
// 之后，必须同步更新本测试 —— 否则未来读者会误以为这个 lock 在阻止 49-02。
// 本测试存在的意义正是"让那次改动有据可查、有门可过"。
func TestGlmKeyLineStillEagerFor49_02(t *testing.T) {
	src := mainGoSource(t)

	if got := strings.Count(src, `envRequired("GLM_API_KEY")`); got != 1 {
		t.Fatalf(`expected exactly 1 occurrence of envRequired("GLM_API_KEY") before 49-02 rewrites it, found %d — if 49-02 already landed, update this lock instead of restoring the eager evaluation`, got)
	}

	// 找到那一行，断言它同时含 LOCAL_API_KEY 的 envOr（D-11 的"本地可覆盖"语义）。
	var line string
	for _, l := range strings.Split(src, "\n") {
		if strings.Contains(l, `envRequired("GLM_API_KEY")`) {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatal(`could not locate the envRequired("GLM_API_KEY") line`)
	}
	if !strings.Contains(line, `envOr("LOCAL_API_KEY"`) {
		t.Fatalf("the GLM_API_KEY line no longer keeps the LOCAL_API_KEY override; got: %s", line)
	}

	// BGE_M3_URL 仍是必填（D-11）：embedded 后端不做进程内 embedding，端点
	// 故障必须暴露，不能被悄悄降级。
	if !strings.Contains(src, `envRequired("BGE_M3_URL")`) {
		t.Fatal(`BGE_M3_URL is no longer required — the embedded backend has no in-process embedder and must not silently degrade (D-01/D-11)`)
	}
}
