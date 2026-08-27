# Phase 17: p10-3 (KPI Dashboard Skill) - Pattern Map

**Mapped:** 2026-08-27
**Files analyzed:** 36 new/modified files
**Analogs found:** 22 / 36

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/kpi/define.go` | model | CRUD | `internal/kpi/define.go` (same file, extending ParamType vocabulary) | exact |
| `internal/kpi/executor.go` | service | CRUD | `internal/kpi/executor.go` (Phase 16 Call six-step flow) | exact |
| `internal/kpi/loader.go` | service | CRUD | `internal/kpi/loader.go` (Phase 16 LoadKPIs pipeline) | exact |
| `internal/kpi/multi_period.go` | utility | batch | `internal/kpi/loader.go` (period resolve pattern) | role-match |
| `internal/kpi/wsl/multi_period_wsl_test.go` | test | batch | `internal/kpi/wsl/snapshot_wsl_test.go` (WSL CGO test pattern) | exact |
| `internal/skill/loader.go` | service | CRUD | `internal/skill/loader.go` (Scan + parseAndValidate) | exact |
| `internal/skill/loader_kpi_test.go` | test | batch | `internal/skill/loader_test.go` (fenced YAML parse) | role-match |
| `internal/agent/tools/kpi_tool.go` | tool | request-response | `internal/agent/tools/kpi_tool.go` (Phase 16 closure) | exact |
| `internal/agent/tools/kpi_tool_test.go` | test | batch | `internal/agent/tools/kpi_tool_test.go` (existing) | exact |
| `internal/wire/chunk.go` | model | streaming | `internal/wire/chunk.go` (Phase 16 DataReference) | exact |
| `internal/wire/chunk_test.go` | test | unit | `internal/wire/chunk_test.go` (existing) | exact |
| `internal/eval/kpi_metrics.go` | service | batch | `cmd/eval/main.go` (frozen eval runner pattern) | role-match |
| `internal/eval/kpi_metrics_test.go` | test | unit | `internal/kpi/executor_test.go` (hermetic test) | role-match |
| `cmd/eval/main.go` | config | batch | `cmd/eval/main.go` (frozen eval --frozen=compliance) | exact |
| `examples/skills/kpi-dashboard/SKILL.md` | config | file-I/O | `examples/skills/compliance/SKILL.md` | exact |
| `data/staging/testdata.duckdb` | data | file-I/O | `data/staging/` (staging path, Phase 14 D14-D4) | exact |
| `kpis/testdata.yaml` | model | file-I/O | `kpis/loans.yaml` (Phase 16 KPI YAML) | exact |
| `eval/golden/kpi.jsonl` | data | batch | `eval/golden/compliance.jsonl` | exact |
| `eval/frozen/kpi.lock.json` | config | batch | `eval/frozen/compliance.lock.json` | exact |
| `eval/frozen/reid.jsonl` | data | batch | `eval/golden/platform-floor.jsonl` (attack cases) | role-match |
| `web/src/types/wire-types.ts` | model | request-response | `web/src/types/wire-types.ts` (DataReference TS mirror) | exact |
| `web/src/components/chat/MessageBubble.tsx` | component | streaming | `web/src/components/chat/MessageBubble.tsx` (data-decision branch) | exact |
| `web/src/components/chat/ChatInput.tsx` | component | request-response | `web/src/components/chat/ChatInput.tsx` (local text state + onSend) | exact |
| `web/src/components/chat/parts/Sparkline.tsx` | component | streaming | `web/src/components/chat/parts/DataPolicyPart.tsx` (shadcn Tooltip usage) | role-match |
| `web/src/components/chat/parts/Sparkline.test.tsx` | test | unit | `web/src/components/chat/parts/` (existing test pattern) | role-match |
| `web/src/components/chat/parts/DataReferenceChip.tsx` | component | streaming | `web/src/components/chat/parts/Citation.tsx` (inline hover) | role-match |
| `web/src/components/chat/parts/DataReferencePopup.tsx` | component | streaming | `web/src/components/chat/parts/DataPolicyPart.tsx` (Tooltip popup) | role-match |
| `web/src/components/chat/parts/DrillChip.tsx` | component | request-response | `web/src/components/chat/parts/Badge.tsx` (shadcn Badge) | role-match |
| `web/src/store/app-store.ts` | store | request-response | `web/src/store/app-store.ts` (existing Zustand store) | exact |
| `scripts/verify-phase-17.sh` | script | batch | `scripts/verify-phase-16.sh` (11-stage gate) | exact |
| `Makefile` | config | batch | `Makefile` (existing verify-phase-16 target) | exact |

---

## Pattern Assignments

### `internal/kpi/define.go` — extending ParamType vocabulary

**Analog:** `internal/kpi/define.go` (same file, extending D16-A3 vocabulary)

**Imports pattern** (lines 1-17):
```go
package kpi
```

**ParamType vocabulary extension** (lines 24-27 — add `ParamTypeArray`):
```go
// Param type vocabulary (D16-A3): v1 supports string (pattern-constrained)
// and enum (value-set-constrained) only. A type not in this vocabulary is
// fail-closed rejected by ValidateKPIFile — no silent degradation.
const (
    ParamTypeString = "string" // pattern-constrained free text
    ParamTypeEnum   = "enum"   // explicit value allowlist
    ParamTypeArray  = "array"  // NEW (D17-B3): periods:[] multi-period KPI
)
```

**ParamSpec for array type** — add to `ParamSpec` struct (after line 99):
```go
// ParamSpec declares the type and value domain of a KPI parameter.
type ParamSpec struct {
    Type string `yaml:"type"`
    Values []string `yaml:"values"`
    Pattern string `yaml:"pattern"`
    Required bool `yaml:"required"`
    // NEW (D17-B3): for ParamTypeArray
    Items *ParamSpecItems `yaml:"items,omitempty"` // minItems, maxItems, pattern
}

