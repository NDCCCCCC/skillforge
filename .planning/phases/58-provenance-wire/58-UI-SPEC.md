---
phase: "58"
slug: provenance-wire
status: draft
shadcn_initialized: false
preset: none
created: 2026-11-23
reviewed_at: pending
---

# Phase 58 — UI Design Contract

> Visual and interaction contract for Phase 58 来源标注 wire（前端呈现部分，PROV-02/PROV-03）：citation 角标分型（vector=🔍 语义 / graph=🕸 本体 / hybrid 双标）、回答头部通道摘要行、KG 缺席徽标与 degraded 态、拒答卡通道统计、audit 面板 channels 统计列。锚定 v1.8-ROADMAP Phase 58 SC-1–SC-6、D-58-01（明确缺席）、D-58-02（wire 只加可选字段）、D-57-01（正交原则）与 D2-03（citation verbatim grounding 不变量）。**核心不变式：来源标注的语义是「这个 chunk 通过什么通道被找到」——它标注通道，不改变引用的支撑语义。** 格式沿用 39-UI-SPEC。

---

## Design System

| Property | Value |
|----------|-------|
| Tool | Tailwind CSS v4 (CSS-first, `@theme` in `web/src/styles/theme.css`) |
| Preset | not applicable — 无 `components.json`；theme.css pinned token set 就是设计系统（同 20/29/39 gate 结论） |
| Component library | 无新库。本 surface 是 chat 消息流 + audit 时间线的**既有惯例扩展**，全部新元素映射到既有模式（见下表） |
| Icon library | lucide-react 继续用于既有图标；通道 glyph 用**文本 emoji** 🔍（vector）/ 🕸（graph）——prompt 明确指定，lucide 无等价物，不为此加图标库 |
| Font | Plus Jakarta Sans Variable（`--font-sans`）；JetBrains Mono（`--font-mono`）用于 chunk_id、实体 label、关系路径、channels 计数 |

**零新依赖、零新 token、零新 CSS。** 新文件只有 `web/src/components/chat/parts/{RetrievalPart.tsx, OriginBadge.tsx}`；其余全部是对既有文件的**加法修改**（optional props + 新 case 分支），既有契约字段零改动。

**一致性基线：** 每个新元素映射到既有模式：

| 新元素 | 继承模式（事实来源） |
|--------|---------------------|
| 回答头部通道摘要行 | HistoryRewriteHint 的「消息内 caption」idiom（`text-xs text-text-muted` 行，无卡片 chrome） |
| OriginBadge 徽标 | common/StatusBadge pill 几何（`rounded-full px-2 py-0.5 text-[11px] font-medium leading-4`）+ emoji glyph |
| chip 内通道角标 | SourceDocumentPart 既有 `citation-tier-pill` 先例（chip 内 pill，与 tier pill 并列） |
| citation popover 扩展 | CitationCluster 既有 popover 容器（`w-80 max-w-xs rounded-md border border-border bg-surface p-3`）**加一行**，不加新浮层机制（150ms hover delay 原样复用） |
| 拒答统计行 | RefusalCard 既有 `text-xs text-text-muted` 段落 idiom（同 您可以尝试换一种提问方式 段落） |
| audit channels 段 | AuditTimelineTab meta row 的 `font-mono` hash 段 idiom（`hash {…}` 同行追加段） |
| KG 缺席/降级徽标 | common/StatusBadge（tone 覆盖：gray=未启用 / amber=降级，DEFAULT_MAP 已有 degraded 词条） |
| 数据接线 | MessageBubble 既有 prop-drilling 惯例（chatText / verdictStatus 同款：part → 部件，不走新 store 字段） |

---

## Spacing Scale

继承 `--space-*` token。本 phase 全部为既有 idiom 内的既有间距，无新例外：

| Token | Value | Usage in this phase |
|-------|-------|---------------------|
| space-1 | 4px | 徽标内 glyph/label gap（`gap-1`）、摘要行内段间 `gap-1` |
| space-2 | 8px | 摘要行 margin（`mt-1`/`mt-2` 与 HistoryRewriteHint 同款）、popover 行距 |
| space-3 | 12px | popover padding（p-3，既有）、拒答统计段 margin（mt-2） |

例外：无。

---

## Typography

