// downsample.go implements audio downsampling for FT8 decoding.
//
// Port of subroutine ft8_downsample from wsjt-wsjtx/lib/ft8/ft8_downsample.f90
// and subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90.

package decode

import (
	"github.com/bh4gdf/goft8/internal/dsp"
	ft8params "github.com/bh4gdf/goft8/params"
	"math"
	"math/cmplx"
)

// Downsampler holds cached FFT state so the expensive 192000-point
// transform is only recomputed when the input audio changes.
//
// Port of the save'd state in ft8_downsample.f90 (lines 8–14):
//
//	complex cx(0:NFFT1/2)
//	real taper(0:100)
//	save x,cx,first,taper
type Downsampler struct {
	cx    []complex128 // Cached spectrum (NFFT1DS/2+1 elements)
	taper [101]float64 // Raised-cosine edge taper
	c1buf []complex128 // Reused buffer for downsample intermediate (nfft2)
	xbuf  []float32    // Reused buffer for input scaling
	ready bool
}

const cxLen = ft8params.NFFT1DS/2 + 1 // 96001

// NewDownsampler creates a Downsampler and precomputes the edge taper.
//
// Port of ft8_downsample.f90 lines 16–21:
//
//	if(first) then
//	   pi=4.0*atan(1.0)
//	   do i=0,100
//	     taper(i)=0.5*(1.0+cos(i*pi/100))
//	   enddo
//	   first=.false.
//	endif
func NewDownsampler() *Downsampler {
	d := &Downsampler{}
	pi := math.Pi
	for i := 0; i <= 100; i++ {
		d.taper[i] = 0.5 * (1.0 + math.Cos(float64(i)*pi/100.0))
	}
	return d
}

// CloneFrom returns a new Downsampler that shares the pre-computed FFT
// spectrum (cx) from `src`, avoiding the expensive 192000-point FFT.
// The clone has its own c1buf and xbuf but reads from src's cached cx.
// This is safe because Downsample only reads cx (never writes) after the
// initial FFT computation.
func CloneFrom(src *Downsampler) *Downsampler {
	d := &Downsampler{
		taper: src.taper,
		cx:    src.cx, // shared read-only reference
		ready: true,
	}
	return d
}

// Downsample mixes the audio in dd to baseband at f0 Hz, then decimates
// from 12000 Hz to 200 Hz (NDOWN=60×), returning a complex signal of
// length NFFT2 (3200 samples).
//
// When newdat is true the forward FFT of dd is recomputed; when false the
// cached spectrum from the previous call is reused.  On return newdat is
// set to false.
//
// This is a direct port of subroutine ft8_downsample from
// wsjt-wsjtx/lib/ft8/ft8_downsample.f90 (all 52 lines).
func (d *Downsampler) Downsample(dd []float32, newdat *bool, f0 float64) []complex128 {
	const (
		nfft1 = ft8params.NFFT1DS // 192000
		nfft2 = ft8params.NFFT2   // 3200
	)

	// Fortran lines 23–29:
	//   if(newdat) then
	//     x(1:NMAX)=dd
	//     x(NMAX+1:NFFT1+2)=0.
	//     call four2a(cx,NFFT1,1,-1,0)    !r2c FFT
	//     newdat=.false.
	//   endif
	if *newdat || d.cx == nil {
		if d.cx == nil {
			d.cx = make([]complex128, cxLen)
		}
		// Match MSHV ft8_downsample: x[i] = dd[i] * 0.01
		if len(d.xbuf) < len(dd) {
			d.xbuf = make([]float32, len(dd))
		}
		x := d.xbuf[:len(dd)]
		for i, v := range dd {
			x[i] = v * 0.01
		}
		dsp.RealFFTInto(d.cx, x, nfft1)
		*newdat = false
	}

	// Fortran lines 30–36:
	//   df=12000.0/NFFT1
	//   baud=12000.0/NSPS
	//   i0=nint(f0/df)
	//   ft=f0+8.5*baud
	//   it=min(nint(ft/df),NFFT1/2)
	//   fb=f0-1.5*baud
	//   ib=max(1,nint(fb/df))
	df := ft8params.Fs / float64(nfft1)
	i0 := int(math.Round(f0 / df))

	baud := ft8params.Fs / ft8params.NSPS
	ft := f0 + 8.5*baud
	fb := f0 - 1.5*baud

	it := int(math.Round(ft / df))
	if it > nfft1/2 {
		it = nfft1 / 2
	}
	ib := int(math.Round(fb / df))
	if ib < 1 {
		ib = 1
	}

	// Reuse or allocate c1 buffer.
	if len(d.c1buf) < nfft2 {
		d.c1buf = make([]complex128, nfft2)
	}
	c1 := d.c1buf[:nfft2]
	for i := range c1 {
		c1[i] = 0
	}

	// Fortran lines 37–42:
	//   k=0
	//   c1=0.
	//   do i=ib,it
	//    c1(k)=cx(i)
	//    k=k+1
	//   enddo
	k := 0
	for i := ib; i <= it && k < nfft2; i++ {
		c1[k] = d.cx[i]
		k++
	}

	// Fortran lines 43–44: edge taper.
	for i := 0; i <= 100 && i < k; i++ {
		c1[i] *= complex(d.taper[100-i], 0)
	}
	for i := 0; i <= 100; i++ {
		idx := k - 1 - 100 + i
		if idx >= 0 && idx < nfft2 {
			c1[idx] *= complex(d.taper[i], 0)
		}
	}

	// Fortran line 45: c1=decode.Cshift(c1,i0-ib)
	// In-place circular shift to avoid allocation.
	CshiftInPlace(c1, i0-ib)

	// Reuse c1 as the IFFT output buffer (in-place).
	dsp.IFFTInto(c1, c1)

	// Fortran lines 47–48: scaling.
	// NFFT2 factor compensates for gonum IFFT's 1/N normalization (fftw has none).
	fac := float64(nfft2) / math.Sqrt(float64(nfft1)*float64(nfft2))
	for i := range c1 {
		c1[i] *= complex(fac, 0)
	}

	return c1
}

