#!/usr/bin/env bash
set -u
cd /mnt/d/code/ClaudeCode/ai-agent

echo "== session_meta schema =="
sqlite3 skillforge.db ".schema session_meta"

echo
echo "== session_meta current rows =="
sqlite3 skillforge.db "SELECT id FROM session_meta;"

echo
echo "== session_turn distinct session_ids =="
sqlite3 skillforge.db "SELECT COUNT(DISTINCT session_id) FROM session_turn;"

echo
echo "== audit distinct session_ids =="
sqlite3 skillforge.db "SELECT COUNT(DISTINCT session_id) FROM audit;"

echo
echo "== kpi_calls distinct =="
sqlite3 skillforge.db "SELECT COUNT(DISTINCT session_id) FROM kpi_calls;"

echo
echo "== session_turn per session_id (desc) =="
sqlite3 skillforge.db "SELECT session_id, COUNT(*) FROM session_turn GROUP BY session_id ORDER BY 2 DESC LIMIT 15;"

echo
echo "== /api/sessions vs count =="
curl -sS http://localhost:8888/api/sessions | python3 -c "import sys,json; d=json.load(sys.stdin); print('api_sessions_count:', len(d['sessions']))"