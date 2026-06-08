package goft8

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// Decoder runs the FT8 decode pipeline on 15-second audio cycles.
// Construct with NewDecoder and reuse across multiple Decode calls so
// that cross-cycle state (used by future matched-filter passes)
// persists.
//
// Decoder is NOT safe for concurrent use by multiple goroutines.
type Decoder struct {
	cfg decoderConfig
}

// NewDecoder creates a Decoder configured by the given options. With
// no options it uses defaults equivalent to WSJT-X's "normal" mode
// with AP disabled and a full 200..3000 Hz audio search.
func NewDecoder(opts ...DecoderOption) *Decoder {
	cfg := defaultDecoderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if !cfg.apEnabledSet {
		cfg.apEnabled = cfg.myCall != ""
	}
	return &Decoder{cfg: cfg}
}

// Decode runs the full decode pipeline on one 15-second audio window.
//
// audio must be exactly AudioSamplesPerCycle samples of 12 kHz mono
// float32 PCM in the approximate range [-1, +1]. The decoder
// re-normalizes internally and tolerates modest normalization
// differences.
//
// Returns all distinct decoded signals ordered by pass then by sync
// power. A nil slice means no decodes; error is non-nil only for
// unrecoverable input errors (wrong length, etc.).
func (d *Decoder) Decode(audio []float32) ([]Decoded, error) {
	if len(audio) != AudioSamplesPerCycle {
		return nil, fmt.Errorf("goft8: audio length %d, want %d", len(audio), AudioSamplesPerCycle)
	}

	params := DecodeParams{
		Depth:     d.cfg.depth,
		APEnabled: d.cfg.apEnabled,
		APCQOnly:  d.cfg.cqOnlyAP,
		APWidth:   25.0,
		MyCall:    d.cfg.myCall,
		DxCall:    d.cfg.dxCall,
		MaxPasses: d.cfg.maxPasses,
		Workers:   d.cfg.workers,
	}

	raw := DecodeIterative(audio, params, float64(d.cfg.freqMin), float64(d.cfg.freqMax))

	out := make([]Decoded, len(raw))
	for i, r := range raw {
		out[i] = toDecoded(r)
	}
	return out, nil
}

// DecodeStream runs the full decode pipeline on one 15-second audio window,
// calling onDecode for each successfully decoded signal as soon as it is
// recovered, before the next pass or candidate begins.
//
// The callback is invoked synchronously from the decoder goroutine(s);
// keep it fast. The final return value still contains all decoded signals
// in the same order as the callback firing order.
func (d *Decoder) DecodeStream(audio []float32, onDecode func(Decoded)) ([]Decoded, error) {
	if len(audio) != AudioSamplesPerCycle {
		return nil, fmt.Errorf("goft8: audio length %d, want %d", len(audio), AudioSamplesPerCycle)
	}

	params := DecodeParams{
		Depth:       d.cfg.depth,
		APEnabled:   d.cfg.apEnabled,
		APCQOnly:    d.cfg.cqOnlyAP,
		APWidth:     25.0,
		MyCall:      d.cfg.myCall,
		DxCall:      d.cfg.dxCall,
		MaxPasses:   d.cfg.maxPasses,
		Workers:     d.cfg.workers,
		OnCandidate: func(c DecodeCandidate) { onDecode(toDecoded(c)) },
	}

	raw := DecodeIterative(audio, params, float64(d.cfg.freqMin), float64(d.cfg.freqMax))

	out := make([]Decoded, len(raw))
	for i, r := range raw {
		out[i] = toDecoded(r)
	}
	return out, nil
}

// Reset clears cross-cycle state maintained by the Decoder, equivalent
// to starting a fresh receive session. Call this when changing band,
// after a long idle, or to discard stale callsign history. Does not
// reset Decoder configuration.
func (d *Decoder) Reset() {
	// v0.1 keeps no cross-cycle state — this is a placeholder so
	// station-manager can wire the call site now and benefit
	// automatically once JTDX features land in later minor versions.
}

// DecodeWAV is a one-shot convenience: loads a 12 kHz mono 15-second
// WAV file, runs the decoder once, and returns the decoded signals.
// For a receive loop, create a Decoder once and reuse it.
func DecodeWAV(path string, opts ...DecoderOption) ([]Decoded, error) {
	audio, err := ReadWAVMono12k(path)
	if err != nil {
		return nil, err
	}
	if len(audio) < AudioSamplesPerCycle {
		padded := make([]float32, AudioSamplesPerCycle)
		copy(padded, audio)
		audio = padded
	} else if len(audio) > AudioSamplesPerCycle {
		audio = audio[:AudioSamplesPerCycle]
	}
	return NewDecoder(opts...).Decode(audio)
}

func toDecoded(c DecodeCandidate) Decoded {
	var tones [79]int
	copy(tones[:], c.Tones[:])
	return Decoded{
		Message:     c.Message,
		Freq:        c.Freq,
		DT:          c.DT,
		SNR:         clampSNR(c.SNR),
		Pass:        c.Pass,
		APType:      c.APType,
		NHardErrors: c.NHardErrors,
		Tones:       tones,
	}
}

func clampSNR(snr float64) int {
	n := int(math.Round(snr))
	if n < -25 {
		return -25
	}
	if n > 49 {
		return 49
	}
	return n
}

// WriteWAVMono12k writes mono float32 PCM samples as a 16-bit PCM WAV
// file at 12 kHz sample rate.
func WriteWAVMono12k(path string, samples []float32) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dataSize := uint32(len(samples) * 2)
	fileSize := 36 + dataSize

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], fileSize)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)    // subchunk1Size
	binary.LittleEndian.PutUint16(header[20:22], 1)     // audioFormat = PCM
	binary.LittleEndian.PutUint16(header[22:24], 1)     // numChannels = mono
	binary.LittleEndian.PutUint32(header[24:28], 12000) // sampleRate
	binary.LittleEndian.PutUint32(header[28:32], 24000) // byteRate = 12000*1*2
	binary.LittleEndian.PutUint16(header[32:34], 2)     // blockAlign
	binary.LittleEndian.PutUint16(header[34:36], 16)    // bitsPerSample
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	if _, err := f.Write(header); err != nil {
		return err
	}

	buf := make([]byte, len(samples)*2)
	for i, v := range samples {
		s := int16(math.Round(float64(v) * 32767.0))
		if s > 32767 {
			s = 32767
		}
		if s < -32768 {
			s = -32768
		}
		binary.LittleEndian.PutUint16(buf[i*2:i*2+2], uint16(s))
	}

	if _, err := f.Write(buf); err != nil {
		return err
	}
	return f.Close()
}
