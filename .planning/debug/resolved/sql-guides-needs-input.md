---
slug: sql-guides-needs-input
status: resolved
trigger: 用户在聊天问"公司业务部相关数据查询"，检索 [SQL 取数操作手册] 知识层找到 5 条候选，自校验却裁决 needs_input（置信度 89%）需人工复核——但"公司业务部相关数据"的命名/说明明明就在知识库里；且回答还附带了一条英文"条款修订建议"（compliance 条款改写风格），与 SQL 取数场景不符
created: 2026-09-14
updated: 2026-09-15
---

## Symptoms

- Expected behavior: 查询"公司业务部相关数据查询"时，sql-guides KB（SQL 取数操作手册）中已有公司业务部相关数据的命名/说明，应产出可自动采信的结论（answer，带引用），而不是 needs_input
- Actual behavior: 检索到 5 条候选 → 生成结论 → 自校验 → 裁决 needs_input（置信度 89%），提示"置信度未达自动采信阈值，需人工复核"；同时输出"条款修订建议"卡片（原条款/建议改写为英文条款改写文本，内容是 required input 缺失需记录的通用条款），与 SQL 取数查询场景明显不符
- Error messages: 无报错，全流程正常走完，是质量/裁决问题而非崩溃
- Timeline: 不确定 / 用户第一次使用该 KB（sql-guides 语料刚重组为 7 个业务子目录 + 递归 walker 上线）
- Reproduction: 聊天输入"公司业务部相关数据查询"（KB=sql-guides）→ 检索 [SQL 取数操作手册] 知识层 → 找到 5 条候选 → 生成结论 → 自校验 → needs_input 89%
- Scope: 本会话只查 needs_input 误报根因；"sql-guides 中 SQL 相关内容大量重复"已另开独立 session（sql-guides-duplicate-content）

## Current Focus

hypothesis: CONFIRMED & FIXED & LIVE-VERIFIED — 纯 personal-role KB 的 Skill（sql-guides）在 VerdictPipeline 的 grounding closure 必然为空（citableChunksByID 只收 authority-role chunk）→ ApplyDowngrade 确定性降级 + 引用剥空；redline 闸门对任意 Skill 的 needs_input 放行 → compliance 卡片。修复引入 operational-mode（reads 无 authority tier 的 Skill：grounding 对全部检索 chunk 做 + redline 永不触发）。用户 live 探针双查询确认：bug-lock 指纹完全反转，compliance 护城河不变。
test: TDD RED→GREEN（bug-lock 测试修复前以 443 同款指纹失败）+ 用户 live 双探针（server WSL 重建 + runbook 重启后）
expecting: 无（session 已 resolved 归档）
next_action: DONE — 归档至 resolved/，fix + debug docs + knowledge-base 已提交，探针脚本已删除

## Evidence

- timestamp: 2026-09-14
  checked: skillforge.db audit 表 id=443（2026-09-14 15:20:25, skill_id=sql-guides, query=公司业务部相关数据查询）
  found: verdict_json = {"status":"needs_input","confidence":0.89,"citations":[],"review_required":true,"compliance_status":"needs_input"}，chunk_ids 为空
  implication: 置信度恰好 0.89 == verdict.ConfidenceFloor（ApplyDowngrade 封顶值）；compliance_status="needs_input" 是系统提示词禁止 LLM 输出的值（仅允许 compliant|non_compliant|not_applicable），全仓库只有 ApplyDowngrade 的传播规则会写入；citations=[] 与提示词规则 3（ok/needs_input 必须引用 ≥1 chunk）矛盾 → 引用是被 grounded-filter 剥掉的。三者合起来是 ApplyDowngrade 的确定性指纹。
- timestamp: 2026-09-14
  checked: knowledge_bases.yaml + skillforge.db knowledge_bases 表（运行时值）
  found: sql-guides 两处一致 role=personal, retrieval_enabled=1
  implication: kb.Citable（kb.go:267，仅 RoleAuthority 通过）对所有 sql-guides chunk 返回 false → citableChunksByID 对纯 sql-guides 检索返回空 map。
- timestamp: 2026-09-14
  checked: internal/server/pipeline.go:303-314（VerdictPipeline 自校验段）
  found: chunksByID := citableChunksByID(reg, chunks) → GroundingCheck + GroundingChecker 都以它为闭包；!Grounded 时 ApplyDowngrade 改写 + 仅保留 chunksByID 内的 citation（空 map → 全部剥除）
  implication: LLM 无论输出什么都无法通过自校验——提示词规则 3 强制引用，引用必然指向 non-citable chunk。100% 确定性复现，与 LLM 无关。
- timestamp: 2026-09-14
  checked: internal/server/redline.go:115-124（emitRedline 首行闸门）
  found: gate = v.Status != needs_input && v.ComplianceStatus != non_compliant → Status==needs_input 即放行，无任何 Skill 维度判断；redlineSystemPrompt 固定 "policy-drafting assistant ... propose a clause-level rewrite"（英文条款改写）
  implication: "条款修订建议"卡片是 needs_input 降级的级联症状——任何 Skill 的 needs_input 都会触发 compliance 条款改写。
