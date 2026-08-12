# Phase 5: Skill Platform & Full Knowledge Boundaries - Pattern Map

**Mapped:** 2026-08-12
**Files analyzed:** 25 (BE: 21, FE: 4)
**Analogs found:** 25 / 25 (every file has a strong in-repo analog; no greenfield)

> 写给 planner：本阶段是 ship 过的代码的延伸，不是新产物。每一个 SC 在现有代码里都有命名好的锚点（CONTEXT.md `<code_context>` 已确认）。Sentinel `0` vs nullable（Milvus `IS NULL` 在 filter 里总为 FALSE）必须采用 sentinel — 见 RESEARCH.md §Pitfall 2，本文件不再赘述。

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| **BE 新增** | | | | |
| `internal/skill/registry.go` | service/store | event-driven (fsnotify) + atomic snapshot | `internal/skill/loader.go` (当前 `cached []*Manifest` 字段) | exact |
| `internal/skill/router.go` | service | request-response + vector compute | `internal/server/pipeline.go:120-232` VerdictPipeline + `internal/retriever/mongo/rank.go:63` `cosine()` | exact |
| `internal/skill/wizard.go` | service | multi-turn LLM + file I/O | `internal/agent/critique.go:157-179` `runCritiqueLLMCall` + `internal/skill/validate.go` 7-field gate | exact |
| `internal/server/annotate.go` | controller/pipeline-step | request-response + retrieval | `internal/server/pipeline.go:145` retriever call + `internal/server/chat.go:380-405` `emitVerdictStream` | exact |
| `internal/server/redline.go` | controller/pipeline-step | request-response + conditional LLM | `internal/agent/critique.go:105-138` `MaybeCritique` gating + `runCritiqueLLMCall` | exact |
| `internal/wire/redline.go` | model/types | n/a (struct only) | `internal/wire/chunk.go:91-97` `DecisionPayload` (同包内同形态) | exact |
| **BE 修改** | | | | |
| `internal/skill/loader.go` | service | startup + parse | self — 复用 `Scan` 的 parse 逻辑到 registry | exact |
| `internal/skill/manifest.go` | model | n/a | self — 加 `DescriptionVec []float32` 字段（缓存 embedding） | exact |
| `internal/server/pipeline.go` | controller/pipeline | request-response | self — Step 10 后插入 counter-evidence + redline passes | exact |
| `internal/server/chat.go` | controller | request-response | self — `chatHandler` 入口改为路由（取代 `findSkill`） | exact |
| `internal/server/skills.go` | controller | request-response + LLM | self — `POST /api/skills` 旁挂 `/api/skills/wizard` | exact |
| `internal/server/server.go` | container | DI | self — `Deps` 加 `Registry *skill.Registry` 字段 | exact |
| `internal/wire/chunk.go` | model | n/a | self — `DecisionPayload` 加 `SkillScores/Selected`；`ValidTypes` 加 `data-redline` (17→18) | exact |
| `internal/wire/sse.go` | emitter | n/a | self — 不变（D5-04/D5-14 都走 `EmitChunk` 既有接口） | exact |
| `internal/retriever/retriever.go` | interface | n/a | self — `Document` 加 `EffectiveFrom/EffectiveTo int64`（sentinel `0`） | exact |
| `internal/retriever/mongo/rank.go` | utility | compute | self — `docRow` 加 `effective_from/to`；`rank` 不变 | exact |
| `internal/retriever/mongo/mongo.go` | backend | request-response + storage | self — `Insert` 写新字段；`Search` filter 加 `$or` 子句 | exact |
| `internal/retriever/milvus/filter.go` | utility | n/a | self — `BuildMilvusFilter` 加 effective 子句；签名增 `now int64` | exact |
| `internal/retriever/milvus/search.go` | backend | request-response | self — output fields 增列；调用方传 `time.Now().Unix()` | exact |
| `internal/retriever/milvus/seed.go` | schema | DDL | self — `EnsureCollection` 加 `effective_from/to` 两列 | exact |
| `internal/knowledge/indexer.go` | service | filesystem scan + DDL | self — `validateTiers` → `validateEffective` 校验新增字段 | exact |
| `internal/agent/callbacks.go` | service | n/a | self — 不变（SC-4 仅复用 D16 既有事件） | exact |
| **FE 新增** | | | | |
| `web/src/components/inspector/*` | component | UI re-render | `web/src/components/chat/parts/ReasoningPart.tsx` (D-08 progressive disclosure) + `MessageBubble.tsx:30-66` parts switch | exact |
| `web/src/components/graph/*` | component | UI tree | `web/src/components/chat/parts/SkillPicker.tsx:67-89` (列表+选中态) + `web/src/components/center/ChunkSourceView.tsx` | exact |
| **FE 修改** | | | | |
| `web/src/types/wire-types.ts` | types | n/a | self — `DecisionPayload` 加字段；union 加 `data-redline` 变体；`_driftCheck` 必须保持 true | exact |
| `web/src/components/chat/MessageBubble.tsx` | dispatcher | UI | self — `renderPart` switch 加 `case 'data-redline'` | exact |
| `web/src/components/chat/parts/SkillPicker.tsx` | component | UI | self — 接收 `scores` prop 支持 D5-03 ambiguity 选择 | exact |
| `web/src/components/center/VerdictCard.tsx` | component | UI | self — 加 `counterEvidenceChunks` prop（SC-6 annotation card） | exact |
| `web/src/components/center/ChunkSourceView.tsx` | component | UI | self — 加 chunk 图节点点击跳转支持（SC-5 graph→chunk detail） | exact |

---

## Pattern Assignments

### `internal/skill/registry.go` (NEW, service/store)

**Analog:** `internal/skill/loader.go` (整个 `cached []*Manifest` 机制 + `Scan` 内的 `parseFrontmatter → Unmarshal → Validate → ResolveTiers` 流水线)

