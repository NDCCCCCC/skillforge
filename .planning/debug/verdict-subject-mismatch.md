---
status: implemented-pending-live-verify
trigger: "事实型查询「采购合同 50 万元以下由谁审批？」的结论主体是「合规（置信度 97%）」而非直接回答审批人；用户反馈『现在结论的主体是 是否合规？不应该是回答用户的提问吗？』；用户补充语境：项目是多 Skill 平台而非单纯合规系统，需在平台层评估「合规」用词"
created: 2026-08-17
updated: 2026-08-17
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: implemented-pending-live-verify
---

# Debug: verdict-subject-mismatch

## Symptoms

- **Expected:** 用户问「采购合同 50 万元以下由谁审批？」→ 结论主体直接是「由部门负责人审批」之类的答案；「合规/不合规」状态（若保留）作为辅助属性呈现。
- **Actual:** 结论主体是「合规（置信度 97%）」。答案（"由部门负责人审批"）被埋在依据的 `span` 字段里 —— 用户必须读完引用 span 才能拿到答案。
- **Error:** 无 crash / 无功能性错误（LLM 正确召回 + grounding 通过 + 引用 verbatim）。现象是设计性的（结论主体错位）。
- **Timeline:** 自 Phase 2 VerdictPipeline 上线即如此（2026-08 初），非最近回归。前序 debug session `chat-conclusion-not-in-chat`（2026-08-15 resolved）只解决「结论是否出现在 chat 流」问题，未触及「结论主体是否回答用户问题」。
- **Reproduction:**
  ```bash
  # UTF-8 中文 curl（绕过 GBK 编码陷阱，见前序 debug）
  curl -N localhost:8888/api/chat \
    -H "Content-Type: application/json" \
    -d '{"query":"采购合同 50 万元以下由谁审批？","skill":"compliance"}'
  # 观察 chat 流 text-delta: 「结论：合规（置信度 97%）。依据 权威 tier 条款：「采购合同金额 50 万元以下由部门负责人审批」。」
  # 同样的模式适用于「年假有几天？」「竞业限制期多久？」「差旅费报销标准是什么？」等所有事实型查询
  ```

## Current Focus

- **hypothesis (CONFIRMED via code reading + user feedback):** 根因是架构性的：合规 Skill 的 Verdict schema 强制 `status` ∈ {合规, 不合规, 需补充, unanswered}，LLM 面对事实型查询（"谁审批?" "有几天?" "多久?"）被迫把这 4 个状态之一作为结论主体；`conclusionText` 模板又总是以 `Status` 起头（「结论：{Status}（置信度 {pct}%）。」），所以答案只能落到 span 字段而非结论主体。Skill 名称「Compliance Review」与 description 里涵盖的事实型 trigger（如"采购合同审批权限"）不一致 —— schema 没有为事实型答案留位置。
- **direction (PENDING USER CHOICE):** 见 Resolution 段三种候选方向，待用户在 AskUserQuestion 中选择。

## Evidence

- timestamp: 2026-08-17 (just now)
  checked: `internal/server/chat.go:768-779` `conclusionText` 函数
  found: 模板固定为 `fmt.Fprintf(&b, "结论：%s（置信度 %d%%）。", v.Status, pct)` 然后拼接首条 citation 的 tier+span，最后可选追加「置信度未达自动采信阈值，需人工复核。」
  implication: 结论主体 = Status 字符串。Status 不是「答案」，是「裁决码」。

- timestamp: 2026-08-17 (just now)
  checked: `internal/verdict/validate.go:31-36` `ValidStatuses`
  found: 合规 / 不合规 / 需补充 / unanswered —— 没有「factual」「answer」「信息查询」之类的状态
  implication: schema 层面没给事实型答案留位置；LLM 只能挤进 4 选 1 的裁决码

- timestamp: 2026-08-17 (just now)
  checked: `internal/server/pipeline.go:91-107` `systemInstructionsTemplate`
  found: prompt 强制 LLM 输出 JSON with `status: "合规" | "不合规" | "需补充" | "unanswered"` —— 无 answer 字段
  implication: LLM 没有出口告诉后端「这是事实型答案，请用 answer 渲染」。LLM 只能挑 status，最近的就是「合规」（因为它解释为「这个事实在权威 tier 里有据可查」）

