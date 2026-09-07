# Phase 27 Plan 01: Parser Abstraction — BLOCKED

**Plan:** 27-01
**Tasks Completed:** 0 / 3
**Status:** BLOCKED at Task 1 (dependency acquisition)

## Blocker Details

**Task 1 failed:** `go get` of all parser dependencies fails with `Failed to connect to github.com port 443: Connection refused`.

This is a repeat of the documented `env-intranet-service-flakiness` issue — GitHub is unreachable from WSL.

### Attempted Approaches

| Approach | Result |
|----------|--------|
| Direct `go get github.com/cloudwego/eino-ext/components/document/parser/pdf@v0.1.13` | Failed — GitHub unreachable |
| Via CN proxy `GOPROXY=https://goproxy.cn,direct` | Failed — GitHub still unreachable |
| `GOPROXY=direct` for excelize | Failed — GitHub unreachable |
| Cache check | Only `github.com/yuin/goldmark v1.8.5` and `github.com/xuri/excelize` (partial) exist in cache |

### Required Packages (all blocked)

| Format | Package | Status |
|--------|---------|--------|
| md | goldmark (already in go.mod:17) | Available |
| pdf | eino-ext/pdf or unidoc/unipdf | BLOCKED |
| docx | eino-ext/docx or nguyenthenguyen/docx | BLOCKED |
| xlsx | xuri/excelize/v2 | BLOCKED |
| pptx | any pure-Go pptx library | BLOCKED |

## Checkpoint: Human Verification Required

**Type:** `checkpoint:human-verify` (gate="blocking-human")
**Plan:** 27-01-PLAN.md

### What Was Attempted

Task 1 of the plan requires `go get` of 5 parser libraries. All attempts to reach GitHub have failed:
- `go get github.com/cloudwego/eino-ext/components/document/parser/pdf@v0.1.13`
- `go get github.com/cloudwego/eino-ext/components/document/parser/docx@v0.1.13`
- `go get github.com/cloudwego/eino-ext/components/document/parser/md@v0.1.13`
- `go get github.com/xuri/excelize/v2`
- Any pptx library

### How to Verify

1. From WSL, test connectivity: `curl -s --connect-timeout 5 https://github.com 2>&1`
2. If blocked, either:
   a. Configure a local Go proxy that can reach GitHub: `export GOPROXY=https://your-proxy.internal:8081,direct`
   b. Vendor the required modules locally
   c. Pre-populate the Go module cache with the needed packages

### Resume Signal

Once GitHub connectivity is restored (or a proxy is configured), re-run `/gsd:execute-phase 27 01` — Task 1 will complete `go get` and the rest of the plan can execute.