**替换 loader 内的 `cached` 字段为 `atomic.Pointer[map[string]*Entry]`**，新增 `Watch(ctx, skillsDir)` goroutine + 1s debounce。每个 `Entry` 包含 `Manifest`、`DescVec []float32`（BGE-M3 嵌入缓存）、`Version int64`。

**核心模式：原子快照发布**（D5-06 in-flight version pinning）
```go
// 仿照 internal/skill/loader.go:51-54 当前结构：
//   type Loader struct {
//       skillsDir string    // absolute path; rejected-against root for symlinks
//       cached    []*Manifest // populated by Scan
//   }
// 改为：
type Registry struct {
    skillsDir  string
    snapshot   atomic.Pointer[map[string]*Entry] // immutable; copy-on-write
    embedder   embedder.Embedder
    fsDebounce *time.Timer
}

type Entry struct {
    Manifest *skill.Manifest
    DescVec  []float32 // BGE-M3 of "name + description" (D5-01)
    Version  int64     // monotonic; bumped on each reload
}
```

**Watcher debounce 模式**（D5-07 + D5-08：validation 失败保留旧版本）
```go
// Source: 仿照 fsnotify issue #553 canonical debounce + internal/skill/loader.go:98-166
// V12 ASVS symlink guard 必须沿用：loader.go:102-119 的 EvalSymlinks 检查要在 reloadOne 入口重跑。
func (r *Registry) reloadOne(path string) {
    raw, err := os.ReadFile(path)
    if err != nil { /* log + keep old */ return }
    front, err := parseFrontmatter(raw) // 复用 internal/skill/loader.go:186-210
    if err != nil { r.emitLoadError(path, err); return } // D5-08: never silent-drop
    var m skill.Manifest
    if err := yaml.Unmarshal(front, &m); err != nil { r.emitLoadError(path, err); return }
    if err := skill.Validate(&m); err != nil { r.emitLoadError(path, err); return }
    tiers, _ := skill.ResolveTiers(m.Reads)
    m.SetResolvedTiersForTest(tiers) // 见 internal/skill/manifest.go:97-99 既有 test affordance

    descVec, _ := r.embedder.Embed(context.Background(), m.Name + " " + m.Description)
    next := r.snapshot.Load().clone()
    next[m.Name] = &Entry{Manifest: &m, DescVec: descVec, Version: next[m.Name].Version + 1}
    r.snapshot.Store(next) // atomic.Pointer — in-flight readers keep their Load()ed snapshot
}
```

**目录级 watch 而非文件级**（Pitfall 1）
```go
// Source: 仿照 fsnotify canonical pattern (issue #553) — editors + wizard 用 temp+rename 原子保存。
// 文件级 watch 在 rename 时失效。
w.Add(skillsDir) // 目录，不是 SKILL.md
// 服务端过滤：
if !strings.HasSuffix(event.Name, "/SKILL.md") { continue }
```

---

### `internal/skill/router.go` (NEW, service)

**Analog:** `internal/server/pipeline.go:120-232` VerdictPipeline (10 步编排入口) + `internal/retriever/mongo/rank.go:63-82` `cosine()` (字面量复用)

**职责：** 用 BGE-M3 嵌入查询，对每个 Skill 的 `name + description` 嵌入做 cosine，选 top-1；top-1 低分或 top-1/top-2 接近 → ambiguous。

**复用 cosine()**（D5-01，不重新实现）
```go
// Source: verbatim from internal/retriever/mongo/rank.go:63-82 (battle-tested, float32 bit-exact across platforms)
// 推荐在 router.go 内嵌一份或抽取到 internal/skill/cosine.go 让两个包共用：
//   import "m" from internal/retriever/mongo? 不行 — 内部包禁止反向依赖 skill→retriever (// manifest.go:34-36 包所有权约束)。
// 解决：把 cosine() 提到 internal/skill/router.go 的私有 helper（N<100 路径，足够专用），
// 并在 mongo/rank.go 顶部加一行 godoc 注明 "Phase 5 router copies this — keep in sync"，
// 避免两个 cosine 漂移。或者更好：抽到 internal/vector/cosine.go，两个包都引。
func cosine(a, b []float32) float32 {
    if len(a) != len(b) { panic("cosine: length mismatch") }
    var dot, na, nb float32
    for i := range a { dot += a[i]*b[i]; na += a[i]*a[i]; nb += b[i]*b[i] }
    if na == 0 || nb == 0 { return 0 }
    return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
```

**routing trace emit 模式**（D5-04 扩展既有 data-decision，不开新 wire 类型）
```go
// Source: 仿照 internal/server/chat.go:317-323 emitDecision (per-request closure over *app.RequestContext)
// 新 decision 取值 "route" 加到 internal/wire/chunk.go:91-97 DecisionPayload.Decision 枚举。
func (r *Router) EmitRoute(c *app.RequestContext, scores []Score, selected string, ambiguous bool) error {
    payload, _ := err := json.Marshal(wire.DecisionPayload{
        Step: 0, Decision: "route",
        SkillScores: scoresToMap(scores),
        Selected:    selected,
    })
    if err != nil { return err }
    return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload})
}
```

**Ambiguity picker emit**（D5-03：复用前端 SkillPicker 的 schema）
```go
// 复用 internal/server/skills.go:42-51 listSkillsHandler 的 Skills() 输出 schema，
// 加 scores 字段以便前端做 candidate 选择（前端 SkillPicker.tsx 加 scores prop 即可）。
// emit 走 source-document 还是 source-url？不合适 — ambiguity 是结构性 UI 信号，
// 不需要 wire 新类型；用一个 data-decision + SkillScores 字段即可。
```

---

### `internal/skill/wizard.go` (NEW, service)

**Analog:** `internal/agent/critique.go:157-179` `runCritiqueLLMCall`（system + user 双 message 的 LLM 调用 + JSON parse 模式）+ `internal/skill/validate.go:48-122` `Validate`（wizard 输出必须过的 7 字段门）+ `internal/skill/loader.go:186-210` `parseFrontmatter`（写文件前要生成的 frontmatter 形状）