type ParamSpecItems struct {
    Type    string `yaml:"type"`   // must be "string" for periods
    Pattern string `yaml:"pattern"` // e.g. "^\d{4}-(0[1-9]|1[0-2])$"
    MinItems int    `yaml:"minItems"` // e.g. 2
    MaxItems int    `yaml:"maxItems"` // e.g. 24
}
```

---

### `internal/kpi/executor.go` — multi-period Call branch

**Analog:** `internal/kpi/executor.go` (Phase 16 six-step Call orchestration)

**Multi-period Call extension** — add new exported method (after line 360):
```go
// CallMultiPeriod executes kpi_call for N periods in a single invocation.
// Mirrors Call's six steps, but loops over N periods and merges rows.
// D17-B3: single KPI function with periods:[...] param → N rows + 1 audit row + 1 data-reference part.
//
// Step ③: resolve all N period watermarks from datasets registry
// Step ④: loop engine.Query N times, tag each row with its period column
// Step ⑤: k_min suppression applied per-row (shared KMinApplied)
// Step ⑥: snap.Rows = sorted merged rows; snap.Args.periods = sorted periods
func (e *Executor) CallMultiPeriod(ctx context.Context, req CallRequest) (CallOutcome, error) {
    // ①+② same as Call (unknown KPI, param validation including array shape)
    // ③: resolve N period watermarks
    periods := req.Params["periods"]
    periodsList, ok := periods.([]any)
    if !ok {
        return CallOutcome{Refusal: &Refusal{
            ReasonCode: ReasonInvalidParams,
            HumanExplanation: "periods 参数必须是数组",
        }}, nil
    }
    // ... resolve each period watermark via e.meta.Periods(ctx, dataset)
    // ... loop engine.Query per period, tag rows with period column
    // ... merge and sort rows by period
    // ... k_min suppression
    // ... single audit row with merged rows
}
```

**validateParams extension for array type** (around line 411, add after `ParamTypeString`):
```go
case ParamTypeArray:
    arr, isArr := v.([]any)
    if !isArr {
        return refuse(fmt.Sprintf("参数 %q 必须是数组类型。", n))
    }
    items := cp.Items
    if items == nil {
        return refuse(fmt.Sprintf("参数 %q 声明为数组但未提供 items 定义。", n))
    }
    if items.MinItems > 0 && len(arr) < items.MinItems {
        return refuse(fmt.Sprintf("参数 %q 数组长度 %d 低于最小要求 %d。", n, len(arr), items.MinItems))
    }
    if items.MaxItems > 0 && len(arr) > items.MaxItems {
        return refuse(fmt.Sprintf("参数 %q 数组长度 %d 超过最大限制 %d。", n, len(arr), items.MaxItems))
    }
    for i, elem := range arr {
        s, isStr := elem.(string)
        if !isStr {
            return refuse(fmt.Sprintf("参数 %q[%d] 必须是字符串类型。", n, i))
        }
        if items.Pattern != "" {
            re := regexp.MustCompile(items.Pattern)
            if !re.MatchString(s) {
                return refuse(fmt.Sprintf("参数 %q[%d] 取值 %q 不匹配格式约束（%s）。", n, i, s, items.Pattern))
            }
        }
    }
