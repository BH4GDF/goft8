// subtract.go — Signal subtraction for the research package.
//
// Port of subroutine subtractft8 from wsjt-wsjtx/lib/ft8/subtractft8.f90.
//
// Pure port — no production dependency.

package goft8

import (
	"math"
	"math/cmplx"
	"sync"
)

const subtractNFILT = 4000

// SubtractSignal holds parameters for subtracting one decoded signal.
type SubtractSignal struct {
	Tones [NN]int
	Freq  float64
	DT    float64 // unadjusted DT (before the -0.5 display adjustment)
}


var (
	subtractOnce          sync.Once
	subtractFilterFreq    []complex128 // FFT of normalized cos² window (length NMAX)
	subtractEndCorrection [subtractNFILT/2 + 1]float64
)

// initSubtractFilter builds the low-pass filter and end-correction table.
//
// Port of the first==.true. block in subtractft8.f90 (lines 25–42).
//
// The filter is a cos² window of length NFILT+1, normalized by its sum,
// circular-shifted, and forward-FFT'd. The end-correction factors compensate
// for the filter transient at the edges of the frame.
//
// Normalization note: the Fortran applies fac=1/N after the forward FFT
// because four2a is unnormalized in both directions. Our FFT() is also
// unnormalized, but IFFT() normalizes by 1/N. To get the correct convolution
// result from IFFT(FFT(signal) * filter), we must NOT divide the filter by N.
func initSubtractFilter() {
	subtractOnce.Do(func() {
		halfFilt := subtractNFILT / 2 // 2000

		// Build cos² window: window[j] for j = -halfFilt..+halfFilt
		// Indexed as window[j+halfFilt] for j in [-halfFilt, halfFilt].
		windowLen := subtractNFILT + 1
		window := make([]float64, windowLen)
		sumw := 0.0
		for j := -halfFilt; j <= halfFilt; j++ {
			v := math.Cos(math.Pi * float64(j) / float64(subtractNFILT))
			window[j+halfFilt] = v * v
			sumw += v * v
		}

		// Place normalized window into complex array of length NMAX,
		// then circular-shift by NFILT/2+1.
		cw := make([]complex128, NMAX)
		for i := 0; i < windowLen; i++ {
			cw[i] = complex(window[i]/sumw, 0)
		}
		cw = cshift(cw, halfFilt+1)

		// Forward FFT (unnormalized).
		subtractFilterFreq = FFT(cw)

		// Precompute end-correction factors.
		// endcorrection[j] = 1 / (1 - sum(window[j-1:halfFilt]) / sumw)
		// where j runs from 1 to halfFilt+1 (Fortran 1-based).
		// In 0-based window indexing: window[j-1+halfFilt .. 2*halfFilt].
		for j := 1; j <= halfFilt+1; j++ {
			partialSum := 0.0
			for k := j - 1; k <= halfFilt; k++ {
				partialSum += window[k+halfFilt]
			}
			subtractEndCorrection[j-1] = 1.0 / (1.0 - partialSum/sumw)
		}
	})
}

// cwavePool reuses the NFRAME-length complex reference waveforms.
var cwavePool = sync.Pool{
	New: func() interface{} {
		return make([]complex128, NFRAME)
	},
}

// cfiltPool reuses the NMAX-length complex buffers for SubtractFT8.
var cfiltPool = sync.Pool{
	New: func() interface{} {
		return make([]complex128, NMAX)
	},
}

