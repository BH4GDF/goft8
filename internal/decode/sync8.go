// sync8.go implements FT8 sync detection and candidate search.
//
// Port of subroutine sync8 from wsjt-wsjtx/lib/ft8/sync8.f90.

package decode

import (
	"github.com/bh4gdf/goft8/internal/dsp"
	ft8params "github.com/bh4gdf/goft8/params"
	"math"
	"runtime"
	"sort"
	"sync"
)

// fftBufPool caches float32 slices used as FFT input buffers.
// Key sizes: NSPS=1920 (spectrogram) and NFFT1=3840 (baseline).
var fftBufPool = sync.Pool{
	New: func() any {
		// Largest size needed is NFFT1; callers slice down if they need less.
		return make([]float32, ft8params.NFFT1)
	},
}

// min returns the smaller of a and b.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Constants ────────────────────────────────────────────────────────────

const (
	jz         = 62   // max sync lag ±2.5s  (sync8.f90 line 7: JZ=62)
	maxPreCand = 1000 // pre-filter cap       (sync8.f90 line 4: MAXPRECAND=1000)
)

// sync2dPool caches the large sync2d matrix used by computeSync2D.
// For standard FT8 (nfos=2) the matrix is (NH1+14) × 125 float64 ≈ 1.9 MB.
var sync2dPool = sync.Pool{
	New: func() any {
		nh1Pad := ft8params.NH1 + 2*6 + 1 // nfos=2
		lagCols := 2*jz + 1
		backing := make([]float64, (nh1Pad+1)*lagCols)
		sync2d := make([][]float64, nh1Pad+1)
		for i := range sync2d {
			sync2d[i] = backing[i*lagCols : (i+1)*lagCols]
		}
		return sync2d
	},
}

// ── Types ────────────────────────────────────────────────────────────────

// Candidate holds a single sync8 candidate detection result.
type Candidate struct {
	Freq      float64 // Hz
	DT        float64 // seconds (relative to nominal 0.5 s TX start)
	SyncPower float64 // normalized sync metric
}

// numWorkers returns the effective number of goroutines to use.
// workers > 0  = use that many goroutines explicitly (capped at 8).
// workers == 0 = serial (default, no concurrency).
// workers < 0  = auto-detect: min(4, runtime.NumCPU()) to avoid
//
//	hyper-threading overhead and excessive allocations.
func NumWorkers(workers int) int {
	if workers > 0 {
		if workers > 16 {
			return 16
		}
		return workers
	}
	if workers == 0 {
		return 1 // default: serial
	}
	n := runtime.NumCPU()
	if n > 16 {
		n = 16 // soft cap: diminishing returns beyond 12-16 on typical workloads
	}
	if n < 1 {
		n = 1
	}
	return n
}

// Spectrogram holds the power spectrogram s(freq, time) and its
// derived quantities, matching the Fortran local variables.
//
// Indices are 1-based to match Fortran: s[i][j] where
//
//	i = 1..NH1   (frequency bin, df = 3.125 Hz)
//	j = 1..NHSYM (time step, tstep = 0.04 s)
//
// Index 0 is allocated but unused.
type Spectrogram struct {
	S    [][]float64 // power: s[freq][time], 1-indexed, padded in freq
	Savg []float64   // average spectrum across all time steps, 1-indexed
}

// ── Sync8 — top-level entry point ────────────────────────────────────────

// Sync8 performs spectrogram-based candidate detection using the
// Costas-array sync pattern.
//
// Port of subroutine sync8(dd,npts,nfa,nfb,syncmin,nfqso,maxcand,candidate,ncand,sbase)
// from wsjt-wsjtx/lib/ft8/sync8.f90.
func Sync8(dd [ft8params.NMAX]float32, npts int, nfa, nfb int, syncmin float64, nfqso int, maxcand int, workers int) (candidates []Candidate, sbase [ft8params.NH1]float64) {

	// ── Derived constants (sync8.f90 lines 29–31) ────────────────────
	tstep := float64(ft8params.NSTEP) / ft8params.Fs // 0.04 s per spectrogram step
	df := ft8params.Fs / float64(ft8params.NFFT1)    // 3.125 Hz per frequency bin
	nssy := ft8params.NSPS / ft8params.NSTEP         // 4: spectrogram steps per symbol
	nfos := ft8params.NFFT1 / ft8params.NSPS         // 2: frequency oversampling factor
	jstrt := int(0.5 / tstep)                        // 12: 0.5 s offset in steps (Fortran truncation of 12.5)

	// ── Step 1: Compute spectrogram (sync8.f90 lines 28–43) ──────────
	// *** START HERE ***
	spec := computeSpectrogram(dd[:], npts, workers)
	minSpec, maxSpec := 1e9, -1e9
	for _, v := range spec.Savg {
		if v < minSpec {
			minSpec = v
		}
		if v > maxSpec {
			maxSpec = v
		}
	}

	// ── Step 1b: Spectrum baseline (sync8.f90 line 44) ───────────────
	sbase = getSpectrumBaseline(dd[:], nfa, nfb, workers)

	// ── Step 2: 2D Costas correlation (sync8.f90 lines 53–84) ────────
	sync2d := computeSync2D(spec, nfa, nfb, df, nssy, nfos, jstrt, workers)
	if sync2d != nil {
		defer sync2dPool.Put(sync2d)
	}

	// ── Step 3: Peak finding (sync8.f90 lines 86–98) ─────────────────
	jpeak, red, jpeak2, red2 := findPeaks(sync2d, nfa, nfb, df)

	// ── Step 4: 40th-percentile normalization (sync8.f90 lines 99–116)
	indx := normalizeByPercentile(red, red2, nfa, nfb, df)

	// ── Step 5: Extract pre-candidates (sync8.f90 lines 117–134) ─────
	preCands := extractPreCandidates(red, red2, jpeak, jpeak2, indx, nfa, nfb, df, tstep, syncmin)

	// ── Step 6: Near-dupe suppression (sync8.f90 lines 137–149) ──────
	suppressDuplicates(preCands)

	// ── Step 7: Sort + QSO prioritization (sync8.f90 lines 153–174) ──
	candidates = finalSort(preCands, syncmin, nfqso, maxcand)

	return candidates, sbase
}

