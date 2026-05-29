package goft8

import (
	"sync"
	
	"gonum.org/v1/gonum/dsp/fourier"
)

// gonum FFT pools — one per size used in the FT8 pipeline.
// gonum FFT objects are NOT safe for concurrent use, so each goroutine
// borrows its own from the pool.
var (
	fftPool3840    = &sync.Pool{New: func() interface{} { return fourier.NewFFT(3840) }}
	fftPool192000  = &sync.Pool{New: func() interface{} { return fourier.NewFFT(192000) }}
	fftPool3200    = &sync.Pool{New: func() interface{} { return fourier.NewFFT(3200) }}
	fftPool180000  = &sync.Pool{New: func() interface{} { return fourier.NewFFT(180000) }}

	cmplxFFTPool3840    = &sync.Pool{New: func() interface{} { return fourier.NewCmplxFFT(3840) }}
	cmplxFFTPool1920    = &sync.Pool{New: func() interface{} { return fourier.NewCmplxFFT(1920) }}
	cmplxFFTPool96000   = &sync.Pool{New: func() interface{} { return fourier.NewCmplxFFT(96000) }}
	cmplxFFTPool3200    = &sync.Pool{New: func() interface{} { return fourier.NewCmplxFFT(3200) }}
	cmplxFFTPool180000  = &sync.Pool{New: func() interface{} { return fourier.NewCmplxFFT(180000) }}
)

func init() {
	// Pre-warm pools so the expensive gonum.Reset allocations happen
	// at init time rather than on the hot decode path.
	const warmCount = 32
	for i := 0; i < warmCount; i++ {
		fftPool3840.Put(fourier.NewFFT(3840))
		fftPool192000.Put(fourier.NewFFT(192000))
		fftPool3200.Put(fourier.NewFFT(3200))
		fftPool180000.Put(fourier.NewFFT(180000))
		cmplxFFTPool3840.Put(fourier.NewCmplxFFT(3840))
		cmplxFFTPool1920.Put(fourier.NewCmplxFFT(1920))
		cmplxFFTPool96000.Put(fourier.NewCmplxFFT(96000))
		cmplxFFTPool3200.Put(fourier.NewCmplxFFT(3200))
		cmplxFFTPool180000.Put(fourier.NewCmplxFFT(180000))
	}
}

func getFFT(n int) *fourier.FFT {
	switch n {
	case 3840: return fftPool3840.Get().(*fourier.FFT)
	case 192000: return fftPool192000.Get().(*fourier.FFT)
	case 3200: return fftPool3200.Get().(*fourier.FFT)
	case 180000: return fftPool180000.Get().(*fourier.FFT)
	default: return fourier.NewFFT(n)
	}
}

func putFFT(n int, t *fourier.FFT) {
	switch n {
	case 3840: fftPool3840.Put(t)
	case 192000: fftPool192000.Put(t)
	case 3200: fftPool3200.Put(t)
	case 180000: fftPool180000.Put(t)
	}
}

func getCmplxFFT(n int) *fourier.CmplxFFT {
	switch n {
	case 3840: return cmplxFFTPool3840.Get().(*fourier.CmplxFFT)
	case 1920: return cmplxFFTPool1920.Get().(*fourier.CmplxFFT)
	case 96000: return cmplxFFTPool96000.Get().(*fourier.CmplxFFT)
	case 3200: return cmplxFFTPool3200.Get().(*fourier.CmplxFFT)
	case 180000: return cmplxFFTPool180000.Get().(*fourier.CmplxFFT)
	default: return fourier.NewCmplxFFT(n)
	}
}

func putCmplxFFT(n int, t *fourier.CmplxFFT) {
	switch n {
	case 3840: cmplxFFTPool3840.Put(t)
	case 1920: cmplxFFTPool1920.Put(t)
	case 96000: cmplxFFTPool96000.Put(t)
	case 3200: cmplxFFTPool3200.Put(t)
	case 180000: cmplxFFTPool180000.Put(t)
	}
}