**多轮对话模式**（D5-10 guided interview）
```go
// Source: 仿照 internal/agent/critique.go:157-179 单次 LLM 调用结构。Wizard 是 N 轮；用 React-style
// state machine 维护 interview state（purpose / reads / type / guardrails / temperature）。
// 每轮 emit 一个 "wizard-step" SSE chunk（前端 interviewer UI 渲染问题）→
// 用户回复 → 进下一轮。最后一轮返回完整 SKILL.md 前必须跑 skill.Validate()。

type WizardState struct {
    Step          int  // 0..N
    Purpose       string
    Reads         []string
    SkillType     string  // compliance/qa/creative
    Guardrails    []string
    Temperature   *float32
}

// 引导 prompt 模板（仿 critiquePromptTemplate:59-65 的 system-only prompt 形态）：
const wizardInterviewSystemPrompt = `You are a Skill authoring assistant. Ask ONE question at a time about the user's intent. After enough information, output the complete SKILL.md in YAML+markdown.`
```

**结构化输出 + 7-字段验证**（Pitfall 4 闭合环）
```go
// Source: 仿照 internal/server/pipeline.go:180-183 的 response_format + thinking disabled
// 选项注入 + internal/skill/validate.go:48 Validate() 必经门。
// spike 003 验证 response_format=json_object 在 GLM-5.2 上 100% schema-valid（沿用）。

// 预览阶段：marshal 完的 manifest 立即跑 Validate；失败返回 wizard-step 类型 "validation-error" 给前端展示。
// 用户确认后：写文件到 skills/<name>/SKILL.md — 然后 fsnotify（registry.Watch）自动 reload。
// 路径净化（V12 ASVS）—— 仿 internal/skill/loader.go:104-119：
name := sanitizeName(userInput) // ^[a-z0-9-]+$ + max 64 chars
if !validSkillNameRE.MatchString(name) { return errorRefusal("invalid skill name") }
path := filepath.Join(skillsDir, name, "SKILL.md")
```

**Temperature 推荐**（D5-12 by Skill type）
```go
// Source: 仿照 internal/skill/validate.go:116-121 的 SYSK-08 校验。
// 推荐值 (0.0 for compliance, 0.2 for QA, 0.7 for creative) 写入 wizard 引导模板的 system prompt。
// 用户可 override —— 流程是 wizard 提问 → 用户回答 → wizard 写入 manifest.Temperature。
```

---

### `internal/server/annotate.go` (NEW, controller/pipeline-step)

**Analog:** `internal/server/pipeline.go:140-148` retriever.Search call + `internal/server/pipeline.go:230-232` emitVerdictStream insertion point

**关键不变量（counter-evidence ≠ grounding）** —— 必须用独立函数 + 独立 chunk map（D5-13）

```go
// Source: 仿照 internal/server/pipeline.go:145-148 调用 Retriever.Search。
// 关键区别：tiers=["历史"] 而非 manifest.ResolvedTiers()；返回的 chunks 绝不进入
// pipeline.go:207-210 的 chunksByID 映射（那是 GroundingChecker 读的 map）。

func (s *Deps) emitCounterEvidence(ctx context.Context, c *app.RequestContext, qvec []float32) error {
    // 第二条 Search 调用 —— 唯一一处 历史 进入路径。
    // 与 verdict pipeline 的 Search 共用 qvec（同源简洁）。Open Q2 in RESEARCH.md:
    // 是否需要基于 verdict citations 的 refined query？CONTEXT.md 沉默 → 先用同 qvec。
    histChunks, err := s.Retriever.Search(ctx, qvec, []string{skill.TierEnum["historical"]}, 5)
    if err != nil {
        // 非致命：counter-evidence 是 annotation，不阻断 verdict 流程。
        slog.Warn("counter-evidence search failed", "err", err)
        return nil // 已 emit verdict；annotation 失败不冒泡。
    }
    for _, ch := range histChunks {
        // 复用 source-document 类型 —— 不新增 wire 类型（D5-13 + RESEARCH.md §Counter-evidence Pattern）。
        // marker 通过 title 字段标注（前端 UI 可以筛出）。
        _ = wire.EmitChunk(c, wire.UIMessageChunk{
            Type:     "source-document",
            ID:       "cnt-" + fmt.Sprintf("%d", ch.ID),
            SourceID: fmt.Sprintf("%d", ch.ID),
            Title:    ch.Source + " (counter-evidence)",
        })
        // 同时 emit data-policy 不合适；用 data-subprogress 模式？
        // 最佳：复用 source-document 但 TierBadge 渲染时标红（前端 VerdictCard 加 counterEvidenceChunks prop）。
    }
    return nil
}

// 插入点：pipeline.go:232 之后（emitVerdictStream 已完成）。
//   return s.emitCounterEvidence(ctx, c, qvec)  // 注释：从 qvec 是 local var；需要把它从 step 2 提到顶部。
```

---

### `internal/server/redline.go` (NEW, controller/pipeline-step)

**Analog:** `internal/agent/critique.go:105-138` `MaybeCritique`（gating 模式，conditional LLM call） + `internal/agent/critique.go:157-179` `runCritiqueLLMCall`（JSON 输出 + parse）

**条件门控**（Pitfall 5：D5-14 严格 "no gap → no call"）
```go
// Source: 仿照 internal/agent/critique.go:110-113 的 gating 形态（multiHop / status / confidence band）。
// Redline 的 gate 更简单：verdict.Status ∈ {"不合规", "需补充"}。