- timestamp: 2026-08-17 (just now)
  checked: `examples/skills/compliance/SKILL.md` description + steps
  found: description 把「采购合同审批权限」「年假折算」「竞业限制期」列为 trigger —— 这些都是事实型查询而非合规审查
  implication: Skill 设计意图就涵盖了事实型查询，但 schema/prompt 把它当成合规裁决 —— 设计与实现脱节

- timestamp: 2026-08-17 (just now)
  checked: `integration/contract/contract_test.go:225-280` `TestContractStructuredOutputSchema`
  found: contract test 硬编码 4 个 status keys + Verdict 4 字段 round-trip
  implication: 加 `answer` 字段需要同步更新 contract test（契约升级），但 import 是 `verdict.Verdict`，drift 编译期暴露

## Eliminated

- hypothesis: LLM 没召回对 chunk —— ELIMINATED（用户引用 `[1]` 命中「采购合同金额 50 万元以下由部门负责人审批」chunk，grounding 必通过否则会触发 `需补充` 降级）
- hypothesis: LLM 没引用 —— ELIMINATED（citations 非空 + span verbatim）
- hypothesis: 前端把「由部门负责人审批」藏起来了 —— ELIMINATED（chat 流 text-delta 完整发送，FE 忠实渲染；问题在 text-delta 文本本身的措辞）
- hypothesis: grounding 把答案截断了 —— ELIMINATED（grounding 只校验 span 出现与否，不修改 span 内容；用户截图显示完整 span 都在）
- hypothesis: 系统提示词已经指引 LLM 输出答案字段 —— ELIMINATED（系统提示词 4 行 JSON schema 没有 answer 字段，LLM 无出口）
- hypothesis: 改 conclusionText 模板就能修 —— PARTIALLY FALSE（仅靠模板变化无法在没有 answer 数据时凭空生成答案；可缓解症状但答案仍是 verbatim span 拼接，对「年假有几天？」"X 标准是什么？"等答案非 span 直述的场景仍不够）

## Resolution

**root_cause:** `internal/verdict/verdict.go` 自称「SINGLE SOURCE OF TRUTH for the compliance-review JSON schema」—— 即框架本身是 Compliance-specific 的，但平台是**多 Skill 架构**（CLAUDE.md：3 个示例 Skill = compliance / simple-qa / calculator，目标是「同一个 agent 内核通过可插拔 Skill 适配任意业务领域」）。框架与平台语义错位：

1. **Status 枚举被 Compliance 词汇绑架**：{合规, 不合规, 需补充, unanswered} 这 4 个状态只能表达合规审查场景，simple-qa 的事实问答 / calculator 的算术结果 没有「合规/不合规」概念
2. **factual 答案无字段**：事实型查询（如「采购合同 50 万元以下由谁审批？」「年假有几天？」「竞业限制期多久？」）的回答是直接答案，不是裁决码，但 schema 只有 Status，没有 Answer/Result 字段
3. **LLM 被迫二选一**：事实型查询送到 compliance Skill 后，LLM 只能挤进 4 个 Status 之一，最接近的是「合规」（语义错位：「合规」≠「回答了」），导致结论主体错位
4. **conclusionText 模板固化 Status 起头**：把 Status 当成「结论主体」渲染，事实型查询的真正答案只能埋在 `span` 字段

**用户补充语境（2026-08-17）**：项目不是单纯合规检查系统，是**根据不同 Skill 实现不同功能的统一平台**。「合规」这一用词本身（不仅是 verdict verdict 内的 Status 枚举，还是整个 `verdict.Verdict` 框架命名）是平台层的语义债。

---

**fix direction（在多 Skill 平台语境下重新评估）：**

### 方向 A（平台语义层）：把 Verdict 框架改为 skill-agnostic 信封 + skill-specific payload（推荐）

**核心思路**：把 `verdict.Verdict` 拆成两层 —— 平台层 envelope（任何 Skill 都用）+ Skill-specific payload（每个 Skill 自己定义）。Compliance Skill 的「合规/不合规」只活在它的 payload 里，不污染平台层。

