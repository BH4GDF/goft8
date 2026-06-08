# Repository Guidelines

## Project Structure & Module Organization

This is the `fftw` branch of Go module `github.com/bh4gdf/goft8`, an FT8 encoder and decoder that requires CGO, FFTW3 development libraries, and the MSHV C++ LDPC/OSD decoder path. Root-level `.go` files expose the public API (`Decoder`, `Encoder`, WAV helpers, message parsing, options). Algorithm implementation belongs under `internal/`: `decode/` for the receive pipeline, `encode/` for tone and waveform generation, `dsp/` for FFT utilities, `ldpc/` for LDPC and CRC, and `protocol/` for FT8 packing and unpacking. Shared FT8 constants live in `params/`. CLI tools are in `cmd/decodewav` and `cmd/genwav`. Tests are co-located as `*_test.go`; WAV fixtures are in `testdata/`.

Preserve the branch-specific native backends: `internal/dsp/fft_fftw.go`, `internal/ldpc/decode_cgo.go`, `internal/ldpc/mshv_decode.cpp`, and `internal/ldpc/mshv_decode.h`. Do not replace them with main's pure-Go FFT or LDPC decoder during a main-to-fftw sync.

## Build, Test, and Development Commands

- `go test ./...` runs the full test suite.
- `go test -run TestDecodeCaptures .` runs the root package decode fixture test.
- `go test -bench . ./...` runs encode/decode benchmarks.
- `go run ./cmd/decodewav testdata/ft8_cap1.wav` decodes a sample WAV file.
- `go run ./cmd/genwav -rate 48000 -bits 24` generates a multi-message FT8 WAV file.

Use `go test -race ./...` when changing shared state, streaming decode, or concurrent encode paths.

## Coding Style & Naming Conventions

Format Go code with `gofmt` before committing. Use standard Go tabs for indentation and idiomatic names: exported identifiers in `CamelCase`, unexported identifiers in `lowerCamelCase`, and tests named `TestXxx` or `BenchmarkXxx`. Keep the root package focused on the public API; put protocol, DSP, LDPC, and pipeline details in the appropriate `internal/` package. Prefer small, deterministic helpers over hidden global state.

## Testing Guidelines

Use Go's standard `testing` package. Add API-facing tests at the repository root and package-specific tests beside the implementation under `internal/`. Keep fixture-based tests deterministic and store required audio inputs in `testdata/`. When updating signal-processing behavior, include expected-message or round-trip assertions and run relevant benchmarks to catch large performance regressions. There is no fixed coverage threshold; prioritize coverage for packing, decoding, WAV I/O, and option behavior.

## Commit & Pull Request Guidelines

Recent history uses short, conventional-style subjects such as `fix: ...`, `perf: ...`, and `goft8: ...`. Follow that pattern with an imperative, specific summary. Pull requests should describe the behavior change, note API or fixture changes, link related issues, and list commands run, especially `go test ./...` and any benchmarks for performance-sensitive work. Avoid committing ad hoc generated WAVs, test binaries, or timing output unless they are intentional fixtures.

## Agent, Skill, and MCP Configuration

Role prompts for the six-person improvement team live in `.agents/`. The project-specific Codex skill is `.codex/skills/goft8-improvement/`; use it for tasks from `IMPROVEMENT_PLAN.md`. Use `.codex/skills/goft8-fftw-sync/` when comparing `main` with `fftw` or applying main-branch updates to this native branch. MCP templates are in `.mcp.json` and `.codex/mcp.toml`, covering local filesystem and Git access for this repository.