func (s *Deps) MaybeRedline(ctx context.Context, c *app.RequestContext, v verdict.Verdict, manifest *skill.Manifest) error {
    // 严格门控 —— Pitfall 5：合规 verdict 绝不能 emit redline chunk（cost control）。
    if v.Status != "不合规" && v.Status != "需补充" {
        return nil // no LLM call
    }

    // 仿 critique.go:runCritiqueLLMCall 的 system + user message 结构。
    prompt := buildRedlinePrompt(v, manifest)
    out, err := s.ChatModel.Generate(ctx, []*schema.Message{
        schema.SystemMessage(redlineSystemPrompt),
        schema.UserMessage(prompt),
    })
    if err != nil {
        // 非致命（仿 critique.go:127-128）：redline 是建议，verdict 已稳定。
        slog.Warn("redline LLM call failed", "err", err)
        return nil
    }
    var parsed wire.RedlinePayload
    if err := json.Unmarshal([]byte(out.Content), &parsed); err != nil {
        slog.Warn("redline JSON parse failed", "err", err, "raw", truncate(out.Content, 200))
        return nil
    }
    payload, _ := json.Marshal(parsed)
    return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-redline", Data: payload})
    // 注意 data-redline 必须先注册到 internal/wire/chunk.go:121-139 ValidTypes（17→18）。
}

// 插入点：pipeline.go:232 emitVerdictStream 之后、emitCounterEvidence 之前/之后均可。
// 推荐顺序：verdict → counter-evidence → redline（redline 最后，最贵）。
```

**LLM call 模板**（D5-12 temperature 来自 manifest）
```go
// 仿照 internal/server/pipeline.go:176-190 的 temperature + response_format + thinking:disabled 三件套。
// 模型失败 → 静默 + log（仿 critique.go:127 _ = err 的明确处理）。
```

---

### `internal/wire/redline.go` (NEW, model/types)

**Analog:** `internal/wire/chunk.go:91-97` DecisionPayload（同包同形态）

```go
// Source: 与 DecisionPayload 完全同形态 —— 7 行字段，3 个必有 + 0 个 optional，
// 加 json tags 对齐 wire 包既有规范。
package wire

// RedlinePayload is the JSON shape carried by data-redline SSE chunks (D5-14, 17→18 wire type).
// Only emitted when verdict flags a policy gap (status=不合规 or 需补充).
// Schema (planner defines exact field set per Open Question in RESEARCH.md §3):
//   - OriginalClause:  the verbatim policy text flagged as insufficient.
//   - ProposedRewrite: LLM-suggested clause-level diff.
//   - Rationale:       why the rewrite closes the gap.
type RedlinePayload struct {
    OriginalClause  string `json:"original_clause"`
    ProposedRewrite string `json:"proposed_rewrite"`
    Rationale       string `json:"rationale"`
}

// 注意：必须在 internal/wire/chunk.go:121-139 ValidTypes 注册 "data-redline": true，
// 否则 EmitChunk 会拒写（IsValidType 走 chunk.go:145-147）。
```

---

### `internal/server/pipeline.go` (MODIFIED, controller/pipeline)

**Analog:** self — Step 10 (`emitVerdictStream` at line 232) 是插入点

**插入 post-verdict passes（D5-13 + D5-14）**
```go
// Source: 仿照现有 Step 10 的顺序 —— 现有 pipeline.go:230-232 是
//   1. verdict stream
//   2. (insert counter-evidence) ← Phase 5 SC-6
//   3. (insert conditional redline) ← Phase 5 SC-7
// 现有 Step 1-9 不变（路由前缀在 chat.go 的 chatHandler 里 —— 见下条）。

// Step 10: SSE emit (CMPL-02 + SC-6 + AGNT-03).
if err := emitVerdictStream(c, v, chunks); err != nil {
    return err
}
// Phase 5 SC-6: post-verdict counter-evidence annotation pass.
// chunks 局部变量是 []retriever.Hit —— emitCounterEvidence 需要 qvec；pipeline.go:136 qvec 还在 scope。
if err := s.emitCounterEvidence(ctx, c, qvec); err != nil {
    slog.Warn("counter-evidence failed (non-fatal)", "err", err)
}
// Phase 5 SC-7: conditional redline pass (gated on verdict gap).
if err := s.MaybeRedline(ctx, c, v, manifest); err != nil {
    slog.Warn("redline failed (non-fatal)", "err", err)
}
return nil
```

**路由前移（路线 A — chatHandler 在调 VerdictPipeline 之前先调 Router）**

`chatHandler` 不直接 `findSkill`，而是 router 选完 → 拿到 manifest → 调 `VerdictPipeline`。详见下条 `chat.go` 修改。

---

### `internal/server/chat.go` (MODIFIED, controller)

**Analog:** self — 现有 `findSkill` 逻辑（line 350-364）被 router 取代

**路由入口**（D5-05 route-then-run）
```go
// Source: 替换 internal/server/chat.go:124-134 的 findSkill + VerdictPipeline 调用块。
// 仿照 internal/server/pipeline.go:120-232 的 signature：
//   func (s *Deps) VerdictPipeline(ctx context.Context, c *app.RequestContext, query string, manifest *skill.Manifest) error
// Router 同样需要 emitChunk 能力 —— Deps 加 Registry 字段后通过 s.Registry.Route(...)

// Step 1 (Phase 5): 路由 → 选 manifest。
// ambiguous → emit data-decision with SkillScores + selected=""; 等用户选完再 continue。
routeResult, err := s.Registry.Route(ctx, c, req.Query) // emit data-decision trace inline
if err != nil {
    _ = emitRefusal(c, verdict.Refusal{Reason: verdict.RefusalReasonOutOfScope, ...})
    return
}
if routeResult.Ambiguous {
    // emit data-decision with SkillScores + selected="" → 前端 SkillPicker 接 scores
    // 直接 emit 一组 source-document 风格的 candidate list (前端 SkillPicker 复用)。
    return // 等前端用户选择 → 再发起 /api/chat with explicit skill
}
manifest := routeResult.Manifest
if err := s.VerdictPipeline(ctx, c, req.Query, manifest); err != nil {
    _ = wire.EmitChunk(c, errorChunk(err.Error()))
    _ = wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"})
    return
}
```

---

### `internal/server/skills.go` (MODIFIED, controller)

**Analog:** self — `validateSkillHandler` (line 67-90) 是最近的 POST handler

**Wizard 端点**（D5-10/11）
```go
// Source: 仿照 internal/server/skills.go:67-90 validateSkillHandler 的 status code 模式
// （400 不可解析 body / 200 + {valid:true/false, error:""}）。
// Wizard 多轮 → POST body shape 是 {state, user_input}；response 是 {state, prompt, options, preview?}。

