# 前端 ESLint 基线（2026-11-09）

ESLint 从零引入。首次运行 `npx eslint src --max-warnings=0` 报 **31 处**违规 —— 这是项目从未跑过 lint 的必然结果，不是本次改造引入的。

**当前剩余 27 处**（2026-11-09 第二次运行）：已清掉 4 处「不是风格问题」的，见下表。

处理原则：**先让门禁存在且可见，再逐批清零**。清零前 CI 里这一步是 `continue-on-error: true`（失败会显示，但不阻断 merge），所以：

- 技术债在 CI 日志里**看得见**，不是假装没有
- 新增代码如果引入新违规，会在 diff 里显形
- 清零后把 `continue-on-error` 删掉，门禁就变成硬的

不要为了「让它绿」去关规则或加 `eslint-disable`。下面每一类都写清了该怎么修。

---

## 已修复的 4 处（2026-11-09）

这 4 处不是风格问题，是先修掉的：

| 文件 | 规则 | 性质 | 修法 |
|---|---|---|---|
| `verdict/CounterEvidenceCard.tsx` | `react-hooks/rules-of-hooks` | **真 bug**：`useAppStore` 在 `if (rows.length === 0) return null` 之后调用。`conflictChunks` 空↔非空切换时 Hook 数量变化 → React 抛 rendered more/fewer hooks 或状态错乱 | Hook 提到提前 return 之前 |
| `layout/DetailPanel.tsx` | `no-fallthrough` | 有意穿透（`chunk-source` / `doc-view` 共用「引用原文」tab），但注释没被 ESLint 认出来 | **ESLint 只检查紧邻下一个 case 的最后一条注释**里的 `commentPattern`（默认 `/falls?\s?through/i`）——关键词必须落在最后一行，写在前几行它看不到 |
| `admin/platform/McpInspectorView.tsx` | `no-empty-object-type` | `interface Props {}` 在 TS 里等价于「任意非空值」（`0`、`""` 都满足），既没表达力又是违规源 | 组件本就不接受 props，直接去掉参数 |
| `verdict/RedlineCard.tsx` | `no-control-regex` | 剥离控制字符是**功能需求**：`clauseId` 会拼进下载文件名，NUL/US 在部分文件系统上产生非法文件名 | `no-control-regex` 对 `\x00` / `\u0000` / 裸字节三种写法**一视同仁地报**，换转义无效 → 提成模块级常量 + 定向 `eslint-disable-next-line` 并写明理由 |

> 修 `RedlineCard` 时踩了一个坑值得记：批量替换把 `.replace(/^-+|-+$/g, '')`（trim 前后连字符）也误替换成常量引用，导致同一个 replace 出现两次。已经恢复。教训：替换 `.replace(/` 这类模式时必须带上更具体的上下文。

---

## 剩余分类与修法

| 规则 | 剩余数量 | 性质 | 修法 |
|---|---|---|---|
| `@typescript-eslint/no-unused-vars` | 18 | 死代码 | 删掉未使用的 import / 变量；若确为对外 API 的占位，改名为 `_x` 或加类型断言使用 |
| `react-hooks/exhaustive-deps` | 4 | **可能是真 bug** | 逐个判断：依赖确实多余 → 从数组删除；确实需要 → 移进 `useCallback` 或改用 `useRef` |
| `prefer-const` | 3 | 纯风格 | `let` → `const` |
| `no-useless-assignment` | 2 | 死赋值 | 删除赋值，或把它变成真正的初值 |
| 合计 | **27** | | |

## 优先修的

`react-hooks/exhaustive-deps` 的 4 处里有 2 处形如「`useCallback` 有一个多余依赖 `terminalStatus`」（`ApprovalRulesPanel.tsx` / `PlanRulesPanel.tsx`）—— 这类通常意味着 effect 的依赖关系与作者预期不一致，值得 review 而不是机械删除。

## 为什么 `no-console` 仍然关闭

`admin/governance/` 下有两处没有 disable 注释的 `console` 调用（`GovernanceTab.tsx` / `ProvenanceGraphView.tsx`）。打开 `no-console` 会让改造范围外的代码直接失败，属于另一批改动。当前那 4 处 `eslint-disable-next-line no-console` 在配置下是"未使用的禁用指令"，因此 `reportUnusedDisableDirectives` 设为 `off`。

## 为什么规则一律 error 而不是 warn

`lint` 脚本带 `--max-warnings=0`：任何 warning 都会让命令失败。所以"能过"和"零 warning"是同一件事，`warn` 只会制造假绿。

## 运行

```bash
cd web
npm run lint        # eslint src --max-warnings=0
npm run typecheck   # tsc --noEmit
npm run verify      # 三者串联
```

## 一个踩过的坑

`eslint.config.js` 初版用的是 `'no-fallthrough': ['error', { allowEmptyFallthrough: true }]` —— 那是 **ESLint 9 的选项名**，ESLint 10 已移除。写上去不会报"配置项错误"，而是整个 lint 直接 `EXIT=2`（配置校验失败），看起来像 ESLint 坏了。ESLint 10 用 `allowEmptyCase` 表达同一件事且默认 `true`，所以那一行删掉即可。