```

---

### `internal/kpi/loader.go` — array param validation on load

**Analog:** `internal/kpi/loader.go` (Phase 16 LoadKPIs + compileKPI pipeline)

**Loader array type handling** (in `compileKPI`, after line 355 switch):
```go
case ParamTypeArray:
    cp.Items = ps.Items
    if cp.Items != nil && cp.Items.Pattern != "" {
        re, err := regexp.Compile(cp.Items.Pattern)
        if err != nil {
            return CompiledParam{}, fmt.Errorf("params.%s: items.pattern %q compile failed: %w", name, cp.Items.Pattern, err)
        }
        cp.Items.PatternRE = re
    }
```

**New ValidateAgainstKPI method** (add to `Loader` after Scan):
```go
// ValidateAgainstKPI cross-checks every skill's fenced YAML routing table
// (inside steps:) against kpi.Registry.Names(). Called at server boot
// AFTER LoadKPIs and AFTER Loader.Scan (D16-C2 hermetic pattern).
// Failure = log.Fatalf (mirrors D16-08-D LoadKPIs failure mode).
func (l *Loader) ValidateAgainstKPI(reg *kpi.Registry) error {
    manifests := l.Skills()
    for _, m := range manifests {
        if !m.HasDataChannel() { continue }
        routes := parseRoutingTable(m.Steps) // parse fenced YAML block
        for _, route := range routes {
            if !reg.Contains(route.KPI) {
                return fmt.Errorf("skill %q references unknown KPI %q in routing table", m.Name, route.KPI)
            }
        }
    }
    return nil
}
```

---

### `internal/agent/tools/kpi_tool.go` — DataRefEmitFunc builds References[]

**Analog:** `internal/agent/tools/kpi_tool.go` (Phase 16 invoke closure, lines 252-263)

**References[] field addition** (line 253 — extend DataReference struct literal):
```go
// Phase 17 extension: DataRefEmitFunc builds References[] from snap.Rows
// for sparkline multi-point rendering (D17-B2).
if emit != nil && snap != nil {
    ref := wire.DataReference{
        Func:        snap.Func,
        Args:        snap.Args,
        ASOfPeriod:  snap.ASOf.Period,
        ASOfTs:      snap.ASOf.Ts,
        RowCount:    len(snap.Rows),
        Suppressed:  snap.Suppressed,
        KMinApplied: snap.KMinApplied,
        References:  buildPointReferences(snap.Rows), // NEW D17-B2
    }
    emit(ref)
}
```

**buildPointReferences helper** (add after renderCatalog):
```go
// buildPointReferences converts snap.Rows to per-point references for sparkline.
// Each row has a `period` column tagged at executor step ⑤. D17-B2.
func buildPointReferences(rows []map[string]any) []wire.PointReference {
    out := make([]wire.PointReference, 0, len(rows))
    for i, row := range rows {
        p := wire.PointReference{PointIndex: i}
        if period, ok := row["period"].(string); ok {
            p.Period = period
        }
        // Last non-period, non-row_count column = the value
        for k, v := range row {
            if k != "period" && k != "row_count" {
                p.Value = v
                break
            }
        }
        out = append(out, p)
    }
    return out
}
```

---

### `internal/wire/chunk.go` — DataReference.References []PointReference

**Analog:** `internal/wire/chunk.go` (Phase 16 DataReference struct, lines 173-201)

**PointReference struct** (add before DataReference, around line 172):
```go
// PointReference is one data point in a sparkline (D17-B2). Attached to
// DataReference.References[] when kpi_call returns N period rows.
type PointReference struct {
    // PointIndex is the 0-based position in the sparkline points array.
    PointIndex int `json:"pointIndex"`
    // Period is the period string for this point (e.g. "2026-07").
    Period string `json:"period"`
    // Value is the numeric value for this point (formattable).
    Value any `json:"value"`
    // KMinApplied is non-zero when this specific point was suppressed.
    KMinApplied int `json:"kMinApplied,omitempty"`
}
```

**DataReference.References field** (extend DataReference struct, line 193):
```go
type DataReference struct {
    Func        string         `json:"func"`
    Args        map[string]any `json:"args,omitempty"`
    ASOfPeriod  string         `json:"asOfPeriod"`
    ASOfTs      string         `json:"asOfTs"`
    RowCount    int            `json:"rowCount"`
    Suppressed  int            `json:"suppressed"`
    KMinApplied int            `json:"kMinApplied"`
    // NEW (D17-B2): multi-point array — N=1 for single-period calls.
    References  []PointReference `json:"references,omitempty"`
}
```

---

### `examples/skills/kpi-dashboard/SKILL.md` — 7-field YAML + fenced routes

**Analog:** `examples/skills/compliance/SKILL.md` (same 7-field template, Phase 2 lock)

**Frontmatter + routing table** (lines 1-60):
```yaml
---
description: 业务数字问答。触发短语：「贷款余额」「不良率」「客户数」「上月销售额」等聚合数字问题。返回数值 + KPI 定义 + 计算时间窗 + as_of + 数据引用。
agent: ChatModelAgent
reads:
  - data:aggregated_only
