// fft.go — Mixed-radix FFT for goft8.
//
// Implements a recursive Cooley-Tukey decimation-in-time FFT supporting
// radix-2, radix-3, and radix-5 butterflies. All FT8 FFT sizes (192000,
// 180000, 3840, 3200, 1920) are 5-smooth and use this path.
//
// Pure Go — no CGO dependency. A CGO-FFTW backend may be added in a
// future release for the hot spectrogram path; the pure-Go version here
// is correct but ~40x slower than FFTW for 3840-point.

package goft8

import (
	"math"
	"sync"
)

// twiddleCache stores pre-computed twiddle factors W_N^k = exp(-j·2π·k/N)
// for each FFT size N encountered.  FT8 only uses a handful of sizes
// (3840, 192000, 3200, 1920, 180000, …) so the total memory footprint
// is small (~3 MiB) and every lookup hits the cache after warm-up.
var twiddleCache sync.Map

func getTwiddles(n int) []complex128 {
	if v, ok := twiddleCache.Load(n); ok {
		return v.([]complex128)
	}
	tw := make([]complex128, n)
	twopi := 2.0 * math.Pi
	for k := 0; k < n; k++ {
		angle := -twopi * float64(k) / float64(n)
		sin, cos := math.Sincos(angle)
		tw[k] = complex(cos, sin)
	}
	twiddleCache.Store(n, tw)
	return tw
}

// SpectrogramFFT3840 computes a 3840-point real-to-complex FFT of a
// real-valued input and returns the power spectrum |X[i]|^2 for bins
// 1..NH1.
func SpectrogramFFT3840(x []float32) [NH1]float64 {
	buf := specFFTx64Pool.Get().([]float64)
	defer specFFTx64Pool.Put(buf)
	dst := specFFTdstPool.Get().([]complex128)
	defer specFFTdstPool.Put(dst)

	n := len(x)
	if n > NFFT1 {
		n = NFFT1
	}
	for i := 0; i < n; i++ {
		buf[i] = float64(x[i])
	}
	for i := n; i < NFFT1; i++ {
		buf[i] = 0
	}
	fft := getFFT(NFFT1)
	defer putFFT(NFFT1, fft)
	spec := fft.Coefficients(dst, buf)
	var pow [NH1]float64
	for i := 1; i <= NH1; i++ {
		re := real(spec[i])
		im := imag(spec[i])
		pow[i-1] = re*re + im*im
	}
	return pow
}

// specFFTx64Pool reuses the float64 input buffer for SpectrogramFFT3840.
var specFFTx64Pool = sync.Pool{New: func() interface{} { return make([]float64, NFFT1) }}

// specFFTdstPool reuses the complex128 output buffer for SpectrogramFFT3840.
// A 3840-point real FFT produces 1921 complex coefficients.
var specFFTdstPool = sync.Pool{New: func() interface{} { return make([]complex128, NFFT1/2+1) }}

// FFT computes the forward complex-to-complex FFT (unnormalized).
//
// X[k] = sum_{n=0}^{N-1} x[n] * exp(-j*2*pi*k*n/N)
//
// Input length must be 5-smooth (only factors of 2, 3, 5).
func FFT(x []complex128) []complex128 {
	n := len(x)
	fft := getCmplxFFT(n)
	defer putCmplxFFT(n, fft)
	return fft.Coefficients(nil, x)
}

// FFTInto computes the forward FFT and stores the result in dst.
// dst may be nil, in which case a new slice is allocated.
func FFTInto(dst, x []complex128) []complex128 {
	n := len(x)
	fft := getCmplxFFT(n)
	defer putCmplxFFT(n, fft)
	return fft.Coefficients(dst, x)
}

// IFFT computes the inverse complex-to-complex FFT (normalized by 1/N).
func IFFT(x []complex128) []complex128 {
	n := len(x)
	fft := getCmplxFFT(n)
	defer putCmplxFFT(n, fft)
	dst := make([]complex128, n)
	fft.Sequence(dst, x)
	scale := 1.0 / float64(n)
	for i := range dst {
		dst[i] *= complex(scale, 0)
	}
	return dst
}

// IFFTInto computes the inverse FFT and stores the result in dst.
// dst must have length >= len(x).
func IFFTInto(dst, x []complex128) {
	n := len(x)
	fft := getCmplxFFT(n)
	defer putCmplxFFT(n, fft)
	fft.Sequence(dst, x)
	scale := 1.0 / float64(n)
	for i := 0; i < n; i++ {
		dst[i] *= complex(scale, 0)
	}
}

