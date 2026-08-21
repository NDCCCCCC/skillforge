# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# Makefile — SkillForge operator entry points.
#
# The default target is verify-phase-11 (the Phase 11 acceptance
# gate). Run `make` or `make verify-phase-11` to execute the gate;
# the banner at the end is the operator's pass/fail signal.

.PHONY: verify-phase-11 verify-phase-12 verify-phase-13 test-unit test-frontend build clean

# verify-phase-11 is the Phase 11 acceptance gate (Plan 11-08 Task 6).
# Invokes scripts/verify-phase-11.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
verify-phase-11:
	bash scripts/verify-phase-11.sh

# verify-phase-12 is the Phase 12 acceptance gate (Plan 12-08).
# Invokes scripts/verify-phase-12.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
verify-phase-12:
	bash scripts/verify-phase-12.sh

# verify-phase-13 is the Phase 13 HERMETIC gate (Plan 13-04).
# Invokes scripts/verify-phase-13.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] on any stage failure.
# Live operator gate (Plan 13-05) is documented in the script header
# and is NOT invoked by this target.
verify-phase-13:
	bash scripts/verify-phase-13.sh

# test-unit runs the full Go test suite for fast iteration.
test-unit:
	go test ./... -count=1

# test-frontend runs the vitest UI suite.
test-frontend:
	cd web && npm test

# build compiles all Go binaries.
build:
	go build ./...

# clean removes generated reports (the acceptance-gate ship evidence).
clean:
	rm -rf reports/