writes: []
steps: |
  # KPI 路由表（按业务域分组；LLM 读取后按业务词选 KPI）

  ```yaml
  routes:
    loans:
      - triggers: [贷款余额, 未偿还本金, loan balance, outstanding]
        kpi: KPI_LOAN_BALANCE
      - triggers: [不良率, NPL ratio, 逾期率]
        kpi: KPI_NPL_RATIO
    customers:
      - triggers: [客户数, 活跃客户, customer count]
        kpi: KPI_CUSTOMER_COUNT
  ```

  ## 调用流程
  1. 根据业务词查路由表 → 选 KPI 名 + 单 period 或 periods[]
  2. 调 kpi_call(name, params)，不得尝试 SQL 或 SELECT
  3. 自然语言答：数值 + KPI 定义 + 时间窗 + as_of（必须出现）+ 数据来源 `data:aggregated_only`
  4. 多 KPI 答案：每个数字独立 hover；可下钻「看月度趋势」「看同比」
guardrails:
  prohibitions:
    - "Never SELECT raw rows or attempt SQL"
    - "Never expose values < k_min suppression threshold"
    - "Never cite numbers not backed by a kpi_call snapshot row"
  required_refusal_phrases:
    - "no_matching_kpi"
    - "dataset_not_available"
  tier_limits:
    "data:aggregated_only": ["kpi_call only; no retrieve"]
