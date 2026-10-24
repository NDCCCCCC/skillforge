---
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick Task 261001-cz7 Summary

**Task:** 补齐 42-03 live 测量工具（4 个 FAIL 门的修复），使 42-operator-bootstrap.sh 重跑可真实记分
**Date:** 2026-10-01
**Branch:** phase-20-p10-6-admin-console
**Duration:** ~2.5h (含 3 次 live LLM 运行 + 1 次 50-case frozen replay)

## One-liner

42-03 的 4 个假门(L1 用不存在的测试 / L5 同 / L4 无条件 retry / L6 stdout grep 永不匹配)全部替换为真实测量路径:两个 env 门控 live 测试 + 脚本记分契约修复,L1 首次跑出诚实数字 **F1=0.3938 (FAIL)**,L5 **recall=1.0000 (PASS)**,L6 诊断出 LOCAL_* 污染 + grep 结构性 broken 并修复后跑通完整 frozen replay。

## Deliverables

| Task | File(s) | Commit |
|------|---------|--------|
| 1 (L1 live F1 test) | `internal/knowledge/ecl/extract_live_f1_test.go` (new) | 794f8d5 |
| 2 (L5 hybrid recall test) | `internal/server/hybrid_recall_live_test.go` (new) | 794f8d5 |
| 3+4 (bootstrap gates) | `.planning/phases/42-v1-4-outstanding-closure/42-operator-bootstrap.sh` | faaeec1 |
| (timeout fix, live 运行中实测发现) | 同脚本 | 204b967 |
| (grep 契约 fix,模拟验证发现) | `extract_live_f1_test.go` | cc1dcae |

## Live 测量结果(真实数字,不造假)

### L1 — TestLiveLLMExtractionF1: **FAIL, F1=0.3938**

- `tables macro-F1 = 0.3938` (P=0.3846 R=0.4103) < 0.8 门;`related macro-F1 = 0.8462` (过线)。
- 模型 glm-5.3-flash,26 golden notes,运行 30 分钟(1800s,触及测试内 30min ctx;041-043 三条因 ctx deadline 记 0 — 即便剔除这 3 条按 best case 重算 ≈ 0.445,FAIL 结论不变)。
- **根因分析(两条,均真实):**
  1. **过预测:** golden 是 writes-only(D-GT-01 policy A:只有 ETL *写入* 的表才进 golden),而生产抽取 prompt 把 note 正文里 *读取引用* 的表也抽出来(note 007/009/010/011/015/018/020/026/029/031/033/036 全部 "live N tables vs golden 0")。stub 之所以过门,是因为 stub 的规则本身编码了 write-shape(INSERT/UPDATE/CTAS);prompt 没有编码这条约定。
  2. **corpus contract 拒答:** LLM 返回 schema 限定名(如 `wuh_dw.etl_policy`)违反 D-Ext-01 的 `^etl_\w+$` 门 → 整条 note 抽取报错记 0。
- **定性:** 这不是测量工具的 bug — 门第一次真实工作了,照出的是 prompt 与 golden 约定的语义鸿沟。修 prompt(编码 writes-only + 去 schema 前缀)会改变 ECL 管线产出 KG 的内容语义(read-only 表引用对 hybrid retrieval 可能反而有用),属产品决策,**本期不擅自改**,交 orchestrator 决定(选项: 改 prompt 对齐 golden / 放宽 golden 口径 / 接受并改门语义)。

### L5 — TestHybridSearchRecall: **PASS, recall=1.0000 (6/6)**

- Milvus 10.62.10.34:19530 + BGE-M3 live;top-5 内 6 个查询全部命中目标文档(per-query 命中源见 `live-f1.log` 同目录 `hybrid` 输出;本 SUMMARY 目录留存 recall 运行 per-query 日志)。
- ≥0.85 门通过。无虚假项。

### L6 — frozen replay 诊断链 + 跑通

