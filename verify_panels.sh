#!/usr/bin/env bash
set -u
cd /mnt/d/code/ClaudeCode/ai-agent

q() { sqlite3 skillforge.db "$1"; }

echo "== state =="
printf "  approval_rules: %s\n" "$(q 'SELECT COUNT(*) FROM approval_rules;')"
printf "  plan_todos:     %s\n" "$(q 'SELECT COUNT(*) FROM plan_todos;')"
printf "  audit #321:     %s\n" "$(q 'SELECT action FROM audit WHERE id=321;')"

echo
echo "== /api/session/rl-burst-98/approval-rules?scope=skill&skill_id=monthly-report =="
curl -sS "http://localhost:8888/api/session/rl-burst-98/approval-rules?scope=skill&skill_id=monthly-report" | python3 -m json.tool

echo
echo "== /api/run/rl-burst-98/plan =="
curl -sS "http://localhost:8888/api/run/rl-burst-98/plan" | python3 -m json.tool