temperature: 0.0
```

---

### `eval/golden/kpi.jsonl` — GEVL-09 50-case golden set

**Analog:** `eval/golden/compliance.jsonl` (50-case JSONL, Phase 3 lock)

**Case shape** (per D17-D2 three-step value match):
```jsonl
{"id":"kpi-001","category":"factual_loans","query":"上月贷款余额是多少？","expected_kpi":"KPI_LOAN_BALANCE","expected_value":42315000000,"allowed_expressions":["4,231.5亿元","423150000000","42315000000"],"expected_period":"2026-07","source":"synthetic"}
{"id":"kpi-002","category":"factual_loans","query":"2026年Q2不良率是多少？","expected_kpi":"KPI_NPL_RATIO","expected_value":0.023,"allowed_expressions":["2.3%","0.023","不良率2.3%"],"expected_period":"2026-Q2","source":"synthetic"}
{"id":"kpi-003","category":"multi_kpi_combination","query":"上月贷款余额和不良率分别是多少？","expected_kpis":["KPI_LOAN_BALANCE","KPI_NPL_RATIO"],"expected_values":[42315000000,0.023],"allowed_expressions":{"KPI_LOAN_BALANCE":["4,231.5亿元"],"KPI_NPL_RATIO":["2.3%"]},"source":"synthetic"}
```

---

### `eval/frozen/kpi.lock.json` — mirror compliance.lock.json + kpi_registry_sha256

**Analog:** `eval/frozen/compliance.lock.json` (5-field lock, Phase 8)

**Lock shape** (D17-D4):
```json
{
  "lock_id": "kpi-v1",
  "model_ver": "glm-5.2",
  "prompt_hash": "<sha256 of rendered system prompt for kpi-dashboard>",
  "seed": 42,
  "eval_set_version": "kpi-v1-2026-08-27",
  "skill_name": "kpi-dashboard",
  "kpi_registry_sha256": "<sha256 of kpis/testdata.yaml content>",
  "schema_version": "v1"
}
```

---

### `web/src/types/wire-types.ts` — DataReference.references?: PointReference[]

**Analog:** `web/src/types/wire-types.ts` (DataReference TS mirror, lines 159-174)

**PointReference interface** (add before DataReference):
```typescript
// Phase 17 — D17-B2 sparkline per-point reference
export interface PointReference {
  /** 0-based index in sparkline points array */
  pointIndex: number;
  /** Period string (e.g. "2026-07") */
  period: string;
  /** The numeric value */
  value: number;
  /** Non-zero when this point was k_min suppressed */
  kMinApplied?: number;
}
```

**DataReference.references field** (extend existing interface):
```typescript
export interface DataReference {
  func: string;
  args?: Record<string, unknown>;
  asOfPeriod: string;
  asOfTs: string;
  rowCount: number;
  suppressed: number;
  kMinApplied: number;
  // NEW (D17-B2): multi-point sparkline references
  references?: PointReference[];
}
```

---

### `web/src/components/chat/MessageBubble.tsx` — data-decision kpi_reference branch

**Analog:** `web/src/components/chat/MessageBubble.tsx` (data-decision branch, lines 101-114)

**kpi_reference branch insertion** (extend case 'data-decision', line 101):
```tsx
case 'data-decision': {
  const dp = (part as { data?: { decision?: string; dataRef?: DataReference } }).data;
  const decision = dp?.decision;
  if (decision === 'counter_evidence') {
    return <CounterEvidenceCard key={i} conflictChunks={dp?.conflictChunks} />;
  }
  if (decision === 'kpi_reference' && dp?.dataRef) {
    return <DataReferenceChip key={i} dataRef={dp.dataRef} />;
  }
  if (decision !== 'route') return null;
  return <RoutingTracePart key={i} part={part as Parameters<typeof RoutingTracePart>[0]['part']} />;
}
```

---

### `web/src/components/chat/ChatInput.tsx` — pendingDraft prefill

**Analog:** `web/src/components/chat/ChatInput.tsx` (local text state pattern, lines 65-136)

**pendingDraft useEffect** (add after line 129):
```tsx
// Phase 17 D17-C1: drill-chip prefill — read from app-store.pendingDraft
const pendingDraft = useAppStore((s) => s.pendingDraft);
const setPendingDraft = useAppStore((s) => s.setPendingDraft);