- **平台层 envelope**（新文件 `internal/response/response.go` 或重命名 verdict 包）：
  ```go
  type Response struct {
      Status         string             `json:"status"`         // 平台通用：{ok, needs_input, refused, error}
      Confidence     float64            `json:"confidence"`
      Citations      []Citation         `json:"citations"`
      ReviewRequired bool               `json:"review_required"`
      Answer         string             `json:"answer,omitempty"`   // 新增：事实型答案直接答案
      SkillPayload   json.RawMessage    `json:"skill_payload,omitempty"` // Skill-specific 数据
  }
  ```
- **平台层 Status 枚举**（中性、技能无关）：
  - `"ok"` —— Skill 成功给出答复（含事实型答案 / 合规裁决）
  - `"needs_input"` —— 信息不足（替换「需补充」）
  - `"refused"` —— 拒答（替换「unanswered」）
  - `"error"` —— 内部错误
  - 不再有「合规/不合规」这种 Skill-specific 概念在平台层
- **Compliance Skill 自己的 payload**（新文件 `internal/skills/compliance/verdict.go`）：
  ```go
  type CompliancePayload struct {
      ComplianceStatus string `json:"compliance_status"` // {合规, 不合规, 不适用} — Skill 内部
  }
  ```
- **路由逻辑**（`internal/server/chat.go`）：
  - 每个 Skill 在 SKILL.md frontmatter 声明 `response_schema`（`compliance` → CompliancePayload；`simple-qa` → QAAnswer；`calculator` → CalcResult）
  - pipeline 根据 skill_id 选择 unmarshal 目标类型；envelope 永远渲染 answer/status/citations，Skill-specific 字段由各 Skill 自己负责渲染
- **conclusionText 模板**：
  - 若 `Answer != ""`：以 `Answer` 起头（修复「事实型查询主体错位」根问题）
  - 否则：以 `Status`（平台层：ok/needs_input/refused/error）+ `SkillPayload` 中的裁决（如 compliance 的「合规/不合规」）合成
- **影响**：
  - `internal/verdict/verdict.go` 改名 `internal/response/response.go` 或保留 verdict 包名但改为平台信封语义
  - 88 个 import verdict.Verdict 的文件：编译期断裂（这是好事，drift 检测）
  - `integration/contract/contract_test.go`：拆为 `TestContractEnvelopeSchema`（平台层）+ `TestContractCompliancePayloadSchema`（Skill 层）
  - FE `web/src/types/verdict.ts`：拆为 `Response` envelope + `CompliancePayload`；VerdictCard 需要识别 Skill 类型
  - SKILL.md frontmatter 加 `response_schema` 字段
- **评估**：**最贴合平台愿景**。这是结构性修复，不是补丁。其他 Skill（simple-qa / calculator）能干净接入，无需绕过 compliance 词汇。代价是中等重构（前序所有 `verdict.Verdict` 引用方需更新），但 gsd-debug 准则的「Compile-time drift detection」让重构安全

### 方向 B（轻量）：保留 Verdict 包但改用中性术语

**核心思路**：保留 `verdict.Verdict` 框架结构，只把 Status 枚举改为中性的英文 / 通用中文，Compliance 词汇迁到 Skill-specific 字段。

- Status 改：`{ok, needs_input, refused, error}`（替换 合规/不合规/需补充/unanswered）
- `compliance_status string \`json:"compliance_status,omitempty"\`` 字段承载 Compliance Skill 的裁决（{合规, 不合规, 不适用}）
- 加 `answer string \`json:"answer,omitempty"\`` 字段（事实型答案）
- `conclusionText` 根据 answer 决定起头
- **影响**：小于方向 A —— 不动包名，drift 检测仍然有效（ValidStatuses 改了，contract test 失败 → 强制更新）。FE 仍需小幅调整
- **评估**：**性价比最高的折衷**。平台语义债清理掉，但结构重组最小。Compliance Skill 的 4 状态语义完整保留在 `compliance_status` 字段里

### 方向 C（最小）：仅加 Answer 字段 + 改 conclusionText 模板（方向 A 旧版）

**核心思路**：原 direction A，不动 Status 枚举，只加 Answer 字段。

