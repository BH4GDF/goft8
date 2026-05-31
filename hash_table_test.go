package goft8

import (
	"testing"

	"github.com/bh4gdf/goft8/internal/protocol"
)

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
			n10 := protocol.HashCall(tc.callsign, 10) & 0x3FF
			n12 := protocol.HashCall(tc.callsign, 12) & 0xFFF
			n22 := protocol.HashCall(tc.callsign, 22) & 0x3FFFFF
			protocol.SaveHashCall(tc.callsign)
			if got := protocol.Hash10(n10); got != tc.callsign {
				t.Errorf("Hash10(%d) = %q, want %q", n10, got, tc.callsign)
			}
			if got := protocol.Hash12(n12); got != tc.callsign {
				t.Errorf("Hash12(%d) = %q, want %q", n12, got, tc.callsign)
			}
			if got := protocol.Hash22(n22); got != tc.callsign {
				t.Errorf("Hash22(%d) = %q, want %q", n22, got, tc.callsign)
			}
		})
	}
}