useEffect(() => {
  if (pendingDraft && !text) {
    setText(pendingDraft);
    setPendingDraft(''); // consume one-shot
  }
}, [pendingDraft, text, setPendingDraft]);
```

**handleSend modification** (line 131 — clear pendingDraft on send):
```tsx
function handleSend() {
  const trimmed = text.trim();
  if (!trimmed) return;
  onSend(trimmed);
  setText('');
  // Phase 17: ensure pendingDraft is cleared on send
  if (pendingDraft) setPendingDraft('');
}
```

---

### `web/src/store/app-store.ts` — pendingDraft field

**Analog:** `web/src/store/app-store.ts` (Phase 5 wizardMode + draftStatus pattern)

**pendingDraft field addition** (in AppState interface, around line 473):
```typescript
// Phase 17 D17-C1: drill-down chip prefill. One-shot signal consumed
// by ChatInput's useEffect (pendingDraft → setText → clear).
// See Pattern 3 / D17-C1.
pendingDraft: string;
```

**setPendingDraft action** (in AppActions interface):
```typescript
setPendingDraft: (text: string) => void;
```

**Default state** (in create(...) defaults, around line 572):
```typescript
pendingDraft: '',
```

**Implementation** (in the store body):
```typescript
setPendingDraft: (text) => set({ pendingDraft: text }),
```

---

### `web/src/components/chat/parts/Sparkline.tsx` — pure SVG zero-dep

**Analog:** `web/src/components/chat/parts/DataPolicyPart.tsx` (shadcn Tooltip wrapper)

**Component signature** (from RESEARCH.md Code Example E-3):
```tsx
import { useMemo } from 'react';

export interface SparklinePoint {
  index: number;
  period: string;
  value: number;
}

export function Sparkline({ points, onPointHover }: {
  points: SparklinePoint[];
  onPointHover: (i: number) => void;
}) {
  const { polyline, circles } = useMemo(() => {
    const values = points.map((p) => p.value);
    const min = Math.min(...values);
    const max = Math.max(...values);
    const range = max - min || 1;
    const padding = range * 0.1;
    const yMin = min - padding;
    const yMax = max + padding;
    const yRange = yMax - yMin;
    const w = 300, h = 32;
    const xStep = points.length > 1 ? w / (points.length - 1) : 0;
    const poly = points
      .map((p, i) => {
        const x = i * xStep;
        const y = h - ((p.value - yMin) / yRange) * h;
        return `${x},${y}`;
      })
      .join(' ');
    const cs = points.map((p, i) => {
      const x = i * xStep;
      const y = h - ((p.value - yMin) / yRange) * h;
      return { cx: x, cy: y, idx: i };
    });
    return { polyline: poly, circles: cs };
  }, [points]);

  if (points.length === 0) return null;

  return (
    <svg viewBox="0 0 300 32" width="100%" height="32"
      style={{ overflow: 'visible' }} data-testid="sparkline"
      role="img" aria-label={`sparkline with ${points.length} points`}>
      <polyline points={polyline} fill="none"
        stroke="var(--color-sparkline, var(--color-primary))" strokeWidth="1.5" />
      {circles.map((c) => (
        <circle key={c.idx} cx={c.cx} cy={c.cy} r="2.5"
          fill="var(--color-sparkline, var(--color-primary))"
          stroke="var(--color-surface-2)" strokeWidth="1"
          onMouseEnter={() => onPointHover(c.idx)}
          onClick={() => onPointHover(c.idx)}
          data-testid={`sparkline-point-${c.idx}`}
          style={{ cursor: 'pointer' }} />
      ))}
    </svg>
  );
}
```

**Test gate (must stay green):**
```bash
grep -rE "recharts|react-sparklines|chart.js" web/src/components/chat/parts/Sparkline.tsx
# must return 0 hits
```

---

### `web/src/components/chat/parts/DataReferenceChip.tsx` — inline number hover

**Analog:** `web/src/components/chat/parts/DataPolicyPart.tsx` (shadcn Tooltip usage)

**Component**:
```tsx
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { DataReference } from '@/types/wire-types';

interface DataReferenceChipProps {
  dataRef: DataReference;
}