// smallestFactor returns the smallest prime factor of n from {2, 3, 5}.
// Panics if n has a prime factor > 5 (not 5-smooth).
func smallestFactor(n int) int {
	if n%2 == 0 {
		return 2
	}
	if n%3 == 0 {
		return 3
	}
	if n%5 == 0 {
		return 5
	}
	panic("fft: size is not 5-smooth")
}

// fftMixedRadix computes an in-place forward FFT using recursive
// decimation-in-time with mixed radix-2/3/5 butterflies.
func fftMixedRadix(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}

	p := smallestFactor(n) // radix
	m := n / p             // sub-transform length

	// Decimation: split into p interleaved sub-sequences of length m.
	subs := make([][]complex128, p)
	for j := 0; j < p; j++ {
		subs[j] = make([]complex128, m)
		for k := 0; k < m; k++ {
			subs[j][k] = x[k*p+j]
		}
	}

	// Recurse on each sub-sequence.
	for j := 0; j < p; j++ {
		subs[j] = fftMixedRadix(subs[j])
	}

	// Combine with twiddle factors and p-point DFT butterflies.
	result := make([]complex128, n)

	switch p {
	case 2:
		tw := getTwiddles(n)
		for k := 0; k < m; k++ {
			w := tw[k]
			t := w * subs[1][k]
			result[k] = subs[0][k] + t
			result[k+m] = subs[0][k] - t
		}

	case 3:
		// W3 = exp(-j*2*pi/3) constants
		const (
			cos3 = -0.5                   // cos(2π/3)
			sin3 = -0.8660254037844386468 // -sin(2π/3)
		)
		tw := getTwiddles(n)
		for k := 0; k < m; k++ {
			s0 := subs[0][k]

			w1 := tw[k]
			s1 := w1 * subs[1][k]

			w2 := tw[2*k]
			s2 := w2 * subs[2][k]

			// 3-point DFT:
			// X[0] = s0 + s1 + s2
			// X[1] = s0 + s1*W3 + s2*W3^2
			// X[2] = s0 + s1*W3^2 + s2*W3
			t1 := s1 + s2
			t2 := s1 - s2

			result[k] = s0 + t1
			result[k+m] = s0 + complex(cos3*real(t1)-sin3*imag(t2), cos3*imag(t1)+sin3*real(t2))
			result[k+2*m] = s0 + complex(cos3*real(t1)+sin3*imag(t2), cos3*imag(t1)-sin3*real(t2))
		}

	case 5:
		// W5 = exp(-j*2*pi/5) constants
		cos1_5 := math.Cos(2.0 * math.Pi / 5)  //  0.30901699...
		sin1_5 := -math.Sin(2.0 * math.Pi / 5) // -0.95105652...
		cos2_5 := math.Cos(4.0 * math.Pi / 5)
		sin2_5 := -math.Sin(4.0 * math.Pi / 5)

		tw := getTwiddles(n)
		for k := 0; k < m; k++ {
			s0 := subs[0][k]

			w1 := tw[k]
			w2 := tw[2*k]
			w3 := tw[3*k]
			w4 := tw[4*k]

			s1 := w1 * subs[1][k]
			s2 := w2 * subs[2][k]
			s3 := w3 * subs[3][k]
			s4 := w4 * subs[4][k]

			// 5-point DFT using W5 roots:
			// X[q] = s0 + sum_{j=1}^{4} s_j * W5^(j*q)
			// W5^0=1, W5^1, W5^2, W5^3=conj(W5^2), W5^4=conj(W5^1)
			result[k] = s0 + s1 + s2 + s3 + s4

			for q := 1; q < 5; q++ {
				// W5^(j*q) for each j
				var sum complex128
				sum = s0
				for j := 1; j <= 4; j++ {
					jq := (j * q) % 5
					var wq complex128
					switch jq {
					case 0:
						wq = 1
					case 1:
						wq = complex(cos1_5, sin1_5)
					case 2:
						wq = complex(cos2_5, sin2_5)
					case 3:
						wq = complex(cos2_5, -sin2_5) // conj of W5^2
					case 4:
						wq = complex(cos1_5, -sin1_5) // conj of W5^1
					}
					sj := [4]complex128{s1, s2, s3, s4}
					sum += wq * sj[j-1]
				}
				result[k+q*m] = sum
			}
		}
	}

	return result
}
