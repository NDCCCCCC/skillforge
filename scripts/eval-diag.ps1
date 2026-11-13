# SPDX-License-Identifier: Apache-2.0
# scripts/eval-diag.ps1 - run ONE eval case with a diagnostic dump, to see
# WHY a case ends as agent_error (the report only counts them, it does not
# keep the reason).
#
# The .env → process-env bridge is the same one run.bat and
# eval-refreeze.ps1 use (the server never parses .env itself).
#
# LOCAL_* vs GLM_* vs PANEL: eval.ResolveLLM resolves the admin panel
# config (~/.skillforge/llm.json) FIRST, then the env preset where LOCAL_*
# outranks GLM_*. A .env that still carries a DEAD local inference server
# (LOCAL_BASE_URL pointing at an offline box) makes every case end as
# agent_error while the report only says "agent_error: 75". -UseLocal
# keeps that behavior (for reproducing the problem); the default drops
# LOCAL_* so the GLM Coding Plan endpoint is used - same as run.bat does.
# The model is NOT hard-pinned here: a panel config wins as-is, and only
# when there is none does the script fall back to env models (any
# LOCAL_MODEL/JUDGE_MODEL kept, else glm-5.3-flash).
param(
    [string]$CaseId = 'adv-001',
    [string]$Golden = 'eval/golden/compliance.jsonl',
    [string]$Dump = 'eval/diag-case.json',
    [switch]$UseLocal
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root

# ── panel config: the same source of truth the server uses ───────────────
# The admin panel's "LLM Provider config" save writes ~/.skillforge/llm.json
# (SKILLFORGE_LLM_CONFIG overrides the path) and internal/llmconfig resolves
# it before any env preset - eval now shares that chain. Inlined helper
# (same self-contained style as the .env bridge below).
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
    Write-Host '[diag] FAIL: .env not found in repo root.'
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

if (-not $UseLocal) {
    # Drop the LOCAL_* trio: it points at an offline inference server and
    # outranks GLM_* (eval.ResolveLLM precedence). run.bat drops it for the
    # same reason.
    foreach ($k in 'LOCAL_API_KEY', 'LOCAL_BASE_URL', 'LOCAL_MODEL') {
        if (Test-Path "Env:$k") {
            Write-Host "[diag] dropping $k (dead local endpoint; GLM_* takes over)"
            Remove-Item "Env:$k" -Force
        }
    }
}

# Model resolution mirrors internal/llmconfig: panel config first, env
# preset second. Only pin env models when there is NO panel config.
$panelPath = Get-PanelConfigPath
$panel = Get-PanelConfig
$effModel = ''
$effBase = ''
if ($panel -and $panel.model) {
    Write-Host ("[diag] panel config = {0} model={1} base_url={2}" -f $panelPath, $panel.model, $panel.base_url)
    Write-Host '[diag] using panel config model - NOT pinning LOCAL_MODEL/JUDGE_MODEL'
    $effModel = $panel.model
    if ($panel.base_url) { $effBase = $panel.base_url }
} else {
    Write-Host '[diag] no panel config - falling back to env models'
    if (-not $env:LOCAL_MODEL) { $env:LOCAL_MODEL = 'glm-5.3-flash' }
    if (-not $env:JUDGE_MODEL) { $env:JUDGE_MODEL = 'glm-5.3-flash' }
    $effModel = $env:LOCAL_MODEL
    Write-Host '[diag] HINT: save the LLM Provider config once in the admin panel'
    Write-Host '[diag]       (~/.skillforge/llm.json) - the eval then uses the SAME'
    Write-Host '[diag]       model the page uses.'
}
if (-not $effBase) {
    # Approximation of the env preset (the Go side can also fall back to the
    # vendor catalog plan when base_url is empty): LOCAL_BASE_URL survives
    # only under -UseLocal, otherwise GLM_BASE_URL.
    if ($UseLocal -and $env:LOCAL_BASE_URL) { $effBase = $env:LOCAL_BASE_URL }
    else { $effBase = $env:GLM_BASE_URL }
}
Write-Host "[diag] effective base_url = $effBase"
Write-Host "[diag] effective model    = $effModel"

Write-Host "[diag] running case $CaseId with dump -> $Dump"
go run ./cmd/eval --golden=$Golden --cases=$CaseId --dump=$Dump
Write-Host "[diag] exit=$LASTEXITCODE"
if (Test-Path $Dump) {
    Write-Host '[diag] ---- dump (err / verdict fields) ----'
    $raw = Get-Content $Dump -Raw
    foreach ($m in [regex]::Matches($raw, '"(err|refusal_reason|refused|status)":\s*("[^"]*"|true|false)')) {
        Write-Host ('  ' + $m.Value)
    }
}
