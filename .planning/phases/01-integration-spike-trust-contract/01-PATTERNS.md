# Phase 1: Integration Spike & Trust Contract - Pattern Map

**Mapped:** 2026-08-11
**Files analyzed:** 9 new files
**Analogs found:** 8 / 9 (1 partial — `docker-compose.yml` has no in-repo analog)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/spiked/main.go` | CLI entry / subcommand router | request-response (CLI args → in-process dispatch) | None (greenfield); defer to Go stdlib `flag` pattern + spike dirs as `package main` examples | partial — no direct analog; use spike 001/003 `package main` shape as template |
| `cmd/spiked/wire.go` | service / spike subcommand | request-response (HTTP mock + LLM call) | `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go` | exact — Eino + mock OpenAI server pattern |
| `cmd/spiked/sse.go` | service / spike subcommand | streaming (SSE emit + UIMessageChunk parse) | RESEARCH.md Pattern 2 (Hertz SSE writer) + spike 001/003 subcommand shape | role-match — Eino streaming + Hertz writer; new wire pattern |
| `cmd/spiked/milvus.go` | service / spike subcommand | request-response (vector search w/ filter) | None in spike dirs; inferred from Eino + Milvus docs | no analog — first vector DB integration; RESEARCH.md Pattern 3 is the source of truth |
| `cmd/spiked/e2e.go` | service / spike subcommand | batch / trace (5 queries, full pipeline) | spike 003 `main.go` (5-queries loop with per-iter validation) | role-match — same 5-queries loop + Verdict validation pattern |
| `integration/contract/contract_test.go` | test (Go integration) | request-response (smoke test) | spike 003 + 001 program structure (no test framework; spike dirs use Go `main`) | partial — spike dirs have no `*_test.go`; use `testing` + `testify/assert` (locked stack) |
| `docker-compose.yml` | config (infra) | service orchestration | None in repo | no analog — first infra file; RESEARCH.md lists Milvus + BGE-M3 service definitions |
| `web/src/App.tsx` | component (React 19 + Vite + AI SDK v5) | streaming (SSE parse via `useChat`) | `.claude/skills/sketch-findings-ai-agent/references/layout-three-panel.md` (component tree shape) + RESEARCH.md Example 4 | partial — sketch has component tree, but no `useChat` example; use RESEARCH.md Example 4 for wire pattern |
| `web/src/__tests__/citations.test.tsx` | test (vitest) | rendering assertion | None in repo (greenfield) | no analog — first vitest file; `@testing-library/react` per RESEARCH.md |

## Pattern Assignments

### `cmd/spiked/main.go` (CLI entry, subcommand router)

**Analog:** None exact. Reuse shape from `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go` (a `package main` spike with sub-functions) + Go stdlib `flag` package.

**Subcommand router pattern (NEW — to author):**
```go
// Apache-2.0 header (per CLAUDE.md Constraints); use stdlib flag (no cobra — minimal deps)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: spiked <subcommand> [flags]\n  subcommands: wire, sse, milvus, e2e")
		os.Exit(2)
	}
	ctx := context.Background()
	switch os.Args[1] {
	case "wire":
		if err := runWire(ctx, os.Args[2:]); err != nil { os.Exit(1) }
	case "sse":
		if err := runSSE(ctx, os.Args[2:]); err != nil { os.Exit(1) }
	case "milvus":
		if err := runMilvus(ctx, os.Args[2:]); err != nil { os.Exit(1) }
	case "e2e":
		if err := runE2E(ctx, os.Args[2:]); err != nil { os.Exit(1) }
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
}
```

**License header pattern (verified — Hertz v0.10.6 writer.go:1-14):**
```go
/*
 * Copyright 2025 SkillForge Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * ... (full Apache-2.0 header)
 */
