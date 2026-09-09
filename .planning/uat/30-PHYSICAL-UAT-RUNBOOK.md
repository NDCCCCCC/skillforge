# v1.2 Physical UAT Runbook — Operator Self-Verify

**Generated**: 2026-09-08 by AI agent  
**Status**: 6/6 UAT items have AI-done API portion + operator-pending physical portion  
**Goal**: Operator亲自跑完标 `[operator]` 的物理项，全部 PASS 后 → `/gsd:complete-milestone`

---

## 🟢 Pre-flight Check（1 分钟）

| Check | Command | Expected |
|---|---|---|
| 服务在跑 | `curl -sS http://localhost:8888/api/knowledge/graph \| head -c 80` | JSON 含 `"tiers"` |
| Server binary 版本 | `stat -c '%y %n' /tmp/skillforge-uat26` | 2026-09-08 11:02 后（rebuild 后） |
| Admin token 加载 | `curl -sS -H "X-Admin-Token: $SKILLFORGE_ADMIN_TOKEN" http://localhost:8888/api/admin/kb \| head -c 80` | JSON `{"bases":...}` |
| 内网服务 | 已由 AI 验证（Milvus + BGE 可达，GLM 可达）| — |

---

## 1️⃣ 27-UAT — Phase 27 doc-upload（1 项物理）

### Step 6：浏览器物理上传（AI 跑过 Step 1-5 + 7）

**Step**：
1. 浏览器开 `http://localhost:8888/admin`
2. 登录 admin token（如果需要）
3. 滚到「上传文档」panel
4. 选 KB → 选 `test.md` → submit
5. 等 1-2 秒
6. 看右下角 toast

**Expected**：
- 绿色 toast「1 个片段已入库」
- 上传后 `/api/admin/kb/authority` 文档数 +1
- `/api/knowledge/graph` 的 authority tier chunks 数 +1

**Record**：
```
27-UAT Step 6: [PASS / FAIL] — toast 文案: ____
27-UAT Item 1 总判定: [PASS / FAIL]
```

---

## 2️⃣ 28-UAT — Phase 28 dynamic-graph（2 项物理）

### Section A 步骤 5：浏览器看 GraphCanvas

**Step**：
1. 浏览器开 `http://localhost:8888/admin` → 知识图谱 tab（或 `/graph` 视图）
2. 等待 graph 加载（5-10s）
3. 观察：
   - Tier 节点（authority / draft / historical / personal）4 个
   - Doc 节点从 tier 节点展开
   - Edges 用不同颜色渲染（keyword_cooccur / section_adjacent / version_chain / citation_cooccur）
4. 点击任一 doc 节点 → 中间视图打开

**Expected**：
- 4 个 tier 节点可见（authority 含 18 chunks）
- Doc 节点从 authority tier 展开（17 个文件）
- Edges 渲染有中文字段标签
- 点击 doc 节点 → DocViewer 打开，显示 markdown 源码

**Record**：
```
28-UAT Section A 步骤 5: [PASS / FAIL] — tier 数: ___ doc 数: ___ edges 类型: ___
```

### Section A 步骤 6：InspectorRetrieval 看"常被一起引用"

**Step**：
1. 浏览器开 `http://localhost:8888/admin` → 选一个 chat session 看 InspectorRetrieval
2. 点一个 citation chunk 节点
3. 观察右侧 "常被一起引用" panel

**Expected**：
- Panel 显示 edges 列表（中文标签：keyword_cooccur / section_adjacent / version_chain / citation_cooccur）
- Edges 数 > 0

**Record**：
```
28-UAT Section A 步骤 6: [PASS / FAIL] — edges shown: ___
28-UAT Item 2 总判定: [PASS / FAIL]
```

---

## 3️⃣ 29-UAT — Phase 29 feedback+ticket（约 8 项物理）

### Test 1.1：浏览器 chat 投票链路

**Step**：
1. 浏览器开 `http://localhost:8888/`
2. 在 chat 框输入「差旅报销标准是什么？」→ Enter
3. 等 assistant 回复（含引用）
4. 看回复下方是否出现 FeedbackRow（有帮助 / 没帮助 / 知识可能过时）

