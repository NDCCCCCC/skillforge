#!/usr/bin/env bash
# one-shot cleanup: delete all rl-burst-* sessions via DELETE /api/session/:id
set -u
cd /mnt/d/code/ClaudeCode/ai-agent

q() { sqlite3 skillforge.db "$1"; }

echo "== before =="
printf "  session_meta  : %s\n" "$(q 'SELECT COUNT(*) FROM session_meta;')"
printf "  session_turn  : %s\n" "$(q 'SELECT COUNT(*) FROM session_turn;')"
printf "  audit         : %s\n" "$(q 'SELECT COUNT(*) FROM audit;')"
printf "  approval_rules: %s\n" "$(q 'SELECT COUNT(*) FROM approval_rules;')"
printf "  plan_todos    : %s\n" "$(q 'SELECT COUNT(*) FROM plan_todos;')"
printf "  feedback      : %s\n" "$(q 'SELECT COUNT(*) FROM feedback;')"
printf "  tickets       : %s\n" "$(q 'SELECT COUNT(*) FROM tickets;')"

echo
echo "== deleting rl-burst-* sessions =="
N_OK=0; N_SKIP=0; N_FAIL=0
while IFS= read -r s; do
  [ -z "$s" ] && continue
  CODE=$(curl -sS -X DELETE "http://localhost:8888/api/session/$s" -o /dev/null -w "%{http_code}")
  case "$CODE" in
    200|204) N_OK=$((N_OK+1)) ;;
    404)     N_SKIP=$((N_SKIP+1)) ;;
    *)       N_FAIL=$((N_FAIL+1)); echo "  FAIL $s -> $CODE" ;;
  esac
done < <(sqlite3 skillforge.db "SELECT id FROM session_meta WHERE id LIKE 'rl-burst-%' ORDER BY id;")

echo "deleted=$N_OK  already_gone=$N_SKIP  failed=$N_FAIL"

echo
echo "== after =="
printf "  session_meta  : %s\n" "$(q 'SELECT COUNT(*) FROM session_meta;')"
printf "  session_turn  : %s\n" "$(q 'SELECT COUNT(*) FROM session_turn;')"
printf "  audit         : %s\n" "$(q 'SELECT COUNT(*) FROM audit;')"

echo
echo "== remaining sessions (top 25 by rowid DESC) =="
sqlite3 skillforge.db "SELECT id FROM session_meta ORDER BY rowid DESC LIMIT 25;"