继承 `--text-*` token：4 sizes × 2 weights，无新增。

| Role | Size | Weight | Line Height |
|------|------|--------|-------------|
| 通道摘要行 / 锚定实体尾段 | 12px (--text-xs) | 400 | 1.5 |
| OriginBadge / KG 徽标 label | 11px（继承徽章字面 `text-[11px] font-medium`） | 500 | 1.33 (leading-4) |
| 拒答统计行 | 12px (--text-xs) | 400 | 1.5 |
| popover provenance 行 / via 路径 | 12px (--text-xs) / mono 11px（既有 popover meta 字面 `text-[11px]`） | 400 | 1.4–1.5 |
| audit channels 段 | 12px (--text-xs) mono | 400 | 1.5 |

Font families：`--font-sans` 散文；`--font-mono` 用于 chunk_id（既有）、实体 label（via.entity）、关系路径（via.rels 序列）、channels 计数（audit 段）。

禁止：表外字号/字重（唯一豁免：继承的 `text-[11px]` 徽章/popover meta 字面，原样或不用）。

---

## Color

继承 theme.css `--color-*` token。**通道编码是本 phase 唯一的新颜色语义映射，且零新 token**——复用两个既有色族：

| 通道（Citation.origin，wire 缺省 vector） | Glyph | Label | Token | Rationale |
|-------------------------------------------|-------|-------|-------|-----------|
| `vector`（或缺省，D-58-02） | 🔍 | 语义 | --color-primary 家族（indigo；与 `--color-citation-text` 同族） | vector 是主检索通道，与现有 citation 琉璃色（indigo tint）同源 |
| `graph` | 🕸 | 本体 | --color-annotation（teal） | teal 在本库已是「补充注解」语义色（counter-evidence 边条 / 历史 tier 徽标 / redline rationale）——与 graph 的「额外通道」语义同族，且与 indigo 高区分 |
| `hybrid` | 🔍🕸（双 glyph） | 双通道 | 双色组合：🔍 indigo + 🕸 teal，无第三色 | 双标即双 glyph 各自本色；不为 hybrid 发明新色 |

**状态色（缺席/降级，走 StatusBadge 既有 tone）：**

| 信号 | Token | Treatment |
|------|-------|-----------|
| kg_available:false（KG 未启用） | --color-status-unanswered（gray） | StatusBadge tone gray，label 本体增强未启用——缺席是中性事实，不是错误 |
| degraded:true（KG 探活失败） | --color-status-needs-info（amber） | StatusBadge tone amber——降级是需注意的运行态（DEFAULT_MAP 已有 degraded 词条） |
| 拒答统计行 | --color-text-muted | 中性说明文字，不染红（拒答卡本身的 border/heading 色语义由既有 refusal reason 决定，通道统计不改变它） |

**Accent reserved for：** 无新增 accent 用途。通道色**不**用于：chunk-source view、TierBadge、verdict 边条、按钮填充——TierBadge 语义是 tier，通道语义是 provenance，两套正交不混色（D-57-01）。

---

## Layout & Structure

### 消息流落位（零重排）

```
assistant message parts（流序，DOM 顺序即渲染顺序）
  ├─ data-subprogress（既有，SubprogressTimeline）
  ├─ data-retrieval   ← 新 SSE 事件（检索完成即发、先于回答生成）
  │     → RetrievalPart：通道摘要行 + 徽标（自然落在回答文本上方）
  ├─ text（结论正文，既有）
  ├─ source-document[]（引用 chips，既有，加通道角标）
  └─ data-policy（verdict/refusal，既有 → RefusalCard 加统计行）
```

- MessageBubble `renderPart` switch 加 `case 'data-retrieval'`——**加法分支**；unknown-part 静默 drop 的 default 行为不变，part 缺失 = 渲染零变化（老消息、老 BE 均不受影响）。
- SSE `data-retrieval` 在检索完成、回答生成前发出（ROADMAP SC-3）——摘要行在流式期间即可见（「查过什么」先于「答了什么」，roadmap 意图）。
- ChatColumn / MessageBubble 的 DOM 顺序规则（D-08 reasoning-before-text）零改动——data-retrieval 按流序自然落位，不做重排逻辑。

### 呈现面总览

