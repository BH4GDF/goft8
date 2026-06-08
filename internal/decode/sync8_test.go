package decode

import (
	"math"
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestNumWorkersBounds(t *testing.T) {
	if got := NumWorkers(0); got != 1 {
		t.Fatalf("NumWorkers(0) = %d, want serial 1", got)
	}
	if got := NumWorkers(3); got != 3 {
		t.Fatalf("NumWorkers(3) = %d, want 3", got)
	}
	if got := NumWorkers(99); got != 16 {
		t.Fatalf("NumWorkers(99) = %d, want cap 16", got)
	}
	if got := NumWorkers(-1); got < 1 || got > 16 {
		t.Fatalf("NumWorkers(-1) = %d, want 1..16", got)
	}
}

func TestIndexxSortsAbsoluteRange(t *testing.T) {
	arr := []float64{99, 3, 1, 2, 88}

	got := indexx(arr, 1, 3)

	want := []int{2, 3, 1}
	assertIntSlice(t, got, want)
}

func TestNormalizeByPercentileNormalizesActiveRange(t *testing.T) {
	df := 10.0
	red := []float64{100, 2, 4, 6, 8, 200}
	red2 := []float64{100, 5, 10, 15, 20, 200}

	indx := normalizeByPercentile(red, red2, 10, 40, df)

	assertIntSlice(t, indx, []int{1, 2, 3, 4})
	assertFloatApprox(t, red[1], 0.5, 1e-12)
	assertFloatApprox(t, red[4], 2, 1e-12)
	assertFloatApprox(t, red2[1], 0.5, 1e-12)
	assertFloatApprox(t, red2[4], 2, 1e-12)
	if red[0] != 100 || red[5] != 200 {
		t.Fatalf("inactive red bins changed: %v", red)
	}
}

func TestExtractPreCandidatesIncludesWideLagWhenDistinct(t *testing.T) {
	df := 3.125
	tstep := 0.04
	red := make([]float64, 6)
	red2 := make([]float64, 6)
	jpeak := make([]int, 6)
	jpeak2 := make([]int, 6)
	red[2] = 7
	red2[2] = 8
	jpeak[2] = 3
	jpeak2[2] = 5
	red[4] = math.NaN()
	red2[4] = 9
	jpeak[4] = 1
	jpeak2[4] = 1

	got := extractPreCandidates(red, red2, jpeak, jpeak2, []int{2, 4}, 0, 0, df, tstep, 6)

	if len(got) != 2 {
		t.Fatalf("candidates = %d, want 2: %#v", len(got), got)
	}
	assertFloatApprox(t, got[0].Freq, 2*df, 1e-12)
	assertFloatApprox(t, got[0].DT, (3-0.5)*tstep, 1e-12)
	assertFloatApprox(t, got[0].SyncPower, 7, 1e-12)
	assertFloatApprox(t, got[1].Freq, 2*df, 1e-12)
	assertFloatApprox(t, got[1].DT, (5-0.5)*tstep, 1e-12)
	assertFloatApprox(t, got[1].SyncPower, 8, 1e-12)
}

func TestSuppressDuplicatesKeepsStrongerCandidate(t *testing.T) {
	cands := []Candidate{
		{Freq: 1000, DT: 0.10, SyncPower: 5},
		{Freq: 1002, DT: 0.12, SyncPower: 7},
		{Freq: 1010, DT: 0.10, SyncPower: 4},
	}

	suppressDuplicates(cands)

	if cands[0].SyncPower != 0 {
		t.Fatalf("weaker duplicate sync = %g, want 0", cands[0].SyncPower)
	}
	if cands[1].SyncPower != 7 || cands[2].SyncPower != 4 {
		t.Fatalf("non-duplicates changed: %#v", cands)
	}
}

func TestFinalSortPrioritizesQSOAndCaps(t *testing.T) {
	cands := []Candidate{
		{Freq: 1500, DT: 0.1, SyncPower: 7},
		{Freq: 2100, DT: 0.2, SyncPower: 9},
		{Freq: 1494, DT: 0.3, SyncPower: 6},
		{Freq: -1800, DT: 0.4, SyncPower: 8},
	}

	got := finalSort(cands, 5, 1500, 3)

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %#v", len(got), got)
	}
	assertFloatApprox(t, got[0].Freq, 1500, 1e-12)
	assertFloatApprox(t, got[1].Freq, 1494, 1e-12)
	assertFloatApprox(t, got[2].Freq, 2100, 1e-12)
}

func TestFindPeaksIntoHandlesNilAndWidePeak(t *testing.T) {
	df := 10.0
	jpeak := make([]int, ft8params.NH1+1)
	red := make([]float64, ft8params.NH1+1)
	jpeak2 := make([]int, ft8params.NH1+1)
	red2 := make([]float64, ft8params.NH1+1)

	findPeaksInto(jpeak, red, jpeak2, red2, nil, 10, 20, df)
	if red[1] != 0 || red2[1] != 0 {
		t.Fatalf("nil sync2d changed outputs")
	}

	sync2d := make([][]float64, 4)
	for i := range sync2d {
		sync2d[i] = make([]float64, 2*jz+1)
	}
	sync2d[1][2+jz] = 3
	sync2d[1][20+jz] = 5
	findPeaksInto(jpeak, red, jpeak2, red2, sync2d, 10, 10, df)

	if jpeak[1] != 2 || red[1] != 3 {
		t.Fatalf("narrow peak = lag %d red %g, want 2/3", jpeak[1], red[1])
	}
	if jpeak2[1] != 20 || red2[1] != 5 {
		t.Fatalf("wide peak = lag %d red %g, want 20/5", jpeak2[1], red2[1])
	}
}

func TestBaselineAndMathHelpers(t *testing.T) {
	if minInt(3, 7) != 3 || minInt(9, 4) != 4 {
		t.Fatal("minInt returned wrong value")
	}

	data := []float64{5, 1, 9, 3}
	if got := percentile(data, 50); got != 5 {
		t.Fatalf("percentile = %g, want 5", got)
	}
	if got := percentileInPlace([]float64{5, 1, 9, 3}, -10); got != 1 {
		t.Fatalf("low percentile = %g, want 1", got)
	}
	if got := percentileInPlace([]float64{5, 1, 9, 3}, 110); got != 9 {
		t.Fatalf("high percentile = %g, want 9", got)
	}

	x := []float64{-1, 0, 1, 2, 3}
	y := []float64{1, 2, 3, 4, 5}
	coef := polyfit5(x, y, 3)
	assertFloatApprox(t, coef[0], 2, 1e-9)
	assertFloatApprox(t, coef[1], 1, 1e-9)

	spec := make([]float64, 80)
	for i := range spec {
		spec[i] = 1 + float64(i%5)
	}
	got := baseline(append([]float64(nil), spec...), 0, 200)
	if len(got) != len(spec) {
		t.Fatalf("baseline len = %d, want %d", len(got), len(spec))
	}
	nonZero := false
	for _, v := range got {
		if v != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("baseline produced all zeros")
	}
}

func assertIntSlice(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("value[%d] = %d, want %d (got %v)", i, got[i], want[i], got)
		}
	}
}

func assertFloatApprox(t *testing.T, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("got %.17g, want %.17g (tol %g)", got, want, tol)
	}
}