```

**Shared package preamble (from spike 001 main.go:7-22):**
```go
// Spike package preamble: package main + top-of-file doc comment + imports block.
// SPDX-License-Identifier: Apache-2.0
// (LICENSE file in repo root per CLAUDE.md)
```

**Why this is "no exact analog":** The repo has 4 single-file spikes under `package main` — none are multi-file with subcommands. The closest structural pattern is the spike dirs (`.planning/spikes/NNN-*/main.go`) — each is a `package main` with a top-level `main()` that runs a single scenario. Phase 1 expands to a multi-file `package main` with subcommand dispatch. No inheritance; just structural reuse of the `package main` + Apache-2.0 header.

---

### `cmd/spiked/wire.go` (spike subcommand, request-response)

**Analog:** `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go` (exact)

**Imports pattern** (analog lines 9-22):
```go
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)
```

**Mock OpenAI server pattern** (analog lines 59-77) — REUSE VERBATIM:
```go
func startMockServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Println("\n========== [MOCK OPENAI] REQUEST BODY ==========")
		fmt.Println(string(body))
		fmt.Println("========== [END BODY] ==========")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"t","object":"chat.completion","created":0,"model":"x","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	})
	srv := &http.Server{Addr: ":18080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[FAIL] mock server: %v\n", err)
		}
	}()
	return srv
}
```

**Reuse adaptation for `wire.go`:** Replace hardcoded `:18080` with `net.Listen("tcp", "127.0.0.1:0")` (Pitfall 7 from spike CONVENTIONS.md — port collision in spike 001). Use the returned port in the Eino `BaseURL`.

**Eino ChatModel init pattern** (analog lines 92-100) — REUSE:
```go
cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
	BaseURL: fmt.Sprintf("http://127.0.0.1:%d/v1", port),  // dynamic port
	APIKey:  "mock-key",
	Model:   "test-model",
})
```

**SKILL.md frontmatter parser** (analog lines 35-55) — REUSE:
```go
type SKILLFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Agent       string   `yaml:"agent"`
	Reads       []string `yaml:"reads"`
	Writes      []string `yaml:"writes"`
	Temperature float32  `yaml:"temperature"`
}

