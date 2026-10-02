#!/usr/bin/env bash
# Seed approval_rules + plan_todos tables directly so admin console
# (ApprovalRulesPanel + PlanRulesPanel) have renderable test data.
#
# Schema:
#   approval_rules(target, args_hash, skill_id, scope_session_id, created_at, last_used_at)
#   plan_todos   (run_id, todo_id, content, active_form, status, ts, op)
#
# Scope session picks a real session from /api/sessions roster so the
# ApprovalRulesPanel's per-session filter has matching rows. plan_todos
# uses a synthetic run_id (plan runner owns run_id space, not session).
set -eu
cd /mnt/d/code/ClaudeCode/ai-agent

SESS=$(curl -sS http://localhost:8888/api/sessions | python3 -c "
import sys,json
d = json.load(sys.stdin)
print(d['sessions'][0]['id'])")
echo "seed session: $SESS"
NOW=$(date +%s)

# ---- approval_rules: 3 rows covering scope=session + scope=skill + 2 targets
sqlite3 skillforge.db <<SQL
INSERT OR REPLACE INTO approval_rules (target, args_hash, skill_id, scope_session_id, created_at, last_used_at) VALUES
  ('memory:facts',   'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2', 'external-lookup',  '$SESS', $NOW, $NOW),
  ('review_report',  'b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3', 'external-lookup',  '$SESS', $NOW, $NOW),
  ('memory:facts',   'c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4', 'monthly-report',   '',     $NOW, $NOW);
SQL
echo "approval_rules now: $(sqlite3 skillforge.db 'SELECT COUNT(*) FROM approval_rules;')"

# ---- plan_todos: a complete plan lifecycle in one run_id
RUN="run-uat-$(date +%s)"
sqlite3 skillforge.db <<SQL
INSERT OR REPLACE INTO plan_todos (run_id, todo_id, content, active_form, status, ts, op) VALUES
  ('$RUN', 'todo-1', '收集 2026-08 月度数据',     '收集月度数据中',       'pending',  $NOW,        'create'),
  ('$RUN', 'todo-1', '收集 2026-08 月度数据',     '收集月度数据中',       'active',  $((NOW+5)),  'update'),
  ('$RUN', 'todo-1', '收集 2026-08 月度数据',     '收集月度数据中',       'completed',$((NOW+30)), 'complete'),
  ('$RUN', 'todo-2', '生成 KPI 摘要表',           '生成 KPI 摘要表中',     'pending',  $((NOW+1)),  'create'),
  ('$RUN', 'todo-2', '生成 KPI 摘要表',           '生成 KPI 摘要表中',     'active',  $((NOW+35)), 'update'),
  ('$RUN', 'todo-3', '渲染月度报告 markdown',     '渲染月度报告中',       'pending',  $((NOW+2)),  'create');
SQL
echo "plan_todos now:    $(sqlite3 skillforge.db 'SELECT COUNT(*) FROM plan_todos;')"
echo "seeded run_id:     $RUN"

echo
echo "== verify via API =="
echo "-- approval-rules?scope=session --"
curl -sS "http://localhost:8888/api/session/$SESS/approval-rules?scope=session" | python3 -m json.tool
echo "-- approval-rules?scope=skill --"
curl -sS "http://localhost:8888/api/session/$SESS/approval-rules?scope=skill&skill_id=monthly-report" | python3 -m json.tool
echo "-- plan replay --"
curl -sS "http://localhost:8888/api/run/$RUN/plan" | python3 -m json.tool