- timestamp: 2026-09-14
  checked: git 考古 8cbe270（fix(26) citability gate, 2026-09-07）与 394c2be（feat(kb) sql-guides 注册, 2026-09-14）
  found: 闸门先于 sql-guides 注册 284 个 commit 落地；8cbe270 提交说明只考虑了 "reads include non-authority KBs"（混合读取）的防护，未考虑 reads 全为 non-authority 的 Skill；.planning/phases/26-kb-registry-foundation/26-06-SUMMARY.md 记录 T-26-06-3 明确接受 "historical-only grounding **refuses**"——但该拒绝闸（ApplyTrustFloor 空闭包 → no_authorized_source）只实现在 agent 路径（opt-in），默认 VerdictPipeline 从未补对应闸门
  implication: 设计缺口 + 默认路径行为漂移：本应"干净拒绝"或"按 Skill 语义应答"的场景，实际落在了最差的中间态（检索+生成全跑完 → 静默降级 → 零引用 89% + compliance 卡片）。
- timestamp: 2026-09-14
  checked: 对照组 audit 行（compliance skill 同日查询 id=427/423）
  found: needs_input verdict confidence=0.95 且 citations 非空（LLM 自己按规则 2 判条件性 needs_input）
  implication: compliance 路径的 needs_input 形态正常（有引用、置信度未被压到 0.89），进一步证明 443 的形态非 LLM 行为而是降级管线产物。
- timestamp: 2026-09-14
  checked: sql-guides 更早 audit 行 436-441
  found: 全部 refusal_reason=internal_error（436-439/441，query 为 GBK mojibake，环境问题）或 no_authorized_source（440）
  implication: 443 是当日唯一走通检索→生成→自校验全链路的 sql-guides verdict；误报在该链路上 1/1 复现。

## Eliminated

- hypothesis: 检索没召回到"公司业务部"内容（排序/重复内容稀释导致 LLM 拿不到素材）—— ELIMINATED：用户观察到 found_candidates=5 且 SQL 命名/说明在库内；且根因指纹（citations 被剥空 + 0.89 封顶）证明裁决改写发生在自校验层，与候选质量无关。重复内容问题归 sql-guides-duplicate-content session。
- hypothesis: LLM 自己输出 needs_input（把 SQL 语料当条件性条款）—— ELIMINATED 作为主因：compliance 对照行（0.95+有引用）证明 LLM 直发 needs_input 时引用会保留、置信度不会被压到 0.89；443 的 compliance_status="needs_input" 只能由 ApplyDowngrade 写入。LLM 原始输出被降级管线覆盖，无论它输出什么结局都是 needs_input@0.89。
- hypothesis: GBK/编码问题（前序 agent-llm-query-rewrite 的坑）—— ELIMINATED：443 的 query 在 audit 中是干净 UTF-8（"公司业务部相关数据查询"），且流程走到了自校验后的 verdict 落库；436-438 的 mojibake 行是另一批环境失败（internal_error），与本 bug 不同路径。
- hypothesis: needs_input 由 review_required 阈值（0.9）独立触发—— ELIMINATED：review_required=true 是 ApplyDowngrade 规则 1 强制写入的，不是独立成因；阈值文案只是降级后的渲染结果。

## Resolution