func (s *Deps) wizardSkillHandler(_ context.Context, c *app.RequestContext) {
    // POST /api/skills/wizard (D5-11: 内部走 wizard.go 的多轮 state machine)
    // 最终 step 完成后：
    //   1. 跑 skill.Validate (来自 internal/skill/validate.go:48-122)
    //   2. 写文件到 skills/<name>/SKILL.md (V12 ASVS 路径净化)
    //   3. 返回 200 + {preview, valid, error:""}
    //   4. fsnotify（registry.Watch）自动 reload — 闭合环 (D5-11)
}

// 注册到 internal/server/server.go:139-144 New() 函数。
```

---

### `internal/server/server.go` (MODIFIED, container)

**Analog:** self — 现有 `Deps` 结构（line 116-127）

```go
// Source: 仿照 internal/server/server.go:116-127 Deps struct。
// 加 Registry 字段（替换 Loader 字段引用 —— Loader 保留用于 /api/skills 列表，
// Registry 接管 hot-reload + routing 入口）。

type Deps struct {
    Loader    *skill.Loader       // 保留：listSkillsHandler 还要用 cached Skills()
    Registry  *skill.Registry     // Phase 5 新增：routing + hot-reload source of truth
    Embedder  Embedder
    Retriever Retriever
    Runner    Runner
    Agent     *agent.Agent
    Store     store.Store
    ChatModel model.ToolCallingChatModel // Phase 5 SC-7 redline pass 直接 LLM call（不走 Runner/Agent）。
}
```

注册新路由：
```go
// Source: 仿照 internal/server/server.go:141-143 的 h.POST / h.GET 模式。
h.POST("/api/skills/wizard", deps.wizardSkillHandler)
h.GET("/api/knowledge/graph", deps.knowledgeGraphHandler) // Phase 5 SC-5 graph browser 数据源
```

---

### `internal/wire/chunk.go` (MODIFIED, model)

**Analog:** self

```go
// Source: 仿照 internal/wire/chunk.go:91-97 现有 DecisionPayload。
// 加路由 trace 字段（D5-04）+ 加 "route" 到 Decision 枚举（Open Question 3 in RESEARCH.md: emit BEFORE verdict）。

type DecisionPayload struct {
    Step       int     `json:"step"`
    Decision   string  `json:"decision"`  // 现有枚举 + "route"
    ToolCallID string  `json:"toolCallId,omitempty"`
    Query      string  `json:"query,omitempty"`
    Confidence float64 `json:"confidence,omitempty"`
    // D5-04 additions:
    SkillScores map[string]float64 `json:"skillScores,omitempty"` // name → cosine
    Selected    string             `json:"selected,omitempty"`
}

// ValidTypes (line 121-139): 加 "data-redline": true —— 17→18。
// 注释同步更新 line 118-120 的 count 文字 + 注释段（line 99-117）的 history 段。
```

---

### `internal/retriever/retriever.go` (MODIFIED, interface)

**Analog:** self

```go
// Source: 仿照 internal/retriever/retriever.go:45-51 现有 Document。
// 加 EffectiveFrom/EffectiveTo int64（sentinel 0 = perpetually valid —— 见 RESEARCH.md §Pitfall 2）。

type Document struct {
    ID             int64
    Tier           string
    Content        string
    Source         string
    Embedding      []float32
    EffectiveFrom  int64  // D5-15: epoch seconds; 0 = unspecified (perpetual)
    EffectiveTo    int64  // D5-15: epoch seconds; 0 = perpetually valid (sentinel — NOT nullable)
    // 务必 NOT nullable — 见 RESEARCH.md §Pitfall 2 + §Counter-evidence Pattern 注释。
}

// Search signature 不变（tiers + topK）；backend 在内部把 EffectiveFrom/To 应用到 PRE-filter。
// Document.Insert 顺序保证 backend 同时接受 5-field 老 chunk（zeros 默认）。
```

---

### `internal/retriever/mongo/mongo.go` (MODIFIED, backend)

**Analog:** self

```go
// Source: 仿照 internal/retriever/mongo/mongo.go:285-303 Insert 现有模式（bson.M 映射）。
// 加 effective_to 写入；Search 的 bson filter 加 $or 子句。

// Insert (line 285):
docsAny[i] = bson.M{
    "id":             d.ID,
    "tier":           d.Tier,
    "content":        d.Content,
    "source":         d.Source,
    "embedding":      d.Embedding,
    "effective_from": d.EffectiveFrom, // 0 if absent (perpetual)
    "effective_to":   d.EffectiveTo,   // 0 if absent (perpetual)
}

// Search (line 320): filter 加 $or —— 注意 OR 与 tier 必须 AND 关系：
filter := bson.D{
    {Key: "tier", Value: bson.D{{Key: "$in", Value: tiers}}},
    {Key: "$and", Value: bson.A{
        bson.D{{Key: "$or", Value: bson.A{
            bson.D{{Key: "effective_to", Value: 0}}, // perpetually valid
            bson.D{{Key: "effective_to", Value: bson.D{{Key: "$gte", Value: nowSec}}}},
        }}},
    }},
}

// Ensure (line 188): 加 {effective_to: 1} 索引（仿现有 {tier:1}）。
// 现在加 multi-index CreateOne 或 CreateMany；测试 mock 同步覆盖。
```

---

### `internal/retriever/mongo/rank.go` (MODIFIED, utility)

**Analog:** self

```go
// Source: 仿照 internal/retriever/mongo/rank.go:49-55 docRow 现有字段。
// 加 effective_from/to。rank() 函数本身不变（effective 已经在 Search 的 filter 层应用）。