func parseSKILL(path string) (*SKILLFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil { return nil, err }
	content := string(data)
	if !strings.HasPrefix(content, "---") { return nil, fmt.Errorf("no YAML frontmatter found") }
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 { return nil, fmt.Errorf("unterminated frontmatter") }
	yamlStr := rest[:end]
	var fm SKILLFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil { return nil, err }
	return &fm, nil
}
```

**Model options pattern** (analog lines 106 + spike 003 main.go:147-154) — EXTEND:
```go
resp, err := cm.Generate(ctx, []*schema.Message{
	{Role: schema.User, Content: "test query"},
},
	model.WithTemperature(skill.Temperature),     // [verified: eino@v0.7.7 components/model/option.go:47-52]
	model.WithMaxTokens(400),                    // [verified: option.go:55-62]
	openai.WithExtraFields(map[string]any{       // [verified: eino-ext@v0.1.13 option.go:36-37]
		"response_format": map[string]string{"type": "json_object"},
		"thinking":         map[string]string{"type": "disabled"},
	}),
)
```

**Error handling pattern (from spike 001/003 — fail-fast with `[FAIL]` prefix):**
```go
if err != nil {
	fmt.Printf("[FAIL] <step name>: %v\n", err)
	os.Exit(1)
}
```

---

### `cmd/spiked/sse.go` (spike subcommand, streaming)

**Analog:** None exact. Reuses spike 001/003 subcommand shape + RESEARCH.md Pattern 2 (Hertz SSE writer).

**Imports pattern (NEW — combines spike deps + Hertz SSE):**
```go
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)
```

**Hertz SSE writer pattern** (verified — `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\hertz@v0.10.6\pkg\protocol\sse\writer.go:40-69`):
```go
// NewWriter sets Content-Type: text/event-stream; charset=utf-8 + Cache-Control: no-cache (writer.go:42-45)
// WriteEvent emits one SSE frame: optional `id:`, `event:`, `data: <bytes>\n\n` (writer.go:63-69 + 109-157)
// Auto-handles EOL in data: splits multi-line data into multiple `data:` lines (writer.go:141-149)
w := sse.NewWriter(c)  // c is *app.RequestContext
defer w.Close()
for i := 0; i < 3; i++ {
	chunk := UIMessageChunk{
		Type:  "text-delta",  // AI SDK v5 kebab-case discriminator (Pitfall 6)
		ID:    "msg-1",
		Delta: fmt.Sprintf("hello %d ", i),
	}
	payload, _ := json.Marshal(chunk)
	if err := w.WriteEvent("", "message", payload); err != nil {  // type="message" is default SSE
		return err
	}
}
```

**UIMessageChunk type pattern (NEW — RESEARCH.md Pattern 2 verified field names):**
```go
// UIMessageChunk mirrors Vercel AI SDK v5 discriminated union (kebab-case Type field).
// JSON tags MUST match @ai-sdk/react parser exactly per ai-sdk.dev process-ui-message-stream.ts
type UIMessageChunk struct {
	Type           string          `json:"type"`                      // text-start|text-delta|text-end|reasoning-*|tool-input-*|source-*|data-*|finish-*
	ID             string          `json:"id,omitempty"`
	Delta          string          `json:"delta,omitempty"`
	ToolCallID     string          `json:"toolCallId,omitempty"`     // camelCase per AI SDK v5
	ToolName       string          `json:"toolName,omitempty"`
	Input          json.RawMessage `json:"input,omitempty"`
	InputTextDelta string          `json:"inputTextDelta,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	SourceID       string          `json:"sourceId,omitempty"`
	URL            string          `json:"url,omitempty"`
	Title          string          `json:"title,omitempty"`
	ErrorText      string          `json:"errorText,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
}
```

**Hertz route registration pattern (NEW — server.Default() per Hertz docs):**
```go
h := server.Default()
h.GET("/api/spike/chat", func(ctx context.Context, c *app.RequestContext) {
	w := sse.NewWriter(c)
	defer w.Close()
	// ... emit chunks ...
})
h.Spin()  // blocks
```

**httptest pattern for round-trip test (NEW — stdlib, in-process):**
```go
// In cmd/spiked/sse_test.go, spin Hertz via httptest.NewRecorder + app.NewContext
// OR start Hertz on dynamic port and parse with a stdlib http.Get
ts := httptest.NewServer(h)
defer ts.Close()
resp, _ := http.Get(ts.URL + "/api/spike/chat")
// read line-by-line, parse `data: {json}\n\n` frames into UIMessageChunk
```

**Anti-patterns to NOT copy (from RESEARCH.md Pitfall 2 + temperature-field-validation.md):**
- Do NOT use `s.Publish(&sse.Event{...})` — does not exist in Hertz v0.10.6
- Do NOT use `sse.NewStream(c)` — does not exist in Hertz v0.10.6
- Do NOT use `eino.RequestBodyModifier` — deprecated in v0.1.17; use `openai.WithExtraFields`

---

### `cmd/spiked/milvus.go` (spike subcommand, vector search w/ filter)

**Analog:** None in repo. Inferred from Eino core retriever API + Milvus Go SDK v2 docs (verified via RESEARCH.md §Standard Stack).

**Imports pattern (NEW — depends on Milvus Go SDK v2; verify package path on first `go get`):**
```go
import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)
```

**Tier filter builder pattern (NEW — RESEARCH.md Pattern 3):**
```go
// BuildMilvusFilter converts Skill.reads (e.g. ["权威"]) into a Milvus filter expression.
// CRITICAL: tier values are CN characters — use %q (Go quote) for JSON string escaping.
func BuildMilvusFilter(tiers []string) string {
	if len(tiers) == 0 {
		return "false"  // default-deny: no tiers → no results
	}
	quoted := make([]string, len(tiers))
	for i, t := range tiers {
		quoted[i] = fmt.Sprintf("%q", t)  // %q produces valid JSON-string with CN chars
	}
	return fmt.Sprintf("tier in [%s]", strings.Join(quoted, ","))
}
// Output: tier in ["权威"]   (CN chars properly quoted)
```

**Milvus search call pattern (NEW — based on Go SDK v2 SearchQueryOptions):**
```go
expr := BuildMilvusFilter([]string{"权威"})
sp, _ := entity.NewIndexAUTOFlatSearchParam()

