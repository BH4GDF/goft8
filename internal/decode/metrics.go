// metrics.go implements symbol spectra and soft-metric extraction.
//
// Port of the spectra/metric blocks in ft8b.f90 lines 154–233.
// Source of truth: wsjt-wsjtx/lib/ft8/ft8b.f90.

package decode

import (
	ft8params "github.com/bh4gdf/goft8/params"
	"math"
)

// ComputeSymbolSpectra extracts the complex and magnitude spectra for all
// NN=79 channel symbols from the downsampled signal cd0, starting at
// sample offset ibest.
//
// Returns:
//
//	cs[tone][symbol]  — complex amplitude (scaled by 1e-3)
//	s8[tone][symbol]  — magnitude of raw FFT output (UNscaled)
//
// Port of ft8b.f90 lines 154–161:
//
//	do k=1,NN
//	  i1=ibest+(k-1)*32
//	  csymb=cmplx(0.0,0.0)
//	  if( i1.ge.0 .and. i1+31 .le. NP2-1 ) csymb=cd0(i1:i1+31)
//	  call four2a(csymb,32,1,-1,1)          !c2c forward FFT
//	  cs(0:7,k)=csymb(1:8)/1e3
//	  s8(0:7,k)=abs(csymb(1:8))
//	enddo
//
// Fortran cs is cs(0:7, 1:NN) — 0-indexed tone, 1-indexed symbol.
// Go cs is cs[0..7][0..NN-1] — 0-indexed in both.
// Fortran k=1 → Go index 0; Fortran csymb(1:8) → Go FFT output bins 0..7.
func ComputeSymbolSpectra(cd0 []complex128, ibest int) ([8][ft8params.NN]complex128, [8][ft8params.NN]float64) {
	var cs [8][ft8params.NN]complex128
	var s8 [8][ft8params.NN]float64

	for k := 1; k <= ft8params.NN; k++ {
		i1 := ibest + (k-1)*32

		// csymb = cmplx(0.0, 0.0)
		// if( i1.ge.0 .and. i1+31 .le. NP2-1 ) csymb = cd0(i1:i1+31)
		var csymb [32]complex128
		if i1 >= 0 && i1+31 <= ft8params.NP2-1 {
			for j := 0; j < 32; j++ {
				csymb[j] = cd0[i1+j]
			}
		}

		// call four2a(csymb,32,1,-1,1)   — 32-point c2c forward FFT
		// Operate in-place on the stack-allocated array (no heap allocation).
		fft32Array(&csymb)

		// cs(0:7,k) = csymb(1:8) / 1e3
		// s8(0:7,k) = abs(csymb(1:8))
		//
		// Fortran csymb(1:8) is 1-indexed → Go csymb[0:8] is 0-indexed.
		// abs() on a complex number = sqrt(re² + im²).
		for t := 0; t < 8; t++ {
			cs[t][k-1] = csymb[t] * complex(1e-3, 0) // /1e3
			r, im := real(csymb[t]), imag(csymb[t])
			s8[t][k-1] = math.Sqrt(r*r + im*im) // abs(complex)
		}
	}

	return cs, s8
}

// ComputeSymbolSpectraPower is like ComputeSymbolSpectra but returns squared
// magnitudes (power) instead of magnitudes. This avoids the expensive sqrt
// when the caller only needs power values for argmax comparisons.
func ComputeSymbolSpectraPower(cd0 []complex128, ibest int) ([8][ft8params.NN]complex128, [8][ft8params.NN]float64) {
	var cs [8][ft8params.NN]complex128
	var s8 [8][ft8params.NN]float64

	for k := 1; k <= ft8params.NN; k++ {
		i1 := ibest + (k-1)*32

		var csymb [32]complex128
		if i1 >= 0 && i1+31 <= ft8params.NP2-1 {
			for j := 0; j < 32; j++ {
				csymb[j] = cd0[i1+j]
			}
		}

		fft32Array(&csymb)

		for t := 0; t < 8; t++ {
			cs[t][k-1] = csymb[t] * complex(1e-3, 0)
			r, im := real(csymb[t]), imag(csymb[t])
			s8[t][k-1] = r*r + im*im // power, not magnitude
		}
	}

	return cs, s8
}

// fft32 computes a 32-point in-place forward FFT (decimation-in-time radix-2).
// Matches four2a with isign=-1: X[k] = sum_n x[n] * exp(-j*2*pi*n*k/32).
// Unnormalized (no 1/N scaling).
func fft32(x []complex128) {
	n := len(x)
	// Bit-reversal permutation.
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
		m := n >> 1
		for m >= 1 && j >= m {
			j -= m
			m >>= 1
		}
		j += m
	}
	// Cooley-Tukey butterfly stages.
	// isign = -1 → exp(-j*2*pi/N) per Fortran convention.
	for stage := 1; stage < n; stage <<= 1 {
		theta := -math.Pi / float64(stage)
		wm := complex(math.Cos(theta), math.Sin(theta))
		for k := 0; k < n; k += stage << 1 {
			w := complex(1, 0)
			for jj := 0; jj < stage; jj++ {
				t := w * x[k+jj+stage]
				u := x[k+jj]
				x[k+jj] = u + t
				x[k+jj+stage] = u - t
				w *= wm
			}
		}
	}
}