type docRow struct {
    ID             int64     `bson:"id"`
    Tier           string    `bson:"tier"`
    Content        string    `bson:"content"`
    Source         string    `bson:"source"`
    Embedding      []float32 `bson:"embedding"`
    EffectiveFrom  int64     `bson:"effective_from"`
    EffectiveTo    int64     `bson:"effective_to"`
}
```

---

### `internal/retriever/milvus/filter.go` (MODIFIED, utility)

**Analog:** self

```go
// Source: 仿照 internal/retriever/milvus/filter.go:36-50 BuildMilvusFilter 现有签名 + tier quoting 模式。
// 加 now int64 参数 + effective 子句（sentinel 设计 —— 见 RESEARCH.md §Pitfall 2）。

func BuildMilvusFilter(tiers []string, now int64) string {
    if len(tiers) == 0 { return "false" } // default-deny 不变 (D-09)
    quoted := make([]string, len(tiers))
    for i, t := range tiers { quoted[i] = fmt.Sprintf("%q", t) }
    // tier AND (perpetual OR not-yet-expired). Sentinel 0 = perpetual.
    return fmt.Sprintf(
        "tier in [%s] and (effective_to == 0 or effective_to >= %d)",
        strings.Join(quoted, ","), now,
    )
}

// 注释必须保留 line 27-35 的"false is default-deny"段（攻击面没变）。
// 调用方（search.go）传 time.Now().Unix()。
```

---

### `internal/retriever/milvus/search.go` (MODIFIED, backend)

**Analog:** self

```go
// Source: 仿照 internal/retriever/milvus/search.go:52 现有 output fields 列。
// 加 "effective_from", "effective_to" 到 output fields；同时 BuildMilvusFilter 加 now 参数。

results, err := m.mc.Search(
    ctx, cfg.CollectionName, nil,
    BuildMilvusFilter(tiers, time.Now().Unix()), // ← 加 now
    []string{"id", "tier", "content", "source", "effective_from", "effective_to"}, // ← 加列
    []entity.Vector{entity.FloatVector(queryVec)},
    "embedding", entity.COSINE, topK, sp,
    client.WithSearchQueryConsistencyLevel(entity.ClStrong),
)
// row.EffectiveFrom / row.EffectiveTo 字段从 sr.Fields.GetColumn(...).GetAsInt64(i) 解析。
```

---

### `internal/retriever/milvus/seed.go` (MODIFIED, schema)

**Analog:** self

```go
// Source: 仿照 internal/retriever/milvus/seed.go:59-64 现有 schema 构造（entity.NewField + WithField）。
// 加 effective_from/to 两个 INT64 scalar field（非 nullable —— 见 RESEARCH.md §Pitfall 2）。

schema := entity.NewSchema().WithName(cfg.CollectionName).WithAutoID(false).
    WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(false)).
    WithField(entity.NewField().WithName("tier").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultTierMaxLen))).
    WithField(entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultContentMaxLen))).
    WithField(entity.NewField().WithName("source").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(DefaultSourceMaxLen))).
    WithField(entity.NewField().WithName("effective_from").WithDataType(entity.FieldTypeInt64)).  // D5-15
    WithField(entity.NewField().WithName("effective_to").WithDataType(entity.FieldTypeInt64)).    // D5-15
    WithField(entity.NewField().WithName("embedding").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(cfg.EmbedDim)))

// SeedDocuments (line 104-148): 同样的 column arrays 加 EffectiveFrom, To；
// 老 corpus 重 seed 时写 0（perpetual sentinel）—— 无 migration。
```

---

### `internal/knowledge/indexer.go` (MODIFIED, service)

**Analog:** self — `validateTiers` (line 266-278) 是参考

```go
// Source: 仿照 internal/knowledge/indexer.go:266-278 validateTiers 现有模式（循环 + fmt.Errorf with doc index/source）。
// 加 validateEffective 校验；frontmatter parse 时读 effective_from/to (可选)。

func validateEffective(docs []retriever.Document) error {
    for i, d := range docs {
        if d.EffectiveTo < 0 {
            return fmt.Errorf("knowledge.Index: doc[%d] source=%s has negative effective_to (D5-15)",
                i, d.Source)
        }
        if d.EffectiveFrom != 0 && d.EffectiveTo != 0 && d.EffectiveTo < d.EffectiveFrom {
            return fmt.Errorf("knowledge.Index: doc[%d] source=%s: effective_to < effective_from (D5-15)",
                i, d.Source)
        }
    }
    return nil
}

// 调用点（line 220 validateTiers 之后）：
if err := validateEffective(docs); err != nil { return err }

// Index 主循环（line 164-209）读 frontmatter → 解析 effective_from/to 字段 → 填入 Document。
// 复用 internal/skill/loader.go:186-210 parseFrontmatter（同一个 frontmatter strip 逻辑）。
// 没有 effective_* 的旧 corpus → 默认 0（perpetual）—— 向后兼容。
```

---

### `internal/agent/callbacks.go` (UNCHANGED — SC-4 reuses existing events)

**Analog:** self — 不变（D16 decision + tool/reasoning events 已经是 SC-4 inspector 的 data source）。

CONTEXT.md Claude's Discretion 明确：SC-4 inspector 是 UI 重渲染，不是新 instrumentation channel。`newDecisionCallback` (line 113-153) 已经 emit data-decision，inspector 直接消费。

---

### `web/src/types/wire-types.ts` (MODIFIED, types)

**Analog:** self

```typescript
// Source: 仿照 web/src/types/wire-types.ts:60-70 现有 DecisionPayload 接口形态。
// 必须保持 line 117-122 的 _driftCheck true —— D15 mirror invariant。

export interface DecisionPayload {
  step: number;
  // 加 'route'（Open Question 3 in RESEARCH.md: emit BEFORE verdict）：
  decision: 'retrieve' | 'answer' | 'refuse' | 'forced_stop' | 'route';
  toolCallId?: string;
  query?: string;
  confidence?: number;
  // D5-04 additions:
  skillScores?: Record<string, number>;
  selected?: string;
}

