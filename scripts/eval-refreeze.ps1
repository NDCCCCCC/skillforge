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
# (reprohash.PromptHash takes it as an input). The shipped lock pins
# model_ver=glm-5.2 (= eval.DefaultJudgeModel), and the unit test asserts
# that equality. So the eval MUST run under glm-5.2 — this script drops
# LOCAL_MODEL / JUDGE_MODEL from the environment before the run (they would
# otherwise make ActiveModelVersion() drift off the pinned model) and clears
# LOCAL_* credentials the way run.bat does (they point at a dead endpoint
# and outrank GLM_*).
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

    # Pin the model to the lock's model_ver: the prompt_hash covers the model
    # id, so the eval and the pin must agree on it (see header note).
    $env:LOCAL_MODEL = 'glm-5.2'
    $env:JUDGE_MODEL = 'glm-5.2'
    # LOCAL_* credentials point at a dead endpoint and outrank GLM_* (run.bat
    # drops them for the same reason).
    foreach ($k in 'LOCAL_API_KEY', 'LOCAL_BASE_URL') {
        if (Test-Path "Env:$k") { Remove-Item "Env:$k" -Force }
    }
    Write-Host '[refreeze] running eval against the CURRENT compliance prompt (judge=glm-5.2)...'
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
