// encode.go implements FT8 tone generation and GFSK signal synthesis.
//
// Port of genft8.f90 (entry get_ft8_tones_from_77bits) and gen_ft8wave.f90.

package encode

import (
	"github.com/bh4gdf/goft8/internal/ldpc"
	ft8params "github.com/bh4gdf/goft8/params"
	"math"
	"sync"
)

// GenFT8Tones generates the 79 channel tones from 77 message bits.
//
// Port of genft8.f90 entry get_ft8_tones_from_77bits (lines 28–44).
//
// Steps: append CRC-14 → encodeLDPCNoCRC → Costas sync + Gray-mapped data tones.
func GenFT8Tones(msgBits [77]int8) [ft8params.NN]int {
	// Build 91-bit message: 77 message bits + 14 CRC bits.
	crc := ldpc.ComputeCRC14(msgBits)
	var message91 [ft8params.LDPCk]int8
	copy(message91[:77], msgBits[:])
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crc >> uint(13-i)) & 1)
	}

	// Encode to 174-bit codeword.
	codeword := ldpc.EncodeLDPCNoCRC(message91)

	// Build tone array: S7 D29 S7 D29 S7
	var itone [ft8params.NN]int

	// Costas sync arrays at positions 0–6, 36–42, 72–78.
	for i := 0; i < 7; i++ {
		itone[i] = ft8params.Icos7[i]
		itone[36+i] = ft8params.Icos7[i]
		itone[ft8params.NN-7+i] = ft8params.Icos7[i]
	}

	// 58 data tones: Gray-map each 3-bit group from the codeword.
	k := 7
	for j := 0; j < 58; j++ {
		i := 3 * j
		if j == 29 {
			k += 7 // skip second Costas block
		}
		indx := int(codeword[i])*4 + int(codeword[i+1])*2 + int(codeword[i+2])
		itone[k] = ft8params.GrayMap[indx]
		k++
	}

	return itone
}

// ── Sample-rate aware pulse cache ────────────────────────────────────────

type pulseCacheEntry struct {
	pulse []float64 // gfskPulse values
	peak  []float64 // dphiPeak * pulse
}

var (
	ft8PulseCaches  = make(map[int]*pulseCacheEntry)
	ft8PulseCacheMu sync.Mutex
)

func getPulseCache(sampleRate int) *pulseCacheEntry {
	ft8PulseCacheMu.Lock()
	defer ft8PulseCacheMu.Unlock()

	if e, ok := ft8PulseCaches[sampleRate]; ok {
		return e
	}

	nsps := ft8params.NSPS
	if sampleRate == 48000 {
		nsps = 4 * ft8params.NSPS // 7680
	}
	pulseLen := 3 * nsps
	dphiPeak := 2.0 * math.Pi / float64(nsps)

	e := &pulseCacheEntry{
		pulse: make([]float64, pulseLen),
		peak:  make([]float64, pulseLen),
	}
	for i := 0; i < pulseLen; i++ {
		tt := (float64(i) - 1.5*float64(nsps)) / float64(nsps)
		p := gfskPulse(2.0, tt)
		e.pulse[i] = p
		e.peak[i] = dphiPeak * p
	}
	ft8PulseCaches[sampleRate] = e
	return e
}

// gfskPulse computes the GFSK frequency-smoothing pulse.
//
// Port of gfsk_pulse.f90.
func gfskPulse(bt, t float64) float64 {
	c := math.Pi * math.Sqrt(2.0/math.Ln2)
	return 0.5 * (math.Erf(c*bt*(t+0.5)) - math.Erf(c*bt*(t-0.5)))
}

const (
	encodeNSym  = ft8params.NN
	encodeTwopi = 2.0 * math.Pi
)

// encodeParams returns the NSPS and derived quantities for a given sample rate.
func EncodeParams(sampleRate int) (nsps, nwave int, dt float64) {
	nsps = ft8params.NSPS
	if sampleRate == 48000 {
		nsps = 4 * ft8params.NSPS // 7680
	}
	nwave = encodeNSym * nsps
	dt = 1.0 / float64(sampleRate)
	return
}

