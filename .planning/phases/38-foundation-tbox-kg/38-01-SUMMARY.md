---
plan: 38-01
title: internal/ontology/emitter + go.mod dependency
status: complete
commit: 29ad95b
requirements: [ONT-01, ONT-02, ONT-03, ONT-05]
---

# Plan 38-01 Summary — TBox Emitter + goRDFlib

## What Was Built

- `internal/ontology/emitter/emitter.go` — 纯 Go TBox 生成器:
  - `Emit()` 从 `skills/*/SKILL.md` (yaml.v3 7-field frontmatter) + `knowledge/` 5-tier walk 生成 Turtle TBox
  - 结构与 spike 005b 对齐: 6 owl:Class (type+label+comment) / 6 owl:ObjectProperty / 8 owl:DatatypeProperty / Tier + KBCategory + SubCategory + SkillAgent + Skill + RefusalPhrase 个体
  - `requiresRefusal` 方向修正为 skill→phrase (符合 rdfs:domain/range 声明)
  - IRI namespace 经构造函数注入 (`SKILLFORGE_ONTOLOGY_NS`), 默认 `http://skillforge.local/ontology#`
  - CJK 子目录名 (其他/取数口径/操作手册/汽车险/理赔与赔付/财务对账/非车险) verbatim 进 IRI, goRDFlib 双向解析 OK
  - 确定性输出 (name-sorted), 同输入字节级一致
- `go.mod` + `go.sum` — `github.com/tggo/goRDFlib v0.1.0` (纯 Go, 无 cgo)
- `internal/ontology/emitter/emitter_test.go` — 6 tests

## Tests

```
TestEmitSkillInstanceCount      PASS (8 Skill instances)
TestEmitTierInstanceCount       PASS (5 KBCategory instances)
TestEmitSubCategoryInstanceCount PASS (7 SubCategory instances)
TestEmitRoundtrip               PASS (emit → parse → serialize → reparse lossless)
TestEmitNamespaceOverride       PASS (@prefix skf: 覆盖生效)
TestEmitSchemaCounts            PASS (6 Class / 6 ObjectProperty / 8 DatatypeProperty)
```

## Deviations from Plan

- **triple count 176 vs spike 快照 215**: skills/*/SKILL.md 内容自 spike (2026-09-21 早) 后有演化 (refusal phrases / reads 变化)。emitter 从源生成, 源变则数字变; roundtrip lossless + 实例计数 (8/5/7) 是硬验收, 不受影响。38-06 verify 脚本用 "当前基线稳定 + reload lossless" 断言而非硬编码 215。
- **`go build ./...` Windows 本机失败 (cgo)**: pre-existing 环境限制 (MSYS2 gcc emutls ABI, memory env-wsl-backend-runbook)。`internal/ontology/...` 以 `CGO_ENABLED=0` Windows 构建通过 + WSL `go build ./...` 全量通过 — ONT-09 (无 cgo) 在 ontology 包级别已验证。

## Self-Check: PASSED

- [x] emitter.go + emitter_test.go + go.mod/go.sum 落盘
- [x] `go test ./internal/ontology/emitter/` 全绿
- [x] `CGO_ENABLED=0 go build ./internal/ontology/...` 通过
- [x] WSL `go build ./...` 全量通过
- [x] ONT-01/02/03/05 覆盖