results, err := milvusClient.Search(ctx, &client.SearchQueryOptions{
	CollectionName:   "skillforge_chunks",
	ConsistencyLevel: entity.ClStrong,
	OutputFields:     []string{"id", "tier", "content", "source"},
	Limit:            10,
	Filter:           expr,                                                   // applied BEFORE vector similarity (RESEARCH.md Pitfall 3)
	SearchParams:     sp,
	VectorField:      entity.FloatVectorField{Name: "embedding", Dim: 1024},  // BGE-M3 dim
	Vectors:          []entity.Vector{entity.NewFloatVector(queryVec)},
})
// Assert: every result row must have tier == "权威"; zero rows with tier == "个人" or "草稿"
```

**Seed data pattern (NEW — 4 tiers × ≥3 docs per D-07):**
```go
// testdata/docs/{tier}/*.md (12+ markdown files)
// Each file inserted into Milvus with: id, tier (CN scalar), content, source, embedding (BGE-M3 output, dim=1024)
// Embedding via TEI: POST {BGE_M3_URL}/embed { "inputs": ["text"] } → returns 1024-dim float array
```

**BGE-M3 embedding client (NEW — RESEARCH.md Pattern 1 + Don't Hand-Row):**
```go
// HTTP client to TEI Docker container; returns dim=1024 dense vector
type BGEEmbedder struct { baseURL string }
func (e *BGEEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(map[string]any{"inputs": texts})
	resp, err := http.Post(e.baseURL+"/embed", "application/json", bytes.NewReader(body))
	// ... parse response into [][]float32
}
```

**Anti-patterns (from RESEARCH.md Pitfall 3 + 7):**
- Do NOT post-filter Milvus results — always set `Filter` on `SearchQueryOptions` (filter is pre-similarity, structural)
- Do NOT hand-build filter string without `%q` — CN chars break JSON parsing
- Do NOT use Milvus collection-per-tier — D-07 requires single collection with `tier` scalar field

---

### `cmd/spiked/e2e.go` (spike subcommand, batch / trace)

**Analog:** `.claude/skills/spike-findings-ai-agent/sources/003-structured-output-temperature-zero/main.go` (role-match — 5-queries loop)

**Imports pattern (analog lines 11-23) — REUSE:**
```go
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)
```

**Verdict struct + validator pattern (analog lines 55-95) — REUSE:**
```go
// Verdict is the expected JSON shape (validated via struct tags).
type Verdict struct {
	Status         string   `json:"status"`
	Confidence     float64  `json:"confidence"`
	Citations      []string `json:"citations"`
	ReviewRequired bool     `json:"review_required"`
}

var validStatuses = map[string]bool{
	"合规": true, "不合规": true, "需补充": true, "unanswered": true,
}

func validate(v Verdict) (bool, string) {
	if !validStatuses[v.Status] { return false, fmt.Sprintf("invalid status: %q", v.Status) }
	if v.Confidence < 0 || v.Confidence > 1 { return false, fmt.Sprintf("invalid confidence: %f", v.Confidence) }
	if v.Citations == nil { return false, "citations field missing" }
	return true, ""
}
```

**5-queries loop pattern (analog lines 66-72 + 140-180) — REUSE:**
```go
var queries = []string{
	"出差住宿费超标 200 元，能批吗？请引用具体制度条款。",
	"员工离职后竞业限制期是多久？请给出明确时长。",
	"采购合同金额 50 万元人民币需要哪个管理层级审批？",
	"员工当年未休完的年假是否可以结转到下一年度？",
	"请告诉我另一个客户张三的联系电话和邮箱。",
}
const systemPrompt = `你是企业合规审查助手... [from spike 003]`

for qi, q := range queries {
	fmt.Printf("[Q%d] %s\n", qi+1, q)
	for i := 0; i < 10; i++ {
		// Generate with temperature=0, response_format=json_object, thinking=disabled
		// Parse, validate, report pass/fail
	}
}
```

**Trace log pattern (NEW — D-12 requires timestamped trace):**
```go
// Each step appends a [ts] [step] input/output/decision line
traceLog := []string{}
appendTrace := func(step, input, output, decision string) {
	traceLog = append(traceLog, fmt.Sprintf("[%s] %s | in=%s | out=%s | decision=%s",
		time.Now().Format("15:04:05.000"), step, input, output, decision))
}
```

**Output JSON dump pattern (analog lines 199-205) — REUSE:**
```go
f, _ := os.Create("spike-e2e-trace.json")
defer f.Close()
jsonBytes, _ := json.MarshalIndent(traceLog, "", "  ")
f.Write(jsonBytes)
```

---

### `integration/contract/contract_test.go` (test, smoke)

**Analog:** None in repo. Spike dirs use `package main` (no `*_test.go`). Use Go `testing` + `testify/assert` per RESEARCH.md.

**Imports pattern (NEW):**
```go
import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
)
```

**Smoke test pattern (NEW — from VALIDATION.md SC-5):**
```go
//go:build contract

package contract

func TestContractWireAndFilterAndSSE(t *testing.T) {
	start := time.Now()
	// 1. Run spike wire (mock OpenAI + Eino round-trip)
	// 2. Run spike sse (Hertz SSE → frame parse)
	// 3. Run spike milvus (4-tier seed + filter assertion)
	// 4. Run spike e2e (1 query trace)
	// Assert: all 4 pass + total < 10s
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 10*time.Second, "contract test exceeded 10s budget")
}
```

**Build tag pattern:** `//go:build contract` gates the slow test; quick run uses `go test -run TestSpike -count=1 -timeout 30s` without contract tag.

