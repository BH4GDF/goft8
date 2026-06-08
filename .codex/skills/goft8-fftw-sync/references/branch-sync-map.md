# GoFT8 FFTW Branch Sync Map

## Branch Shape

- Common ancestor: `47209a36c06c9784a8e0f2a4fbc5fe9791478a17`.
- `main` head during this audit: `b969a6b94db87e793d6118b9da1b8c03b0e26fff`.
- `fftw` head during this audit: `66f9d6d0944aa355bfe71161b8a6e4c28a7998dc`.
- `fftw` adds FFTW3 CGO FFT and MSHV C++ LDPC decode paths on top of the shared decoder/encoder restructuring.

## Main Updates By Class

Direct apply candidates:

- `7dca434` configuration files: `.agents/`, `.codex/`, `.mcp.json`, `AGENTS.md`, `IMPROVEMENT_PLAN.md`, and base CI templates.
- `6527b87` fuzz smoke workflow with FFTW3 development libraries installed in CI.
- Protocol branch and fuzz tests from `8f879c3` and `internal/protocol/fuzz_test.go`, because `fftw` keeps the same protocol package surface.
- Fox/Hound placeholder test from `b5a511c`, because it documents existing behavior.

Adapt candidates:

- WAV hardening from `7dca434`, `15d8154`, `b5a511c`, and `53dcadd`: port the robust shared `ReadWAVMono`, `ReadWAVMono12k`, `ReadWAVParams`, and tests while preserving any `fftw` decoder scaling requirements.
- `cmd/decodewav` JSON/flag support from `b969a6b`: replace the branch-local duplicated WAV parser with shared WAV helpers, then port flags and JSON tests.
- CLI smoke tests from `153c05e`: adapt expected output to `fftw` branch defaults and native dependency requirements.
- Encoder buffer pooling and tests from `da58000` and `33abf3d`: port after checking `EncodeMulti` output equivalence and allocations.
- Decode allocation reductions from `3941d99`, `4bca5fd`, and `e6c93c8`: adapt around FFTW-backed spectrogram and MSHV LDPC result behavior.
- LDPC coverage and benchmarks from `33fd7af` and `53dcadd`: keep CGO decode behavior as the reference for this branch.
- Decode/DSP boundary tests from `2504aa3` and `9492ab8`: adapt expected internals if FFTW numerical output differs by tolerance.

Defer or split candidates:

- Main's pure-Go DSP fallback file `internal/dsp/fft_gonum.go` should not replace `fftw`'s FFTW path in a straight sync.
- CI that runs on GitHub-hosted Linux must keep installing `libfftw3-dev` before `go test ./...`, or the branch must add build tags for optional FFTW.
- Any change that removes `internal/ldpc/decode_cgo.go`, `mshv_decode.cpp`, or `mshv_decode.h` belongs in a separate design change.

## Known Conflict Files From Merge Precheck

- Documentation/tests/CLI: `README.md`, `bench_encode_test.go`, `cmd/decodewav/main.go`, `cmd/genwav/*.go`, `encoder_test.go`.
- Public API and facades: `decode.go`, `decoder.go`, `encoder.go`, `wav.go`.
- Decoder internals: `internal/decode/ap_test.go`, `downsample.go`, `metrics.go`, `subtract.go`, `sync8.go`.
- DSP/LDPC internals: `internal/dsp/fft.go`, `internal/encode/encode.go`, `internal/ldpc/ldpc.go`, `internal/ldpc/ldpc_f32.go`.

## Recommended Sync Order

Use the task IDs in `FFTW_SYNC_PLAN.md`:

1. `T0`: land config-only files and the executable sync plan.
2. `T1`: port low-risk protocol, Fox/Hound, fuzz, and API tests.
3. `T2`: port WAV reader hardening and switch CLI readers to shared helpers.
4. `T3`: port `cmd/decodewav` flags and JSON output.
5. `T4`: port encoder pooling and run encode benchmarks.
6. `T5`: port decode workspace and soft-metric allocation reductions around FFTW.
7. `T6`: port LDPC coverage/benchmark changes around MSHV CGO decode.
8. `T7`: keep CI installing FFTW3 before full branch gates; split build tags into a separate design task if optional FFTW is needed.
9. `T8`: run final merge audit and document deferred items.

## Quality Preconditions

- Baseline `go test ./...` must pass before each task starts.
- `pkg-config --exists fftw3` must pass; this branch should not silently fall back to pure Go.
- Fixture decode messages for `testdata/ft8_cap*.wav` must not decrease.
- Decode performance-sensitive tasks must compare `BenchmarkDecodeWAVCap1` before and after.
- Encoder pooling tasks must compare encoder and `EncodeMulti` benchmarks before and after.
- CGO plan pools, shared buffers, or goroutine changes require `go test -race ./...`.