// ── Step 1: Compute spectrogram ──────────────────────────────────────────
//
// sync8.f90 lines 28–43:
//
//	fac=1.0/300.0
//	do j=1,NHSYM
//	   ia=(j-1)*NSTEP + 1
//	   ib=ia+NSPS-1
//	   x(1:NSPS)=fac*dd(ia:ib)
//	   x(NSPS+1:)=0.
//	   call four2a(x,NFFT1,1,-1,0)           !r2c FFT
//	   do i=1,NH1
//	      s(i,j)=real(cx(i))**2 + aimag(cx(i))**2
//	   enddo
//	   savg=savg + s(1:NH1,j)
//	enddo
//
// For each of NHSYM=372 time steps:
//  1. Extract NSPS=1920 samples from dd, scale by fac=1/300
//  2. Zero-pad to NFFT1=3840 points
//  3. Real-to-complex FFT → NH1=1920 complex bins
//  4. Store power s(i,j) = re² + im²
//  5. Accumulate average spectrum savg
//
// specBackingPool reuses the large contiguous backing array for computeSpectrogram.
// Size: (nh1Pad+1)*cols float64 ≈ 5.5 MiB.
var specBackingPool = sync.Pool{New: func() interface{} {
	nfos := ft8params.NFFT1 / ft8params.NSPS
	nh1Pad := ft8params.NH1 + nfos*6 + 1
	cols := ft8params.NHSYM + 1
	return make([]float64, (nh1Pad+1)*cols)
}}

// specSavgPool reuses the average-spectrum slice for computeSpectrogram.
var specSavgPool = sync.Pool{New: func() interface{} {
	return make([]float64, ft8params.NH1+1)
}}

func computeSpectrogram(dd []float32, npts int, workers int) *Spectrogram {
	const fac float32 = 1.0 / 300.0 // sync8.f90 line 32

	nfos := ft8params.NFFT1 / ft8params.NSPS // 2
	nh1Pad := ft8params.NH1 + nfos*6 + 1
	cols := ft8params.NHSYM + 1
	backing := specBackingPool.Get().([]float64)
	// Zero the backing array (pool may return dirty memory).
	for i := range backing {
		backing[i] = 0
	}
	s := make([][]float64, nh1Pad+1)
	for i := range s {
		s[i] = backing[i*cols : (i+1)*cols]
	}
	savg := specSavgPool.Get().([]float64)
	for i := range savg {
		savg[i] = 0
	}

	if workers <= 1 {
		// Serial path
		buf := fftBufPool.Get().([]float32)
		buf = buf[:ft8params.NSPS]
		for j := 1; j <= ft8params.NHSYM; j++ {
			ia := (j - 1) * ft8params.NSTEP
			for k := 0; k < ft8params.NSPS; k++ {
				idx := ia + k
				if idx < npts {
					buf[k] = fac * dd[idx]
				} else {
					buf[k] = 0
				}
			}
			pow := dsp.SpectrogramFFT3840(buf)
			for i := 1; i <= ft8params.NH1; i++ {
				s[i][j] = pow[i-1]
				savg[i] += pow[i-1]
			}
		}
		fftBufPool.Put(buf[:cap(buf)])
		return &Spectrogram{S: s, Savg: savg}
	}

	nw := workers
	// Parallel path: split NHSYM time steps across workers.
	chunkSize := (ft8params.NHSYM + nw - 1) / nw
	var wg sync.WaitGroup
	localSavgs := make([][]float64, nw)
	for w := 0; w < nw; w++ {
		localSavgs[w] = make([]float64, ft8params.NH1+1)
	}

	for w := 0; w < nw; w++ {
		start := w*chunkSize + 1
		end := start + chunkSize - 1
		if start > ft8params.NHSYM {
			continue
		}
		if end > ft8params.NHSYM {
			end = ft8params.NHSYM
		}
		wg.Add(1)
		go func(id, jStart, jEnd int) {
			defer wg.Done()
			buf := fftBufPool.Get().([]float32)
			buf = buf[:ft8params.NSPS]
			for j := jStart; j <= jEnd; j++ {
				ia := (j - 1) * ft8params.NSTEP
				for k := 0; k < ft8params.NSPS; k++ {
					idx := ia + k
					if idx < npts {
						buf[k] = fac * dd[idx]
					} else {
						buf[k] = 0
					}
				}
				pow := dsp.SpectrogramFFT3840(buf)
				for i := 1; i <= ft8params.NH1; i++ {
					s[i][j] = pow[i-1]
					localSavgs[id][i] += pow[i-1]
				}
			}
			fftBufPool.Put(buf[:cap(buf)])
		}(w, start, end)
	}
	wg.Wait()

	for w := 0; w < nw; w++ {
		for i := 1; i <= ft8params.NH1; i++ {
			savg[i] += localSavgs[w][i]
		}
	}

	return &Spectrogram{S: s, Savg: savg}
}