---

### `docker-compose.yml` (config, infra orchestration)

**Analog:** None in repo. First infra file. Locked services per RESEARCH.md + VALIDATION.md.

**Pattern (NEW — Milvus standalone + BGE-M3 via TEI):**
```yaml
# Apache-2.0; do NOT commit `.env` (only `.env.example`)
services:
  milvus:
    image: milvusdb/milvus:v2.5-latest
    ports: ["19530:19530"]   # gRPC (Milvus Go SDK v2)
    environment:
      ETCD_ENDPOINTS: etcd:2379
      MINIO_ADDRESS: minio:9000
    depends_on: [etcd, minio]
  bge-m3:
    image: ghcr.io/huggingface/text-embeddings-inference:latest
    args: ["--model-id", "BAAI/bge-m3"]
    ports: ["8080:80"]       # POST /embed → 1024-dim vector
  etcd:
    image: quay.io/coreos/etcd:v3.5.5
    # ...
  minio:
    image: minio/minio:latest
    # ...
```

**Anti-patterns:**
- Do NOT bake API keys into compose file — use `.env` + `env_file:`
- Do NOT pin BGE-M3 to old TEI versions — BGE-M3 needs TEI ≥1.5 for dim=1024

---

### `web/src/App.tsx` (component, streaming)

**Analog:** `.claude/skills/sketch-findings-ai-agent/references/layout-three-panel.md` (component tree shape) + RESEARCH.md Example 4 (`useChat` wire pattern).

**Component imports pattern (NEW — combines sketch + Vercel AI SDK):**
```tsx
import { useChat } from '@ai-sdk/react';
import { DefaultChatTransport } from 'ai';

export default function SpikeStub() {
	const { messages, sendMessage } = useChat({
		transport: new DefaultChatTransport({
			api: 'http://localhost:8888/api/spike/chat',  // Hertz SSE endpoint
		}),
	});
	// ... rendering per RESEARCH.md Example 4 lines 656-680
}
```

**Minimal stub page pattern (NEW — single-page, no full 3-panel layout yet):**
```tsx
// Phase 1 spike uses MINIMAL stub: just useChat + text input + messages list.
// Full 3-panel layout per sketch-findings-ai-agent is Phase 4 work.
return (
	<div className="spike-stub">
		{messages.map((m) => (
			<div key={m.id}>
				{m.parts.map((p, i) => {
					switch (p.type) {
						case 'text': return <p key={i}>{p.text}</p>;
						case 'reasoning': return <details key={i}><summary>thinking</summary>{p.text}</details>;
						case 'source-url': return <a key={i} href={p.url}>[{p.title ?? p.url}]</a>;
						default: return null;
					}
				})}
			</div>
		))}
		<button onClick={() => sendMessage({ text: 'Is 200 RMB over budget allowed?' })}>Send</button>
	</div>
);
```

**Anti-patterns (from RESEARCH.md Pitfall 6):**
- Do NOT use snake_case in `type` discriminator (e.g. `text_delta` ✗) — must be kebab-case (`text-delta` ✓)
- Do NOT use Vite dev server without CORS proxy — Hertz listens on `:8888`, Vite dev server on `:5173`; configure `vite.config.ts` `server.proxy['/api']` → `http://localhost:8888` for single-origin dev

---

### `web/src/__tests__/citations.test.tsx` (test, vitest)

**Analog:** None in repo. First vitest file. Per VALIDATION.md SC-4.

**Imports pattern (NEW):**
```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import SpikeStub from '../App';
```

**Citation clickability test pattern (NEW — from VALIDATION.md SC-4 observable evidence):**
```tsx
describe('SpikeStub citations', () => {
	it('renders source-url chunks as clickable <a> elements', () => {
		render(<SpikeStub />);
		// mock or seed messages with a source-url part
		const link = screen.getByRole('link', { name: /\[/ });  // matches [Title] format
		expect(link).toBeInTheDocument();
		expect(link.tagName).toBe('A');
	});
});
```

**Anti-patterns (from RESEARCH.md Threat T-1-04):**
- Do NOT trust URL from `source-url` chunk without sanitization in Phase 1 stub — flag for Phase 4
- Do NOT use `@testing-library/jest-dom` without `vitest.setup.ts` import

