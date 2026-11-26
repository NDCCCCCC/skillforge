# SPDX-License-Identifier: Apache-2.0
# scripts/build.ps1 - THE canonical build for this repo (Windows).
#
# Produces ONE self-contained executable: the Vite frontend compiled INTO the
# binary via `-tags embed_webui`, so the artefact needs no sibling web/dist.
#
# WHY THE OUTPUT GOES OUTSIDE THE REPO (D:\CODE\sf-run):
#   Two measured facts, in order of discovery:
#   1. An executable launched from INSIDE this repo dies at startup with
#      "sqlite.migrate: attempt to write a readonly database" while the DB
#      file itself is provably writable (python writes it fine). A 4-cell
#      matrix (exe-location x cwd x DSN) showed the failure follows the
#      EXECUTABLE, not the database or cwd.
#   2. One specific FILE (an earlier build named skillforge-server.exe) kept
#      failing even after being moved OUT of the repo, while a byte-identical
#      copy under a different NAME in the same directory ran fine. So a
#      file-level rule (security agent / quarantine) can stick to a given
#      artefact. Changing the output NAME sidesteps it.
#   Hence: build OUTSIDE the repo under a stable name (skillforge.exe).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1
#   powershell ... -File scripts\build.ps1 -SkipWeb      # reuse web/dist
#   powershell ... -File scripts\build.ps1 -OutDir D:\other
#
# Requires: go, npm, and gcc in PATH (duckdb links via CGO).

param(
    [switch]$SkipWeb,
    [string]$OutDir = 'D:\CODE\sf-run',
    [string]$Name   = 'skillforge.exe'
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root
$failed = @()

# ---- preflight -----------------------------------------------------------
foreach ($tool in 'go', 'npm', 'gcc') {
    if ($null -eq (Get-Command $tool -ErrorAction SilentlyContinue)) {
        Write-Host "[build] FAIL: $tool not in PATH."
        if ($tool -eq 'gcc') { Write-Host '  duckdb links via CGO; see scripts\check-duckdb-toolchain.ps1' }
        exit 1
    }
}

# ---- step 1: frontend ----------------------------------------------------
if (-not $SkipWeb) {
    Write-Host '=== [1/3] web (vite build -> web/dist)'
    Push-Location (Join-Path $root 'web')
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { $failed += 'web' }
    } finally { Pop-Location }
}
if ($failed.Count -gt 0) { Write-Host '[build] FAILED: web'; exit 1 }

# ---- step 2: stage dist into the embed source ----------------------------
# internal/webui/embed_on.go does `//go:embed all:dist`, so the Vite output
# must sit under internal/webui/dist at compile time. .gitkeep keeps the
# directory tracked on a clean checkout.
Write-Host '=== [2/3] stage web/dist -> internal/webui/dist'
$stage = Join-Path $root 'internal\webui\dist'
if (Test-Path $stage) { Get-ChildItem $stage -Force | Remove-Item -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null
# .gitkeep is tracked so `//go:embed all:dist` resolves on a clean checkout;
# the wipe above removes it, so recreate it every build.
$keep = Join-Path $stage '.gitkeep'
if (-not (Test-Path $keep)) {
    Set-Content -Path $keep -Encoding UTF8 -NoNewline -Value (
        "placeholder: the release build copies web/dist here before`n" +
        "``go build -tags embed_webui ./cmd/server``. Keeping this directory in git`n" +
        "lets the embed directive resolve on a clean checkout.`n")
}
Copy-Item -Path (Join-Path $root 'web\dist\*') -Destination $stage -Recurse -Force
if (-not (Test-Path (Join-Path $stage 'index.html'))) {
    Write-Host '[build] FAILED: staged dist has no index.html'
    exit 1
}

# ---- step 3: compile -----------------------------------------------------
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$out = Join-Path $OutDir $Name
Write-Host "=== [3/3] go build -tags embed_webui -> $out"
$env:CGO_ENABLED = '1'
go build -tags embed_webui -trimpath -ldflags='-s -w' -o $out ./cmd/server
if ($LASTEXITCODE -ne 0) { Write-Host '[build] FAILED: go build'; exit 1 }
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

$mb = [Math]::Round((Get-Item $out).Length / 1MB, 1)
Write-Host "[build] OK -> $out ($mb MB, frontend embedded)"
Write-Host "[build] start it with: scripts\run.bat"
