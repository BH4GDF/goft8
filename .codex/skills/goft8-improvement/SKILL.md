---
name: goft8-improvement
description: Project-specific workflow for improving github.com/bh4gdf/goft8. Use when Codex is asked to plan, implement, review, test, secure, or benchmark changes in this Go FT8 encoder/decoder repository, especially tasks from IMPROVEMENT_PLAN.md involving WAV I/O, protocol packing, LDPC/decoder performance, CLI behavior, CI, fuzzing, or role-based agent coordination.
---

# GoFT8 Improvement

## Workflow

1. Read `AGENTS.md`, `IMPROVEMENT_PLAN.md`, and the relevant code before editing.
2. Classify the task against P0-P5 from `IMPROVEMENT_PLAN.md`.
3. Keep public API changes conservative; prefer package-local fixes under `internal/` when possible.
4. Add or update tests beside the behavior being changed.
5. Run the narrow test first, then `go test ./...`; use `go test -race ./...` for concurrency, streaming decode, shared cache, or pool changes.
6. For performance work, capture `go test -bench ... -benchmem` before and after and compare allocations, not only wall time.

## Project Rules

- Root package files expose the public API; algorithm details belong under `internal/decode`, `internal/encode`, `internal/dsp`, `internal/ldpc`, or `internal/protocol`.
- WAV parsing and writing must reject malformed inputs with errors, not panics.
- Decoder behavior must remain deterministic for `testdata/ft8_cap*.wav`.
- Avoid broad refactors in DSP/LDPC code unless benchmarks and fixture tests justify them.
- Preserve `gofmt` formatting and idiomatic Go test names: `TestXxx`, `BenchmarkXxx`, `FuzzXxx`.

## Role Handoffs

Use `.agents/*.md` for role-specific review checklists:

- `project-manager.md` for backlog, milestone, and PR gate decisions.
- `developer-core.md` for WAV/API/decoder/encoder implementation.
- `developer-tooling.md` for CI, test harnesses, docs, and CLI polish.
- `security-consultant.md` for malformed input and fuzz coverage.
- `performance-tester.md` for benchmark design and regression checks.
- `functional-tester.md` for user-facing decode/encode workflows.

## References

Read `references/project-map.md` when you need package ownership, command baselines, or known risk areas.