export function DataReferenceChip({ dataRef }: DataReferenceChipProps) {
  const refs = dataRef.references ?? [];
  const hasSparkline = refs.length > 1;

  if (hasSparkline) {
    return (
      <span className="kpi-sparkline-wrapper" data-testid="kpi-sparkline-ref">
        <Sparkline points={refs.map((r, i) => ({
          index: i, period: r.period, value: Number(r.value)
        }))} onPointHover={(i) => {/* show popup */}} />
        <DrillChip kpiName={dataRef.func} periodsTemplate="最近12月" />
      </span>
    );
  }

  // Single-value: inline span with tooltip
  const ref = refs[0] ?? { value: null, period: dataRef.asOfPeriod };
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="kpi-value-inline" data-testid="kpi-value">
          {Number(ref.value).toLocaleString()}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-70">
        <DataReferencePopup dataRef={dataRef} pointRef={ref} />
      </TooltipContent>
    </Tooltip>
  );
}
```

---

### `web/src/components/chat/parts/DataReferencePopup.tsx` — shared popup

**Analog:** `web/src/components/chat/parts/DataPolicyPart.tsx` (Tooltip popup)

**Copy layout** (from UI-SPEC §Position 2):
```
4,231.5 亿元          ← value (--color-text)
周期：2026-07         ← period (--color-text-muted)
数据截至：2026-08-01  ← as_of (--color-text-muted)
来源：data:aggregated_only  ← channel (--color-text-dim)
函数：KPI_LOAN_BALANCE ← KPI name (mono)
```

---

### `web/src/components/chat/parts/DrillChip.tsx` — Badge chip with onClick

**Analog:** `web/src/components/chat/parts/Badge.tsx` (shadcn Badge)

**Component**:
```tsx
import { Badge } from '@/components/ui/badge';
import { useAppStore } from '@/store/app-store';

interface DrillChipProps {
  kpiName: string;
  periodsTemplate: string;
}

export function DrillChip({ kpiName, periodsTemplate }: DrillChipProps) {
  const setPendingDraft = useAppStore((s) => s.setPendingDraft);
  const handleClick = () => {
    setPendingDraft(`请调 ${kpiName} periods=${periodsTemplate}`);
  };
  return (
    <Badge variant="secondary" className="kpi-chip cursor-pointer"
      data-testid="drill-chip" onClick={handleClick}>
      看月度趋势
    </Badge>
  );
}
```

**CSS tokens** (from UI-SPEC §Design System):
```css
.kpi-chip {
  background: var(--color-kpi-chip-bg, rgba(74, 158, 255, 0.12));
  border: 1px solid var(--color-kpi-chip-border, rgba(74, 158, 255, 0.3));
}
```

---

### `cmd/eval/main.go` — --frozen=kpi extension

**Analog:** `cmd/eval/main.go` (frozen eval runner, lines 217-254)

**frozen=kpi flag handling** (add after line 217):
```go
frozen := flag.String("frozen", "", "Skill name; scores eval/frozen/<skill>.jsonl and asserts the .lock.json pin")
// Phase 17: also supports "kpi" — eval/golden/kpi.jsonl + eval/frozen/kpi.lock.json
// extends existing --frozen switch with kpi-specific golden path and kpi_registry_sha256 pin
```

**kpi_registry_sha256 pin assertion** (add after line 275 AssertPin call):
```go
if *frozen == "kpi" {
    kpiRegSHA, _ := computeKPIRegistrySHA("kpis/") // hash all kpi YAML files
    if err := eval.AssertKPIPin(lock, kpiRegSHA); err != nil {
        log.Fatalf("[FAIL] cmd/eval: kpi pin mismatch: %v", err)
    }
}
```

---

### `internal/eval/kpi_metrics.go` — GEVL-09/10 programmatic judge

**Analog:** `cmd/eval/main.go` (frozen eval runner, programmatic scoring)

**3-step value match** (from RESEARCH.md Code Example E-5):
```go
package eval

// KPIJudgment is the result of judging one GEVL-09/10 case.
type KPIJudgment struct {
    CaseID    string
    Passed    bool
    Reason    string  // "value_mismatch", "missing_as_of", "missing_channel"
    MatchedKPI string
}

