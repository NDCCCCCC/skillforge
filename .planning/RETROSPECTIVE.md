# Retrospective — SkillForge

## Milestone: v1.5 — Outstanding Closure

**Shipped:** 2026-10-02
**Phases:** 2 (42, 44) | **Plans:** 5 | **Quick tasks:** 10+ (wzl/d5w/d5w2/d5w3/g7x/h8k/cz7/jxb)

### What Was Built

- 8 个 SPARQL 测试修复(CGO 构建暴露)+ D-TIER-03 fail-closed 恢复
- 42-03 runbook 全门执行(Claude 代跑): 8 PASS + L8 PARTIAL + L4/L7 env-SKIP
- live 测量工具从零到一(live F1 scorer / hybrid recall / frozen replay 修复)
- 抽取 prompt writes-only 对齐 + schema 前缀剥离(live F1 0.39→0.87)
- admin 11-tab 全可用(治理白屏/SPARQL 401/MCP wiring/图谱空白画布 4 连修)
- 3D 知识图谱 + 本体视图 + 实体消解(65 险种/38 机构)+ 全屏/聚焦/标签
- TBox v1→v7 本体演进闭环全程走通(提案→人工批准→版本归档)

### What Worked

- **走查驱动**: 浏览器实测代替口头确认,每个 bug 都有截图/日志证据
- **测量先行**: 修 prompt 前先建 live F1 scorer——修完立即可证伪
- **确定性规则优先**: 实体消解用后缀规则而非模糊匹配,零误合并
- **per-gate 真实测量**: bootstrap 门从"捏造测试名"到真跑实测

### What Was Inefficient

- bootstrap 脚本三轮才跑通(CGO=0 遗留 + MCP 必填项 + LOCAL_* 路由)——脚本缺少 smoke 测试
- L1 门首跑 FAIL 后才发现评分器不存在——脚本评审时未核对测试名
- localStorage admin token 无 UI 写入路径——新浏览器必踩

### Patterns Established

- quick task 目录惯例: 诊断→plan/执行→SUMMARY→STATE 行
- live 测试 env 门惯例(SKILLFORGE_LIVE_*=1,缺省 skip 不破 hermetic)
- ONT-04 提案闭环可代跑: LLM 提案 → 人工批准(custom_local_name)→ v{N+1}

### Key Lessons

1. CGO=0 掩盖 go-duckdb 编译需求——构建矩阵必须含 CGO=1
2. 测量工具不存在时门会"假绿/假红"——先核对其存在性再跑门
3. nil-slice 序列化 null 对 FE 是地雷——API 层归一化是单点解
4. Vite HMR 多轮改动后失效,重缓存清理要进排障清单第一位

### Cost Observations

- Model mix: glm-5.3-flash(会话模型)执行;GLM quota 5h 窗口两次触顶均遇重置/充值
- Sessions: 1(跨 3 天)
- Notable: bootstrap 脚本三轮修复的教训已沉淀进 STATE.md
