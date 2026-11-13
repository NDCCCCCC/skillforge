# SPDX-License-Identifier: Apache-2.0
# scripts/eval-refreeze.ps1 - re-freeze the compliance eval baseline
# (Windows). Companion to run.bat: the server reads PROCESS env vars only,
# so this script parses .env into the process environment itself (same
# bridge run.bat does) before driving the eval.
#
# Why this exists: internal/eval's TestShippedFrozenLockIsInSync compares the
# shipped lock's prompt_hash against the CURRENT rendered compliance prompt.
# Editing examples/skills/compliance/SKILL.md (e.g. 260817-vsm added the
# Step-0 query classification) changes the rendered prompt, so the pin drifts
# and the unit suite goes red. The honest fix is a REAL re-freeze: re-score
# the golden set under the current prompt, then re-author the pin from that
# same prompt.
#
# ORDER (load-bearing):
#   1. run the eval  → produces eval/report.json + eval/calibration.json
#                      under the CURRENT prompt (the archived scores must
#                      describe the same prompt the lock will pin);
#   2. re-pin       → scripts/repin rewrites the lock's prompt_hash via
#                      eval.PromptHashFor (the same function that verifies it)
#                      and prints the new file sha256;
#   3. sync the byte-identity pins → the printed sha must replace
#                      EXPECTED_COMPLIANCE_SHA in scripts/verify-phase-{13,14,15,16}.sh
#      (step 3 is done by the orchestrator — the script prints the value).
#
# MODEL VERSION (load-bearing): the prompt_hash covers the model id too
# (reprohash.PromptHash takes it as an input), so the eval and the re-pin
# must agree on the model. The source of truth is the admin panel config
# (~/.skillforge/llm.json - what the page saves): internal/eval.ResolveLLM
# resolves the panel config FIRST, env preset second. This script therefore
#   - panel config present -> leaves LOCAL_MODEL/JUDGE_MODEL alone (pinning
#     env models would only obscure which model actually ran) and prints it;
#   - no panel config -> falls back to env (keeps any LOCAL_MODEL/JUDGE_MODEL,
#     else glm-5.3-flash) and asks the user to save the panel config once.
# LOCAL_* credentials are still cleared: .env's LOCAL_BASE_URL points at a
# dead local inference server and outranks GLM_* on the env-only path
# (run.bat drops them for the same reason). With a panel config the Go side
# reads the panel first anyway, so clearing LOCAL_* is always safe.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\eval-refreeze.ps1
#   powershell ... -File scripts\eval-refreeze.ps1 -Version compliance-2026-11-13-v2
#   powershell ... -File scripts\eval-refreeze.ps1 -SkipEval   # re-pin only
#
# Requires: .env in the repo root (same as run.bat), go + gcc in PATH
# (duckdb links via CGO), and a reachable retrieval backend + LLM endpoint.

param(
    [string]$Version = '',
    [switch]$SkipEval
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root

if (-not $Version) {
    $Version = 'compliance-' + (Get-Date -Format 'yyyy-MM-dd') + '-v2'
}

# ── panel config: the same source of truth the server uses ───────────────
# The admin panel's "LLM Provider config" save writes ~/.skillforge/llm.json
# (SKILLFORGE_LLM_CONFIG overrides the path) and internal/llmconfig resolves
# it before any env preset - eval now shares that chain, so "what the page
# says" is what the eval runs. PowerShell has no shared module here, so the
# helper is inlined (same self-contained style as the .env bridge below).
function Get-PanelConfigPath {
    if ($env:SKILLFORGE_LLM_CONFIG) { return $env:SKILLFORGE_LLM_CONFIG }
    return (Join-Path $HOME '.skillforge\llm.json')
}

function Get-PanelConfig {
    $p = Get-PanelConfigPath
    if (-not (Test-Path $p)) { return $null }
    try { return (Get-Content $p -Raw | ConvertFrom-Json) } catch { return $null }
}

# ── load .env into the process environment ───────────────────────────────
# The server never parses .env itself, so every tool that needs those
# variables must do this bridge (same logic as run.bat, written natively
# here instead of nesting a cmd-style powershell -Command string).
if (-not $SkipEval) {
    if (-not (Test-Path '.env')) {
        Write-Host '[refreeze] FAIL: .env not found in repo root (copy .env.example to .env first).'
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
            $q1 = $v[0]
            $q2 = $v[$v.Length - 1]
            if (($q1 -eq '"' -and $q2 -eq '"') -or ($q1 -eq "'" -and $q2 -eq "'")) {
                $v = $v.Substring(1, $v.Length - 2)
            }
        }
        Set-Item -Path "Env:$k" -Value $v
    }

    # LOCAL_* credentials point at a dead endpoint and outrank GLM_* (run.bat
    # drops them for the same reason). With a panel config the Go side reads
    # the panel first anyway, so clearing LOCAL_* is always safe.
    foreach ($k in 'LOCAL_API_KEY', 'LOCAL_BASE_URL') {
        if (Test-Path "Env:$k") { Remove-Item "Env:$k" -Force }
    }

    # Model resolution mirrors internal/llmconfig: panel config first, env
    # preset second (see header note). Only pin env models when there is NO
    # panel config - over the panel they are dead weight that hides which
    # model actually ran.
    $panelPath = Get-PanelConfigPath
    $panel = Get-PanelConfig
    if ($panel -and $panel.model) {
        Write-Host ("[refreeze] panel config = {0} model={1} base_url={2}" -f $panelPath, $panel.model, $panel.base_url)
        Write-Host "[refreeze] using panel config model=$($panel.model) - NOT pinning LOCAL_MODEL/JUDGE_MODEL"
    } else {
        Write-Host '[refreeze] no panel config - falling back to env models'
        if (-not $env:LOCAL_MODEL) { $env:LOCAL_MODEL = 'glm-5.3-flash' }
        if (-not $env:JUDGE_MODEL) { $env:JUDGE_MODEL = 'glm-5.3-flash' }
        Write-Host '[refreeze] HINT: save the LLM Provider config once in the admin panel'
        Write-Host '[refreeze]       (~/.skillforge/llm.json) - the eval then uses the SAME'
        Write-Host '[refreeze]       model the page uses.'
    }
    $effModel = if ($panel -and $panel.model) { $panel.model } else { $env:LOCAL_MODEL }
    Write-Host "[refreeze] running eval against the CURRENT compliance prompt (model=$effModel)..."
    go run ./cmd/eval --golden=eval/golden/compliance.jsonl --archive
    if ($LASTEXITCODE -ne 0) {
        Write-Host '[refreeze] eval FAILED - lock NOT re-pinned (a pin without fresh scores would be a lie).'
        exit 1
    }
    Write-Host '[refreeze] eval done - scores written to eval\report.json (+ archived under eval\reports\).'
} else {
    Write-Host '[refreeze] -SkipEval: re-pinning WITHOUT fresh scores (use only when you know the scores are current).'
}

Write-Host "[refreeze] re-pinning lock with eval_set_version=$Version"
go run ./scripts/repin --skill compliance --skills-dir examples/skills --lock eval/frozen/compliance.lock.json --version $Version
if ($LASTEXITCODE -ne 0) {
    Write-Host '[refreeze] re-pin FAILED.'
    exit 1
}

Write-Host ''
Write-Host '[refreeze] done. Remaining manual steps:'
Write-Host '  1. copy NEW_SHA256 above into EXPECTED_COMPLIANCE_SHA in'
Write-Host '     scripts\verify-phase-13.sh / -14.sh / -15.sh / -16.sh'
Write-Host '  2. go test ./internal/eval/ -run TestShippedFrozenLockIsInSync'