**Expected**：FeedbackRow 出现

**Record**：`29-UAT Test 1.1: [PASS / FAIL]`

### Test 1.3：点击「有帮助」按钮

**Step**：
1. 继续 Test 1.1
2. 点「有帮助」按钮
3. 看 Network 面板：`POST /api/feedback` → **201**，`{"status":"recorded","verdict":"helpful"}`
4. 看 pill 变 active，其余两项禁用
5. 看右下角 toast「已记录反馈：有帮助」

**Expected**：
- 201 + body `{"status":"recorded","verdict":"helpful"}`
- Pill 锁定 active
- Toast 渲染「已记录反馈：有帮助」

**Record**：`29-UAT Test 1.3: [PASS / FAIL]`

### Test 2.1：重复点反馈项（幂等）

**Step**：
1. 继续 Test 1.3
2. 再点同一条消息的反馈项（任何）
3. 看 Network：`POST /api/feedback` → **200** + `{"status":"already_recorded","verdict":"helpful"}`
4. 看 info toast「该消息已反馈过：有帮助」

**Expected**：
- 200 + already_recorded
- Toast 出现
- **不**出现第二个反馈行

**Record**：`29-UAT Test 2.1: [PASS / FAIL]`

### Test 2.3：刷新页面 → 锁定态恢复

**Step**：
1. 继续 Test 2.1
2. 刷新浏览器
3. 重新打开同 session
4. 看该消息 FeedbackRow 是否直接呈锁定态（active pill = helpful）
5. Network 面板：零额外 POST 请求（数据来源是 history voted_verdict 回显）

**Expected**：刷新后直接锁定态，零额外 POST

**Record**：`29-UAT Test 2.3: [PASS / FAIL]`

### Test 3.1：点「知识可能过时」+ comment

**Step**：
1. 在 chat 里对另一轮回复点「知识可能过时」
2. 输入框出现，填一行说明（如「24个月太长」）
3. 提交
4. Network：201 + `{"status":"recorded","verdict":"outdated"}`
5. 输入框收起

**Record**：`29-UAT Test 3.1: [PASS / FAIL]`

### Test 3.4-7：/admin 复核 tab 视觉

**Step**：
1. 浏览器开 `/admin` → 复核 tab（第 7 个 tab）
2. 找刚才那条 outdated 报告（chunk_id 应该一致）
3. 验证：
   - 聚合行（report_count）
   - FBK-05 信任横幅「不会自动修改知识库内容」常驻
   - 点「说明 (n)」展开看 comments
   - 点三处置之一（已更新 / 误报 / 忽略）→ 201 + closed_count toast
   - 切到已处置过滤 → 三色 badge（compliant / non-compliant / unanswered）

**Record**：`29-UAT Test 3.4-7: [PASS / FAIL]`

### Test 4：重开语义

**Step**：
1. 对 Test 3 已处置的 chunk 重新发起新对话
2. 再次点「知识可能过时」
3. /admin 复核 tab 看该 chunk 重新出现（report_count 从新批次起算）
4. 再处置一次 → 历史列表保留两条处置记录

**Record**：`29-UAT Test 4: [PASS / FAIL]`

### Test 5.1-6：/admin 反馈 tab 视觉

**Step**：
1. 浏览器 `/admin` → 反馈 tab（第 6 个 tab）
2. 验证：
   - 页面常驻全票口径注（帮助率/过时率的分母=全部投票）
   - 选 0 票维度 → 比率显示「—」（非 0% 或 NaN）
   - 时间范围 7/30 切换 + 维度 Skill/知识库切换 → 数据重取
   - per-KB 区域双计脚注可见
   - 点「待复核 N 条」cross-link → 跳到复核 tab

**Record**：`29-UAT Test 5.1-6: [PASS / FAIL]`

### Test 6.1-2：工单 provider=none 视觉

