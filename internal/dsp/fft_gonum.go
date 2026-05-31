package dsp

import (
	"sync"

	"gonum.org/v1/gonum/dsp/fourier"
)

// fixedPool is a non-GC-cleared object pool.
// Unlike sync.Pool, items cached here survive garbage collection,
// avoiding expensive re-allocations of gonum FFT internal buffers.
type fixedPool[T any] struct {
	mu     sync.Mutex
	items  []T
	maxLen int
}

func newFixedPool[T any](maxLen int) *fixedPool[T] {
	return &fixedPool[T]{maxLen: maxLen}
}

func (p *fixedPool[T]) get(newFn func() T) T {
	p.mu.Lock()
	if n := len(p.items); n > 0 {
		item := p.items[n-1]
		p.items = p.items[:n-1]
		p.mu.Unlock()
		return item
	}
	p.mu.Unlock()
	return newFn()
}

func (p *fixedPool[T]) put(item T) {
	p.mu.Lock()
	if len(p.items) < p.maxLen {
		p.items = append(p.items, item)
	}
	p.mu.Unlock()
}

// gonum FFT pools — one per size used in the FT8 pipeline.
// gonum FFT objects are NOT safe for concurrent use, so each goroutine
// borrows its own from the pool.
var (
	fftPool3840   = newFixedPool[*fourier.FFT](128)
	fftPool192000 = newFixedPool[*fourier.FFT](128)
	fftPool3200   = newFixedPool[*fourier.FFT](128)
	fftPool180000 = newFixedPool[*fourier.FFT](128)

	cmplxFFTPool3840   = newFixedPool[*fourier.CmplxFFT](128)
	cmplxFFTPool1920   = newFixedPool[*fourier.CmplxFFT](128)
	cmplxFFTPool96000  = newFixedPool[*fourier.CmplxFFT](128)
	cmplxFFTPool3200   = newFixedPool[*fourier.CmplxFFT](128)
	cmplxFFTPool180000 = newFixedPool[*fourier.CmplxFFT](128)
)

func init() {
	// Pre-warm pools so the expensive gonum.Reset allocations happen
	// at init time rather than on the hot decode path.
	const warmCount = 64
	for i := 0; i < warmCount; i++ {
		fftPool3840.put(fourier.NewFFT(3840))
		fftPool192000.put(fourier.NewFFT(192000))
		fftPool3200.put(fourier.NewFFT(3200))
		fftPool180000.put(fourier.NewFFT(180000))
		cmplxFFTPool3840.put(fourier.NewCmplxFFT(3840))
		cmplxFFTPool1920.put(fourier.NewCmplxFFT(1920))
		cmplxFFTPool96000.put(fourier.NewCmplxFFT(96000))
		cmplxFFTPool3200.put(fourier.NewCmplxFFT(3200))
		cmplxFFTPool180000.put(fourier.NewCmplxFFT(180000))
	}
}

func getFFT(n int) *fourier.FFT {
	switch n {
	case 3840:
		return fftPool3840.get(func() *fourier.FFT { return fourier.NewFFT(3840) })
	case 192000:
		return fftPool192000.get(func() *fourier.FFT { return fourier.NewFFT(192000) })
	case 3200:
		return fftPool3200.get(func() *fourier.FFT { return fourier.NewFFT(3200) })
	case 180000:
		return fftPool180000.get(func() *fourier.FFT { return fourier.NewFFT(180000) })
	default:
		return fourier.NewFFT(n)
	}
}

func putFFT(n int, t *fourier.FFT) {
	switch n {
	case 3840:
		fftPool3840.put(t)
	case 192000:
		fftPool192000.put(t)
	case 3200:
		fftPool3200.put(t)
	case 180000:
		fftPool180000.put(t)
	}
}

func getCmplxFFT(n int) *fourier.CmplxFFT {
	switch n {
	case 3840:
		return cmplxFFTPool3840.get(func() *fourier.CmplxFFT { return fourier.NewCmplxFFT(3840) })
	case 1920:
		return cmplxFFTPool1920.get(func() *fourier.CmplxFFT { return fourier.NewCmplxFFT(1920) })
	case 96000:
		return cmplxFFTPool96000.get(func() *fourier.CmplxFFT { return fourier.NewCmplxFFT(96000) })
	case 3200:
		return cmplxFFTPool3200.get(func() *fourier.CmplxFFT { return fourier.NewCmplxFFT(3200) })
	case 180000:
		return cmplxFFTPool180000.get(func() *fourier.CmplxFFT { return fourier.NewCmplxFFT(180000) })
	default:
		return fourier.NewCmplxFFT(n)
	}
}

func putCmplxFFT(n int, t *fourier.CmplxFFT) {
	switch n {
	case 3840:
		cmplxFFTPool3840.put(t)
	case 1920:
		cmplxFFTPool1920.put(t)
	case 96000:
		cmplxFFTPool96000.put(t)
	case 3200:
		cmplxFFTPool3200.put(t)
	case 180000:
		cmplxFFTPool180000.put(t)
	}
}
