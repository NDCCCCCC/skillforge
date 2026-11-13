# SPDX-License-Identifier: Apache-2.0
# scripts/rebuild.ps1 - one-shot rebuild for daily iteration (Windows).
#
# Companion to run.bat. run.bat only builds the server binary WHEN IT IS
# MISSING, so code changes silently stop taking effect once bin\ exists.
# This script is the "I changed code and want it live" path:
#
#   1. web:    npm run build  (web/ -> web/dist, served by Hertz same-origin)
#   2. server: go build -> bin\skillforge-server.exe (the exact path run.bat runs)
#
# It does NOT start or stop any process - start/stop stays with run.bat
# (foreground, Ctrl+C). If an old server is still listening, stop it first
# (Ctrl+C in its console), then run.bat again.
#
# NOT for release artifacts: use scripts\build-release.ps1 (dual-OS
# binaries into dist\).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\rebuild.ps1
#   powershell ... -File scripts\rebuild.ps1 -SkipWeb     # backend only
#   powershell ... -File scripts\rebuild.ps1 -SkipServer  # frontend only
#
# Requires: gcc in PATH (duckdb CGO; see scripts\check-duckdb-toolchain.ps1),
#           node/npm in PATH.

param(
    [switch]$SkipWeb,
    [switch]$SkipServer
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root
$failed = @()

# ── preflight ────────────────────────────────────────────────────────────
if (-not $SkipServer) {
    if ($null -eq (Get-Command gcc -ErrorAction SilentlyContinue)) {
        Write-Host '[rebuild] FAIL: gcc not in PATH - duckdb links via CGO.'
        Write-Host '  Diagnose with: powershell -File scripts\check-duckdb-toolchain.ps1'
        exit 1
    }
    if ($null -eq (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host '[rebuild] FAIL: go not in PATH.'
        exit 1
    }
}
if (-not $SkipWeb) {
    if ($null -eq (Get-Command npm -ErrorAction SilentlyContinue)) {
        Write-Host '[rebuild] FAIL: npm not in PATH.'
        exit 1
    }
}

# ── step 1: web ──────────────────────────────────────────────────────────
if (-not $SkipWeb) {
    Write-Host '=== rebuild web (vite build -> web/dist)'
    Push-Location (Join-Path $root 'web')
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { $failed += 'web' }
        else { Write-Host '  OK -> web\dist' }
    } finally {
        Pop-Location
    }
}

# ── step 2: server ───────────────────────────────────────────────────────
if (-not $SkipServer) {
    Write-Host "=== rebuild server (go build -> bin\skillforge-server.exe)"
    New-Item -ItemType Directory -Force -Path (Join-Path $root 'bin') | Out-Null
    go build -o (Join-Path $root 'bin\skillforge-server.exe') ./cmd/server
    if ($LASTEXITCODE -ne 0) { $failed += 'server' }
    else { Write-Host '  OK -> bin\skillforge-server.exe' }
}

# ── wrap up ──────────────────────────────────────────────────────────────
if ($failed.Count -gt 0) {
    Write-Host ("[rebuild] FAILED: " + ($failed -join ', '))
    exit 1
}
Write-Host '[rebuild] done.'
Write-Host '  Start/restart with: scripts\run.bat'
Write-Host '  (if an old server is still listening on 8888, stop it first -'
Write-Host '   Ctrl+C in its console; this script never kills processes).'