**Step**：
1. chat 里对一条回复展开「创建工单」
2. 看表单：预填 chatText（截 200 字）+ mono 引用条
3. 填并提交
4. 看：201 + 「已建单」禁用 trigger + 文案「已受理落库、无外部系统」

**Record**：`29-UAT Test 6.1-2: [PASS / FAIL]`

### Test 7：工单 webhook 实发（最复杂，需要本地假端点）

**Step**：
1. 起一个本地假 webhook：
```bash
# 另开 WSL terminal
python3 -c "
from http.server import BaseHTTPRequestHandler, HTTPServer
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        secret = self.headers.get('X-SkillForge-Ticket-Secret', '')
        ln = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(ln).decode('utf-8')
        print('SECRET:', secret, 'BODY:', body[:200])
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'ok')
HTTPServer(('127.0.0.1', 9090), H).serve_forever()
"
```
2. 用 webhook 配置重启 server：
```bash
wsl -e bash -lc "
kill 597964 2>/dev/null
sleep 2
cd /mnt/d/code/ClaudeCode/ai-agent
nohup env \
  BGE_M3_AUTH='' \
  MILVUS_ADDRESS='10.62.10.34:19530' \
  SKILLFORGE_ADMIN_TOKEN='uat-local-admin' \
  MILVUS_TOKEN='' \
  BGE_M3_MODEL='bge-m3' \
  GLM_API_KEY='42d522ee19f340b89b8b446700a8e061.BQRskkZTvjdRrLGi' \
  GLM_BASE_URL='https://open.bigmodel.cn/api/coding/paas/v4' \
  BGE_M3_URL='http://10.62.0.123:11434/v1/embeddings' \
  RETRIEVER_BACKEND='milvus' \
  SKILLFORGE_TICKET_PROVIDER='webhook' \
  SKILLFORGE_TICKET_WEBHOOK_URL='http://127.0.0.1:9090/hook' \
  SKILLFORGE_TICKET_WEBHOOK_SECRET='s3cret' \
  /tmp/skillforge-uat26 \
  >> /mnt/d/code/ClaudeCode/ai-agent/build/uat26-final.log 2>&1 &
disown
"
```
3. chat 里创建一张工单
4. 看：
 4.1. 201 + dispatch_status="queued"
 4.2. 假端 Python 脚本 stdout 收到 POST + X-SkillForge-Ticket-Secret: s3cret
 4.3. URL 无 query（rawQuery == ""）
5. 改假端返 5xx 再建一单 → 仍 201 queued + audit 行 failed

**Record**：`29-UAT Test 7: [PASS / FAIL]`

### Test 8.5：Phase 27→29 联合验收 — 知识库变更只由上传产生

**Step**：
1. chat 提问命中你刚上传的 doc
2. 回复引用新 chunk（[ref-N]）
3. GraphCanvas + InspectorRetrieval 同步显示新 doc / chunk

**Expected**：KB 变更唯一入口是上传通道（已由 API 验证：disposition 后 KB bases 数不变）

**Record**：`29-UAT Test 8.5: [PASS / FAIL]`

### 29-UAT Item 3 总判定

```
29-UAT Item 3: [PASS / FAIL] — failed tests (if any): ___
```

---

## 4️⃣ 29-HUMAN-UAT — Phase 29 human-verification（9 项）

### Test 1：实时投票流程
- 操作：chat 发起 → assistant 回复 → 点「有帮助」→ 201 + pill 锁定 + 真实 toast「已记录反馈：…」
- 期望：上述全部
- **Record**：`Test 1: [PASS / FAIL]`

### Test 2：幂等重复点击 + 恢复回显
- 操作：重复点 → already_recorded toast「该消息已反馈过：…」
- 操作：刷新/恢复会话 → 锁定态由 history voted_verdict 回显
- 期望：上述 + Network 面板无额外 POST
- **Record**：`Test 2: [PASS / FAIL]`

### Test 3：outdated → /admin 复核 → 处置
- 操作：报告知识过时（可附说明）→ /admin 复核 tab 聚合行 → 处置 → closed_count toast
- 期望：FBK-05 信任横幅「不会自动修改知识库内容」常驻
- **Record**：`Test 3: [PASS / FAIL]`

