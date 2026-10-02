-- Simulate the user clicking "Approve + 记住" in the ApprovalCard:
-- Create approval_rule for plan_approve target with the actual
-- args_hash from the pending audit row, so PlanRulesPanel renders it.

INSERT OR REPLACE INTO approval_rules (target, args_hash, skill_id, scope_session_id, created_at, last_used_at) VALUES
  ('plan_approve', '6696b933', 'monthly-report', 'rl-burst-98', strftime('%s','now'), strftime('%s','now'));

-- Mark the audit row as approved (D11-C3 fail-closed semantics)
UPDATE audit SET action = 'action_rule_approved' WHERE id = 321;

-- Add plan_todos rows (since the runner is interrupted, no runner wrote them yet;
-- mirror what the runner would have written as the plan progressed)
INSERT OR REPLACE INTO plan_todos (run_id, todo_id, content, active_form, status, ts, op) VALUES
  ('rl-burst-98', 'step-1', '调用核心 KPI（period=2026-07）',           '调用核心 KPI',     'pending',  strftime('%s','now'),                         'create'),
  ('rl-burst-98', 'step-2', '调用结构类 KPI（桶分布/逾期率）',            '调用结构类 KPI',   'pending',  strftime('%s','now') + 1,                     'create'),
  ('rl-burst-98', 'step-3', '基于 KPI 快照渲染月报章节 slot 文本',         '渲染报告章节',     'pending',  strftime('%s','now') + 2,                     'create'),
  ('rl-burst-98', 'step-4', '通过 report_publish 提交 2026-07 月报',     '提交月报审批',     'pending',  strftime('%s','now') + 3,                     'create');