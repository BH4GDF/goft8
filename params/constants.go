// Package params defines all FT8 algorithm constants.
//
// These match ft8_params.f90 and WSJT-X 2.7.0 exactly.
package params

// ── Constants previously imported as ft8x.* ──────────────────────────────

const (
	// NP2 is the length of the downsampled complex buffer.
	NP2 = 2812

	// NFFT2 is the FFT size for the downsampled signal.
	NFFT2 = 3200

	// NFFT1DS is the FFT size used during downsampling (192000, 5-smooth).
	NFFT1DS = 192000

	// NZ is the number of audio samples in the full 15 s waveform.
	NZ = NSPS * NN // 151680

	// NFRAME is the number of audio samples in one FT8 frame.
	NFRAME = NN * NSPS // 151680

	// Fs2 is the downsampled sample rate (200 Hz).
	Fs2 = Fs / NDOWN // 200.0

	// Dt2 is the sample period at the downsampled rate.
	Dt2 = 1.0 / Fs2

	// Baud is the symbol rate.
	Baud = Fs / NSPS // 6.25 Hz

	// ScaleFac is the LLR scaling factor applied before LDPC.
	ScaleFac = 2.83

	// MaxIterations is the maximum number of BP decoder iterations.
	MaxIterations = 30
	// LDPCn is the codeword length of the (174,91) LDPC code.
	LDPCn = 174
	// LDPCk is the message length (77 payload bits + 14 CRC bits).
	LDPCk = 91
	// LDPCm is the number of parity-check equations.
	LDPCm = 83
	// LDPCncw is the number of checks per bit (column weight).
	LDPCncw = 3
)

// GrayMap maps 3-bit binary index to 8-FSK tone number.
var GrayMap = [8]int{0, 1, 3, 2, 5, 6, 4, 7}

// GrayUnmap is the inverse of GrayMap: tone number → 3-bit binary value.
var GrayUnmap = [8]int{0, 1, 3, 2, 6, 7, 5, 4}