// ComputeSpectrogramForTest exposes computeSpectrogram for testing.
func ComputeSpectrogramForTest(dd []float32, npts int, workers int) *Spectrogram {
	return computeSpectrogram(dd, npts, workers)
}

// ComputeSync2DForTest exposes computeSync2D for testing.
func ComputeSync2DForTest(spec *Spectrogram, nfa, nfb int, df float64, nssy, nfos, jstrt int, workers int) [][]float64 {
	return computeSync2D(spec, nfa, nfb, df, nssy, nfos, jstrt, workers)
}

// ── Step 1b: Spectrum baseline ───────────────────────────────────────────
//
// sync8.f90 line 44:
//
//	call get_spectrum_baseline(dd,nfa,nfb,sbase)
//
// Computes noise floor per frequency bin.  Used by ft8b for xsnr2, not
// by the candidate detection itself.
func getSpectrumBaseline(dd []float32, nfa, nfb int, workers int) [ft8params.NH1]float64 {
	const (
		NF  = 93
		NST = ft8params.NFFT1 / 2 // 960
	)

	window := nuttallWindow(ft8params.NFFT1)
	summ := 0.0
	for _, v := range window {
		summ += v
	}
	summ = summ * float64(ft8params.NSPS) * 2.0 / 300.0
	for i := range window {
		window[i] /= summ
	}

	savg := make([]float64, ft8params.NH1)

	if workers <= 1 {
		// Serial path
		for j := 0; j < NF; j++ {
			ia := j * NST
			ib := ia + ft8params.NFFT1 - 1
			if ib >= ft8params.NMAX {
				break
			}

			x := make([]float32, ft8params.NFFT1)
			for z := 0; z < ft8params.NFFT1; z++ {
				if ia+z < len(dd) {
					x[z] = float32(float64(dd[ia+z]) * window[z])
				}
			}

			pow := dsp.SpectrogramFFT3840(x)
			for i := 0; i < ft8params.NH1; i++ {
				savg[i] += pow[i]
			}
		}
	} else {
		// Parallel path: split NF segments across workers.
		nw := workers
		chunkSize := (NF + nw - 1) / nw
		localSavgs := make([][]float64, nw)
		for w := 0; w < nw; w++ {
			localSavgs[w] = make([]float64, ft8params.NH1)
		}
		var wg sync.WaitGroup
		for w := 0; w < nw; w++ {
			start := w * chunkSize
			end := start + chunkSize
			if start >= NF {
				continue
			}
			if end > NF {
				end = NF
			}
			wg.Add(1)
			go func(id, jStart, jEnd int) {
				defer wg.Done()
				x := fftBufPool.Get().([]float32)
				x = x[:ft8params.NFFT1]
				for j := jStart; j < jEnd; j++ {
					ia := j * NST
					ib := ia + ft8params.NFFT1 - 1
					if ib >= ft8params.NMAX {
						break
					}

					for z := 0; z < ft8params.NFFT1; z++ {
						if ia+z < len(dd) {
							x[z] = float32(float64(dd[ia+z]) * window[z])
						} else {
							x[z] = 0
						}
					}

					pow := dsp.SpectrogramFFT3840(x)
					for i := 0; i < ft8params.NH1; i++ {
						localSavgs[id][i] += pow[i]
					}
				}
				fftBufPool.Put(x[:cap(x)])
			}(w, start, end)
		}
		wg.Wait()
		for w := 0; w < nw; w++ {
			for i := 0; i < ft8params.NH1; i++ {
				savg[i] += localSavgs[w][i]
			}
		}
	}

	minS, maxS := 1e9, -1e9
	for _, v := range savg {
		if v < minS {
			minS = v
		}
		if v > maxS {
			maxS = v
		}
	}
	sbase := baseline(savg, nfa, nfb)
	var result [ft8params.NH1]float64
	copy(result[:], sbase)
	return result
}