| Surface | 变化 |
|---------|------|
| chat 流（RetrievalPart） | 回答头部通道摘要行 + KG 缺席/降级徽标 + 锚定实体尾段 |
| chat 流（SourceDocumentPart chips） | chip 内追加 OriginBadge（与 tier pill 并列） |
| chat 流（RefusalCard） | 通道统计句（server 有数据才渲染） |
| CenterPanel / 任意 CitationCluster 出现处 | popover 加 provenance 行 + 本体图谱链接 |
| inspector → AuditTimelineTab | meta row 追加 channels 统计段 |

---

## Component Inventory

### 1. types/verdict.ts（modify）

- `Citation` 加 optional 字段（镜像 Phase 58 SC-1 wire；**contract test 钉死的三字段不动语义**，D-58-02）：
  ```typescript
  /** 通道来源（PROV-01）。缺省 vector（老消费者零破坏）。 */
  origin?: 'vector' | 'graph' | 'hybrid';
  /** 经哪个实体/关系路径被找到。形状与 BE ViaEntity/ViaRels 对齐（planner 确认）。 */
  via?: { entity?: string; rels?: string[] };
  /** 原始通道集合（双通道命中 = hybrid 且保留原始集合，SC-2）。 */
  channels?: string[];
  ```
- 新增 `RetrievalStats` + 类型守卫（data-retrieval SSE payload，SC-3 形状）：
  ```typescript
  export interface RetrievalStats {
    channels: { vector: number; graph: number; hybrid: number };
    anchor_entities?: string[];
    kg?: { available: boolean; entities_matched: number };
    degraded?: boolean;
  }
  export function isRetrievalStats(value: unknown): value is RetrievalStats;
  ```

### 2. OriginBadge（new: `web/src/components/chat/parts/OriginBadge.tsx`）

- `<OriginBadge origin={…} />` → pill：glyph + label（🔍 语义 / 🕸 本体 / 🔍🕸 双通道），色板 §Color。`data-testid="origin-badge"` `data-origin={origin}`。
- 导出 `resolveCitationOrigin(citation): 'vector' | 'graph' | 'hybrid'`：`citation.origin ?? 'vector'`（D-58-02 wire 缺省语义，唯一判定点）。

### 3. RetrievalPart（new: `web/src/components/chat/parts/RetrievalPart.tsx`）

- Props: `{ data: unknown }`；`isRetrievalStats` 守卫失败 → `return null`（同 MessageBubble unknown-part silent drop 惯例——不渲染假数据）。
- 渲染（`data-testid="retrieval-summary"`，`text-xs text-text-muted`，HistoryRewriteHint caption idiom）：
  - 摘要行（模板见 §Copywriting，数字全部来自 server）。
  - `anchor_entities` 非空 → 尾段 锚定实体：{labels 顿号连接}（mono label）。
  - `kg.available === false` → StatusBadge gray 「本体增强未启用」（`data-testid="kg-absent-badge"`）。
  - `degraded === true` → StatusBadge amber 「本体暂不可用，已降级为纯语义检索」（`data-testid="kg-degraded-badge"`）。
- `aria-label="检索通道分布"` 容器；两徽标可并存（未启用 + 探活失败是不同事实）。

### 4. MessageBubble（modify: `web/src/components/chat/MessageBubble.tsx`）

- `renderPart` 加 `case 'data-retrieval'` → `<RetrievalPart data={part.data} />`。
- 派生 `originByChunk: Map<string, { origin, via }>`：从同消息 data-policy part 的 verdict citations 构建（key = `citation.chunk_id`）。chatText/verdictStatus 同款 prop-drilling，不进 store。
- `<SourceDocumentPart …>` 传入 optional `provenance`（按 `normalizeChunkID(part.sourceId)` 查 map；无匹配 → 不传）。
- `<DataPolicyPart …>` 传入 optional `retrieval`（同消息 data-retrieval part 的 data，供 RefusalCard）。

### 5. SourceDocumentPart（modify: `…/chat/parts/SourceDocumentPart.tsx`）

- 加 optional prop `provenance?: { origin: string; via?: … }`；存在 → chip 内 tier pill 之后追加 `<OriginBadge>`（`citation-tier-pill` 先例）。无 provenance → chip 零变化。
- 点击行为不变（chunk-source 导航契约，D2-03 不动）。

