# SPDX-License-Identifier: Apache-2.0
# scripts/eval-models.ps1 - list the models the configured LLM endpoint
# actually serves, and check whether the effective model is among them.
#
# The effective model comes from the admin panel config
# (~/.skillforge/llm.json - what the page saves), the same source
# internal/llmconfig resolves for both the server and eval. If the
# deployment's account group does not serve that model id, every eval
# case ends as agent_error with a 404 ("Model X is not supported by any
# configured account in this group") - a failure mode the report only
# summarises as a count, not a cause. This script shows the served model
# list so the page-selected model can be aligned with reality BEFORE a
# 75-case run. Without a panel config it probes GLM_BASE_URL/GLM_API_KEY
# instead and says so.
param(
    [string]$Model = ''
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root

# ── panel config: the same source of truth the server uses ───────────────
# SKILLFORGE_LLM_CONFIG overrides the path. Inlined helper (PowerShell has
# no shared module here; same self-contained style as the .env bridge).
function Get-PanelConfigPath {
    if ($env:SKILLFORGE_LLM_CONFIG) { return $env:SKILLFORGE_LLM_CONFIG }
    return (Join-Path $HOME '.skillforge\llm.json')
}

function Get-PanelConfig {
    $p = Get-PanelConfigPath
    if (-not (Test-Path $p)) { return $null }
    try { return (Get-Content $p -Raw | ConvertFrom-Json) } catch { return $null }
}

if (-not (Test-Path '.env')) {
    Write-Host '[models] FAIL: .env not found in repo root.'
    exit 1
}

$envPath = (Resolve-Path '.env').Path
foreach ($raw in [System.IO.File]::ReadAllLines($envPath, [System.Text.Encoding]::UTF8)) {
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
    Set-Item -Path "Env:$k" -Value $v
}

foreach ($k in 'LOCAL_API_KEY', 'LOCAL_BASE_URL', 'LOCAL_MODEL') {
    if (Test-Path "Env:$k") { Remove-Item "Env:$k" -Force }
}

# Endpoint + key resolution mirrors internal/llmconfig: panel config first,
# env preset (GLM_*) second - so this probe hits the SAME endpoint the
# server and eval now use.
$panelPath = Get-PanelConfigPath
$panel = Get-PanelConfig
$base = $null
$key = $null
$baseSrc = ''
$keySrc = ''
if ($panel) {
    Write-Host ("[models] panel config = {0} model={1} base_url={2}" -f $panelPath, $panel.model, $panel.base_url)
    if ($panel.base_url) { $base = $panel.base_url; $baseSrc = 'panel base_url' }
    # ResolveKey semantics (internal/llmconfig/store.go): api_key_env wins
    # over plaintext api_key; an env var that is set-but-empty counts as
    # missing (that config would only ever 401).
    if ($panel.api_key_env) {
        $key = [Environment]::GetEnvironmentVariable($panel.api_key_env)
        $keySrc = "panel api_key_env=$($panel.api_key_env)"
        if (-not $key) {
            Write-Host "[models] WARNING: env var '$($panel.api_key_env)' is not set or empty - requests will 401."
        }
    } elseif ($panel.api_key) {
        $key = $panel.api_key
        $keySrc = 'panel api_key'
    }
} else {
    Write-Host '[models] no panel config - falling back to env (GLM_BASE_URL / GLM_API_KEY)'
    Write-Host '[models] HINT: save the LLM Provider config once in the admin panel'
    Write-Host '[models]       (~/.skillforge/llm.json) - the probe then uses the SAME'
    Write-Host '[models]       endpoint the page uses.'
}
if (-not $base) { $base = $env:GLM_BASE_URL; $baseSrc = 'env GLM_BASE_URL' }
if (-not $key) { $key = $env:GLM_API_KEY; $keySrc = 'env GLM_API_KEY' }
if (-not $base) {
    Write-Host '[models] FAIL: no endpoint - save one in the admin panel (LLM Provider config) or set GLM_BASE_URL.'
    exit 1
}

# Target model to check: -Model param, else the page-selected one.
$target = $Model
$targetSrc = '-Model param'
if (-not $target -and $panel -and $panel.model) {
    $target = $panel.model
    $targetSrc = 'panel config'
}

Write-Host "[models] endpoint = $base (from $baseSrc)"
Write-Host "[models] api key  = from $keySrc"
try {
    $resp = Invoke-WebRequest -Uri "$base/models" -Headers @{ Authorization = "Bearer $key" } -TimeoutSec 30 -UseBasicParsing
    $json = $resp.Content | ConvertFrom-Json
    $ids = @($json.data | ForEach-Object { $_.id })
    Write-Host "[models] $($ids.Count) model(s) served:"
    $ids | Sort-Object | ForEach-Object { Write-Host "  $_" }
    if ($target) {
        if ($ids -contains $target) {
            Write-Host "[models] OK - '$target' (from $targetSrc) IS served by this endpoint."
        } else {
            Write-Host "[models] WARNING - '$target' (from $targetSrc) is NOT served by this endpoint."
            Write-Host '[models] eval resolves its model from the panel config first; a 404 on that'
            Write-Host '[models] id means every eval case scores as agent_error. Pick a served id'
            Write-Host '[models] above in the admin panel (LLM Provider config).'
        }
    }
} catch {
    Write-Host ("[models] FAIL: " + $_.Exception.Message)
    exit 1
}