// nuttallWindow generates a 4-term Nuttall window of length n.
func nuttallWindow(n int) []float64 {
	const (
		a0 = 0.3635819
		a1 = 0.4891775
		a2 = 0.1365995
		a3 = 0.0106411
	)
	w := make([]float64, n)
	for i := 0; i < n; i++ {
		f := 2.0 * math.Pi * float64(i) / float64(n-1)
		w[i] = a0 - a1*math.Cos(f) + a2*math.Cos(2*f) - a3*math.Cos(3*f)
	}
	return w
}

// baseline fits a low-order polynomial to the noise floor of a spectrum.
// Port of MSHV's baseline() (from decoderft8.cpp).
func baseline(s []float64, nfa, nfb int) []float64 {
	df := ft8params.Fs / float64(ft8params.NFFT1)
	ia := int(math.Max(0, float64(nfa)/df))
	ib := int(float64(nfb) / df)
	if ib >= len(s) {
		ib = len(s) - 1
	}
	if ia > ib {
		ia = ib
	}

	const nseg = 10
	const npct = 10

	for i := ia; i <= ib; i++ {
		if s[i] < 1e-18 {
			s[i] = 1e-18
		}
		s[i] = 10.0 * math.Log10(s[i])
	}

	nlen := (ib - ia) / nseg
	if nlen < 1 {
		nlen = 1
	}
	i0 := (ib - ia) / 2

	x := make([]float64, 0, 1000)
	y := make([]float64, 0, 1000)

	for n := 0; n < nseg; n++ {
		ja := ia + n*nlen
		jb := ja + nlen
		if jb > ib {
			jb = ib + 1
		}
		if ja >= jb {
			continue
		}
		seg := make([]float64, jb-ja)
		copy(seg, s[ja:jb])
		base := percentile(seg, float64(npct))
		for i := ja; i < jb; i++ {
			if s[i] <= base {
				x = append(x, float64(i-i0))
				y = append(y, s[i])
			}
		}
	}

	nterms := 5
	// Normalize x to [-1, 1] for numerical stability in polyfit.
	xscale := 1.0
	if len(x) > 0 {
		maxX := 0.0
		for _, v := range x {
			if math.Abs(v) > maxX {
				maxX = math.Abs(v)
			}
		}
		if maxX > 0 {
			xscale = maxX
		}
	}
	xNorm := make([]float64, len(x))
	for i, v := range x {
		xNorm[i] = v / xscale
	}
	minY, maxY, avgY := 1e9, -1e9, 0.0
	for _, v := range y {
		if v < minY {
			minY = v
		}
		if v > maxY {
			maxY = v
		}
		avgY += v
	}
	avgY /= float64(len(y))
	aNorm := polyfit(xNorm, y, nterms)

	sbase := make([]float64, len(s))
	for i := range sbase {
		sbase[i] = 0
	}
	for i := ia; i <= ib; i++ {
		t := float64(i-i0) / xscale
		sbase[i] = aNorm[0] + t*(aNorm[1]+t*(aNorm[2]+t*(aNorm[3]+t*aNorm[4]))) + 0.65
	}
	return sbase
}

