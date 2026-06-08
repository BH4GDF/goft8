package decode

import (
	"math"
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestComputeSoftMetricsMatchesReference(t *testing.T) {
	cd0 := metricsTestCD0()
	cs, _ := ComputeSymbolSpectra(cd0, 100)

	wantA, wantB, wantC, wantD, wantE := referenceComputeSoftMetrics(&cs)
	gotA, gotB, gotC, gotD, gotE := ComputeSoftMetrics(&cs)

	compareMetricSet(t, "bmeta", gotA, wantA)
	compareMetricSet(t, "bmetb", gotB, wantB)
	compareMetricSet(t, "bmetc", gotC, wantC)
	compareMetricSet(t, "bmetd", gotD, wantD)
	compareMetricSet(t, "bmete", gotE, wantE)
}

func compareMetricSet(t *testing.T, name string, got, want [174]float64) {
	t.Helper()

	for i := range got {
		diff := math.Abs(got[i] - want[i])
		if diff > 1e-12 {
			t.Fatalf("%s[%d] = %.17g, want %.17g (diff %.3g)", name, i, got[i], want[i], diff)
		}
	}
}

func metricsTestCD0() []complex128 {
	cd0 := make([]complex128, ft8params.NP2)
	for i := range cd0 {
		t := float64(i) / ft8params.Fs2
		re := 0.7*math.Cos(2*math.Pi*2.5*t) + 0.2*math.Cos(2*math.Pi*7.25*t)
		im := 0.7*math.Sin(2*math.Pi*2.5*t) + 0.2*math.Sin(2*math.Pi*7.25*t)
		cd0[i] = complex(re, im)
	}
	return cd0
}

func referenceComputeSoftMetrics(cs *[8][ft8params.NN]complex128) (bmeta, bmetb, bmetc, bmetd, bmete [174]float64) {
	graymap := ft8params.GrayMap
	var s2buf [512]float64

	for nsym := 1; nsym <= 3; nsym++ {
		nt := 1 << (3 * nsym)
		s2 := s2buf[:nt]

		for ihalf := 1; ihalf <= 2; ihalf++ {
			for k := 1; k <= 29; k += nsym {
				var ks int
				if ihalf == 1 {
					ks = k + 7
				} else {
					ks = k + 43
				}

				for i := 0; i < nt; i++ {
					i1 := i / 64
					i2 := (i & 63) / 8
					i3 := i & 7

					switch nsym {
					case 1:
						z := cs[graymap[i3]][ks-1]
						r, im := real(z), imag(z)
						s2[i] = math.Sqrt(r*r + im*im)
					case 2:
						z := cs[graymap[i2]][ks-1] + cs[graymap[i3]][ks]
						r, im := real(z), imag(z)
						s2[i] = math.Sqrt(r*r + im*im)
					case 3:
						z := cs[graymap[i1]][ks-1] + cs[graymap[i2]][ks] + cs[graymap[i3]][ks+1]
						r, im := real(z), imag(z)
						s2[i] = math.Sqrt(r*r + im*im)
					}
				}

				i32 := (k-1)*3 + (ihalf-1)*87
				ibmax := 3*nsym - 1

				for ib := 0; ib <= ibmax; ib++ {
					bitPos := ibmax - ib
					max1 := -1e30
					max0 := -1e30
					for idx := 0; idx < nt; idx++ {
						if (idx>>uint(bitPos))&1 != 0 {
							if s2[idx] > max1 {
								max1 = s2[idx]
							}
						} else if s2[idx] > max0 {
							max0 = s2[idx]
						}
					}

					idx := i32 + ib
					if idx >= 174 {
						continue
					}

					bm := max1 - max0
					switch nsym {
					case 1:
						bmeta[idx] = bm
						den := max1
						if max0 > den {
							den = max0
						}
						if den > 0 {
							bmetd[idx] = bm / den
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

	normalizeBmet(bmeta[:])
	normalizeBmet(bmetb[:])
	normalizeBmet(bmetc[:])
	normalizeBmet(bmetd[:])
	normalizeBmet(bmete[:])

	return
}
