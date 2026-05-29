// encode.go — Tone generation and GFSK signal synthesis for the research package.
//
// Port of genft8.f90 (entry get_ft8_tones_from_77bits) and gen_ft8wave.f90.
//
// Pure port — no production dependency.

package goft8

import (
	"math"
	"sync"
)

// GenFT8Tones generates the 79 channel tones from 77 message bits.
//
// Port of genft8.f90 entry get_ft8_tones_from_77bits (lines 28–44).
//
// Steps: append CRC-14 → encodeLDPCNoCRC → Costas sync + Gray-mapped data tones.
func GenFT8Tones(msgBits [77]int8) [NN]int {
	// Build 91-bit message: 77 message bits + 14 CRC bits.
	crc := computeCRC14(msgBits)
	var message91 [LDPCk]int8
	copy(message91[:77], msgBits[:])
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crc >> uint(13-i)) & 1)
	}

	// Encode to 174-bit codeword.
	codeword := encodeLDPCNoCRC(message91)

	// Build tone array: S7 D29 S7 D29 S7
	var itone [NN]int

	// Costas sync arrays at positions 0–6, 36–42, 72–78.
	for i := 0; i < 7; i++ {
		itone[i] = Icos7[i]
		itone[36+i] = Icos7[i]
		itone[NN-7+i] = Icos7[i]
	}

	// 58 data tones: Gray-map each 3-bit group from the codeword.
	k := 7
	for j := 0; j < 58; j++ {
		i := 3 * j
		if j == 29 {
			k += 7 // skip second Costas block
		}
		indx := int(codeword[i])*4 + int(codeword[i+1])*2 + int(codeword[i+2])
		itone[k] = GrayMap[indx]
		k++
	}

	return itone
}

// ft8PulseCache holds the pre-computed GFSK pulse and its scaled version.
// BT=2.0 is fixed for FT8, so the pulse shape never changes.
var ft8PulseCache struct {
	once  sync.Once
	pulse []float64 // gfskPulse values
	peak  []float64 // dphiPeak * pulse
}

func initFT8PulseCache() {
	ft8PulseCache.once.Do(func() {
		pulseLen := 3 * NSPS
		ft8PulseCache.pulse = make([]float64, pulseLen)
		ft8PulseCache.peak = make([]float64, pulseLen)
		dphiPeak := 2.0 * math.Pi / float64(NSPS)
		for i := 0; i < pulseLen; i++ {
			tt := (float64(i) - 1.5*float64(NSPS)) / float64(NSPS)
			p := gfskPulse(2.0, tt)
			ft8PulseCache.pulse[i] = p
			ft8PulseCache.peak[i] = dphiPeak * p
		}
	})
}

// gfskPulse computes the GFSK frequency-smoothing pulse.
//
// Port of gfsk_pulse.f90.
func gfskPulse(bt, t float64) float64 {
	c := math.Pi * math.Sqrt(2.0/math.Ln2)
	return 0.5 * (math.Erf(c*bt*(t+0.5)) - math.Erf(c*bt*(t-0.5)))
}

const (
	encodeNSym    = NN
	encodeNSPS    = NSPS
	encodeFSample = Fs
	encodeNWave   = encodeNSym * encodeNSPS // NFRAME = 151680
	encodeTwopi   = 2.0 * math.Pi
	encodeDT      = 1.0 / encodeFSample
)

// genFT8DPhi builds the smoothed frequency waveform dphi shared by
// GenFT8CWave and GenFT8Wave.
func genFT8DPhi(itone [NN]int, f0 float64) []float64 {
	initFT8PulseCache()
	pulsePeak := ft8PulseCache.peak
	pulseLen := len(pulsePeak)

	dphiLen := (encodeNSym + 2) * encodeNSPS
	dphi := make([]float64, dphiLen)

	// Accumulate pulse-shaped frequency deviation for each symbol.
	for j := 0; j < encodeNSym; j++ {
		ib := j * encodeNSPS
		tone := float64(itone[j])
		for s := 0; s < pulseLen; s++ {
			dphi[ib+s] += pulsePeak[s] * tone
		}
	}

	// Dummy symbol at beginning (tone = itone[0]).
	tone0 := float64(itone[0])
	for s := encodeNSPS; s < pulseLen; s++ {
		dphi[s-encodeNSPS] += pulsePeak[s] * tone0
	}

	// Dummy symbol at end (tone = itone[encodeNSym-1]).
	toneLast := float64(itone[encodeNSym-1])
	ib := encodeNSym * encodeNSPS
	for s := 0; s < 2*encodeNSPS; s++ {
		dphi[ib+s] += pulsePeak[s] * toneLast
	}

	// Add carrier frequency offset.
	f0dphi := encodeTwopi * f0 * encodeDT
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
func GenFT8CWave(itone [NN]int, f0 float64) []complex128 {
	dphi := genFT8DPhi(itone, f0)

	// Generate complex waveform (skip the leading dummy symbol).
	// math.Sincos internally reduces the argument, so math.Mod is unnecessary.
	cwave := make([]complex128, encodeNWave)
	phi := 0.0
	for k := 0; k < encodeNWave; k++ {
		j := encodeNSPS + k // offset past the dummy symbol
		sin, cos := math.Sincos(phi)
		cwave[k] = complex(cos, sin)
		phi += dphi[j]
	}

	// Envelope shaping — raised-cosine ramp on first and last nramp samples.
	nramp := encodeNSPS / 8 // 240
	for i := 0; i < nramp; i++ {
		ramp := (1.0 - math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0
		cwave[i] = complex(real(cwave[i])*ramp, imag(cwave[i])*ramp)
	}
	k1 := encodeNSym*encodeNSPS - nramp
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
// Returns a []float32 of length NFRAME.
func GenFT8Wave(itone [NN]int, f0 float64) []float32 {
	dphi := genFT8DPhi(itone, f0)

	wave := make([]float32, encodeNWave)
	phi := 0.0
	for k := 0; k < encodeNWave; k++ {
		j := encodeNSPS + k
		_, cos := math.Sincos(phi)
		wave[k] = float32(cos)
		phi += dphi[j]
	}

	// Envelope shaping.
	nramp := encodeNSPS / 8
	for i := 0; i < nramp; i++ {
		ramp := float32((1.0 - math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0)
		wave[i] *= ramp
	}
	k1 := encodeNSym*encodeNSPS - nramp
	for i := 0; i < nramp; i++ {
		ramp := float32((1.0 + math.Cos(encodeTwopi*float64(i)/float64(2*nramp))) / 2.0)
		wave[k1+i] *= ramp
	}

	return wave
}