- `Verdict.Answer string` + `conclusionText` 检测 answer 起头
- **问题**：未触及用户提出的「合规用词」问题。Status 仍然是「合规/不合规」，平台仍被 Compliance 词汇绑架
- **评估**：**仅修复症状**，不修复用户识别的语义债

---

**对比与推荐**：

| 维度 | 方向 A（重构 envelope） | 方向 B（中性术语） | 方向 C（仅加 answer） |
|------|------------------------|-------------------|---------------------|
| 修复事实型主体错位 | ✅ | ✅ | ✅ |
| 修复「合规」平台语义债 | ✅✅ | ✅ | ❌ |
| 其他 Skill 接入 | ✅✅ 干净 | ✅ 可行 | ⚠️ 仍需绕开 |
| 实施成本 | 高（88 文件 rename） | 中（10-15 文件更新） | 低（5 文件） |
| 编译期 drift 检测保留 | ✅ 强化 | ✅ | ✅ |
| FE 改动 | 中（拆 VerdictCard） | 小（加 answer 渲染） | 零 |
| 风险 | 中（重构需充分测试） | 低 | 低 |

**推荐：方向 B**（中性术语）。理由：
1. 直接解决用户识别的语义债（Status 不再是「合规/不合规」）
2. 实施成本适中（不重命名包，只改语义）
3. 保留「Compile-time drift detection」约束（contract test 强制更新所有 Status 引用方）
4. 为未来 Skill 接入铺路（simple-qa / calculator 用 platform status = ok/refused 即可）
5. Compliance Skill 自己的 4 状态语义通过 `compliance_status` 字段完整保留（业务逻辑不丢）

**files_changed (per direction):**
- A: `internal/verdict/verdict.go` (重命名/重设计) + `internal/skills/compliance/payload.go` (新) + 88 个 import 更新 + `integration/contract/contract_test.go` (拆分) + `web/src/types/verdict.ts` (拆分) + VerdictCard (FE) + 3 个 SKILL.md 加 response_schema
- B: `internal/verdict/verdict.go` (Status 枚举改 + 加 Answer/ComplianceStatus) + `internal/server/pipeline.go` (prompt 更新) + `internal/server/chat.go` (conclusionText 分支) + `integration/contract/contract_test.go` (Status 断言改) + `web/src/types/verdict.ts` (加 answer 渲染) + `examples/skills/compliance/SKILL.md` (说明 compliance_status 语义)
- C: `internal/verdict/verdict.go` (仅加 Answer) + `internal/server/chat.go` (conclusionText 分支) + `internal/server/pipeline.go` (prompt 微调) + `examples/skills/compliance/SKILL.md` (step 0) + 新增回归测试

## Notes

- 这是设计性问题，不是功能性 bug。现有 grounding / retrieval / refusal 全部工作正常。
- 选 A 之前要先确认 eval-gate 是否仍然兼容 —— `faithfulness_threshold: 0.9` 是基于 citations 是否 grounded 来评，与 status/answer 字段无关，方向 A 兼容。
- 选 C 之前要先 grep 回归测试里所有断言「结论：合规」之类硬编码文本，避免测试因措辞变更失败 —— 涉及 `internal/server/chat_test.go` 的 `TestVerdictStreamCarriesConclusionText` 等。
- 前序 debug `chat-conclusion-not-in-chat` 关闭时，本问题被识别为「次生 backlog」但未开新 session，今日用户报告即对应 backlog。

---

## 实施进度（2026-08-17 partial — context 余量受限中断）

### 已完成改动（atomic commits 待 user 执行）

**核心 verdict 包**（compile-time drift detection 主力）：
- ✅ `internal/verdict/verdict.go`：Verdict 增加 `Answer string` + `ComplianceStatus string` 字段（都 omitempty）
- ✅ `internal/verdict/validate.go`：ValidStatuses = `{ok, needs_input, refused, error}`（平台中性）
- ✅ `internal/verdict/grounding.go`：
  - DowngradeStatus = `"needs_input"`（was "需补充"）
  - NonCompliantStatus = `"non_compliant"`（was "不合规"）
  - 新增 `CompliantStatus = "compliant"`、`NotApplicableStatus = "not_applicable"`、`ComplianceStatuses` enum map
  - `ApplyDowngrade` 当 ComplianceStatus 已设置时同步覆盖