### 6. CitationCluster（modify: `…/center/CitationCluster.tsx`）

- HoverState 加 `origin` / `via`（从 `citation.origin ?? 'vector'` / `citation.via` 取，同步路径无新增 fetch）。
- popover 头部 meta 行（TierBadge / chunk_id / version / 生效 / source 之后）追加 provenance 行（`data-testid="citation-provenance"`）：
  - `<OriginBadge origin={…} />`
  - `via.entity` → 「经实体 {entity}」（entity mono）
  - `via.rels` 非空 → 「路径：A → rel → B」（mono，` → ` 连接；rels 渲染 server 原文）
  - 图谱链接（`data-testid="citation-graph-link"`）：`localStorage.getItem('sf_admin_token')` 非空 → 渲染 「查看本体图谱」 link（click → `setRoute('admin')` + `setAdminSubroute('ontology')` + pushState '/admin'）；token 缺失 → muted hint 「本体图谱需管理员令牌（/admin → 本体）」（不渲染死链）。
- 上标 superscript 本体与点击行为零改动（仍进 chunk-source verbatim 视图）。

### 7. RefusalCard（modify: `…/center/RefusalCard.tsx`）

- 加 optional prop `retrieval?: RetrievalStats`。存在 → 原因段落之后渲染统计行（`data-testid="refusal-retrieval-stats"`，12px `text-text-muted`），模板见 §Copywriting。
- 无 retrieval → 不渲染该行（永不伪造「0 命中」）。

### 8. DataPolicyPart（modify: `…/chat/parts/DataPolicyPart.tsx`）

- 加 optional prop `retrieval?: unknown`；`isRefusal` 分支 → `<RefusalCard retrieval={retrieval} />` 透传（守卫后透传）。verdict / unknown 分支零改动。

### 9. AuditTimelineTab（modify: `web/src/components/AuditTimelineTab.tsx`）

- audit row FE 类型加 optional `channels?: { vector: number; graph: number; hybrid: number }`（ROI-01：auditchain 记录每次检索 channels 统计）。
- meta row（promptHash 段同行）追加 channels 段（`data-testid="audit-row-channels"`，mono）：`🔍 {vector} · 🕸 {graph} · 双 {hybrid}`。
- **形状驱动渲染**：row 携带 channels 才渲染段；无 → 不渲染任何占位（不渲染 「—」 假列——audit 行按形状自描述）。

### 10. 不改动清单（契约保护）

- **ChatColumn**：零改动（消息列表/流式状态传递不动）。
- **chat-restore.ts**：零改动。历史恢复消息只合成 text/data-turn/data-policy part——无 data-retrieval part → 摘要行在历史消息上不渲染（诚实缺席）；分型徽章照常工作（持久化 verdict citations 携带 origin；pre-58 存量 verdict 的 origin 缺省 vector，是 wire 语义不是 FE 发明）。
- **TextPart / ToolPart / ReasoningPart / SubprogressTimeline / verbatim 引用流**：零改动。

---

## Channel Encoding（wire → 视觉，唯一映射表）

| Citation.origin（wire） | FE 判定 | Glyph | Label | Token |
|--------------------------|---------|-------|-------|-------|
| `"vector"` / 字段缺省 | resolveCitationOrigin | 🔍 | 语义 | primary（indigo） |
| `"graph"` | 同上 | 🕸 | 本体 | annotation（teal） |
| `"hybrid"` | 同上 | 🔍🕸 | 双通道 | 双 glyph 各本色 |
| 未知值 | 同上 fallback | raw 字符串渲染（无 glyph） | 原文 | text-text-muted |

> 未知 origin 渲染原文——39 §Stage Mapping 的 unknown-render-raw 规则推广到 provenance 枚举。FE 不做通道推断（永不按 tier/分数/文本猜 origin）。

---

## Interaction Behaviors

### 流式时序

1. 检索完成 → BE 发 `data-retrieval`（先于回答生成）→ 摘要行立即可见（生成等待期用户已看到「查过什么」）。
2. 回答文本流入 → 结论渲染（既有）。
3. 引用 chips 流入 → 无角标（verdict 未落地，无 server 通道真相）。
4. verdict（data-policy）落地 → chips 按命中渲染 OriginBadge（徽章出现时机 = server 真相出现时机，非乐观渲染）。

