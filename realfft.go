// realfft.go — Real-to-complex FFT wrapper.
//
// The implementation delegates to gonum's fourier.FFT, which internally
// applies the "pack and unpack" trick (equivalent to FFTW's r2c) for
// roughly 2× speedup over a naïve complex FFT of real data.

package goft8

import "sync"

// realFFTBufPool caches float64 input buffers for RealFFT, keyed by size.
// Only the hot-path sizes (192000 for downsampling, 3840 for spectrogram)
// are pre-warmed.
var realFFTBufPool sync.Map // int -> *sync.Pool{New: func() []float64 }

func getRealFFTBuf(n int) []float64 {
	if p, ok := realFFTBufPool.Load(n); ok {
		return p.(*sync.Pool).Get().([]float64)
	}
	return make([]float64, n)
}

func putRealFFTBuf(n int, buf []float64) {
	if p, ok := realFFTBufPool.Load(n); ok {
		p.(*sync.Pool).Put(buf)
	}
}

func init() {
	// Pre-warm RealFFT buffers for hot-path sizes.
	for _, n := range []int{3840, 192000} {
		size := n
		p := &sync.Pool{New: func() interface{} { return make([]float64, size) }}
		for i := 0; i < 20; i++ {
			p.Put(make([]float64, size))
		}
		realFFTBufPool.Store(size, p)
	}
}

// RealFFT computes the forward FFT of a real-valued signal.
//
// x contains the real samples (maybe shorter than n; missing values are
// treated as zero).  n must be even.
//
// Returns n/2+1 complex values representing the positive-frequency half
// of the spectrum (the negative half is the complex conjugate mirror).
func RealFFT(x []float32, n int) []complex128 {
	dst := make([]complex128, n/2+1)
	RealFFTInto(dst, x, n)
	return dst
}

// RealFFTInto computes the forward real FFT and stores the result in dst.
// dst must have length >= n/2+1.  x may be shorter than n; missing values
// are treated as zero.
func RealFFTInto(dst []complex128, x []float32, n int) {
	fft := getFFT(n)
	defer putFFT(n, fft)
	x64 := getRealFFTBuf(n)
	defer putRealFFTBuf(n, x64)
	lx := len(x)
	if lx > n {
		lx = n
	}
	for i := 0; i < lx; i++ {
		x64[i] = float64(x[i])
	}
	for i := lx; i < n; i++ {
		x64[i] = 0
	}
	fft.Coefficients(dst, x64)
}