**业务代码**：
- ✅ `internal/server/pipeline.go`：systemInstructionsTemplate 改为新 schema + Step 0 查询类型分类 + answer/compliance_status 字段定义 + 决策规则
- ✅ `internal/server/chat.go` `conclusionText`：按 Answer 分支；factual 查询用 Answer 起头，否则用 ComplianceStatus（fallback Status）
- ✅ `internal/server/redline.go`：cost-control gate 改为 `Status != DowngradeStatus && ComplianceStatus != NonCompliantStatus`

**已更新测试**：
- ✅ `internal/verdict/verdict_test.go`：5 测试用例改用 ok/needs_input/refused + 新增 ComplianceStatusesEnum 测试
- ✅ `internal/verdict/grounding_test.go`：4 测试 + idempotent 测试改用 DowngradeStatus 常量

**构建状态**：`go build ./...` ✅ 绿

### 未完成（需 user 在新 session 继续）

**Go 测试更新（FAIL 待修）**：
- ❌ `internal/agent/critique.go`：注释中 "需补充" 字符串（仅注释，无功能影响）
- ❌ `internal/agent/agent_test.go`、`internal/agent/critique_test.go`
- ❌ `internal/eval/calibration.go`
- ❌ `internal/llm/extract_test.go`
- ❌ `internal/server/chat_test.go`（含 `TestVerdictStreamCarriesConclusionText`）
- ❌ `internal/server/chat_session_test.go`
- ❌ `internal/server/pipeline_default_test.go`
- ❌ `internal/server/pipeline_stream_test.go`
- ❌ `internal/server/pipeline_subprogress_test.go`
- ❌ `internal/server/redline_test.go`
- ❌ `integration/contract/contract_test.go`：`TestContractStructuredOutputSchema` 断言需更新 ValidStatuses + 加 Answer round-trip + ComplianceStatus
- ❌ `scripts/spiked/e2e.go` + `e2e_test.go`：硬编码 "合规" / "不合规" 字符串需替换
- ❌ `cmd/eval/main.go`：可能引用 verdict 字段

**前端**：
- ❌ `web/src/types/verdict.ts`：加 answer + compliance_status 字段
- ❌ `web/src/components/center/VerdictCard.tsx`：分支渲染 Answer
- ❌ `web/src/components/common/StatusBadge.tsx`：新 Status 映射（ok/needs_input/refused/error + ComplianceStatus）
- ❌ `web/src/__fixtures__/*.json`：更新 fixtures
- ❌ `web/src/__tests__/VerdictCard.test.tsx` 等

**文档**：
- ❌ `examples/skills/compliance/SKILL.md`：文档化 compliance_status 语义 + Step 0

**验证**：
- ❌ `go test ./...` 全绿
- ❌ `cd web && pnpm test` 全绿
- ❌ Live 验证：重启 server，curl「采购合同 50 万元以下由谁审批？」→「结论：由部门负责人审批。依据 ...」

### 用户可继续命令

```bash

# 提交当前进度为一个 atomic commit

git add internal/verdict/ internal/server/pipeline.go internal/server/chat.go internal/server/redline.go
git commit -m "feat(260817-vsm): verdict package platform-decoupling (partial)

- Verdict adds Answer + ComplianceStatus fields (omitempty)
- ValidStatuses becomes platform-neutral {ok, needs_input, refused, error}
- Compliance Skill-specific codes (compliant/non_compliant/not_applicable)
  live on ComplianceStatus; ComplianceStatuses enum gates them
- DowngradeStatus/NonCompliantStatus constants migrated to English
- ApplyDowngrade propagates DowngradeStatus to ComplianceStatus when set
- System prompt teaches query-type classification (factual vs review)
- conclusionText leads with Answer for factual queries
- Redline cost-control gate checks ComplianceStatus too

Partial: remaining test files + FE + SKILL.md pending.

Co-Authored-By: Claude <noreply@anthropic.com>"

# 然后继续更新剩余测试文件（grep -l '"合规"\|"不合规"\|"需补充"\|"unanswered"' 列出）

# 最后 FE + SKILL.md + go test + vitest + live verify

```