// cshift is Fortran's CSHIFT(array, shift): circular left-shift by shift
// positions.  Matches Fortran intrinsic CSHIFT semantics exactly.
func Cshift(x []complex128, shift int) []complex128 {
	n := len(x)
	if n == 0 {
		return x
	}
	shift = ((shift % n) + n) % n
	if shift == 0 {
		return x
	}
	out := make([]complex128, n)
	copy(out, x[shift:])
	copy(out[n-shift:], x[:shift])
	return out
}

// CshiftInPlace performs a circular left-shift on x without allocation.
func CshiftInPlace(x []complex128, shift int) {
	n := len(x)
	if n == 0 {
		return
	}
	shift = ((shift % n) + n) % n
	if shift == 0 {
		return
	}
	// Triple-reverse in-place rotation.
	reverseComplex(x[:shift])
	reverseComplex(x[shift:])
	reverseComplex(x)
}

func reverseComplex(a []complex128) {
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
}

// TwkFreq1 applies a polynomial frequency correction to the complex signal ca,
// returning the corrected signal cb.  a[0] is the primary frequency offset in Hz
// (with sign flipped: positive a[0] shifts the signal down by a[0] Hz).
//
// Port of subroutine twkfreq1 from wsjt-wsjtx/lib/ft8/twkfreq1.f90 (all 27 lines).
func TwkFreq1(ca []complex128, fsample float64, a [5]float64) []complex128 {
	cb := make([]complex128, len(ca))
	TwkFreq1Into(cb, ca, fsample, a)
	return cb
}

// TwkFreq1Into applies the frequency correction and stores the result in cb.
// cb must have length >= len(ca).
func TwkFreq1Into(cb, ca []complex128, fsample float64, a [5]float64) {
	npts := len(ca)
	twopi := 2.0 * math.Pi
	w := complex(1.0, 0.0)
	x0 := 0.5 * float64(npts+1)
	s := 2.0 / float64(npts)

	for i := 1; i <= npts; i++ {
		x := s * (float64(i) - x0)
		p2 := 1.5*x*x - 0.5
		p3 := 2.5*(x*x*x) - 1.5*x
		p4 := 4.375*(x*x*x*x) - 3.75*(x*x) + 0.375
		dphi := (a[0] + x*a[1] + p2*a[2] + p3*a[3] + p4*a[4]) * (twopi / fsample)
		wstep := cmplx.Exp(complex(0, dphi))
		w *= wstep
		cb[i-1] = w * ca[i-1]
	}
}