诊断(why "32 秒无 faithfulness 输出"):
1. **LOCAL_* 污染(主因):** bootstrap 顶部 `set -a; source .env` 把 `LOCAL_MODEL=Qwen3.8-27B-UD-Q4_K_XL` 带进 eval 进程;`eval.ResolveLLM` 的 LOCAL_* 优先级使 judge+agent 指向死端点(10.62.10.36:8888),且 `ActiveModelVersion()` 返回 Qwen3.8-… ≠ lock `model_ver="glm-5.2"` → `AssertPin` model_ver drift → `log.Fatalf`。~32s = embedder/Milvus/knowledge.Index 的 bootstrap 时长,Fatal 消息被脚本 grep 吞掉 → "无输出"。
2. **grep 契约结构性 broken:** `cmd/eval` stdout 从不打印 `faithfulness=` — per-case 行是 `faith=`,最终值只写进 `eval/report.json` 的 `.overall.faithfulness`。旧 grep 即使跑通也永远取空 → 记 0.00。
3. **修复:** L6 段 re-source .env 时过滤 `LOCAL_(BASE_URL|API_KEY|MODEL)`,改读 `jq '.overall.faithfulness' eval/report.json`;run 失败单独记 FAIL(区分 pin drift)。
4. **验证:** 去除 LOCAL_* 后 pin HOLDS(model_ver glm-5.2 == DefaultJudgeModel,prompt_hash 5b12acd9… 与当前 RenderSystemPrompt 一致 — SKILL.md 未漂移),完整 50-case replay 跑通(51 分钟,EVAL_CASE_DELAY=2s / EVAL_CASE_TIMEOUT=300s):**faithfulness = 0.9036 ≥ 0.9 → L6 PASS**;overall context_recall=0.92 / context_precision=0.6113 / answer_relevance=0.515;per-category faithfulness: adversarial=0.8764, multi-hop=0.8150, refusal-required=0.9750。归档 eval/reports/2026-10-01T03-24-21Z.json,calibration recommended_threshold=0.5090。

> 无需重锁:lock 的 glm-5.2 与 ambient default 一致,prompt_hash 未漂移。v1.6 的"frozen 重锁"决策项**不成立**(除非未来换 judge 模型)。
>
> 环境注:本 WSL 无 jq — bootstrap 脚本大量依赖 jq(pipeline status 轮询、L6 读数)。L6 读数本机可先用 `grep -A4 '"overall"' eval/report.json` 替代,或安装 jq 后重跑(运行手册前置项)。

### L4 / L7 — SKIP 语义(本机验证)

- WSL 无 docker(`command -v docker` 失败)→ L4/L7 走 SKIP 路径,不再假 FAIL。
- L4 retry 改为条件式:仅当 status `failures[]` 含 `status=="dead"` 才 retry 该 note;无死信记 `PASS retry=skipped(no_dead_notes)`。旧逻辑对健康管线 retry note 001 必报错。
- CR-01/IN-05 回归保护由 hermetic 测试承担(manager_in03/in05/in04 已绿),脚本注释已注明。

## Deviations from Plan

**1. [Rule 3 - blocking spec defect] L1 评分约定不能字面照搬 ScoreTables 裸值**
- 计划说"零分母/空输出约定遵循 f1.go"。字面执行(裸 ScoreTables)下,writes-only golden 的 22/26 双空行全记 0 → 宏 F1 数学上不可能 ≥0.8 → 门恒假 FAIL。改用与 hermetic harness 相同的 agreeOrScore(true-negative 记 1,LLM 幻觉仍记 0,f1.go 零分母约定原样保留)。已在测试注释中写明理由。

**2. [Rule 1 - bug] go test 默认 600s 包超时杀死 L1 live 运行**
- 首次正式运行 panic at 600.023s(~14/26 notes)。脚本两处 live 门加 `-timeout 40m/10m`(204b967)。

**3. [Rule 1 - bug] per-note 日志的 "F1=" 会截胡 bootstrap grep**
- 脚本 grep 第一个 `F1=` token → 取到 note 001 的 0.5714 而非宏 0.3938(模拟真实 grep 时发现)。per-note 行改小写 `f1=`,宏行保持 `F1=` 唯一(cc1dcae)。

**4. [范围外] 任务 4 的 commit 型别按实际产出取 fix 而非 docs**
- 诊断结论直接落地为脚本 L6 段修复(与任务 3 同文件),故 faaeec1 合并覆盖 task3+4 的脚本改动;无独立 docs commit,诊断链记录于本 SUMMARY。

## Known Stubs

无。两个新测试无 placeholder 数据路径;L6 的 faithfulness=0.9036 来自真实 50-case replay。

## Threat Flags

无新增信任边界表面(测试只读 golden/corpus,网络调用仅 GLM/Milvus/BGE 既有端点,凭据走既有 env)。

## Deferred Issues

- **[pre-existing] TestEmitSubCategoryInstanceCount FAIL**(`internal/ontology/emitter/emitter_test.go:83`,SubCategory=6 want 7):emitter 从工作树 skills/+knowledge/ 现算 TBox 并 pin 计数,语料漂移后计数过期(quick-260928 实体/KB 工作之后)。与本任务 diff 无关(本任务仅 2 个新测试文件 + .sh),按 scope-boundary 规则不修,记录于同目录 `deferred-items.md`。
- 完整 bootstrap 重跑(2-4h)未在本会话执行:与 frozen replay 抢 8888 端口 + GLM 配额;L1/L5/L4/L7/L6 各门已按上述单门验证。orchestrator 择机跑 `42-operator-bootstrap.sh` 全量。