// SubtractFT8 removes a decoded signal from audio using the FFT-based
// low-pass filter method.
//
// Port of subroutine subtractft8 from wsjt-wsjtx/lib/ft8/subtractft8.f90
// (lrefinedt=.false. path).
func SubtractFT8(dd []float32, itone [NN]int, f0, xdt float64) {
	initSubtractFilter()

	halfFilt := subtractNFILT / 2

	// Generate complex reference waveform into a pooled buffer.
	cref := cwavePool.Get().([]complex128)
	defer cwavePool.Put(cref)
	copyCWave(cref, itone, f0)

	// Compute starting sample index.
	nstart := int(xdt*Fs) + 1 // Fortran: nstart = dt*12000 + 1

	// Conjugate-multiply: camp[i] = dd[nstart-1+i] * conj(cref[i])
	cfilt := cfiltPool.Get().([]complex128)
	for i := 0; i < NMAX; i++ {
		cfilt[i] = 0
	}
	for i := 0; i < NFRAME; i++ {
		j := nstart - 1 + i // 0-based index into dd
		if j >= 0 && j < NMAX {
			cfilt[i] = complex(float64(dd[j]), 0) * cmplx.Conj(cref[i])
		}
	}
	// cfilt[NFRAME:] is already zero.

	// Forward FFT (in-place).
	FFTInto(cfilt, cfilt)

	// Multiply by filter in frequency domain.
	for i := range cfilt {
		cfilt[i] *= subtractFilterFreq[i]
	}

	// Inverse FFT (in-place).
	IFFTInto(cfilt, cfilt)

	// Apply end-correction to compensate for filter transients.
	for j := 0; j <= halfFilt; j++ {
		cfilt[j] *= complex(subtractEndCorrection[j], 0)
	}
	for j := 0; j <= halfFilt; j++ {
		idx := NFRAME - 1 - j
		cfilt[idx] *= complex(subtractEndCorrection[j], 0)
	}

	// Subtract the reconstructed signal.
	for i := 0; i < NFRAME; i++ {
		j := nstart - 1 + i
		if j >= 0 && j < NMAX {
			z := cfilt[i] * complex(real(cref[i]), imag(cref[i]))
			dd[j] -= 2.0 * float32(real(z))
		}
	}
	cfiltPool.Put(cfilt)
}

// copyCWave fills dst with the complex GFSK reference waveform.
// dst must have length >= NFRAME.
func copyCWave(dst []complex128, itone [NN]int, f0 float64) {
	// Reuse the shared dphi computation.
	dphi := genFT8DPhi(itone, f0)

	phi := 0.0
	for k := 0; k < NFRAME; k++ {
		j := NSPS + k
		sin, cos := math.Sincos(phi)
		dst[k] = complex(cos, sin)
		phi += dphi[j]
	}

	// Envelope shaping.
	nramp := NSPS / 8
	for i := 0; i < nramp; i++ {
		ramp := (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(2*nramp))) / 2.0
		dst[i] = complex(real(dst[i])*ramp, imag(dst[i])*ramp)
	}
	k1 := NN*NSPS - nramp
	for i := 0; i < nramp; i++ {
		ramp := (1.0 + math.Cos(2.0*math.Pi*float64(i)/float64(2*nramp))) / 2.0
		dst[k1+i] = complex(real(dst[k1+i])*ramp, imag(dst[k1+i])*ramp)
	}
}

// BatchSubtractFT8 removes multiple decoded signals from audio in parallel.
//
// For each signal it performs the same FFT-based low-pass filter subtraction
// as SubtractFT8, but the expensive FFT/filter/IFFT stages run concurrently
// across signals.  The final dd[j] -= signal updates are applied sequentially
// to avoid write races.
func BatchSubtractFT8(dd []float32, signals []SubtractSignal) {
	if len(signals) == 0 {
		return
	}
	if len(signals) == 1 {
		SubtractFT8(dd, signals[0].Tones, signals[0].Freq, signals[0].DT)
		return
	}

	initSubtractFilter()
	halfFilt := subtractNFILT / 2

	type result struct {
		nstart int
		cref   []complex128
		cfilt  []complex128
	}

	results := make([]result, len(signals))
	var wg sync.WaitGroup

	for i, sig := range signals {
		wg.Add(1)
		go func(idx int, sig SubtractSignal) {
			defer wg.Done()

			cref := GenFT8CWave(sig.Tones, sig.Freq)
			nstart := int(sig.DT*Fs) + 1

			cfilt := make([]complex128, NMAX)

			for j := 0; j < NFRAME; j++ {
				k := nstart - 1 + j
				if k >= 0 && k < NMAX {
					cfilt[j] = complex(float64(dd[k]), 0) * cmplx.Conj(cref[j])
				}
			}

			cfilt = FFT(cfilt)
			for j := range cfilt {
				cfilt[j] *= subtractFilterFreq[j]
			}
			cfilt = IFFT(cfilt)

			for j := 0; j <= halfFilt; j++ {
				cfilt[j] *= complex(subtractEndCorrection[j], 0)
				idx2 := NFRAME - 1 - j
				cfilt[idx2] *= complex(subtractEndCorrection[j], 0)
			}

			results[idx] = result{nstart: nstart, cref: cref, cfilt: cfilt}
		}(i, sig)
	}

	wg.Wait()

	// Serial subtract to avoid races on dd.
	for _, r := range results {
		for i := 0; i < NFRAME; i++ {
			j := r.nstart - 1 + i
			if j >= 0 && j < NMAX {
				z := r.cfilt[i] * complex(real(r.cref[i]), imag(r.cref[i]))
				dd[j] -= 2.0 * float32(real(z))
			}
		}
	}
}
