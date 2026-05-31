# goft8

A Go implementation of the FT8 digital mode encoder and decoder.

> **This is the `fftw` branch** — a high-performance variant that uses
> FFTW3 (via CGO) for FFT and MSHV's C++ LDPC/OSD decoder.
> For the pure-Go version, see the [`main`](https://github.com/BH4GDF/goft8/tree/main) branch.

## Features

- **Decoder**: Full FT8 decode pipeline (sync8, LDPC BP+OSD, CRC, message unpack)
  ported from WSJT-X 2.7.0 algorithms, with behavioural alignment to
  [MSHV](https://www.mshv.org/) v2.76 for SNR calibration, frequency limits,
  and decode parameters.
- **Encoder**: Full message packing matching MSHV priority order:
  - Standard messages (CQ, grid, reports, RRR/RR73/73, /R /P suffixes)
  - DXpedition, ARRL Field Day, ARRL RTTY Contest, EU VHF Contest
  - Non-standard callsigns, Telemetry, Free text
  - CRC-14, LDPC (174,91), GFSK tone generation, waveform synthesis
  - Configurable output sample rate (12 kHz / 48 kHz) and bit depth (16/24/32-bit)
- **CGO + FFTW3 + MSHV C++ LDPC**: This branch requires CGO and the FFTW3
  development libraries. Decode is ~2.2× faster than the pure-Go `main` branch.

## Installation

**Prerequisites**

```bash
# Debian/Ubuntu
sudo apt-get install libfftw3-dev

# macOS
brew install fftw

# Fedora
sudo dnf install fftw-devel
```

```bash
go get github.com/bh4gdf/goft8
```

## Quick Start

### Decode a WAV file

```go
package main

import (
    "fmt"
    "log"
    "github.com/bh4gdf/goft8"
)

func main() {
    decodes, err := goft8.DecodeWAV("ft8_cap1.wav",
        goft8.WithDepth(goft8.DepthDeep),
        goft8.WithMyCall("W1ABC"),
    )
    if err != nil {
        log.Fatal(err)
    }
    for _, d := range decodes {
        fmt.Printf("%3d dB  %s\n", d.SNR, d.Message)
    }
}
```

### Encode a message

```go
package main

import (
    "log"
    "github.com/bh4gdf/goft8"
)

func main() {
    enc := goft8.NewEncoder(goft8.WithTxFreq(1500))
    waveform, err := enc.Encode("CQ W1ABC FN20")
    if err != nil {
        log.Fatal(err)
    }
    // waveform is []float32 at the configured sample rate (default 48 kHz)
}
```

## Architecture

The public API is intentionally thin—only `Decoder`, `Encoder`, `Decoded`,
`Message`, and a few facade helpers live in the root package. All algorithmic
detail is hidden under `internal/`:

```
goft8/
├── params/              # FT8 algorithm constants (Fs, NN, NSPS, LDPC params, Gray map)
├── internal/
│   ├── dsp/             # FFT / IFFT / RealFFT utilities (FFTW3 via CGO)
│   ├── ldpc/            # LDPC (174,91) encoder/decoder + CRC-14 (MSHV C++ via CGO)
│   ├── decode/          # Decode pipeline (downsample, sync8, sync_d, metrics, AP, subtract)
│   ├── encode/          # Tone generation and GFSK waveform synthesis
│   └── protocol/        # Message packing/unpacking (pack77, pack28, hash tables)
├── cmd/
│   ├── decodewav/       # Command-line WAV decoder
│   └── genwav/          # Command-line WAV generator
└── *.go (root)          # Public API surface
```

## References & Acknowledgements

- **WSJT-X** — The original FT8 specification and Fortran reference
  implementation (WSJT-X 2.7.0).
- **MSHV** — [MSHV](https://www.mshv.org/) by Christo LZ2HV. This project uses
  MSHV v2.76 as the behavioural reference for SNR calibration, TX frequency
  clamping, multi-message FDMA spacing, frequency search limits, and message
  packing priority order. Several parameters (e.g. `frq00_limit`, `dflimit`,
  input scaling constants) are aligned directly with MSHV source values.
- **goft8 (ColonelBlimp)** — An earlier Go port of WSJT-X 2.7.0 by
  [ColonelBlimp](https://github.com/ColonelBlimp/goft8) (MIT), which provided
  the starting point for the LDPC and sync8 implementation.

## License

GPL v3 — see [LICENSE](LICENSE).

Parts of the decoder implementation are derived from
[goft8](https://github.com/ColonelBlimp/goft8) by ColonelBlimp (MIT),
which is a clean-room port of WSJT-X 2.7.0 Fortran sources.