### hover 卡（既有 popover 机制内扩展）

- 既有 150ms hover delay / onMouseLeave 清理逻辑零改动；provenance 行是 popover 内**追加行**。
- 图谱链接 click → 落点 `/admin` 本体视图。**planner 注意点**：AdminDashboard 挂载 effect 在 path === '/admin' 时把 subroute 重置为 'batches'——需加 1 行条件守卫（store 已带非默认 subroute 时尊重之），否则链接落点被 clobber。守卫不得破坏「/admin 直达默认 batches」的既有 deep-link 行为。

### 拒答

- refusal 与 data-retrieval 同消息共存（检索发生了才有拒答）→ RefusalCard 渲染统计句。
- 传输优先级（§Discretion）：`refusal.retrieval`（PROV-03 内嵌字段，持久化、restore 可用）优先；缺省回退同消息 data-retrieval part；都无 → 不渲染。

---

## Copywriting Contract

全部 user-facing copy zh-CN；实体 label / 关系名 / chunk_id / 计数保持 server 原文。数字 {N}{M}{K} 全部来自 server 字段（channels.vector / channels.graph / channels.hybrid），FE 不推导。

| Element | Copy |
|---------|------|
| 回答头部通道摘要行 | 本次回答：{N} 条语义检索 · {M} 条本体关联 · 其中 {K} 条双重命中 |
| 锚定实体尾段 | · 锚定实体：{label1、label2、…} |
| KG 缺席徽标（kg_available:false） | 本体增强未启用 |
| degraded 徽标 | 本体暂不可用，已降级为纯语义检索 |
| OriginBadge labels | vector=语义 · graph=本体 · hybrid=双通道 |
| OriginBadge aria-label | 来源通道：{语义\|本体\|双通道} |
| popover provenance 行（经实体） | 经实体 {entity_label} |
| popover provenance 行（路径） | 路径：{A → rel → B → rel → C}（rels server 原文） |
| popover 图谱链接 | 查看本体图谱 |
| 图谱链接无 token hint | 本体图谱需管理员令牌（/admin → 本体） |
| 拒答统计句（canonical，逐字） | 语义搜索 0 命中、本体未识别到相关实体 |
| 拒答统计句（vector 0 + 实体命中） | 语义搜索 0 命中、本体识别到 {n} 个相关实体但未形成可用依据 |
| 拒答统计句（KG 未启用） | 语义搜索 {v} 命中；本体增强未启用 |
| 拒答统计句（degraded） | 语义搜索 {v} 命中；本体探活失败，已降级 |
| 拒答统计句（有命中未达阈值） | 语义搜索 {v} 命中、本体关联 {g} 条，均未达到引用阈值 |
| audit channels 段 | 🔍 {v} · 🕸 {g} · 双 {h} |

> 模板选择规则（FE 判定树，全部由 server 字段驱动）：`kg.available === false` → 未启用句；`degraded === true` → 降级句；`channels.vector === 0 && kg.entities_matched === 0` → canonical 句；`channels.vector === 0 && kg.entities_matched > 0` → 实体命中句；其余 → 未达阈值句。无 retrieval 数据 → 不渲染任何统计句。

---

## Accessibility

- **摘要行：** 容器 `aria-label="检索通道分布"`；徽标 StatusBadge 自带 `aria-label="状态: {label}"`（既有）。
- **OriginBadge：** `aria-label="来源通道：{label}"`；chip 内徽标随 chip 的既有可达性（chip 本身是 button）。
- **popover provenance 行：** 随既有 popover `role="tooltip"`；via 路径 mono 渲染，` → ` 连接符对读屏朗读为一连串「→」，接受（同 Phase 20 eval tooltip 的 hover-only 容忍度）。
- **audit channels 段：** 装饰性 emoji（🔍/🕸）随 mono 文本朗读，`aria-hidden` 不加——emoji 语义（通道名）由相邻计数与 label 承担，emoji 本身是冗余编码（颜色+glyph+文字三通道）。
- **拒答统计行：** 普通 `text-xs` 段落，随拒答卡朗读。
- **Keyboard：** 新增可交互元素只有 popover 图谱链接（native `<a>`/button 行为）；其余为展示性内容。

