// foxhound.go — Fox/Hound mode support (placeholder).
//
// MSHV implements Fox/Hound as a completely separate decoder (DecoderSFox in
// decodersfox.cpp, ~2800 lines). It uses:
//   - Multi-tone sync patterns (different from standard FT8 Costas)
//   - QPC (Quasi-Parallel Convolutional) coding instead of LDPC
//   - Frequency-grid detection for multiple simultaneous signals
//   - Specialized subtract and demod pipelines
//
// Porting Fox/Hound is a major undertaking (~3000+ lines) and is tracked
// as future work. This file holds the public API surface so callers can
// detect Fox/Hound mode even before the decoder is fully ported.

package goft8

// FoxHoundMode indicates the operating mode for Fox/Hound.
type FoxHoundMode int

const (
	// ModeStandard disables Fox/Hound logic (normal FT8).
	ModeStandard FoxHoundMode = iota
	// ModeFox enables Fox-mode transmission scheduling.
	ModeFox
	// ModeHound enables Hound-mode reception and decoding.
	ModeHound
)

// FoxHoundParams holds tunable parameters for Fox/Hound operation.
type FoxHoundParams struct {
	Mode   FoxHoundMode
	TxFreq float64 // nominal transmit frequency (Hz)
	RxFreq float64 // nominal receive frequency (Hz)
	NSlots int     // number of slots (Fox only)
}

// DecodeFoxHound is a reserved API for future Fox/Hound decoding support.
//
// TODO: full implementation pending port of DecoderSFox from MSHV.
// It is not implemented yet and currently returns nil unconditionally; callers
// must not treat nil as proof that a Fox/Hound signal was absent.
func DecodeFoxHound(audio []float32, params FoxHoundParams) []DecodeCandidate {
	_ = audio
	_ = params
	return nil
}