root_cause: 双层。主因：VerdictPipeline 的 grounding closure 单一来源 citableChunksByID 只收 authority-role chunk（KBM-03 / 8cbe270），而 sql-guides 是首个 reads 全为 personal-role 的 Skill（394c2be）→ 闭包恒为空 → ApplyDowngrade 确定性把任何带引用的 verdict 改写为 needs_input@0.89 + review_required + 引用剥空。设计上 26-06 只对 agent 路径落地了"空闭包→干净拒绝"（T-26-06-3 accepted），默认 VerdictPipeline 未处理该形态。次因（级联）：emitRedline 闸门只看 Status==needs_input，无 Skill 维度 → 对 operational 型 Skill 也发 compliance 条款改写卡片。
fix: operational-mode 判定（internal/server/pipeline.go 新增 readsCitableTier + allChunksByID）：当 Skill 的 resolved reads 不含任何 citable-role KB 时，grounding closure = 本次检索返回的全部 chunk（反伪造校验对 LLM 实际见过的全部素材做）；含 authority tier 的 Skill 闭包保持 citable-only 字节不变（KBM-03 护城河不动）。emitRedline 增加 operational bool 参数（redline.go 首行闸门，先于 D5-14 cost-control gate）：operational Skill 永不触发条款改写。混合读取 Skill（compliance + personal）引用 non-citable chunk 仍降级 + 剥除——TestVerdictPipeline_MixedReadsKeepCitableOnlyClosure 锁定。
verification: 自验证 + 用户 live 验证均通过。(1) TDD RED→GREEN——两个 bug-lock 测试修复前按 audit-443 同款指纹失败（confidence 0.92→0.89 封顶、citations 剥空、review 强制、redline LLM 被调用），修复后全绿；(2) moat-guard 测试修复前后均绿（混合读取行为字节不变）；(3) go build ./...（WSL）+ go vet ./... 干净；(4) go test ./... 全仓绿；(5) go test -race ./internal/server/ 绿；(6) TestNoLegacyInterruptAPIPhase11 失败经 git-stash 验证为分支 pre-existing（hitl StatefulInterrupt 遗留 API 检查，与本修复无关）。
  LIVE 验证（用户 2026-09-15，server WSL 重建 + runbook 重启后双探针）：
  - 探针 1（原失败查询「公司业务部相关数据查询」, skill=sql-guides）：data-policy confidence=0.95（LLM 真实值，非 0.89 ApplyDowngrade 封顶）+ citations 非空（chunk 23, tier sql-guides, span=实际表名 ids_wuh.auto_agreement_t 等）——bug-lock 指纹完全反转；data-redline 帧数=0（条款修订建议卡片消失，operational 闸门生效）；source-document 帧引用 sql-guides 语料（取数口径/001_公司业务部… chunk 23）。status 仍为 needs_input 但形态是 LLM 自身裁决（0.95 + 有引用 + 渲染文本），与正常 compliance needs_input 形态一致（对照 audit 427/423）——模型判定宽泛查询为条件性，非管线劫持（无 compliance_status=needs_input 伪影）。
  - 探针 2（compliance 回归「出差住宿费超过500元合规吗」）：needs_input@0.95 + authority 引用 + data-redline 帧在场——redline 护城河完好，行为不变。
  - 环境注意事项：(a) Windows go build 在 cgo 处失败（已知，仅 WSL 构建）；(b) 探针必须打 http://[::1]:8888 —— netpoll 监听 [::]:8888 时不调度 IPv4-mapped 127.0.0.1 连接（curl 零字节挂起；[::1] 和 localhost 正常），此问题消耗两轮探针排障。
files_changed:
  - internal/server/pipeline.go — grounding 闭包 mode-aware（operational := !readsCitableTier；operational → allChunksByID，否则 citableChunksByID 原样）；emitVerdictWithChunks 同谓词推导 operational 并传入 emitRedline
  - internal/server/redline.go — emitRedline 签名 +operational bool；首行 operational gate（先于 cost-control gate）；godoc 触发集更新
  - internal/server/redline_test.go — runRedlinePass +operational 参数（7 处调用点传 false）；新增 TestRedline_OperationalSkillNeverFires
  - internal/server/pipeline_operational_test.go — 新增：TestVerdictPipeline_OperationalSkillGroundsAgainstRetrievedChunks（bug lock）/ TestVerdictPipeline_OperationalSkillSuppressesRedlineOnNeedsInput（redline 抑制 lock）/ TestVerdictPipeline_MixedReadsKeepCitableOnlyClosure（moat lock）

## Post-fix notes

- internal/runtime/trust_stage.go 的 ApplyTrustStage 有同款 citable-only 闭包（Phase 12 hoist），但仅被 orchestrator archetype 路径（orchestrator.go:663/722）调用，不在默认 VerdictPipeline 路径上；orchestrator Skill 均读 authority KB，空闭包条件不会出现。留档不改（超本 session 最小修复范围）。
- agent 路径（opt-in runAgentPath / ApplyTrustFloor）对 personal-only Skill 仍结构性拒绝（no_authorized_source，T-26-06-3 accepted），默认不走，不改。
- FE TierBadge registry-driven（KBM-02），sql-guides 引用 tier 经 registry 投影显示名，无 FE 改动需求。

## Notes

- 环境陷阱（live 探针排障耗时两轮）：Hertz netpoll 监听 `[::]:8888` 时不调度 IPv4-mapped 127.0.0.1 连接——curl 打 `http://127.0.0.1:8888` 会零字节挂起；探针必须打 `http://[::1]:8888` 或 `localhost`。另：Windows 侧 go build 在 cgo 处失败，构建/启动仅走 WSL（runbook）。
- sql-guides SKILL.md 自身契约："返回操作步骤 + 原文 SQL + 涉及表/字段提示，**不参与合规引用裁决**"、guardrail "Never suppress source attribution (always show which note/table the SQL comes from)" —— 用户期望（answer + 引用）与 skill 声明一致；修复方向以"operational 应答"为准，而非"干净拒绝"（后者与 Symptoms.expected 直接矛盾）。
- 混合读取 Skill（compliance + personal）：闸门保持原样——引用 personal chunk 仍会降级。本次修复不放宽 KBM-03 对 compliance verdict 的约束。
- agent 路径（opt-in runAgentPath / ApplyTrustFloor）对 personal-only Skill 仍是结构性拒绝（no_authorized_source），属 T-26-06-3 已接受行为，本 session 不改（默认路径不走它）。
