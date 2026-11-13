@echo off
REM SPDX-License-Identifier: Apache-2.0
REM scripts/run.bat - SkillForge one-shot launcher (Windows).
REM
REM Why this exists: the server reads PROCESS env vars only - it never
REM parses .env itself. A bare `go run ./cmd/server` therefore dies with
REM "[FAIL] cmd/server: BGE_M3_URL env var not set". This script does what
REM .planning/tmp/start-milvus.ps1 does, from the repo root, with no
REM hardcoded .planning/tmp paths:
REM   1. parse .env into the process environment
REM   2. drop LOCAL_* (they point at a dead endpoint and outrank GLM_*)
REM   3. apply sane defaults (timeouts, MCP service skill, admin token)
REM   4. build the server binary if missing (bin\skillforge-server.exe)
REM   5. run it in the foreground (Ctrl+C stops it)
REM
REM Env knobs (optional, set BEFORE calling):
REM   SF_ADDR   - listen address, default 127.0.0.1:8888
REM   SF_BINARY - server binary path, default bin\skillforge-server.exe

setlocal
cd /d "%~dp0.."

if not exist ".env" (
    echo [run.bat] .env not found in repo root - copy .env.example to .env first.
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$ErrorActionPreference='Stop';" ^
  "$root=(Get-Location).Path;" ^
  "foreach ($raw in [System.IO.File]::ReadAllLines((Join-Path $root '.env'), [System.Text.Encoding]::UTF8)) {" ^
  "  $line=$raw.Trim();" ^
  "  if ($line -eq '' -or $line.StartsWith('#')) { continue };" ^
  "  if ($line -notmatch '^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$') { continue };" ^
  "  $k=$matches[1]; $v=$matches[2].Trim();" ^
  "  if ($v.Length -ge 2) { $q1=$v[0]; $q2=$v[$v.Length-1]; if (($q1 -eq '\"' -and $q2 -eq '\"') -or ($q1 -eq \"'\" -and $q2 -eq \"'\")) { $v=$v.Substring(1,$v.Length-2) } };" ^
  "  [Environment]::SetEnvironmentVariable($k,$v,'Process');" ^
  "};" ^
  "foreach ($k in 'LOCAL_API_KEY','LOCAL_BASE_URL','LOCAL_MODEL') { if (Test-Path \"Env:$k\") { Remove-Item \"Env:$k\" -Force } };" ^
  "if (-not $env:VERDICT_STREAM_TIMEOUT) { $env:VERDICT_STREAM_TIMEOUT='180s' };" ^
  "if (-not $env:SKILLFORGE_MCP_SERVICE_SKILL) { $env:SKILLFORGE_MCP_SERVICE_SKILL='simple-qa' };" ^
  "if (-not $env:SKILLFORGE_ADMIN_TOKEN) { $env:SKILLFORGE_ADMIN_TOKEN='dev-admin-token' };" ^
  "if (-not $env:SERVER_ADDR) { $env:SERVER_ADDR = if ($env:SF_ADDR) { $env:SF_ADDR } else { '127.0.0.1:8888' } };" ^
  "$bin = if ($env:SF_BINARY) { $env:SF_BINARY } else { Join-Path $root 'bin\skillforge-server.exe' };" ^
  "if (-not (Test-Path $bin)) {" ^
  "  Write-Host '[run.bat] binary missing - building (needs gcc in PATH for duckdb CGO)...';" ^
  "  New-Item -ItemType Directory -Force -Path (Split-Path $bin) | Out-Null;" ^
  "  go build -o $bin ./cmd/server;" ^
  "  if ($LASTEXITCODE -ne 0) { Write-Host '[run.bat] build FAILED'; exit 1 }" ^
  "};" ^
  "Write-Host (\"[run.bat] addr={0} model={1} retriever={2}\" -f $env:SERVER_ADDR, $(if($env:GLM_MODEL){$env:GLM_MODEL}else{'glm-5.3-flash(default)'}), $env:RETRIEVER_BACKEND);" ^
  "& $bin"

endlocal