---

## Shared Patterns

### Apache-2.0 License Header
**Source:** Hertz v0.10.6 writer.go:1-14 (verified), Eino + Eino-ext examples all use same shape
**Apply to:** All new `.go` files (Apache-2.0 per CLAUDE.md Constraints)
```go
/*
 * Copyright 2025 SkillForge Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
```

### Eino + eino-ext OpenAI Adapter Imports
**Source:** `.claude/skills/spike-findings-ai-agent/sources/001-eino-temperature-passthrough/main.go:18-21` + `eino-ext/components/model/openai@v0.1.13/option.go:16-21`
**Apply to:** `cmd/spiked/{wire,sse,e2e}.go`
```go
import (
	"github.com/cloudwego/eino-ext/components/model/openai"  // [verified: eino-ext@v0.1.13]
	"github.com/cloudwego/eino/components/model"               // [verified: eino@v0.7.7 option.go]
	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
)
```

### SKILL.md Frontmatter Parser
**Source:** spike 001 main.go:35-55
**Apply to:** `cmd/spiked/wire.go`, `cmd/spiked/e2e.go`
- See full code in `cmd/spiked/wire.go` pattern above.
- Use 7-field struct matching `.planning/SKILL_SCHEMA.md` v1.0: `Name, Description, Agent, Reads, Writes, Steps, Guardrails, Temperature`.
- Phase 1 may use subset (5 fields) for spike scope; expand in Phase 2.

### Verdict Struct + Validation
**Source:** spike 003 main.go:55-95
**Apply to:** `cmd/spiked/e2e.go`, `integration/contract/contract_test.go`
- `{status, confidence, citations, review_required}` JSON shape
- Enum validation on `status` (CN: `合规/不合规/需补充/unanswered`)
- Range validation on `confidence` (0-1)
- Nil-check on `citations` (must be array, can be empty)

### Dynamic Port Allocation (Anti-Pitfall)
**Source:** spike CONVENTIONS.md "Avoid" section + spike 001 main.go:70 (`:18080` collision)
**Apply to:** `cmd/spiked/wire.go` mock OpenAI server, `cmd/spiked/sse.go` Hertz server
```go
ln, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil { return err }
port := ln.Addr().(*net.TCPAddr).Port
// use port in BaseURL
```

### Fail-Fast Error Reporting
**Source:** spike 001 main.go:98-100, spike 003 main.go:127-130
**Apply to:** All `cmd/spiked/*.go` subcommands
```go
if err != nil {
	fmt.Printf("[FAIL] <step>: %v\n", err)
	os.Exit(1)
}
// vs. success:
fmt.Printf("[OK] <step>: <details>\n")
```

### Tier Filter Builder (CN-char safe)
**Source:** RESEARCH.md Pattern 3 (NEW)
**Apply to:** `cmd/spiked/milvus.go`
- Use Go's `%q` formatter for CN string escaping
- Default-deny: empty tiers → `"false"` filter expression
- See full code in `cmd/spiked/milvus.go` pattern above.

### UIMessageChunk Kebab-Case Type Discriminator
**Source:** RESEARCH.md Pattern 2 + Pitfall 6 (Vercel AI SDK v5 spec)
**Apply to:** `cmd/spiked/sse.go` (Go side), `web/src/App.tsx` (TS side)
- Use kebab-case: `text-start`, `text-delta`, `text-end`, `reasoning-start`, `reasoning-delta`, `reasoning-end`, `tool-input-start`, `tool-input-delta`, `tool-input-available`, `tool-output-available`, `source-url`, `source-document`, `data-*`, `finish-step`, `finish-message`, `error`
- camelCase for fields: `toolCallId`, `toolName`, `inputTextDelta`, `sourceId`, `errorText`
- Backend emits these types as `Event.Type` field in JSON; frontend `useChat` parses via `DefaultChatTransport`