## Verification Record

- `go build ./...` WSL CGO_ENABLED=1: PASS
- `go test ./internal/...`: 全绿,除上述 pre-existing emitter 一项(env 门 skip 不破 hermetic:`TestLiveLLMExtractionF1`/`TestHybridSearchRecall` 无 env 时 SKIP)
- `SKILLFORGE_LIVE_F1=1`: F1=0.3938 FAIL(诚实记录,见根因分析)
- `SKILLFORGE_LIVE_HYBRID=1`: recall=1.0000 PASS
- `bash -n 42-operator-bootstrap.sh`: PASS
- L1 grep 契约模拟(对真实日志): 抽出 0.3938 ✓
- docker guard 模拟(WSL): SKIP 路径 ✓
- frozen replay: pin holds + 50/50 cases 完成,faithfulness 见上

## Follow-up: orchestrator decision 修 prompt 再归档 (2026-10-02)

**决策落地:对 L1 实测抓到的两个根因做产品修复,而非仅记录。**

| 修复 | 内容 | Commit |
|------|------|--------|
| Writes-only prompt | `extractionPromptTmpl` scope 从 "every table the SQL touches" 改写为 WRITES-ONLY DML 目标(INSERT INTO/UPDATE/DELETE FROM/MERGE INTO/CREATE/SELECT INTO 的目标表);SELECT/FROM/JOIN 只读引用 MUST NOT be listed;kind 约定对齐 stub 的 39-03 pin(DELETE FROM/MERGE INTO→update,SELECT INTO→select);bare-name 指令 | 1efebda |
| Schema 前缀剥离 | `stripSchemaPrefix`(LastIndexByte)在校验前剥离 `wuh_dw.etl_policy`→`etl_policy`;剥后仍过 `^etl_\w+$` 合同,非 etl 名照旧拒收(合同不放松);prompt 同步要求 bare name | 1efebda |
| Prompt byte-identity pin 重 pin | entities_test.go 的 D-NZK-04 pin 从旧文案迁到新文案(带 2026-10-01 决策注记) | 1efebda |
| Live 门加固 | openai ChatModelConfig.Timeout 默认 NO timeout → GLM http2 流偶发永久卡死冻结整个测量;改 2m/call + 每 note 一次 retry(基建 flake 重试,质量不重试) | 750684b |

TDD 门: RED `83a3652`(三个测试按正确原因失败:prompt 语义断言 mock 断言文本、stripSchemaPrefix 纯函数、fake-LLM 端到端)→ GREEN `1efebda`(全包绿,含重 pin)。

### 新旧对比(真实测量,GLM glm-5.3-flash,26 notes)

| 指标 | 修复前 | 修复后 |
|------|--------|--------|
| tables macro F1 | **0.3938 FAIL**(P=0.3846 R=0.4103) | **0.8663 PASS**(P=0.8622 R=0.8718) |
| related macro F1 | 0.8462 | 0.8462 |
| 读只读 note 的误抽 | 12+ notes "live N tables vs golden 0" 全零分 | 002-038 全部静默满分(日志无 f1<1.0 行) |
| schema 前缀拒答 | 018/026 整 note 抽取报错 | 剥离后正常抽出 |
| note 001 | f1=0.5714 | f1=0.6667 |
| note 039 | (ctx 截断) | f1=0.8571 |

**残留:** 041-043 三条仍被修复前版本的 30-min 共享 ctx deadline 清零(该轮运行尚无 2m/call 加固);即便带这 3 条污染零分,门仍过(0.8663 ≥ 0.8)。若剔除按 best-case ≈ 0.98。按"不超过 2 轮"约束,**不做第 2 轮 live 重跑** — 门已 PASS,加固留给下次 bootstrap 自然生效。日志: `live-f1-after-fix.log`(同目录)。

## Self-Check: PASSED

- 8/8 关键文件存在(2 新测试 + prompt TDD 测试、脚本、SUMMARY、deferred-items、三份 live 运行日志)
- 7/7 commit 在 git log 中:794f8d5 (test) / faaeec1 (fix) / 204b967 (fix) / cc1dcae (fix) / 83a3652 (test RED) / 1efebda (fix GREEN) / 750684b (fix 加固)
- follow-up 后全量 `go build ./...` + `go test ./internal/...`: 全绿(唯一 FAIL 仍为范围外 pre-existing emitter SubCategory drift,见 deferred-items)
- 任务路径下工作树无未预期残留;eval/report.json + eval/reports/* 为 replay 产物(未跟踪生成物,不入 commit)
