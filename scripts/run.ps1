# SPDX-License-Identifier: Apache-2.0
# scripts/run.ps1 - THE canonical launcher for this repo (Windows / PowerShell).
#
# Why this exists (two hard constraints, both measured on this host):
#   1. The server reads PROCESS env vars only -- it never parses .env itself.
#      A bare `go run ./cmd/server` therefore dies with
#      "[FAIL] cmd/server: BGE_M3_URL env var not set".
#   2. The BINARY MUST LIVE OUTSIDE THE REPO. An exe launched from inside the
#      repo dies at startup with "sqlite.migrate: attempt to write a readonly
#      database" even though the DB file is writable (python writes it fine).
#      A 4-cell matrix (exe-location x cwd x DSN) proved the failure follows
#      the EXECUTABLE, not the database. Separately, ONE earlier artefact kept
#      failing even after moving out of the repo while a byte-identical copy
#      under another NAME ran fine -- so the artefact NAME matters too.
#      This script therefore launches $SfRunDir\skillforge.exe while keeping
#      the repo as the working directory (skills/ + knowledge/ resolve
#      relative to cwd).
#
# What it does:
#   1. parse .env into the process environment (line-by-line, no sourcing)
#   2. drop LOCAL_* (they point at a DEAD endpoint and would outrank GLM_*)
#   3. apply defaults (stream timeout, MCP service skill, admin token, DSN)
#   4. make SQLITE_DSN absolute and point it at $SfRunDir (outside the repo)
#   5. build the binary if missing (via scripts\build.ps1)
#   6. run it in the foreground (Ctrl+C stops it)
#
# Env knobs (set BEFORE calling; the script honours them):
#   SF_RUN_DIR - where the binary + default DB live   (default D:\CODE\sf-run)
#   SF_ADDR    - listen address                       (default 127.0.0.1:8888)
#   SF_BINARY  - explicit binary path                  (overrides SF_RUN_DIR)
#   SF_DSN     - explicit SQLite path                 (overrides the default)

param(
    [switch]$NoBuild,
    [switch]$SkipEnv
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root

$runDir = if ($env:SF_RUN_DIR) { $env:SF_RUN_DIR } else { 'D:\CODE\sf-run' }
$bin    = if ($env:SF_BINARY)  { $env:SF_BINARY }  else { Join-Path $runDir 'skillforge.exe' }

# ---- 1+2: .env -> process env, minus the dead LOCAL_* triple -------------
if (-not $SkipEnv) {
    $envFile = Join-Path $root '.env'
    if (-not (Test-Path $envFile)) {
        Write-Host '[run] .env not found - copy .env.example to .env first.'
        exit 1
    }
    $loaded = 0
    foreach ($raw in [System.IO.File]::ReadAllLines($envFile, [System.Text.Encoding]::UTF8)) {
        $line = $raw.Trim()
        if ($line -eq '' -or $line.StartsWith('#')) { continue }
        if ($line -notmatch '^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$') { continue }
        $k = $matches[1]
        $v = $matches[2].Trim()
        if ($v.Length -ge 2) {
            $q1 = $v[0]; $q2 = $v[$v.Length - 1]
            if (($q1 -eq '"' -and $q2 -eq '"') -or ($q1 -eq "'" -and $q2 -eq "'")) {
                $v = $v.Substring(1, $v.Length - 2)
            }
        }
        [Environment]::SetEnvironmentVariable($k, $v, 'Process')
        $loaded++
    }
    foreach ($k in 'LOCAL_API_KEY', 'LOCAL_BASE_URL', 'LOCAL_MODEL') {
        if (Test-Path "Env:$k") { Remove-Item "Env:$k" -Force }
    }
    Write-Host "[run] loaded $loaded variable(s) from .env (values not echoed)"
}

# ---- 3: defaults ---------------------------------------------------------
if (-not $env:VERDICT_STREAM_TIMEOUT)      { $env:VERDICT_STREAM_TIMEOUT = '180s' }
if (-not $env:SKILLFORGE_MCP_SERVICE_SKILL) { $env:SKILLFORGE_MCP_SERVICE_SKILL = 'simple-qa' }
if (-not $env:SERVER_ADDR) {
    $env:SERVER_ADDR = if ($env:SF_ADDR) { $env:SF_ADDR } else { '127.0.0.1:8888' }
}

# ---- 4: absolute DSN outside the repo -----------------------------------
New-Item -ItemType Directory -Force -Path $runDir | Out-Null
if ($env:SF_DSN) {
    $env:SQLITE_DSN = $env:SF_DSN
} elseif (-not $env:SQLITE_DSN -or -not [System.IO.Path]::IsPathRooted($env:SQLITE_DSN)) {
    # A relative DSN resolves against CWD (the repo). Pin it to $runDir.
    $env:SQLITE_DSN = Join-Path $runDir 'skillforge.db'
} else {
    # An absolute DSN from .env is honoured as-is; only guard the repo case.
    $absDsn = [System.IO.Path]::GetFullPath($env:SQLITE_DSN)
    if ($absDsn.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) {
        Write-Host "[run] WARN: .env SQLITE_DSN points inside the repo ($absDsn)."
        Write-Host "[run]       DLP may block writes there; prefer $runDir\skillforge.db"
    }
    $env:SQLITE_DSN = $absDsn
}

# ---- 5: build if missing -------------------------------------------------
if (-not (Test-Path $bin)) {
    if ($NoBuild) {
        Write-Host "[run] FAIL: $bin is missing and -NoBuild was given."
        Write-Host '[run]       build it: powershell -File scripts\build.ps1'
        exit 1
    }
    Write-Host "[run] binary missing - building via scripts\build.ps1 (~2-4 min)"
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $root 'scripts\build.ps1') -OutDir $runDir
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $bin)) {
        Write-Host '[run] FAIL: build did not produce the binary.'
        exit 1
    }
}

# ---- 6: launch -----------------------------------------------------------
Write-Host ("[run] binary  = {0}" -f $bin)
Write-Host ("[run] cwd     = {0}" -f $root)
Write-Host ("[run] addr    = {0}" -f $env:SERVER_ADDR)
Write-Host ("[run] dsn     = {0}" -f $env:SQLITE_DSN)
Write-Host ("[run] model   = {0}" -f $(if ($env:GLM_MODEL) { $env:GLM_MODEL } else { 'glm-5.3-flash (default)' }))
Write-Host ("[run] retriever= {0}" -f $env:RETRIEVER_BACKEND)
Write-Host ''
& $bin
