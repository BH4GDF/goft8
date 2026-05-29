package goft8

import "testing"

func TestHashTableRoundTrip(t *testing.T) {
	tests := []struct {
		callsign string
	}{
		{"BH4GDF/P"},
		{"3DA0RS"},
		{"XX0YY/TEST"},
		{"<GB100RSGB>"},
		{"M0AAA/P"},
	}
	for _, tc := range tests {
		t.Run(tc.callsign, func(t *testing.T) {
			n10 := hashCall(tc.callsign, 10) & 0x3FF
			n12 := hashCall(tc.callsign, 12) & 0xFFF
			n22 := hashCall(tc.callsign, 22) & 0x3FFFFF
			SaveHashCall(tc.callsign)
			if got := Hash10(n10); got != tc.callsign {
				t.Errorf("Hash10(%d) = %q, want %q", n10, got, tc.callsign)
			}
			if got := Hash12(n12); got != tc.callsign {
				t.Errorf("Hash12(%d) = %q, want %q", n12, got, tc.callsign)
			}
			if got := Hash22(n22); got != tc.callsign {
				t.Errorf("Hash22(%d) = %q, want %q", n22, got, tc.callsign)
			}
		})
	}
}
