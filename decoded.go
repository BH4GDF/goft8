package goft8

// Decoded is a single FT8 signal successfully decoded from one
// 15-second audio window.
type Decoded struct {
	// Message is the 37-character-max decoded text, e.g. "CQ W1ABC FN31".
	Message string

	// Freq is the refined carrier frequency in Hz (sub-Hz precision).
	Freq float64

	// DT is the time offset relative to the nominal 0.5 s TX start in
	// seconds. Matches WSJT-X display convention; a signal arriving on
	// time has DT ≈ 0.
	DT float64

	// SNR is the estimated signal-to-noise ratio in dB (2500 Hz
	// bandwidth), clamped to [-24, +40].
	SNR int

	// Pass is the iterative-decode pass this signal was recovered on
	// (1, 2, or 3).
	Pass int

	// APType is the a-priori decoding type used, 0 if none.
	APType int

	// NHardErrors is the number of hard-decision bit errors corrected
	// by the LDPC decoder. 0 means clean; higher values indicate a
	// weaker decode that was still CRC-validated.
	NHardErrors int

	// Tones holds the 79 channel tone indices (0..7) representing this
	// signal, useful for subtraction or re-encoding.
	Tones [79]int
}

// Public constants describing the sample format expected by Decoder.Decode.
const (
	// AudioSamplesPerCycle is the number of float32 samples per FT8
	// cycle (15 s × 12 kHz).
	AudioSamplesPerCycle = 180000

	// AudioSampleRate is the required audio sample rate in Hz.
	AudioSampleRate = 12000
)

// LDPC decoder depth presets accepted by WithDepth.
const (
	DepthFast   = 1 // BP only
	DepthNormal = 2 // BP + OSD(0) — default
	DepthDeep   = 3 // BP + OSD(2)
)

// AP (a-priori) decoding types. The value matches the WSJT-X iaptype
// field reported on each decoded signal.
const (
	APTypeNone     = 0
	APTypeCQ       = 1 // CQ ??? ???
	APTypeMyCall   = 2 // MyCall ??? ???
	APTypeMyDx     = 3 // MyCall DxCall ???
	APTypeMyDxRRR  = 4 // MyCall DxCall RRR
	APTypeMyDx73   = 5 // MyCall DxCall 73
	APTypeMyDxRR73 = 6 // MyCall DxCall RR73
)
