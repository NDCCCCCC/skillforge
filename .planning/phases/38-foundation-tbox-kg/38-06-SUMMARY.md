---
plan: 38-06
title: verify-phase-38.sh — 8 SC 全自动验证
status: complete
requirements: [ONT-01, ONT-02, ONT-03, ONT-04, ONT-05, ONT-06, ONT-07, ONT-08, ONT-09]
---

# Plan 38-06 Summary — Hermetic Verify Gate

## What Was Built

`scripts/verify-phase-38.sh` — 13 checks 覆盖 ROADMAP SC-1..8 / ONT-01..09:
1. SC-1/2 unit suites (ontology 3 包 + admin handlers)
2. SC-1 emit from live skills/ + knowledge/
3. SC-1 boot artifacts (v1/v2 archive + pointer + restart reuse)
4. SC-1 reload lossless ×2 (TTL roundtrip + KG snapshot roundtrip)
5. SC-3 CGO_ENABLED=0 build
6. SC-3 dep-graph 无 librdf/redland/rdf2go/owlgo
7. SC-4 instance enumeration 8/5/7
8. SC-5 KG pattern query + tbox endpoint
9. SC-6/7 induction approve loop (stubbed LLM)
10. SC-7 reject lifecycle + IRI conflict + debounce single-write
11. SC-8 toolchain build (cmd/server)

**运行结果: 13/13 PASS — "PHASE 38 HERMETIC GATE: PASS"**

## Deviations from Plan

- **未做 live server curl 流程** (原计划的 start_server + curl) — hermetic 化: HTTP 面由 `TestAdminOntology*` handler 测试 (hertz ut.PerformRequest, 真 engine) 覆盖, 避免 verify 脚本需要端口/环境。live Claude induction (真实 LLM key) 保持为 manual operator gate (脚本头注明)
- **指针式 driver 二进制被放弃** (internal/ 无法被 scratch module 导入): boot artifact 契约由 induction 的 TestApproveIncrementsTBoxVersion + TestQueuePersistsAcrossRestart 覆盖

## Self-Check: PASSED

- [x] `bash scripts/verify-phase-38.sh` (WSL) 退出码 0, 13/13 PASS
- [x] Hermetic — 无 GLM/Milvus/BGE 依赖