// D5-14: 加 RedlinePayload + 新的 union 变体：
export interface RedlinePayload {
  original_clause: string;
  proposed_rewrite: string;
  rationale: string;
}

// 在 SkillForgeChunk union（line 76-108）加：
//   | { type: 'data-redline'; id?: string; data: RedlinePayload }
//
// 加完后 _driftCheck 必须仍然为 true —— 否则 D15 编译期 alarm 会炸。
```

---

### `web/src/components/inspector/*` (NEW, FE component)

**Analog:** `web/src/components/chat/parts/ReasoningPart.tsx` (D-08 progressive disclosure pattern) + `web/src/components/chat/MessageBubble.tsx:30-66` (parts switch dispatch)

**目录结构（仿 `web/src/components/center/` 既有 layout）**
```
web/src/components/inspector/
├── AgentStateInspector.tsx     # 顶层 panel：tool calls / retrievals / reasoning tree
├── ToolCallList.tsx            # 子组件：复用 ToolPart 渲染（已经有）
├── RetrievalList.tsx           # 子组件：source-document parts 折叠
└── ReasoningTrace.tsx          # 子组件：复用 ReasoningPart 思路（不是直接 import，自己包一层）
```

**消费 SSE 流**（SC-4 "pure UI re-render"）
```tsx
// Source: 仿照 web/src/components/center/CenterPanel.tsx 的 panel container 形态。
// Inspector 是新的 panel —— 注册到 ThreePanel 的左侧或右侧（per sketch-findings 三栏布局）。
//
// 关键：inspector 不监听 useChat 自己的 message stream 副本 ——
// 它订阅 data-decision / tool-* / reasoning-* parts。useChat 的 messages
// 数组由 MessageBubble 渲染（line 73-85），inspector 是另一个消费者，
// 通过 Zustand store（web/src/store/app-store.ts）拿 part 列表。
//
// Zustand 不能放 messages（D-04 / Wave 2 已经锁）——
// 但可以放"current turn parts"作为 useChat 流之外的镜像。
// 仔细读 RESEARCH.md §SC-4 inspector data source：Claude's discretion 是
// "reuse existing events" —— 不开新 channel。所以 inspector 的最简实现是：
// useChat.messages → 过滤 type ∈ {tool-*, data-decision, reasoning-*} → 列表渲染。
```

---

### `web/src/components/graph/*` (NEW, FE component)

**Analog:** `web/src/components/chat/parts/SkillPicker.tsx:67-89` (list rendering with active state) + `web/src/components/center/ChunkSourceView.tsx` (chunk metadata display)

**目录结构（轻量 tree，per CONTEXT.md Claude's Discretion）**
```
web/src/components/graph/
├── KnowledgeGraphBrowser.tsx   # 顶层 panel：tier→doc→chunk tree
└── (可选) GraphNode.tsx        # 节点渲染
```

**数据源**（Open Question 4 in RESEARCH.md）
```tsx
// 新增 /api/knowledge/graph 端点（在 internal/server/server.go 注册）。
// 返回 JSON 形如：
//   {
//     tiers: [
//       { name: "权威", docs: [
//         { id: "...", path: "authority/travel_expense.md", chunks: [
//           { id: 1, snippet: "...", effectiveTo: 0 }
//         ]}
//       ]}
//     ]
//   }
// backend 从 retriever.Document 数组按 tier 分组（Mongo / Milvus 通用），
// 可走 retriever.ListAll (新方法) 或 store.Store (已有 audit 行；graph 需要 knowledge metadata，
// 决定路径后 planner 实现)。
```

---

### `web/src/components/chat/MessageBubble.tsx` (MODIFIED, FE dispatcher)

**Analog:** self — 现有 switch (line 30-66)

```tsx
// Source: 仿照 web/src/components/chat/MessageBubble.tsx:30-66 现有 case 块。
// 加 case 'data-redline'：
case 'data-redline':
  return <RedlineCard key={i} part={part as Parameters<typeof RedlineCard>[0]['part']} />;
```

---

### `web/src/components/chat/parts/SkillPicker.tsx` (MODIFIED, FE)

**Analog:** self

```tsx
// Source: 仿照现有 SkillPicker.tsx:44-48 handleSelect。
// D5-03 ambiguity 时 router 不返回 selected manifest，返回 scores + 让前端选。
// SkillPicker 加 scores: Record<string, number> prop；handleSelect 把 chosen skill id 派发到 store
// 触发下一次 /api/chat with explicit skill。
// 复用现有 useAppStore.getState().setCurrentSkill(...) 模式（line 45-46）。
```

---

### `web/src/components/center/VerdictCard.tsx` (MODIFIED, FE)

**Analog:** self

```tsx
// Source: 仿照现有 web/src/components/center/VerdictCard.tsx:36-74 形态。
// D5-13 counter-evidence annotation card — 加 counterEvidenceChunks: Chunk[] prop。
// 渲染为可折叠的"反面观点"区块（与现有 CitationCluster 平级）。
// CSS 模式：复用 STATUS_BORDER (line 18-23) 加一个 counterEvidence 颜色 token。
```

---

### `web/src/components/center/ChunkSourceView.tsx` (MODIFIED, FE)

**Analog:** self

```tsx
// Source: 仿照现有 ChunkSourceView.tsx:32-46 形态。
// D5-15 effective date/version 字段已经在 props 里（line 19-23）。
// Phase 5 加：onClick 跳转到 graph browser 的对应节点 ——
// 仿 D-02 CitationCluster → setCenterView({ kind: 'chunk-source', ... }) 模式（DataPolicyPart.tsx:12）。
```

---

## Shared Patterns

### Wire chunk emission pattern (D-04/D12/D15 invariant)

**Source:** `internal/wire/sse.go:80-93` `EmitChunk` + `internal/wire/chunk.go:121-147` `ValidTypes` + `web/src/types/wire-types.ts:117-122` `_driftCheck`

**Apply to:** ALL new/modified files that emit a SSE chunk (router.go, redline.go, annotate.go emit + wire/chunk.go extend).

```go
// Every emit site MUST:
//   1. Register the type in wire.ValidTypes (line 121-139) FIRST.
//   2. Use wire.EmitChunk (NOT raw sse.NewWriter + WriteEvent — see wire/sse.go:80-93).
//   3. Test that IsValidType returns true (TestDecisionChunkType is the precedent).
//   4. Mirror the JSON shape in web/src/types/wire-types.ts (D15 drift alarm).
```

### Structured-output LLM call (D5-12/14 temperature + response_format + thinking)

**Source:** `internal/server/pipeline.go:176-190` `model.WithTemperature + openai.WithExtraFields`

**Apply to:** `wizard.go` (D5-12 by type) + `redline.go` (D5-14 conditional) + `internal/agent/critique.go:runCritiqueLLMCall` (already uses this).

```go
// Always pass: manifest.Temperature (SYSK-08 — pointer dereferenced; nil → 0.0),
// response_format=json_object, thinking=disabled. spike 003 verified shape.
extraFields := map[string]any{
    "response_format": map[string]string{"type": "json_object"},
    "thinking":        map[string]string{"type": "disabled"},
}
opts := []model.Option{
    model.WithTemperature(*manifest.Temperature),
    model.WithMaxTokens(600),
    openai.WithExtraFields(extraFields),
}
```

### Tier PRE-filter (D7 + D5-15 extension)

**Source:** `internal/retriever/milvus/filter.go:36-50` + `internal/retriever/mongo/mongo.go:328-337`

**Apply to:** both backends, with effective extension:

```go
// Always build the filter, always let the backend evaluate it (default-deny structural,
// D-09 — never special-case "empty" in Go). The same applies to effective dates:
//   tier in [...] AND (effective_to == 0 OR effective_to >= now)
// Sentinel 0 (NOT nullable) per RESEARCH.md §Pitfall 2.
```

### Skill validation gate (D2-07 + D5-11 closed loop)

**Source:** `internal/skill/validate.go:48-122` `Validate`

**Apply to:** `wizard.go` (every emit step) + `registry.go reloadOne` (file-driven registration gate).

```go
// Wizard preview → confirm → write flow:
//   1. Build manifest from wizard state.
//   2. skill.Validate(&m) — if err, return preview + validation error to user.
//   3. User confirms → write to skills/<name>/SKILL.md (V12 path traversal guard).
//   4. fsnotify (registry.Watch) picks it up — closed loop (D5-11).
//   5. Next /api/chat routes to it via cosine (D5-01).
```

### V12 ASVS path traversal (symlink + name sanitize)

**Source:** `internal/skill/loader.go:102-119` EvalSymlinks guard + `internal/skill/validate.go` `^[a-z0-9-]+$` Skill name pattern (implicit in D-09 refusal test).

**Apply to:** `registry.go reloadOne` (re-run symlink check on each reload — atomic save+rename can replace a file with a symlink) + `wizard.go write` (sanitize user-supplied skill name to `^[a-z0-9-]+$`).

```go
// On every reload (NOT just startup Scan):
if d.Type()&fs.ModeSymlink != 0 {
    target, err := filepath.EvalSymlinks(path)
    if !strings.HasPrefix(target, root+string(filepath.Separator)) { /* reject */ }
}
```

### Atomic.Pointer snapshot publication (D5-06 in-flight version pinning)

**Source:** `sync/atomic.Pointer` Go stdlib (Go 1.19+; project is 1.25).

**Apply to:** `registry.go` only (the router/verdict pipeline all Load() once per request).

```go
// Watcher goroutine Store()s the new map after debounce + successful validate.
// In-flight Run()s captured a pointer at entry → they keep using the old snapshot.
// Readers (every /api/chat) Load() once per request.
type Registry struct {
    snapshot atomic.Pointer[map[string]*Entry] // immutable; clone-before-Store
}
```

---

## No Analog Found

None. Every file in this phase has a strong in-repo analog — this is the entire point of the phase's "extension, not greenfield" architecture (RESEARCH.md §Summary, line 70).

| File | Reason |
|------|--------|
| (none) | All files reuse named extension points in shipped code. The closest "novelty" is `registry.go`'s atomic-pointer pattern, which uses Go stdlib (no new analog needed; it's a well-known idiomatic pattern). |

---

## Metadata

**Analog search scope:** `internal/{skill,server,wire,retriever,milvus,mongo,agent,embedder,knowledge,store}` + `web/src/{types,components,store}` + `internal/retriever/mongo/rank.go:63` (D5-01 cosine reuse)

**Files scanned:** 35 Go + 13 TS/TSX (every file mentioned in CONTEXT.md `<code_context>` + RESEARCH.md §Pitfalls + RESEARCH.md §Code Examples)

**Pattern extraction date:** 2026-08-12

**Key planner reminders:**
- Sentinel `0` for `effective_from/to` is a deviation from CONTEXT.md's literal "null" wording — RESEARCH.md §Pitfall 2 documents why (Milvus NULL-evaluates-FALSE). Planner must surface to user.
- Counter-evidence MUST NOT share the `chunksByID` map that `GroundingChecker` reads (D5-13 / RESEARCH.md §Pitfall 3). Separate function in `annotate.go` is the structural guarantee.
- Redline's "no gap → no call" gate (D5-14 / RESEARCH.md §Pitfall 5) must be the first line of `MaybeRedline`.
- D15 two-PR rule: Go + TS wire change must ship together (`_driftCheck` stays true).
- D5-04 reserves the "new wire type" budget for `data-redline` (D5-14); routing trace extends `data-decision` instead.
- Wizard output MUST pass `skill.Validate` before write (Pitfall 4 closed loop).
- Watch the `skills/` directory, NOT individual `SKILL.md` files (Pitfall 1: atomic save+rename).