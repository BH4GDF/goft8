package goft8

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/bh4gdf/goft8/internal/encode"
	"github.com/bh4gdf/goft8/internal/protocol"
)

// NTXSamples is the number of 12 kHz mono samples in a single FT8
// transmission (12.64 s × 12 kHz).
const NTXSamples = 151680

// FT8 TX frequency limits, matching MSHV v2.76:
//
//	frq00_limit=20, frq01_limit=4980, dflimit=60
//	clamped to [frq00_limit+dflimit, frq01_limit-dflimit].
const (
	ft8TxFreqMin = 80.0   // 20 + 60
	ft8TxFreqMax = 4920.0 // 4980 - 60
)

// Encoder generates the audio waveform for an FT8 message, for
// TX-side use.
type Encoder struct {
	cfg encoderConfig
}

type encoderConfig struct {
	txFreq     float64
	sampleRate int
	bitDepth   int
}

func defaultEncoderConfig() encoderConfig {
	return encoderConfig{
		txFreq:     1500,
		sampleRate: 48000, // 48 kHz default (sound card compatible)
		bitDepth:   24,    // 24-bit PCM default (MSHV TX style)
	}
}

// MessageFreq pairs a text message with its TX frequency for FDMA.
type MessageFreq struct {
	Message string
	Freq    float64
}

// EncoderOption configures an Encoder at construction time.
type EncoderOption func(*encoderConfig)

// WithTxFreq sets the carrier frequency in Hz.
// The value is clamped to the FT8 band [80, 4920] Hz (MSHV limits).
// Defaults to 1500.
func WithTxFreq(hz float64) EncoderOption {
	return func(c *encoderConfig) { c.txFreq = clampTxFreq(hz) }
}

func clampTxFreq(hz float64) float64 {
	if hz < ft8TxFreqMin {
		return ft8TxFreqMin
	}
	if hz > ft8TxFreqMax {
		return ft8TxFreqMax
	}
	return hz
}

// WithSampleRate sets the output sample rate in Hz.
// Supported values are 12000 and 48000. Defaults to 48000.
func WithSampleRate(sr int) EncoderOption {
	return func(c *encoderConfig) { c.sampleRate = sr }
}

// WithBitDepth sets the output PCM bit depth.
// Supported values are 16, 24, and 32. Defaults to 24.
func WithBitDepth(bits int) EncoderOption {
	return func(c *encoderConfig) { c.bitDepth = bits }
}

// NewEncoder creates an Encoder with the given options.
func NewEncoder(opts ...EncoderOption) *Encoder {
	cfg := defaultEncoderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Encoder{cfg: cfg}
}

// txSamples returns the number of samples for one FT8 frame at the
// encoder's configured sample rate.
func (e *Encoder) txSamples() int {
	_, nwave, _ := encode.EncodeParams(e.cfg.sampleRate)
	return nwave
}

func (e *Encoder) validateSampleRate() error {
	if e.cfg.sampleRate != 12000 && e.cfg.sampleRate != 48000 {
		return fmt.Errorf("ft8: unsupported sample rate %d (want 12000 or 48000)", e.cfg.sampleRate)
	}
	return nil
}

func (e *Encoder) validatePCMConfig() error {
	if err := e.validateSampleRate(); err != nil {
		return err
	}
	if e.cfg.bitDepth != 16 && e.cfg.bitDepth != 24 && e.cfg.bitDepth != 32 {
		return fmt.Errorf("ft8: unsupported bit depth %d (want 16, 24, or 32)", e.cfg.bitDepth)
	}
	return nil
}

// encodeBufPools reuse the txSamples-length accumulation buffers needed by
// EncodeMulti. Keep one pool per supported sample rate so 48 kHz calls do not
// repeatedly allocate and then return truncated 12 kHz buffers.
var encodeBufPools = map[int]*sync.Pool{
	12000: {
		New: func() interface{} {
			return make([]float32, NTXSamples)
		},
	},
	48000: {
		New: func() interface{} {
			return make([]float32, NTXSamples*4)
		},
	},
}

func encodeBufGet(sampleRate int) []float32 {
	pool := encodeBufPools[sampleRate]
	return pool.Get().([]float32)
}

func encodeBufPut(sampleRate int, buf []float32) {
	pool := encodeBufPools[sampleRate]
	if pool == nil {
		return
	}
	want := NTXSamples
	if sampleRate == 48000 {
		want *= 4
	}
	if cap(buf) < want {
		return
	}
	pool.Put(buf[:want])
}

// Encode generates the GFSK-modulated waveform for a single FT8
// message. Returns float32 samples at the configured sample rate.
func (e *Encoder) Encode(msg string) ([]float32, error) {
	if err := e.validateSampleRate(); err != nil {
		return nil, err
	}

	bits, _, _, ok := protocol.Pack77(msg)
	if !ok {
		return nil, fmt.Errorf("ft8: cannot pack message %q", msg)
	}

	itone := encode.GenFT8Tones(bits)
	f0 := clampTxFreq(e.cfg.txFreq)
	return encode.GenFT8WaveSR(itone, f0, e.cfg.sampleRate), nil
}

