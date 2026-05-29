package goft8

import (
	"testing"
)

func TestPack77RoundTrip(t *testing.T) {
	tests := []struct {
		msg    string
		want   string
		wantOK bool
	}{
		// Standard messages (i3=1)
		{"CQ BH4GDF PM00", "CQ BH4GDF PM00", true},
		{"BH4GDF BH4ABC PM00", "BH4GDF BH4ABC PM00", true},
		{"BH4GDF BH4ABC -10", "BH4GDF BH4ABC -10", true},
		{"BH4GDF BH4ABC +05", "BH4GDF BH4ABC +05", true},
		{"BH4GDF BH4ABC R-10", "BH4GDF BH4ABC R-10", true},
		{"BH4GDF BH4ABC R +10", "BH4GDF BH4ABC R+10", true},
		{"BH4GDF BH4ABC RRR", "BH4GDF BH4ABC RRR", true},
		{"BH4GDF BH4ABC RR73", "BH4GDF BH4ABC RR73", true},
		{"BH4GDF BH4ABC 73", "BH4GDF BH4ABC 73", true},
		{"CQ W1ABC FN20", "CQ W1ABC FN20", true},
		{"W1ABC K1ABC -12", "W1ABC K1ABC -12", true},
		// R + Grid (critical bug fix from MSHV review)
		{"BH4GDF BH4ABC R PM00", "BH4GDF BH4ABC R PM00", true},
		{"W1ABC K1ABC R FN20", "W1ABC K1ABC R FN20", true},
		// No third part
		{"BH4GDF BH4ABC", "BH4GDF BH4ABC", true},
		// Free text
		{"TEST 123", "TEST 123", true},
		{"HELLO WORLD", "HELLO WORLD", true},
		// Non-standard callsign (i3=4) — hash tables not maintained,
		// so non-standard calls decode as "<...>"
		{"BH4GDF <BH4ABC/P>", "<...> BH4ABC/P", true},
		{"<BH4GDF/P> BH4ABC", "BH4GDF/P <...>", true},
		{"CQ <BH4GDF/P>", "CQ BH4GDF/P", true},
		// Invalid formats
		{"HELLO WORLD 123", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got, ok := Pack77RoundTrip(tt.msg)
			if ok != tt.wantOK {
				t.Errorf("Pack77RoundTrip(%q) ok=%v, want %v", tt.msg, ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}
			if got != tt.want {
				t.Errorf("Pack77RoundTrip(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestPack77FreeText(t *testing.T) {
	tests := []string{
		"TEST 123",
		"HELLO WORLD",
		"CQ DX",
	}
	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			got, ok := Pack77RoundTrip(msg)
			if !ok {
				t.Errorf("Pack77RoundTrip(%q) failed", msg)
				return
			}
			if got != msg {
				t.Errorf("Pack77RoundTrip(%q) = %q, want %q", msg, got, msg)
			}
		})
	}
}

func TestPack77RGridBug(t *testing.T) {
	// Regression test for the R+Grid encoding bug found in MSHV review.
	// Before fix: "CALL1 CALL2 R FN42" was encoded with ir=0, losing the R.
	msg := "W1ABC K1ABC R FN20"
	bits, _, _, ok := Pack77(msg)
	if !ok {
		t.Fatalf("Pack77 failed for %q", msg)
	}
	c77 := BitsToC77(bits)
	// Verify ir bit (position 58) is 1.
	if c77[58] != '1' {
		t.Errorf("R+Grid message %q encoded with ir=%c, want 1", msg, c77[58])
	}
	// Verify round-trip.
	unpacked, ok2 := Unpack77(c77)
	if !ok2 {
		t.Fatalf("Unpack77 failed for %q", msg)
	}
	if unpacked != msg {
		t.Errorf("round-trip failed: %q -> %q", msg, unpacked)
	}
}
