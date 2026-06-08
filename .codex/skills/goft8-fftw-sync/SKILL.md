---
name: goft8-fftw-sync
description: Project-specific workflow for evaluating, planning, and applying updates from the pure-Go main branch onto the goft8 fftw branch while preserving FFTW3 CGO FFT behavior, MSHV C++ LDPC decode behavior, branch-specific README notes, and validation gates. Use when Codex compares main vs fftw, resolves main-to-fftw merge conflicts, updates FFTW branch skill/MCP/CI/test configuration, or plans cherry-picks across DSP, LDPC, decoder, WAV, CLI, fuzz, and benchmark changes.
---

# GoFT8 FFTW Sync

## Workflow

1. Read `AGENTS.md`, `IMPROVEMENT_PLAN.md`, `FFTW_SYNC_PLAN.md`, and `references/branch-sync-map.md` before changing code.
2. Execute only one `FFTW_SYNC_PLAN.md` task ID at a time (`T0` through `T8`), unless the user explicitly asks for a broader batch.
3. Run the task's preflight commands and confirm the task dependencies before editing.
4. Compare with `git log --left-right --cherry-pick fftw...main` and `git diff --name-status $(git merge-base fftw main)..main`.
5. Classify incoming changes as direct apply, adapt, defer, or reject.
6. Preserve branch-specific CGO behavior unless the task explicitly asks to introduce a pure-Go fallback:
   - `internal/dsp/fft.go` and `internal/dsp/fft_fftw.go` assume FFTW3.
   - `internal/ldpc/decode_cgo.go`, `mshv_decode.cpp`, and `mshv_decode.h` provide MSHV LDPC/OSD decode.
7. Modify only the files allowed by the current task. Stop if another file must change and update the plan before continuing.
8. Resolve conflicts by behavior, not by blanket choosing one side. Keep `fftw` backend code and port `main` fixes/tests around it.
9. Run the task's narrow validation first, then `go test ./...`. Run `go test -race ./...` when shared pools, CGO plan reuse, streaming decode, or concurrent encode paths are touched.
10. For performance-sensitive changes, record before/after `go test -bench ... -benchmem` numbers, including whether FFTW3 development libraries were installed.

## Update Classes

- Direct apply: agent roster, Codex skill/MCP templates, GitHub fuzz workflow, most protocol and CLI tests once code conflicts are resolved.
- Adapt: WAV reader hardening, `cmd/decodewav` JSON/flag support, encoder buffer pooling, decode workspace pooling, LDPC tests and benchmarks.
- Defer or split: replacing FFTW DSP files with pure-Go `internal/dsp/fft_gonum.go`, removing CGO LDPC paths, or CI jobs that assume no native FFTW3 package is needed.

## Validation Gates

- Preflight: `git status --short --branch`, `pkg-config --exists fftw3`, and `go test ./...`
- Config-only sync: `python3 /home/yida/.codex/skills/.system/skill-creator/scripts/quick_validate.py .codex/skills/goft8-fftw-sync`
- CLI/WAV sync: `go test ./cmd/decodewav .`
- Decoder/DSP/LDPC sync: `go test ./internal/decode ./internal/dsp ./internal/ldpc .`
- Full sync: `go test ./...`
- Shared state or CGO pool changes: `go test -race ./...`

## Stop Conditions

- Baseline `go test ./...` fails before the task starts.
- `pkg-config --exists fftw3` fails and FFTW3 cannot be installed in the environment.
- The task would remove or bypass FFTW3/MSHV CGO behavior.
- Fixture decode messages decrease, or decode benchmark allocation/performance regresses beyond the threshold in `FFTW_SYNC_PLAN.md`.
- A required change falls outside the current task's allowed file list.

## References

Read `references/branch-sync-map.md` when deciding which `main` commits can be applied directly to `fftw` and which conflict zones need an adaptation plan.
