package goft8

import (
	"fmt"
	"sync"
)

// NTXSamples is the number of 12 kHz mono samples in a single FT8
// transmission (12.64 s × 12 kHz).
const NTXSamples = 151680

// Encoder generates the 12 kHz audio waveform for an FT8 message, for
// TX-side use. v0.1 ships the API shape only so that callers can
// import and reference ft8.Encoder immediately; the transmit path
// will be implemented in v0.2.
type Encoder struct {
	cfg encoderConfig
}

type encoderConfig struct {
	txFreq float64
}

// MessageFreq pairs a text message with its TX frequency for FDMA.
type MessageFreq struct {
	Message string
	Freq    float64
}

func defaultEncoderConfig() encoderConfig {
	return encoderConfig{txFreq: 1500}
}

// EncoderOption configures an Encoder at construction time.
type EncoderOption func(*encoderConfig)

// WithTxFreq sets the carrier frequency in Hz. Defaults to 1500.
func WithTxFreq(hz float64) EncoderOption {
	return func(c *encoderConfig) { c.txFreq = hz }
}

// NewEncoder creates an Encoder with the given options.
func NewEncoder(opts ...EncoderOption) *Encoder {
	cfg := defaultEncoderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Encoder{cfg: cfg}
}

// encodeBufPool reuses the NTXSamples-length buffers needed by
// EncodeMulti.  Each goroutine borrows its own.
var encodeBufPool = sync.Pool{
	New: func() interface{} {
		return make([]float32, NTXSamples)
	},
}

// Encode generates the GFSK-modulated waveform for a single FT8
// message. Returns NTXSamples samples of 12 kHz mono PCM.
func (e *Encoder) Encode(msg string) ([]float32, error) {
	// Pack the message into 77 bits.
	bits, _, _, ok := Pack77(msg)
	if !ok {
		return nil, fmt.Errorf("ft8: cannot pack message %q", msg)
	}

	// Generate 79 channel tones from the 77-bit message.
	itone := GenFT8Tones(bits)

	// Generate the real-valued GFSK waveform (avoids complex128 allocation).
	return GenFT8Wave(itone, e.cfg.txFreq), nil
}

// EncodeMulti generates a single waveform that contains multiple FT8
// messages on different carrier frequencies (FDMA). The individual
// real GFSK waveforms are linearly summed. The caller may scale
// the result before playback or writing to a file to avoid clipping.
func (e *Encoder) EncodeMulti(msgs []MessageFreq) ([]float32, error) {
	if len(msgs) == 0 {
		return nil, fmt.Errorf("ft8: no messages to encode")
	}

	// Borrow a zeroed accumulation buffer from the pool.
	sum := encodeBufPool.Get().([]float32)
	for i := range sum {
		sum[i] = 0
	}

	for _, mf := range msgs {
		bits, _, _, ok := Pack77(mf.Message)
		if !ok {
			encodeBufPool.Put(sum)
			return nil, fmt.Errorf("ft8: cannot pack message %q", mf.Message)
		}
		itone := GenFT8Tones(bits)
		wave := GenFT8Wave(itone, mf.Freq)
		for i, v := range wave {
			sum[i] += v
		}
	}

	// Copy out and scale to avoid clipping.
	scale := float32(len(msgs))
	waveform := make([]float32, NTXSamples)
	for i, v := range sum {
		waveform[i] = v / scale
	}

	encodeBufPool.Put(sum)
	return waveform, nil
}