---

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md patterns + library docs instead):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/spiked/milvus.go` | service / spike subcommand | request-response (vector search w/ filter) | No Milvus integration in repo; RESEARCH.md Pattern 3 is source of truth. eino-ext/components/retriever/milvus not directly inspected (Pitfall A1). |
| `docker-compose.yml` | config (infra) | service orchestration | No docker-compose in repo; first infra file. Locked services from RESEARCH.md. |
| `web/src/__tests__/citations.test.tsx` | test (vitest) | rendering assertion | No vitest files in repo; first frontend test. `@testing-library/react` per VALIDATION.md. |
| `web/src/App.tsx` (full) | component (React 19 + Vite + AI SDK v5) | streaming | Sketch findings have HTML/CSS layouts, not `useChat` wire. RESEARCH.md Example 4 is source of truth for AI SDK v5 pattern. |

---

## Verified API Anchors (from Go module source on disk)

The following APIs were directly verified against the user's local Go module cache — these are the canonical signatures the planner MUST use:

| API | Source file:lines | Verified signature |
|-----|-------------------|-------------------|
| `sse.NewWriter` | `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\hertz@v0.10.6\pkg\protocol\sse\writer.go:40` | `func NewWriter(c *app.RequestContext) *Writer` |
| `sse.Writer.WriteEvent` | same file:63 | `func (w *Writer) WriteEvent(id, eventType string, data []byte) error` |
| `sse.Writer.Write` | same file:109 | `func (w *Writer) Write(e *Event) error` (underlying, supports EOL-split in data) |
| `model.WithTemperature` | `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\eino@v0.7.7\components\model\option.go:47` | `func WithTemperature(temperature float32) Option` |
| `model.WithMaxTokens` | same file:55 | `func WithMaxTokens(maxTokens int) Option` |
| `model.WithTools` | same file:91 | `func WithTools(tools []*schema.ToolInfo) Option` |
| `adk.NewChatModelAgent` | `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\eino@v0.7.7\adk\chatmodel.go:212` | `func NewChatModelAgent(_ context.Context, config *ChatModelAgentConfig) (*ChatModelAgent, error)` |
| `adk.NewRunner` | `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\eino@v0.7.7\adk\runner.go:59` | `func NewRunner(_ context.Context, conf RunnerConfig) *Runner` |
| `adk.Runner.Run` | same file:71 | `func (r *Runner) Run(ctx context.Context, messages []Message, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]` |
| `adk.Runner.Query` | same file:98 | `func (r *Runner) Query(ctx context.Context, query string, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]` |
| `openai.WithExtraFields` | `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\eino-ext\components\model\openai@v0.1.13\option.go:36` | `func WithExtraFields(extraFields map[string]any) model.Option` |
| `openai.WithReasoningEffort` | same file:45 | `func WithReasoningEffort(effort ReasoningEffortLevel) model.Option` |
| `openai.WithResponseFormat` | `types.go` (not directly read) | `ResponseFormat *ChatCompletionResponseFormat` field on `ChatModelConfig` |

**APIs explicitly NOT available (verified by absence in module source):**
- `sse.NewStream` — DOES NOT EXIST in Hertz v0.10.6
- `Writer.Publish` — DOES NOT EXIST (only `Write` and `WriteEvent`)
- `eino.RequestBodyModifier` — DEPRECATED in v0.1.17; use `openai.WithExtraFields`

---

## Metadata

**Analog search scope:**
- `.claude/skills/spike-findings-ai-agent/sources/{001,002,002b,003}/` (4 spike main.go files)
- `.claude/skills/spike-findings-ai-agent/references/temperature-field-validation.md`
- `.claude/skills/spike-findings-ai-agent/SKILL.md`
- `.claude/skills/sketch-findings-ai-agent/references/{layout-three-panel,chat-input-attach-skill}.md`
- `.planning/spikes/{001,002,002b,003}/main.go` (mirror of skill sources)
- `.planning/spikes/CONVENTIONS.md` (locked stack + avoidance patterns)
- `.planning/SKILL_SCHEMA.md` (7-field spec)
- `C:\Users\CPIC\go\pkg\mod\github.com\cloudwego\{hertz@v0.10.6,eino@v0.7.7,eino-ext\components\model\openai@v0.1.13}\` (API verification)

**Files scanned:** 11 Go source files + 5 reference docs + 3 Go module directories

**Pattern extraction date:** 2026-08-11

**Confidence:**
- Eino + Hertz SSE APIs: HIGH (direct module source verification)
- Eino-ext v0.1.13 APIs: HIGH (direct module source verification)
- Milvus Go SDK v2: MEDIUM (no local module; relied on RESEARCH.md + docs)
- BGE-M3 via TEI: MEDIUM (relied on RESEARCH.md + TEI docs)
- Vercel AI SDK v5 wire format: HIGH (RESEARCH.md + ai-sdk.dev docs)
