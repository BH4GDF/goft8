package decode

import (
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestAPQSOProgress(t *testing.T) {
	// Verify Naptypes2 dispatch matches MSHV decoderft8.cpp.
	tests := []struct {
		prog     int
		apIdx    int
		wantType int
	}{
		{0, 0, 1}, // CQ
		{0, 1, 2}, // MyCall
		{1, 0, 2}, // MyCall
		{1, 1, 3}, // MyCall+DxCall
		{3, 0, 3}, // MyCall+DxCall
		{3, 1, 4}, // RRR
		{3, 2, 5}, // 73
		{3, 3, 6}, // RR73
		{5, 0, 3}, // MyCall+DxCall
		{5, 1, 1}, // CQ
		{5, 2, 2}, // MyCall
	}
	for _, tc := range tests {
		got := Naptypes2[tc.prog][tc.apIdx]
		if got != tc.wantType {
			t.Errorf("Naptypes2[%d][%d] = %d, want %d", tc.prog, tc.apIdx, got, tc.wantType)
		}
	}
}

func TestNappasses_2(t *testing.T) {
	want := [6]int{2, 2, 2, 4, 4, 3}
	if Nappasses2 != want {
		t.Errorf("Nappasses2 = %v, want %v", Nappasses2, want)
	}
}

func TestComputeAPSymbolsSentinelsAndKnownCalls(t *testing.T) {
	invalid := ComputeAPSymbols("K1", "W9XYZ")
	if invalid[0] != 99 || invalid[29] != 99 {
		t.Fatalf("invalid mycall sentinels = %d, %d", invalid[0], invalid[29])
	}

	unknownDX := ComputeAPSymbols("K1ABC", "")
	if unknownDX[0] == 99 {
		t.Fatal("valid mycall left apsym[0] sentinel")
	}
	if unknownDX[29] != 99 {
		t.Fatalf("missing hiscall sentinel = %d, want 99", unknownDX[29])
	}

	known := ComputeAPSymbols("K1ABC", "W9XYZ")
	if known[0] == 99 || known[29] == 99 {
		t.Fatalf("known calls left sentinels: %d, %d", known[0], known[29])
	}
	if known[28] != -1 || known[57] != -1 {
		t.Fatalf("ipa/ipb bits = %d/%d, want -1/-1", known[28], known[57])
	}
}

func TestApplyAPType1CQ(t *testing.T) {
	var llrz [ft8params.LDPCn]float64
	var apmask [ft8params.LDPCn]int8
	apsym := ComputeAPSymbols("K1ABC", "W9XYZ")

	ApplyAP(&llrz, &apmask, 1, apsym, 3.5, 0)

	for i := 0; i < 29; i++ {
		if apmask[i] != 1 {
			t.Fatalf("apmask[%d] = %d, want 1", i, apmask[i])
		}
		if llrz[i] != 3.5*float64(mcq[i]) {
			t.Fatalf("llrz[%d] = %g, want %g", i, llrz[i], 3.5*float64(mcq[i]))
		}
	}
	assertAPBits(t, llrz, apmask, 3.5, map[int]float64{74: -1, 75: -1, 76: +1})
}

func TestApplyAPType2ContestVariants(t *testing.T) {
	apsym := ComputeAPSymbols("K1ABC", "W9XYZ")

	tests := []struct {
		name        string
		contestType int
		wantBits    map[int]float64
		wantMask    []int
	}{
		{
			name:        "standard",
			contestType: 0,
			wantBits:    map[int]float64{74: -1, 75: -1, 76: +1},
			wantMask:    []int{0, 28, 74, 75, 76},
		},
		{
			name:        "eu vhf",
			contestType: 2,
			wantBits:    map[int]float64{71: -1, 72: +1, 73: -1, 74: -1, 75: -1, 76: -1},
			wantMask:    []int{0, 27, 71, 72, 73, 74, 75, 76},
		},
		{
			name:        "field day",
			contestType: 3,
			wantBits:    map[int]float64{74: -1, 75: -1, 76: -1},
			wantMask:    []int{0, 27, 74, 75, 76},
		},
		{
			name:        "rtty",
			contestType: 4,
			wantBits:    map[int]float64{74: -1, 75: +1, 76: +1},
			wantMask:    []int{1, 28, 74, 75, 76},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var llrz [ft8params.LDPCn]float64
			var apmask [ft8params.LDPCn]int8
			ApplyAP(&llrz, &apmask, 2, apsym, 2, tt.contestType)

			for _, idx := range tt.wantMask {
				if apmask[idx] != 1 {
					t.Fatalf("apmask[%d] = %d, want 1", idx, apmask[idx])
				}
			}
			assertAPBits(t, llrz, apmask, 2, tt.wantBits)
		})
	}
}

func TestApplyAPType3AndSignoffMessages(t *testing.T) {
	apsym := ComputeAPSymbols("K1ABC", "W9XYZ")

	var llrz [ft8params.LDPCn]float64
	var apmask [ft8params.LDPCn]int8
	ApplyAP(&llrz, &apmask, 3, apsym, 4, 0)
	if apmask[0] != 1 || apmask[57] != 1 || apmask[74] != 1 || apmask[76] != 1 {
		t.Fatalf("type3 mask missing expected AP bits")
	}
	assertAPBits(t, llrz, apmask, 4, map[int]float64{74: -1, 75: -1, 76: +1})

	for _, tc := range []struct {
		iaptype int
		msg     [19]int
	}{
		{4, mrrr},
		{5, m73},
		{6, mrr73},
	} {
		var signoffLLR [ft8params.LDPCn]float64
		var signoffMask [ft8params.LDPCn]int8
		ApplyAP(&signoffLLR, &signoffMask, tc.iaptype, apsym, 1.5, 0)
		for i := 0; i < 77; i++ {
			if signoffMask[i] != 1 {
				t.Fatalf("iaptype %d mask[%d] = %d, want 1", tc.iaptype, i, signoffMask[i])
			}
		}
		for i := 0; i < 19; i++ {
			want := 1.5 * float64(tc.msg[i])
			if signoffLLR[58+i] != want {
				t.Fatalf("iaptype %d llrz[%d] = %g, want %g", tc.iaptype, 58+i, signoffLLR[58+i], want)
			}
		}
	}
}

func assertAPBits(t *testing.T, llrz [ft8params.LDPCn]float64, apmask [ft8params.LDPCn]int8, apmag float64, want map[int]float64) {
	t.Helper()
	for idx, sign := range want {
		if apmask[idx] != 1 {
			t.Fatalf("apmask[%d] = %d, want 1", idx, apmask[idx])
		}
		if llrz[idx] != apmag*sign {
			t.Fatalf("llrz[%d] = %g, want %g", idx, llrz[idx], apmag*sign)
		}
	}
}