// percentile returns the p-th percentile of data (0 <= p <= 100).
func percentile(data []float64, p float64) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	idx := int(math.Round(p / 100.0 * float64(len(sorted)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// polyfit performs a least-squares polynomial fit of degree (nterms-1).
// Returns coefficients a[0..nterms-1] for y = a0 + a1*x + a2*x^2 + ...
func polyfit(x, y []float64, nterms int) []float64 {
	n := len(x)
	if n < nterms {
		nterms = n
	}
	a := make([]float64, nterms)
	if nterms == 0 {
		return a
	}

	// Build normal equations: (V^T V) a = V^T y
	ata := make([][]float64, nterms)
	for i := range ata {
		ata[i] = make([]float64, nterms)
	}
	aty := make([]float64, nterms)

	for i := 0; i < n; i++ {
		xi := 1.0
		for j := 0; j < nterms; j++ {
			aty[j] += xi * y[i]
			xj := 1.0
			for k := 0; k < nterms; k++ {
				ata[j][k] += xi * xj
				xj *= x[i]
			}
			xi *= x[i]
		}
	}

	// Gaussian elimination with partial pivoting on augmented matrix [ata | aty]
	aug := make([][]float64, nterms)
	for i := range aug {
		aug[i] = make([]float64, nterms+1)
		copy(aug[i], ata[i])
		aug[i][nterms] = aty[i]
	}

	for col := 0; col < nterms; col++ {
		// Partial pivoting
		pivot := col
		maxVal := math.Abs(aug[col][col])
		for row := col + 1; row < nterms; row++ {
			if v := math.Abs(aug[row][col]); v > maxVal {
				maxVal = v
				pivot = row
			}
		}
		if maxVal < 1e-12 {
			continue
		}
		aug[col], aug[pivot] = aug[pivot], aug[col]

		// Normalize pivot row
		div := aug[col][col]
		for j := col; j <= nterms; j++ {
			aug[col][j] /= div
		}

		// Eliminate other rows
		for row := 0; row < nterms; row++ {
			if row == col {
				continue
			}
			factor := aug[row][col]
			if math.Abs(factor) < 1e-12 {
				continue
			}
			for j := col; j <= nterms; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	for i := 0; i < nterms; i++ {
		a[i] = aug[i][nterms]
	}
	return a
}

// ── Step 2: 2D Costas correlation ────────────────────────────────────────
//
// sync8.f90 lines 53–84:
//
// For each freq bin i in [ia..ib] and lag j in [-JZ..+JZ]:
//
//	Correlate spectrogram against three Costas arrays (a, b, c)
//	Compute ratio-metric sync: signal / noise
//	Store sync2d[i][j+jz] = max(sync_abc, sync_bc)
//
// Returns sync2d[0..NH1+pad][0..2*jz], offset by +jz in second index.
func computeSync2D(spec *Spectrogram, nfa, nfb int, df float64, nssy, nfos, jstrt int, workers int) [][]float64 {
	s := spec.S

	// sync8.f90 lines 46–47: frequency bin bounds
	iaFreq := int(math.Round(float64(nfa) / df)) // nint(nfa/df)
	if iaFreq < 1 {
		iaFreq = 1
	}
	ibFreq := int(math.Round(float64(nfb) / df)) // nint(nfb/df)
	// Clamp ibFreq so i+nfos*6 stays within padded s dimension.
	// No lower clamp on iaFreq is needed: the smallest Costas tone offset
	// is nfos*Icos7[3] = nfos*0 = 0, so i+0 = iaFreq ≥ 1 is always safe.
	// The largest offset is nfos*6 = 12, which is upward — handled here.
	if ibFreq+nfos*6 >= len(s) {
		ibFreq = len(s) - nfos*6 - 1
	}
	if ibFreq < iaFreq {
		return nil
	}

	nh1Pad := ft8params.NH1 + nfos*6 + 1
	lagCols := 2*jz + 1

	// Reuse pooled sync2d matrix (≈1.9 MB).  For standard FT8 (nfos=2) the
	// pool always has the right size; fall back to fresh allocation for
	// non-standard parameters (e.g. tests).
	var sync2d [][]float64
	if nfos == 2 {
		sync2d = sync2dPool.Get().([][]float64)
	} else {
		backing := make([]float64, (nh1Pad+1)*lagCols)
		sync2d = make([][]float64, nh1Pad+1)
		for i := range sync2d {
			sync2d[i] = backing[i*lagCols : (i+1)*lagCols]
		}
	}

	if workers <= 1 {
		// Serial path
		for i := iaFreq; i <= ibFreq; i++ {
			// Pre-compute row pointers for this freq bin, hoisted out of the
			// j loop.  sCos[n] = s[i + nfos*Icos7[n]] (Costas tone rows),
			// sNoise[k] = s[i + nfos*k] (noise-sum rows).  Eliminates
			// redundant index arithmetic across 125 lag × 7 tone iterations.
			var sCos [7][]float64
			var sNoise [7][]float64
			for n := 0; n < 7; n++ {
				sCos[n] = s[i+nfos*ft8params.Icos7[n]]
				sNoise[n] = s[i+nfos*n]
			}

			for j := -jz; j <= jz; j++ {
				var ta, tb, tc float64
				var t0a, t0b, t0c float64

				for n := 0; n <= 6; n++ {
					// sync8.f90 line 63: m = j + jstrt + nssy*n
					m := j + jstrt + nssy*n

					// ── Array a: first Costas (symbols 0–6) ──────────
					// sync8.f90 lines 64–67
					if m >= 1 && m <= ft8params.NHSYM {
						ta += sCos[n][m]
						for k := 0; k <= 6; k++ {
							t0a += sNoise[k][m]
						}
					}

					// ── Array b: second Costas (symbols 36–42) ───────
					// sync8.f90 lines 68–69 (no bounds check in Fortran)
					mb := m + nssy*36
					if mb >= 1 && mb <= ft8params.NHSYM {
						tb += sCos[n][mb]
						for k := 0; k <= 6; k++ {
							t0b += sNoise[k][mb]
						}
					}

					// ── Array c: third Costas (symbols 72–78) ────────
					// sync8.f90 lines 70–73
					mc := m + nssy*72
					if mc >= 1 && mc <= ft8params.NHSYM {
						tc += sCos[n][mc]
						for k := 0; k <= 6; k++ {
							t0c += sNoise[k][mc]
						}
					}
				}

				// sync8.f90 lines 75–78: ratio-metric sync for all three arrays
				t := ta + tb + tc
				t0 := t0a + t0b + t0c
				t0 = (t0 - t) / 6.0
				syncABC := 0.0
				if t0 > 0 {
					syncABC = t / t0
				}

				// sync8.f90 lines 79–82: ratio-metric sync for b+c only
				// (helps late-arriving signals where array a is clipped)
				t = tb + tc
				t0 = t0b + t0c
				t0 = (t0 - t) / 6.0
				syncBC := 0.0
				if t0 > 0 {
					syncBC = t / t0
				}

				// sync8.f90 line 83: sync2d(i,j) = max(sync_abc, sync_bc)
				if syncBC > syncABC {
					sync2d[i][j+jz] = syncBC
				} else {
					sync2d[i][j+jz] = syncABC
				}
			}
		}
	} else {
		// Parallel path: split frequency bins across workers.
		nw := workers
		total := ibFreq - iaFreq + 1
		chunkSize := (total + nw - 1) / nw
		if chunkSize < 1 {
			chunkSize = 1
		}
		var wg sync.WaitGroup
		for w := 0; w < nw; w++ {
			start := iaFreq + w*chunkSize
			end := start + chunkSize - 1
			if start > ibFreq {
				continue
			}
			if end > ibFreq {
				end = ibFreq
			}
			wg.Add(1)
			go func(iStart, iEnd int) {
				defer wg.Done()
				for i := iStart; i <= iEnd; i++ {
					var sCos [7][]float64
					var sNoise [7][]float64
					for n := 0; n < 7; n++ {
						sCos[n] = s[i+nfos*ft8params.Icos7[n]]
						sNoise[n] = s[i+nfos*n]
					}

					for j := -jz; j <= jz; j++ {
						var ta, tb, tc float64
						var t0a, t0b, t0c float64

						for n := 0; n <= 6; n++ {
							m := j + jstrt + nssy*n

							if m >= 1 && m <= ft8params.NHSYM {
								ta += sCos[n][m]
								for k := 0; k <= 6; k++ {
									t0a += sNoise[k][m]
								}
							}

							mb := m + nssy*36
							if mb >= 1 && mb <= ft8params.NHSYM {
								tb += sCos[n][mb]
								for k := 0; k <= 6; k++ {
									t0b += sNoise[k][mb]
								}
							}

							mc := m + nssy*72
							if mc >= 1 && mc <= ft8params.NHSYM {
								tc += sCos[n][mc]
								for k := 0; k <= 6; k++ {
									t0c += sNoise[k][mc]
								}
							}
						}

						t := ta + tb + tc
						t0 := t0a + t0b + t0c
						t0 = (t0 - t) / 6.0
						syncABC := 0.0
						if t0 > 0 {
							syncABC = t / t0
						}

						t = tb + tc
						t0 = t0b + t0c
						t0 = (t0 - t) / 6.0
						syncBC := 0.0
						if t0 > 0 {
							syncBC = t / t0
						}

						if syncBC > syncABC {
							sync2d[i][j+jz] = syncBC
						} else {
							sync2d[i][j+jz] = syncABC
						}
					}
				}
			}(start, end)
		}
		wg.Wait()
	}

	return sync2d
}

// ── Step 3: Peak finding ─────────────────────────────────────────────────
//
// sync8.f90 lines 86–98:
//
// For each freq bin i in [ia..ib]:
//
//	jpeak[i]  = lag of max sync2d within ±10 (narrow search)
//	red[i]    = sync2d at jpeak[i]
//	jpeak2[i] = lag of max sync2d within ±JZ (wide search)
//	red2[i]   = sync2d at jpeak2[i]
func findPeaks(sync2d [][]float64, nfa, nfb int, df float64) (jpeak []int, red []float64, jpeak2 []int, red2 []float64) {
	if sync2d == nil {
		return make([]int, ft8params.NH1+1), make([]float64, ft8params.NH1+1),
			make([]int, ft8params.NH1+1), make([]float64, ft8params.NH1+1)
	}

	// sync8.f90 lines 87–88: initialize to zero
	jpeak = make([]int, ft8params.NH1+1)
	red = make([]float64, ft8params.NH1+1)
	jpeak2 = make([]int, ft8params.NH1+1)
	red2 = make([]float64, ft8params.NH1+1)

	// sync8.f90 lines 46–47: recompute freq bin bounds (same as computeSync2D)
	iaFreq := int(math.Round(float64(nfa) / df))
	if iaFreq < 1 {
		iaFreq = 1
	}
	ibFreq := int(math.Round(float64(nfb) / df))

	// sync8.f90 lines 89–90
	mlag := 10  // narrow search ±10 lags (±0.4 s)
	mlag2 := jz // wide search ±62 lags (±2.48 s)

	// sync8.f90 lines 91–98
	for i := iaFreq; i <= ibFreq; i++ {
		if i >= len(sync2d) {
			break
		}

		// sync8.f90 line 92: ii = maxloc(sync2d(i,-mlag:mlag)) - 1 - mlag
		// Narrow search: find lag with max sync2d in [-mlag, +mlag]
		bestJ := -mlag
		bestV := sync2d[i][-mlag+jz]
		for lag := -mlag + 1; lag <= mlag; lag++ {
			if v := sync2d[i][lag+jz]; v > bestV {
				bestV = v
				bestJ = lag
			}
		}
		jpeak[i] = bestJ
		red[i] = bestV

		// sync8.f90 line 95: ii = maxloc(sync2d(i,-mlag2:mlag2)) - 1 - mlag2
		// Wide search: find lag with max sync2d in [-jz, +jz]
		bestJ2 := -mlag2
		bestV2 := sync2d[i][0] // sync2d[i][-jz + jz] = sync2d[i][0]
		for lag := -mlag2 + 1; lag <= mlag2; lag++ {
			if v := sync2d[i][lag+jz]; v > bestV2 {
				bestV2 = v
				bestJ2 = lag
			}
		}
		jpeak2[i] = bestJ2
		red2[i] = bestV2
	}

	return
}

// ── Step 4: 40th-percentile normalization ────────────────────────────────
//
// sync8.f90 lines 99–116:
//
// Sort red and red2, find the 40th percentile value as baseline,
// divide all values by it.  This normalizes so that syncmin thresholds
// are relative to the noise floor.
func normalizeByPercentile(red, red2 []float64, nfa, nfb int, df float64) []int {
	// sync8.f90 lines 46–47: frequency bin bounds
	ia := int(math.Round(float64(nfa) / df))
	if ia < 1 {
		ia = 1
	}
	ib := int(math.Round(float64(nfb) / df))
	if ib >= len(red) {
		ib = len(red) - 1
	}

	// sync8.f90 line 99: iz = ib - ia + 1
	iz := ib - ia + 1
	if iz < 1 {
		return nil
	}

	// sync8.f90 line 101: npctile = nint(0.40 * iz)
	npctile := int(math.Round(0.40 * float64(iz)))
	if npctile < 1 {
		// sync8.f90 lines 102–104: bail out
		return nil
	}

	// ── Normalize red ────────────────────────────────────────────────
	// sync8.f90 line 100: call indexx(red(ia:ib), iz, indx)
	// indexx returns ascending-order indices into red[ia..ib].
	indx := indexx(red, ia, ib)

	// sync8.f90 line 106: ibase = indx(npctile) - 1 + ia
	// indx is 0-based here (Go), so indx[npctile-1] is the npctile-th element.
	ibase := indx[npctile-1]
	if ibase < 1 {
		ibase = 1
	}
	if ibase > ft8params.NH1 {
		ibase = ft8params.NH1
	}

	// sync8.f90 lines 109–110: base = red(ibase); red = red / base
	// Only normalize the active range [ia..ib] — bins outside this range
	// are unused and normalizing them would be an invariant violation.
	base := red[ibase]
	if base > 0 {
		for i := ia; i <= ib; i++ {
			red[i] /= base
		}
	}

	// ── Normalize red2 ───────────────────────────────────────────────
	// sync8.f90 lines 111–116: same for red2
	indx2 := indexx(red2, ia, ib)

	ibase2 := indx2[npctile-1]
	if ibase2 < 1 {
		ibase2 = 1
	}
	if ibase2 > ft8params.NH1 {
		ibase2 = ft8params.NH1
	}

	base2 := red2[ibase2]
	if base2 > 0 {
		for i := ia; i <= ib; i++ {
			red2[i] /= base2
		}
	}

	return indx
}

// indexx returns indices that sort arr[ia..ib] in ascending order.
// The returned indices are absolute indices into arr (not relative to ia).
// This matches Fortran's indexx: indx(k) points to the k-th smallest
// element of arr(ia:ib), with indx values offset by (ia-1) so they
// reference arr directly.
func indexx(arr []float64, ia, ib int) []int {
	iz := ib - ia + 1
	if iz <= 0 {
		return nil
	}
	indx := make([]int, iz)
	for i := 0; i < iz; i++ {
		indx[i] = ia + i // absolute index into arr
	}
	sort.Slice(indx, func(a, b int) bool {
		return arr[indx[a]] < arr[indx[b]]
	})
	return indx
}

// ── Step 5: Extract pre-candidates ───────────────────────────────────────
//
// sync8.f90 lines 117–134:
//
// Walk frequency bins in descending sync order.
// For each bin where red[n] >= syncmin: emit candidate from narrow peak.
// If wide peak differs from narrow: emit second candidate from wide peak.
// Up to MAXPRECAND=1000 pre-candidates.
func extractPreCandidates(red, red2 []float64, jpeak, jpeak2 []int, indx []int, nfa, nfb int, df, tstep, syncmin float64) []Candidate {
	if indx == nil {
		return nil
	}

	iz := len(indx)
	cands := make([]Candidate, 0, maxPreCand)

	// sync8.f90 lines 117–134:
	// Walk indx in reverse (descending red order: strongest first).
	//   n = ia + indx(iz+1-i) - 1   →  in Go: indx[iz-i] (already absolute)
	limit := maxPreCand
	if iz < limit {
		limit = iz
	}

	for i := 1; i <= limit; i++ {
		if len(cands) >= maxPreCand {
			break
		}

		// sync8.f90 line 118: n = ia + indx(iz+1-i) - 1
		// Our indx already stores absolute indices, and iz+1-i maps to
		// Go's indx[iz-i] (descending order).
		n := indx[iz-i]

		// sync8.f90 lines 120–124: emit narrow-peak candidate if red >= syncmin
		if n >= 0 && n < len(red) && red[n] >= syncmin && !math.IsNaN(red[n]) {
			cands = append(cands, Candidate{
				Freq:      float64(n) * df,                   // candidate0(1,k) = n*df
				DT:        (float64(jpeak[n]) - 0.5) * tstep, // candidate0(2,k) = (jpeak(n)-0.5)*tstep
				SyncPower: red[n],                            // candidate0(3,k) = red(n)
			})
		}

		// sync8.f90 line 126: if(abs(jpeak2(n)-jpeak(n)).eq.0) cycle
		// Skip wide-peak candidate if it's at the same lag as narrow peak.
		if jpeak2[n] == jpeak[n] {
			continue
		}

		if len(cands) >= maxPreCand {
			break
		}

		// sync8.f90 lines 128–133: emit wide-peak candidate if red2 >= syncmin
		if n >= 0 && n < len(red2) && red2[n] >= syncmin && !math.IsNaN(red2[n]) {
			cands = append(cands, Candidate{
				Freq:      float64(n) * df,
				DT:        (float64(jpeak2[n]) - 0.5) * tstep,
				SyncPower: red2[n],
			})
		}
	}

	return cands
}

// ── Step 6: Near-dupe suppression ────────────────────────────────────────
//
// sync8.f90 lines 137–149:
//
// For any two candidates within 4 Hz and 0.04 s, zero out the weaker one.
func suppressDuplicates(cands []Candidate) {
	// sync8.f90 lines 138–149: O(n²) near-dupe suppression.
	// For any pair within 4 Hz and 0.04 s, zero out the weaker one's SyncPower.
	for i := 1; i < len(cands); i++ {
		for j := 0; j < i; j++ {
			fdiff := math.Abs(cands[i].Freq - cands[j].Freq)
			tdiff := math.Abs(cands[i].DT - cands[j].DT)
			if fdiff < 4.0 && tdiff < 0.04 {
				if cands[i].SyncPower >= cands[j].SyncPower {
					cands[j].SyncPower = 0
				} else {
					cands[i].SyncPower = 0
				}
			}
		}
	}
}

// ── Step 7: Sort + QSO-frequency prioritization ─────────────────────────
//
// sync8.f90 lines 153–174:
//
// 1. Place candidates within ±10 Hz of nfqso at the top.
// 2. Append the remaining in descending sync power order.
// 3. Cap at maxcand.
func finalSort(cands []Candidate, syncmin float64, nfqso, maxcand int) []Candidate {
	if len(cands) == 0 {
		return nil
	}

	// sync8.f90 line 154: call indexx(candidate0(3,1:ncand),ncand,indx)
	// Sort indices by SyncPower ascending (we'll walk in reverse for descending).
	indx := make([]int, len(cands))
	for i := range indx {
		indx[i] = i
	}
	sort.Slice(indx, func(a, b int) bool {
		return cands[indx[a]].SyncPower < cands[indx[b]].SyncPower
	})

	var out []Candidate
	if maxcand > 0 {
		out = make([]Candidate, 0, maxcand)
	}

	// sync8.f90 lines 156–162: place candidates within ±10 Hz of nfqso first.
	for i := 0; i < len(cands); i++ {
		if math.Abs(cands[i].Freq-float64(nfqso)) <= 10.0 && cands[i].SyncPower >= syncmin {
			out = append(out, cands[i])
			cands[i].SyncPower = 0 // mark as consumed
		}
	}

	// sync8.f90 lines 165–173: append remaining in descending sync order.
	for i := len(indx) - 1; i >= 0; i-- {
		j := indx[i]
		if cands[j].SyncPower >= syncmin {
			out = append(out, Candidate{
				Freq:      math.Abs(cands[j].Freq),
				DT:        cands[j].DT,
				SyncPower: cands[j].SyncPower,
			})
			if len(out) >= maxcand {
				break
			}
		}
	}

	return out
}