// EncodeMulti generates a single waveform that contains multiple FT8
// messages on different carrier frequencies (FDMA). The individual
// real GFSK waveforms are linearly summed. The caller may scale
// the result before playback or writing to a file to avoid clipping.
//
// Each message frequency is clamped to [80, 4920] Hz.  Messages are
// processed in list order; earlier messages have higher priority.
// A later message must be at least 60 Hz away from *all* previously
// accepted messages.  If it is too close to any earlier one, an error
// is returned and that message is rejected.
func (e *Encoder) EncodeMulti(msgs []MessageFreq) ([]float32, error) {
	if len(msgs) == 0 {
		return nil, fmt.Errorf("ft8: no messages to encode")
	}
	if err := e.validateSampleRate(); err != nil {
		return nil, err
	}

	nsamples := e.txSamples()

	// Borrow a zeroed accumulation buffer from the pool.
	sum := encodeBufGet(e.cfg.sampleRate)
	for i := range sum {
		sum[i] = 0
	}
	sum = sum[:nsamples]

	// Keep track of accepted frequencies (in list order) for spacing checks.
	var accepted []float64

	type waveJob struct {
		f0   float64
		bits [77]int8
	}
	jobs := make([]waveJob, 0, len(msgs))

	for i, mf := range msgs {
		f0 := clampTxFreq(mf.Freq)
		for _, prev := range accepted {
			if math.Abs(f0-prev) < 60.0 {
				encodeBufPut(e.cfg.sampleRate, sum)
				return nil, fmt.Errorf("ft8: message %d frequency %.1f Hz is too close to earlier message at %.1f Hz (min spacing 60 Hz)", i, f0, prev)
			}
		}
		accepted = append(accepted, f0)

		bits, _, _, ok := protocol.Pack77(mf.Message)
		if !ok {
			encodeBufPut(e.cfg.sampleRate, sum)
			return nil, fmt.Errorf("ft8: cannot pack message %q", mf.Message)
		}
		jobs = append(jobs, waveJob{f0: f0, bits: bits})
	}

	// Generate waveforms in parallel — tone generation and GFSK synthesis
	// are CPU-bound and independent per message.
	waves := make([][]float32, len(jobs))
	var wg sync.WaitGroup
	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j waveJob) {
			defer wg.Done()
			itone := encode.GenFT8Tones(j.bits)
			wave := encodeBufGet(e.cfg.sampleRate)[:nsamples]
			encode.GenFT8WaveInto(wave, itone, j.f0, e.cfg.sampleRate)
			waves[idx] = wave
		}(i, job)
	}
	wg.Wait()
	defer func() {
		for _, wave := range waves {
			encodeBufPut(e.cfg.sampleRate, wave)
		}
	}()

	// Accumulate waveforms. For 3+ messages use parallel reduction over
	// segments of the sample buffer to saturate memory bandwidth.
	if len(waves) >= 3 {
		nseg := runtime.GOMAXPROCS(0)
		if nseg > len(waves) {
			nseg = len(waves)
		}
		chunk := (nsamples + nseg - 1) / nseg
		var wg sync.WaitGroup
		for s := 0; s < nseg; s++ {
			start := s * chunk
			end := start + chunk
			if start >= nsamples {
				continue
			}
			if end > nsamples {
				end = nsamples
			}
			wg.Add(1)
			go func(s, e int) {
				defer wg.Done()
				for _, wave := range waves {
					for i := s; i < e; i++ {
						sum[i] += wave[i]
					}
				}
			}(start, end)
		}
		wg.Wait()
	} else {
		for _, wave := range waves {
			for i, v := range wave {
				sum[i] += v
			}
		}
	}

	// Copy out and scale to avoid clipping.
	scale := float32(len(msgs))
	waveform := make([]float32, nsamples)
	for i, v := range sum[:nsamples] {
		waveform[i] = v / scale
	}

	encodeBufPut(e.cfg.sampleRate, sum)
	return waveform, nil
}

// EncodeToBytes generates the waveform and converts it to raw PCM bytes
// at the configured sample rate and bit depth.
func (e *Encoder) EncodeToBytes(msg string) ([]byte, error) {
	if err := e.validatePCMConfig(); err != nil {
		return nil, err
	}

	wave, err := e.Encode(msg)
	if err != nil {
		return nil, err
	}
	return FloatToPCM(wave, e.cfg.bitDepth), nil
}

// FloatToPCM converts float32 samples in [-1,1] to raw PCM bytes.
func FloatToPCM(samples []float32, bitDepth int) []byte {
	switch bitDepth {
	case 24:
		return floatToPCM24(samples)
	case 32:
		return floatToPCM32(samples)
	default:
		return floatToPCM16(samples)
	}
}

func floatToPCM16(samples []float32) []byte {
	data := make([]byte, len(samples)*2)
	for i, v := range samples {
		if v > 1.0 {
			v = 1.0
		} else if v < -1.0 {
			v = -1.0
		}
		s := int16(v * 32767.0)
		binary.LittleEndian.PutUint16(data[i*2:], uint16(s))
	}
	return data
}

func floatToPCM24(samples []float32) []byte {
	data := make([]byte, len(samples)*3)
	for i, v := range samples {
		if v > 1.0 {
			v = 1.0
		} else if v < -1.0 {
			v = -1.0
		}
		// 24-bit signed, scaled to 8380000 (MSHV style, near full scale).
		s := int32(v * 8380000.0)
		if s > 8388607 {
			s = 8388607
		}
		if s < -8388608 {
			s = -8388608
		}
		data[i*3+0] = byte(s & 0xFF)
		data[i*3+1] = byte((s >> 8) & 0xFF)
		data[i*3+2] = byte((s >> 16) & 0xFF)
	}
	return data
}

func floatToPCM32(samples []float32) []byte {
	data := make([]byte, len(samples)*4)
	for i, v := range samples {
		if v > 1.0 {
			v = 1.0
		} else if v < -1.0 {
			v = -1.0
		}
		s := int32(v * 2147483647.0)
		binary.LittleEndian.PutUint32(data[i*4:], uint32(s))
	}
	return data
}