### Test 4：重开语义
- 处置后同一 chunk 收到新 outdated 报告 → 重新进入待复核
- **Record**：`Test 4: [PASS / FAIL]`

### Test 5：指标 tab 视觉
- /admin 反馈 tab：口径注常驻、0 票「—」非 0%、日/周趋势切换、待复核 cross-link 跳转
- **Record**：`Test 5: [PASS / FAIL]`

### Test 6：工单 provider=none
- 创建工单 → TKT-{n} + 诚实文案「未配置外发渠道，工单已留档」；重复点击返既有编号
- **Record**：`Test 6: [PASS / FAIL]`

### Test 7：工单 webhook 实发
- 配置 SKILLFORGE_TICKET_* webhook → 建单 → 排队 → 外发携带 X-SkillForge-Ticket-Secret
- dispatch 审计行可见于 /api/admin/audit
- 5xx 端点 → failed 审计行 + 主流程不受影响
- **Record**：`Test 7: [PASS / FAIL]`

### Test 8：Phase 27→29 联合验收
- 复核处置后知识库/检索零变化（处置 = 信号状态 + 审计）
- 知识库变更唯一来源仍是 27 上传流程（FBK-05 实机确认）
- **Record**：`Test 8: [PASS / FAIL]`

### Test 9：视觉走查
- FeedbackRow 布局/间距按 29-UI-SPEC
- toast 在 chat/eval/admin 三处均渲染
- /admin 7 tab 顺序（反馈第 6、复核第 7）
- **Record**：`Test 9: [PASS / FAIL]`

### 29-HUMAN-UAT 总判定

```
29-HUMAN-UAT Item 4: [PASS / FAIL] — failed tests: ___
```

---

## 6️⃣ Item #6 Phase 30 加固修复 — WR-06 视觉走查

### WR-06：迟到 votedVerdict 锁定生效

**Step**：
1. chat 发起提问
2. **不点反馈**
3. 在另一台机器/另一窗口触发 feedback POST（脚本或 curl）：
```bash
echo '{"session_id":"<your_session>","turn_id":<turn_id>,"verdict":"helpful"}' > /tmp/late-fb.json
curl -sS -X POST http://localhost:8888/api/feedback \
  -H 'Content-Type: application/json; charset=utf-8' \
  --data-binary @/tmp/late-fb.json \
  -w '\nHTTP %{http_code}\n'
```
4. 回浏览器，看 FeedbackRow 是否自动锁定为 helpful（无需点击）
5. Network：应该有 `data-turn` part 携带 `voted_verdict` 字段

**Expected**：
- FeedbackRow 自动锁定 active
- 零额外 POST
- 客户端零反馈行消失

**Record**：`WR-06: [PASS / FAIL]`

### Item #6 总判定

```
Item #6: [PASS / FAIL] — failed items: ___
```

---

## 全部 PASS 后

1. 更新 `.planning/phases/30-hygiene-milestone-gate/30-HUMAN-UAT.md` 改 `status: complete` + passed 计数
2. 跑 `/gsd:complete-milestone`：
```
/gsd:complete-milestone
```
3. 自动产出：
   - CHANGELOG.md 更新
   - RELEASE-NOTES.md 写入
   - milestone-summary 写入 `.planning/milestones/v1.2/`
   - ROADMAP.md v1.2 状态 flip 到 `shipped`

---

## 失败时（任一项 FAIL）

1. 把 FAIL 项 result 记录在上面
2. 把缺口反馈到 GSD：
```
/gsd:plan-phase 30 --gaps
```
3. 创建 gap-closure plan 并 re-execute

---

## 联系方式

- AI 已 PASS 部分详见：`.planning/uat/30-cycle-run-2026-09-08.md`
- 本 runbook：`.planning/uat/30-PHYSICAL-UAT-RUNBOOK.md`（本文件）
- 30-HUMAN-UAT 状态文件：`.planning/phases/30-hygiene-milestone-gate/30-HUMAN-UAT.md`