// JudgeKPIReferenceConsistency implements GEVL-10 (faithfulness extension).
// Step 1: as_of token present in answer text
// Step 2: channel-name token "data:aggregated_only" present
// Step 3: every number matches a snapshot row (±0.5% OR allowed_expressions)
func JudgeKPIReferenceConsistency(c KPICase, answerText string, snapRows []map[string]any) KPIJudgment {
    if !strings.Contains(answerText, "as_of") && !strings.Contains(answerText, "数据截至") {
        return KPIJudgment{CaseID: c.ID, Passed: false, Reason: "missing_as_of"}
    }
    if !strings.Contains(answerText, "data:aggregated_only") {
        return KPIJudgment{CaseID: c.ID, Passed: false, Reason: "missing_channel"}
    }
    numbers := extractNumbers(answerText)
    for _, n := range numbers {
        if !matchesAnySnapshot(n, snapRows, c.AllowedExpressions, 0.005) {
            return KPIJudgment{CaseID: c.ID, Passed: false, Reason: fmt.Sprintf("value_mismatch: %v", n)}
        }
    }
    return KPIJudgment{CaseID: c.ID, Passed: true}
}

func matchesAnySnapshot(num float64, rows []map[string]any, allowed []string, tol float64) bool {
    for _, row := range rows {
        for _, v := range row {
            f, ok := toFloat(v)
            if !ok { continue }
            if math.Abs(num - f) / math.Max(math.Abs(f), 1e-9) <= tol { return true }
        }
    }
    for _, expr := range allowed {
        f, ok := toFloat(expr)
        if ok && math.Abs(num - f) / math.Max(math.Abs(f), 1e-9) <= tol { return true }
    }
    return false
}

func extractNumbers(text string) []float64 { /* regex \d[\d,]*\.?\d* */ }
func toFloat(v any) (float64, bool) { /* type switch */ }
```

---

## Shared Patterns

### Wire lockstep D15 (Go + TS must commit together)

**Source:** `internal/wire/chunk.go` + `web/src/types/wire-types.ts`
**Apply to:** `DataReference.References` field additions

Rule: Two commits per D15. Commit 1 BE adds field + test; Commit 2 FE mirrors. If FE drifts, `npx tsc --noEmit` exits 1.

### Skill 7-field schema (SYSK-02)

**Source:** `examples/skills/compliance/SKILL.md` + `internal/skill/validate.go`
**Apply to:** `kpi-dashboard/SKILL.md` — MUST remain 7 fields. Routing table is fenced YAML inside `steps:`, NOT an 8th frontmatter field.

### DataReference.References[] field addition (not union variant)

**Source:** `internal/wire/chunk.go` (D16-B3 overload pattern)
**Apply to:** All new wire field additions — add field to existing struct, do NOT create new UIMessageChunk variant.

### Sparkline zero-dependency rule

**Source:** `web/src/components/chat/parts/Sparkline.tsx` (hand-coded SVG)
**Apply to:** Sparkline component. Test gate: `grep -rE "recharts|react-sparklines|chart.js"` returns 0 hits.

### pendingDraft consume-once pattern

**Source:** `web/src/store/app-store.ts` (draftStatus pattern)
**Apply to:** DrillChip onClick → setPendingDraft → ChatInput useEffect consumes and clears.

---

## No Analog Found

Files with no close match in the codebase (planner should use RESEARCH.md patterns instead):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/kpi/multi_period.go` | utility | batch | No existing multi-period resolve helper — new pattern per D17-B3 |
| `internal/skill/loader_kpi_test.go` | test | batch | No existing fenced YAML routing table test — new pattern per D17-A1 |
| `eval/frozen/reid.jsonl` | data | batch | No existing reid attack subset — new data type per D17-D3 |
| `kpis/testdata.yaml` | model | file-I/O | No existing mini KPI registry fixture — new per D17-D1 |

---

## Metadata

**Analog search scope:** `internal/kpi/`, `internal/skill/`, `internal/agent/tools/`, `internal/wire/`, `internal/eval/`, `cmd/eval/`, `web/src/components/chat/`, `web/src/store/`, `web/src/types/`, `examples/skills/`, `eval/`
**Files scanned:** ~200 Go + TS files
**Pattern extraction date:** 2026-08-27
