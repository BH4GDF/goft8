// realfft.go — Real-to-complex FFT wrapper.
//
// The fftw branch delegates real FFTs to FFTW3 through CGO.

package dsp

// realFFTBufPools caches float64 input buffers for RealFFT, keyed by size.
var (
	realFFTBufPool3840   = newFixedPool[[]float64](128)
	realFFTBufPool192000 = newFixedPool[[]float64](128)
)

func init() {
	// Pre-warm RealFFT buffers for hot-path sizes.
	for i := 0; i < 64; i++ {
		realFFTBufPool3840.put(make([]float64, 3840))
		realFFTBufPool192000.put(make([]float64, 192000))
	}
}

func getRealFFTBuf(n int) []float64 {
	switch n {
	case 3840:
		buf := realFFTBufPool3840.get(func() []float64 { return make([]float64, 3840) })
		return buf[:3840]
	case 192000:
		buf := realFFTBufPool192000.get(func() []float64 { return make([]float64, 192000) })
		return buf[:192000]
	default:
		return make([]float64, n)
	}
}

func putRealFFTBuf(n int, buf []float64) {
	switch n {
	case 3840:
		realFFTBufPool3840.put(buf)
	case 192000:
		realFFTBufPool192000.put(buf)
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
	fftwRealFFTInto(dst, x64, n)
}