---

## Honesty & Trust-Semantics Rules

1. **正交不变式（D-57-01 / D2-03，本 phase 最高红线）。** provenance 只标注「通道」，不改变引用的支撑语义：角标/chip 点击仍进 chunk-source verbatim 视图；FE 不渲染任何 server 未声明为 citation 的 chunk（graph-only chunk 由 BE 向量验证门把关，FE 无关）；通道色不入侵 TierBadge / verdict 边条 / chunk-source。
2. **Server truth only.** 分型唯一来源是 `citation.origin ?? 'vector'`（D-58-02 wire 缺省语义）；FE 从不按 tier/分数/文本/路径猜通道。
3. **明确缺席（D-58-01）。** `kg.available:false` → 本体增强未启用 徽标；`degraded:true` → 降级徽标——不静默省略、不静默降质。
4. **No fabricated stats.** 拒答统计句仅在有 server 检索统计时渲染；无数据 → 无句子。canonical 句的 「0 命中」「未识别到」 必须来自 server 字段值，不是 FE 默认值。
5. **History honesty.** 历史恢复消息无 data-retrieval part → 摘要行不渲染（诚实缺席，不伪造头部行）；分型徽章按持久化 citation.origin 渲染（pre-58 存量缺省 vector 是 wire 语义）。
6. **Graph link honesty.** 无 admin token → muted hint，不渲染会 403 的死链。
7. **Malformed data-retrieval → null.** 守卫失败不渲染（unknown-part silent drop 既有惯例）；不渲染降级占位文案（部分渲染优于假数据）。
8. **不破坏契约。** Citation 三字段契约字段语义零改动（只加 optional 字段）；MessageBubble unknown-part default 行为零改动；ChatColumn 零改动；D-08 DOM 顺序零改动。

---

## Forbidden List

- No new design tokens（通道色复用 primary/annotation 家族）、no raw hex、no new fonts/sizes/weights。
- No new npm dependencies、no new icon library（🔍/🕸 是文本 emoji）、no new CSS files、no shadcn CLI components。
- 不为 hybrid 发明第三种颜色（双 glyph 双色组合）。
- 不改动 `internal` 三字段 Citation 契约字段的语义与命名（types/verdict.ts 只加 optional 字段，snake_case 不动）。
- 不重排 message parts DOM 顺序；不移除/不改写任何既有 part renderer；unknown part 仍静默 drop。
- FE 不做通道推断（origin 只来自 wire；无启发式、无 tier→channel 映射）。
- 通道色不用于：chunk-source view、TierBadge、verdict 边条、RefusalCard 边框、任何按钮填充。
- 不渲染 ROI 看板（§Discretion：本 spec 范围外）。
- 不为本 surface 加 SSE 解析代码——data-* part 经既有 useChat transport 抵达，FE 只消费 part。

---

## Discretion Resolutions（默认已选，planner 可在 PLAN 覆盖）

| Open point | Resolution | Rationale |
|------------|-----------|-----------|
| `via` wire 形状 | `{ entity?: string; rels?: string[] }`（FE mirror 草案） | ROADMAP SC-1 只定 ViaEntity/ViaRels 字段名；planner 与 BE 对齐后同步本表与 types/verdict.ts |
| 拒答统计的传输位置 | `refusal.retrieval` 内嵌优先，回退同消息 data-retrieval part | PROV-03 是 refusal wire 的可选扩展；回退让 live 流先行可用，内嵌让 restore 可用 |
| ROI 看板 UI | **本 spec 不含** | ROADMAP SC-5 是「三查询出数可查」——audit channels 列 + BE 查询已覆盖可查性；图形看板属新面，单独规划（scope-creep 红线） |
| anchor_entities 展示 | 全量顿号连接，不截断 | 实体 label 来自 162 条 rdfs:label 字典，量级小；容器 wrap 承担超长 |
| 通道色选型 | indigo（vector）/ teal（graph），零新 token | teal 已是「补充注解」语义色（counter-evidence / redline rationale），与「额外通道」同族；与 indigo 高区分；colorblind 下 glyph+文字仍可辨（三重编码） |
| summary line 是否卡片化 | 不卡片化（caption idiom） | HistoryRewriteHint 先例；摘要行是元信息不是内容，卡片 chrome 会与结论正文竞争注意力 |
| chips 无 citation 匹配时 | 不渲染徽章 | server truth only：source-document part 与 verdict citations 是两个 wire 事实，FE 不跨事实推断 |
| AdminDashboard deep-link 守卫 | 挂载 effect 尊重 store 已设 subroute（1 行条件） | 防止图谱链接落点被 'batches' 默认值 clobber；不得破坏既有 /admin 直达行为 |

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official | none（无 CLI 组件新增） | not required |
| third-party | none declared | not applicable |