// genFT8DPhi builds the smoothed frequency waveform dphi shared by
// GenFT8CWaveSR and GenFT8WaveSR.
func GenFT8DPhi(itone [ft8params.NN]int, f0 float64, sampleRate int) []float64 {
	nsps, _, dt := EncodeParams(sampleRate)
	cache := getPulseCache(sampleRate)
	pulsePeak := cache.peak
	pulseLen := len(pulsePeak)

	dphiLen := (encodeNSym + 2) * nsps
	dphi := make([]float64, dphiLen)

	// Accumulate pulse-shaped frequency deviation for each symbol.
	for j := 0; j < encodeNSym; j++ {
		ib := j * nsps
		tone := float64(itone[j])
		for s := 0; s < pulseLen; s++ {
			dphi[ib+s] += pulsePeak[s] * tone
		}
	}

	// Dummy symbol at beginning (tone = itone[0]).
	tone0 := float64(itone[0])
	for s := nsps; s < pulseLen; s++ {
		dphi[s-nsps] += pulsePeak[s] * tone0
	}

	// Dummy symbol at end (tone = itone[encodeNSym-1]).
	toneLast := float64(itone[encodeNSym-1])
	ib := encodeNSym * nsps
	for s := 0; s < 2*nsps; s++ {
		dphi[ib+s] += pulsePeak[s] * toneLast
	}

	// Add carrier frequency offset.
	f0dphi := encodeTwopi * f0 * dt
	for i := range dphi {
		dphi[i] += f0dphi
	}

	return dphi
}

// GenFT8CWave generates the complex GFSK reference waveform for a signal
// at frequency f0 with the given tone sequence.
//
// Port of gen_ft8wave.f90 with FT8 parameters (nsym=79, nsps=1920, bt=2.0,
// fsample=12000, icmplx=1).
//
// Returns a []complex128 of length NFRAME (= NN*NSPS = 151680).
func GenFT8CWave(itone [ft8params.NN]int, f0 float64) []complex128 {
	return GenFT8CWaveSR(itone, f0, ft8params.Fs)
}

// GenFT8CWaveSR generates the complex GFSK waveform at an arbitrary sample
// rate.  Supported rates are 12000 and 48000 Hz.
func GenFT8CWaveSR(itone [ft8params.NN]int, f0 float64, sampleRate int) []complex128 {
	nsps, nwave, _ := EncodeParams(sampleRate)
	dphi := GenFT8DPhi(itone, f0, sampleRate)

	cwave := make([]complex128, nwave)
	phi := 0.0
	for k := 0; k < nwave; k++ {
		j := nsps + k // offset past the dummy symbol
		sin, cos := math.Sincos(phi)
		cwave[k] = complex(cos, sin)
		phi += dphi[j]
	}

	// Envelope shaping — raised-cosine ramp on first and last nramp samples.
	nramp := nsps / 8
	for i := 0; i < nramp; i++ {
		ramp := (1.0 - math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[i] = complex(real(cwave[i])*ramp, imag(cwave[i])*ramp)
	}
	k1 := encodeNSym*nsps - nramp
	for i := 0; i < nramp; i++ {
		ramp := (1.0 + math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[k1+i] = complex(real(cwave[k1+i])*ramp, imag(cwave[k1+i])*ramp)
	}

	return cwave
}

// GenFT8Wave generates the real-valued GFSK waveform for a signal at
// frequency f0 with the given tone sequence.  This is equivalent to
// real(GenFT8CWave(...)) but avoids the complex128 allocation.
//
// Returns a []float32 of length NFRAME (151680 samples @ 12 kHz).
func GenFT8Wave(itone [ft8params.NN]int, f0 float64) []float32 {
	return GenFT8WaveSR(itone, f0, ft8params.Fs)
}

// GenFT8WaveSR generates the real-valued GFSK waveform at an arbitrary
// sample rate.  Supported rates are 12000 and 48000 Hz.
func GenFT8WaveSR(itone [ft8params.NN]int, f0 float64, sampleRate int) []float32 {
	nsps, nwave, _ := EncodeParams(sampleRate)
	dphi := GenFT8DPhi(itone, f0, sampleRate)

	wave := make([]float32, nwave)
	phi := 0.0
	for k := 0; k < nwave; k++ {
		j := nsps + k
		_, cos := math.Sincos(phi)
		wave[k] = float32(cos)
		phi += dphi[j]
	}

	// Envelope shaping.
	nramp := nsps / 8
	for i := 0; i < nramp; i++ {
		ramp := float32((1.0 - math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0)
		wave[i] *= ramp
	}
	k1 := encodeNSym*nsps - nramp
	for i := 0; i < nramp; i++ {
		ramp := float32((1.0 + math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0)
		wave[k1+i] *= ramp
	}

	return wave
}
