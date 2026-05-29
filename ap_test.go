package goft8

import "testing"

func TestAPQSOProgress(t *testing.T) {
	// Verify naptypes_2 dispatch matches MSHV decoderft8.cpp.
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
		got := naptypes_2[tc.prog][tc.apIdx]
		if got != tc.wantType {
			t.Errorf("naptypes_2[%d][%d] = %d, want %d", tc.prog, tc.apIdx, got, tc.wantType)
		}
	}
}

func TestNappasses_2(t *testing.T) {
	want := [6]int{2, 2, 2, 4, 4, 3}
	if nappasses_2 != want {
		t.Errorf("nappasses_2 = %v, want %v", nappasses_2, want)
	}
}