// fft32Array is an optimized 32-point FFT operating on a fixed-size array.
// This avoids heap allocation from the slice-based fft32 wrapper.
func fft32Array(x *[32]complex128) {
	const n = 32
	// Bit-reversal permutation.
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
		m := n >> 1
		for m >= 1 && j >= m {
			j -= m
			m >>= 1
		}
		j += m
	}
	// Cooley-Tukey butterfly stages (5 stages for n=32).
	for stage := 1; stage < n; stage <<= 1 {
		theta := -math.Pi / float64(stage)
		wm := complex(math.Cos(theta), math.Sin(theta))
		for k := 0; k < n; k += stage << 1 {
			w := complex(1, 0)
			for jj := 0; jj < stage; jj++ {
				t := w * x[k+jj+stage]
				u := x[k+jj]
				x[k+jj] = u + t
				x[k+jj+stage] = u - t
				w *= wm
			}
		}
	}
}

// ComputeSoftMetrics computes the five sets of soft-decision metrics
// (bmeta, bmetb, bmetc, bmetd, bmete) for the 174 LDPC LLR values from the
// complex symbol spectra.
//
// Port of ft8b.f90 lines 182–233 plus MSHV ws300rc1 bmete enhancement:
// bmete[i] = argmax(abs(bmeta[i]), abs(bmetb[i]), abs(bmetc[i]))
//
//	do nsym=1,3
//	  nt=2**(3*nsym)
//	  do ihalf=1,2
//	    do k=1,29,nsym
//	      if(ihalf.eq.1) ks=k+7
//	      if(ihalf.eq.2) ks=k+43
//	      ...
//	      i32=1+(k-1)*3+(ihalf-1)*87
//	      ...
//	    enddo
//	  enddo
//	enddo
//	call normalizebmet(bmeta,174)
//	call normalizebmet(bmetb,174)
//	call normalizebmet(bmetc,174)
//	call normalizebmet(bmetd,174)
//	call normalizebmet(bmete,174)  // MSHV enhancement
func ComputeSoftMetrics(cs *[8][ft8params.NN]complex128) (bmeta, bmetb, bmetc, bmetd, bmete [174]float64) {
	// Fortran: data graymap/0,1,3,2,5,6,4,7/
	graymap := ft8params.GrayMap

	for nsym := 1; nsym <= 3; nsym++ {
		// Fortran: ibmax = 2,5,8 for nsym = 1,2,3
		ibmax := 3*nsym - 1

		for ihalf := 1; ihalf <= 2; ihalf++ {
			for k := 1; k <= 29; k += nsym {
				// Fortran: if(ihalf.eq.1) ks=k+7; if(ihalf.eq.2) ks=k+43
				// Fortran ks is 1-indexed symbol index.
				// Go cs is 0-indexed in symbol dim, so ks-1 for access.
				var ks int
				if ihalf == 1 {
					ks = k + 7
				} else {
					ks = k + 43
				}

				var sym0, sym1, sym2 [8]complex128
				for i := 0; i < 8; i++ {
					tone := graymap[i]
					sym0[i] = cs[tone][ks-1]
					if nsym >= 2 {
						sym1[i] = cs[tone][ks]
					}
					if nsym == 3 {
						sym2[i] = cs[tone][ks+1]
					}
				}

				var maxHigh, maxMid, maxLow [8]float64

				// Fortran lines 189–202 compute candidate magnitudes s2, then
				// lines 207–209 scan maxima for each bit. Because sqrt is
				// monotonic, aggregate maxima in the power domain by the
				// 3-bit symbol fields and only take sqrt for final per-bit
				// maxima.
				switch nsym {
				case 1:
					for i := 0; i < 8; i++ {
						z := sym0[i]
						r, im := real(z), imag(z)
						maxLow[i] = r*r + im*im
					}
				case 2:
					for i2 := 0; i2 < 8; i2++ {
						z2 := sym0[i2]
						for i3 := 0; i3 < 8; i3++ {
							z := z2 + sym1[i3]
							r, im := real(z), imag(z)
							power := r*r + im*im
							if power > maxMid[i2] {
								maxMid[i2] = power
							}
							if power > maxLow[i3] {
								maxLow[i3] = power
							}
						}
					}
				case 3:
					for i1 := 0; i1 < 8; i1++ {
						z1 := sym0[i1]
						for i2 := 0; i2 < 8; i2++ {
							z12 := z1 + sym1[i2]
							for i3 := 0; i3 < 8; i3++ {
								z := z12 + sym2[i3]
								r, im := real(z), imag(z)
								power := r*r + im*im
								if power > maxHigh[i1] {
									maxHigh[i1] = power
								}
								if power > maxMid[i2] {
									maxMid[i2] = power
								}
								if power > maxLow[i3] {
									maxLow[i3] = power
								}
							}
						}
					}
				}

				// Fortran line 203: i32 = 1 + (k-1)*3 + (ihalf-1)*87
				// This is 1-based in Fortran. For Go 0-based: i32 = (k-1)*3 + (ihalf-1)*87
				i32 := (k-1)*3 + (ihalf-1)*87

				for ib := 0; ib <= ibmax; ib++ {
					bitPos := ibmax - ib

					// bm = maxval(s2(0:nt-1), one(0:nt-1, ibmax-ib))
					//    - maxval(s2(0:nt-1), .not.one(0:nt-1, ibmax-ib))
					var bm, den float64
					switch {
					case bitPos >= 6:
						bm, den = softMetricFromGroups(&maxHigh, uint(bitPos-6))
					case bitPos >= 3:
						bm, den = softMetricFromGroups(&maxMid, uint(bitPos-3))
					default:
						bm, den = softMetricFromGroups(&maxLow, uint(bitPos))
					}

					// Fortran: if(i32+ib .gt. 174) cycle
					// Fortran i32 is 1-based, so i32+ib > 174.
					// Go i32 is 0-based, so i32+ib >= 174.
					idx := i32 + ib
					if idx >= 174 {
						continue
					}

					switch nsym {
					case 1:
						bmeta[idx] = bm
						// den = max(maxval with one, maxval without one)
						if den > 0.0 {
							bmetd[idx] = bm / den
						} else {
							bmetd[idx] = 0.0
						}
					case 2:
						bmetb[idx] = bm
					case 3:
						bmetc[idx] = bm
					}
				}
			}
		}
	}

	// MSHV ws300rc1: bmete[i] = best of bmeta/bmetb/bmetc at each position.
	for i := 0; i < 174; i++ {
		best := bmeta[i]
		maxAbs := math.Abs(best)
		if v := math.Abs(bmetb[i]); v > maxAbs {
			best = bmetb[i]
			maxAbs = v
		}
		if v := math.Abs(bmetc[i]); v > maxAbs {
			best = bmetc[i]
		}
		bmete[i] = best
	}

	// Fortran lines 230–233: normalize all five metric arrays.
	normalizeBmet(bmeta[:])
	normalizeBmet(bmetb[:])
	normalizeBmet(bmetc[:])
	normalizeBmet(bmetd[:])
	normalizeBmet(bmete[:])

	return
}