`components.json` absent（同 39 结论）——gate 结论 **Tool: none (existing token system is the design system)**。

---

## Visual Checklist

- [ ] data-retrieval part → 摘要行渲染于回答文本上方；数字与 server channels 逐字段一致
- [ ] 锚定实体尾段：仅 anchor_entities 非空时渲染；label 为 server 原文
- [ ] KG 缺席徽标（gray 本体增强未启用）与 degraded 徽标（amber）可并存；各自 testid 正确
- [ ] OriginBadge：vector=🔍语义(indigo) / graph=🕸本体(teal) / hybrid=🔍🕸双通道；缺省 origin → 语义
- [ ] 引用 chips：verdict 落地后按命中渲染徽章；流式期间无徽章；无匹配 citations 的 chip 无徽章
- [ ] popover provenance 行：OriginBadge + 经实体 + 路径（mono）；上标点击仍进 chunk-source
- [ ] 图谱链接：有 token 渲染链接；无 token 渲染 muted hint；落点本体视图（deep-link 守卫生效）
- [ ] 拒答卡统计句：五模板判定树正确；canonical 句逐字；无数据不渲染
- [ ] audit channels 段：形状驱动渲染；格式 🔍 {v} · 🕸 {g} · 双 {h}
- [ ] 历史恢复消息：无摘要行（不伪造）；徽章按持久化 origin 渲染
- [ ] D-08 顺序 / ChatColumn / unknown-part drop / Citation 三字段契约零改动
- [ ] 全部 copy 与 §Copywriting Contract verbatim 一致

---

## Pre-Populated From

| Source | Decisions Used |
|--------|---------------|
| v1.8-ROADMAP Phase 58 | 6（SC-1 Citation origin/via/channels 可选字段；SC-2 chunk_pk 聚合 + hybrid 保留 channels[]；SC-3 data-retrieval SSE 形状（channels/anchor_entities/kg/degraded）+ 先于回答生成；SC-4 角标分型 + 摘要行 + KG 缺席徽标 + 拒答通道统计；SC-5 auditchain channels + ROI 三查询；SC-6 verify）+ D-58-01 + D-58-02 + D-57-01 |
| ROADMAP Industry References | 3（Azure agentic retrieval per-citation activitySource 通道溯源；Anthropic Citations 可选字段挂载模式；GraphRAG 正交原则/citation laundering 印证） |
| 39-UI-SPEC | 4（文档骨架；unknown-render-raw 枚举规则；Honesty 红线家族；Discretion 表格式） |
| Implemented chat code | 8（MessageBubble renderPart switch + prop-drilling（chatText/verdictStatus 先例）；DataPolicyPart discriminated union + unknown-payload 降级；SourceDocumentPart citation-tier-pill 与 chunk-source 导航契约；CitationCluster popover 机制（150ms delay/fixture 回退）；RefusalCard 段落 idiom 与 REASON_META；HistoryRewriteHint caption idiom；AuditTimelineTab meta row mono 段 idiom；StatusBadge tone 覆盖 + aria-label） |
| theme.css | 3（--color-annotation teal 家族语义（counter-evidence/redline 既有用法）；--color-citation-* indigo 家族；status token 状态色板） |
| User input this session | 2（通道 glyph 指定 🔍/🕸/双标；摘要行与拒答 canonical 句逐字文案） |

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: pending review
- [ ] Dimension 2 Visuals: pending review
- [ ] Dimension 3 Color: pending review
- [ ] Dimension 4 Typography: pending review
- [ ] Dimension 5 Spacing: pending review
- [ ] Dimension 6 Registry Safety: pending review

**Approval:** pending — 待 gsd-ui-checker 审查（本 spec 由 FE 规划 agent 产出，2026-11-23）。
