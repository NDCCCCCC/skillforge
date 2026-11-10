# 前端 ESLint 基线（2026-11-09 清零完成）

**当前：0 处违规。CI 里 `Lint` 是硬门禁。**

## 演进路径

| 阶段 | 违规数 | CI 状态 |
|---|---|---|
| 首次引入 ESLint | 31 | `continue-on-error: true`（可见但不阻断） |
| 第一批修复（机械项） | 7 | 同上 |
| 第二批修复（需判断项） | 0 | — |
| **清零后** | **0** | **硬门禁（已删除 `continue-on-error`）** |

项目此前从未跑过 lint，31 处是历史存量，不是某一次改动引入的。

## 清零过程记录

### 第一批：20 处机械项（零语义风险）

| 规则 | 数量 | 改法 |
|---|---|---|
| `prefer-const` | 3 | `let` → `const` |
| `no-useless-assignment` | 2 | 去掉永不生效的初值（`= {}` / `= null`） |
| `no-unused-vars`（未使用 import） | 8 | 删掉该项或整行 |
| `no-unused-vars`（未使用参数） | 3 | 加 `_` 前缀，**保留 arity** |
| `no-unused-vars`（未使用局部/解构） | 4 | 删该行；数组解构用 `const [, vy, vw, vh]` |

### 第二批：7 处需判断项

**真删 3 处**（确实是死代码）：`DocUpload.test.tsx` 的 `getType()`、`OntologyGraphView.tsx` 的 `nodeStyleFor` useCallback 整块、`MockChatTransport.ts` 的 `draftStatus`（声明后只在 `emitDraft` 里赋值，再没人读）。

**ESLint 误判 3 处 → 定向 disable 并写明理由**：

`ApprovalRulesPanel.tsx` / `PlanRulesPanel.tsx` 的 `terminalStatus`、`useEvalReports.ts` 的 `reloadKey.current` —— 它们**不在 callback body 里引用**，但必须留在依赖数组里：

> 审批被 approve-and-remember 终态化后，这个依赖让 callback 重建，下游
> `useEffect([loadRules])` 才会重新拉列表。**删了 → 审批后列表不刷新。**

`exhaustive-deps` 只检查 body 引用，看不到「把某个值当作失效触发器」这个意图。这类必须定向 disable，不能照它说的删。

**真修 1 处**：`ModelPicker.tsx` 的 cleanup 读 `loadCancelled.current.current`（ref 套 ref）。捕获到闭包常量 `holder` 后再操作，否则 cleanup 执行时 ref 可能已指向新对象，读到别人的状态。

## 两个踩过的坑（写给下一个人）

### 1. `eslint-disable-next-line` 的"下一行"是**紧邻的代码行**

下面这种写法**不生效**（disable 作用到了注释行）：

```tsx
// eslint-disable-next-line react-hooks/exhaustive-deps -- terminalStatus
// 故意不出现在 body 里：……
// 重建 → 下方 useEffect 重新拉列表。
}, [terminalStatus]);   // ← 仍然报错
```

正确写法：说明放在**前面几行**的普通注释里，**最后一条注释必须是单行**的 disable，紧贴目标代码：

```tsx
// terminalStatus 故意不出现在 body 里：它是失效触发器……
// eslint-disable-next-line react-hooks/exhaustive-deps -- 刻意的失效触发器，见上
}, [terminalStatus]);
```

### 2. `no-fallthrough` 的注释必须落在**紧邻下一个 case 的最后一行**

```tsx
case 'chunk-source':
// falls through — 多行说明……
// 这条注释写了中文解释，关键词不在最后一行
case 'doc-view':   // ← 仍然报错
```

`commentPattern`（默认 `/falls?\s?through/i`）只匹配紧邻下一个 case 的**最后一条**注释。

## 维护约定

- **不要为了让它绿而关规则**，也不要无理由加 `eslint-disable`。每一处 disable 必须带 `-- 理由`。
- 新增违规会直接让 CI 变红，这是刻意的。
