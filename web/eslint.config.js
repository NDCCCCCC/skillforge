// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// ESLint 9 flat config —— SkillForge web 前端。
//
// ── 需要先安装依赖 ────────────────────────────────────────────────────
//
// web/package.json 目前没有任何 lint 依赖，执行一次：
//
//   cd web && npm i -D eslint @eslint/js typescript-eslint eslint-plugin-react-hooks globals
//
// （依赖之所以只写成注释：web/package.json 不在本次改造的授权范围内，
// 而 npm install 会连带改写 package-lock.json —— 这里只落地配置 +
// 安装命令，由维护者决定何时执行。）
//
// ── 运行 ──────────────────────────────────────────────────────────────
//
//   cd web && npx eslint src --max-warnings=0
//
// ── 为什么规则一律用 error 而不是 warn ─────────────────────────────────
//
// 仓库的 lint 命令带 --max-warnings=0：任何一条 warning 都会让命令
// 失败。所以"能过"和"零 warning"是同一件事，warn 只会制造假绿。
//
// ── no-console 为什么保持关闭 ─────────────────────────────────────────
//
// 代码里现存 4 处 `eslint-disable-next-line no-console`，但
// admin/governance 下还有 2 处没有 disable 注释的 console 调用
// （GovernanceTab.tsx / ProvenanceGraphView.tsx）。打开 no-console 会
// 让本次改造范围之外的代码直接 lint 失败，因此这里保持关闭 —— 那 4
// 处 disable 注释在当前配置下是"未使用的禁用指令"（ESLint 默认不
// 报错，只有开 --report-unused-disable-directives 才会提示）。
//
// ── 现存的 11 处 eslint-disable-next-line ────────────────────────────
//
// 暂时全部保留（删掉会立刻触发对应规则，属于另一批改动）。每处屏蔽
// 的规则、以及"去掉后会不会报错"的逐条结论见本次改造报告。

import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: ['dist/**', 'coverage/**', 'node_modules/**'],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
        ...globals.node,
      },
      parserOptions: {
        ecmaFeatures: { jsx: true },
      },
    },
    plugins: {
      'react-hooks': reactHooks,
    },
    linterOptions: {
      // 代码里现存 11 处 eslint-disable-next-line 注释（本次改造
      // 刻意保留它们，删除属于另一批改动）。ESLint 9 默认会把"没挡住
      // 任何问题"的禁用指令报成 warning —— 在 --max-warnings=0 下那
      // 等于命令失败，而那些注释此刻恰恰是"死指令"。
      // 关掉这个报告，注释就可以安全地留着等人处理。
      reportUnusedDisableDirectives: 'off',
    },
    rules: {
      // TypeScript 自己负责"未定义标识符"，base 的 no-undef 在 TS 上
      // 会误报类型名/全局（如 DOM 类型、vitest globals）。
      'no-undef': 'off',
      // react-hooks v5 的 recommended 把 exhaustive-deps 定为 warn，
      // 这里提到 error（--max-warnings=0 下 warn 同样会让命令失败）。
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'error',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      // 项目承诺 any 为 0 —— 把这条钉死，避免它悄悄回流。
      '@typescript-eslint/no-explicit-any': 'error',
      'no-console': 'off',
      // 多个 case 共享同一处理（见 DetailPanel 的 chunk-source / doc-view）
      // 是本项目有意的写法，不是漏写 break。
      //
      // 不要在这里加 allowEmptyFallthrough —— 那是 ESLint 9 的选项名，
      // ESLint 10 已经把它移除，写上去会让整个 lint 直接 EXIT=2（配置
      // 校验失败，而不是 lint 报错）。ESLint 10 的 no-fallthrough 用
      // allowEmptyCase 表达同一件事，且默认就是 true，所以这里留空即可：
      // 默认行为已经放行空 case 共享。
      'no-fallthrough': 'error',
      eqeqeq: ['error', 'smart'],
      'prefer-const': 'error',
      'no-var': 'error',
    },
  },
);