func softMetricFromGroups(groups *[8]float64, bit uint) (bm, den float64) {
	var max1, max0 float64
	switch bit {
	case 0:
		max1 = max4(groups[1], groups[3], groups[5], groups[7])
		max0 = max4(groups[0], groups[2], groups[4], groups[6])
	case 1:
		max1 = max4(groups[2], groups[3], groups[6], groups[7])
		max0 = max4(groups[0], groups[1], groups[4], groups[5])
	default:
		max1 = max4(groups[4], groups[5], groups[6], groups[7])
		max0 = max4(groups[0], groups[1], groups[2], groups[3])
	}

	max1Mag := math.Sqrt(max1)
	max0Mag := math.Sqrt(max0)
	bm = max1Mag - max0Mag
	den = max1Mag
	if max0Mag > den {
		den = max0Mag
	}
	return bm, den
}

func max4(a, b, c, d float64) float64 {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	if d > a {
		a = d
	}
	return a
}

// normalizeBmet normalizes a metric array to unit variance.
//
// Port of subroutine normalizebmet from ft8b.f90 lines 466–479:
//
//	bmetav  = sum(bmet) / n
//	bmet2av = sum(bmet*bmet) / n
//	var = bmet2av - bmetav*bmetav
//	if(var > 0) then bmetsig = sqrt(var)
//	else             bmetsig = sqrt(bmet2av)
//	bmet = bmet / bmetsig
func normalizeBmet(bmet []float64) {
	n := float64(len(bmet))

	// bmetav = sum(bmet) / n
	av := 0.0
	for _, v := range bmet {
		av += v
	}
	av /= n

	// bmet2av = sum(bmet*bmet) / n
	av2 := 0.0
	for _, v := range bmet {
		av2 += v * v
	}
	av2 /= n

	// var = bmet2av - bmetav**2
	variance := av2 - av*av
	var sig float64
	if variance > 0 {
		sig = math.Sqrt(variance)
	} else {
		sig = math.Sqrt(av2)
	}
	if sig == 0 {
		return
	}

	// bmet = bmet / bmetsig
	for i := range bmet {
		bmet[i] /= sig
	}
}
