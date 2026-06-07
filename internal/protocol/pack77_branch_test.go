package protocol

import "testing"

func TestPack77ClassifiesMessageTypes(t *testing.T) {
	tests := []struct {
		msg    string
		wantI3 int
		wantN3 int
	}{
		{"123456789ABCDEF123", 0, 5},
		{"K1ABC RR73; W9XYZ <BH4GDF/P> -12", 0, 1},
		{"K1ABC W9XYZ 1A NTX", 0, 3},
		{"K1ABC W9XYZ R 17B DX", 0, 4},
		{"CQ BH4GDF PM00", 1, 0},
		{"BH4GDF/P K1ABC -10", 2, 0},
		{"TU; K1ABC W9XYZ R 599 0001", 3, 0},
		{"BH4GDF <BH4GDF/P> RR73", 4, 0},
		{"<BH4GDF/P> <K1ABC/P> R 520123 PM00AA", 5, 0},
		{"HELLO WORLD", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			_, gotI3, gotN3, ok := Pack77(tt.msg)
			if !ok {
				t.Fatalf("Pack77(%q) failed", tt.msg)
			}
			if gotI3 != tt.wantI3 || gotN3 != tt.wantN3 {
				t.Fatalf("Pack77(%q) type = (%d,%d), want (%d,%d)", tt.msg, gotI3, gotN3, tt.wantI3, tt.wantN3)
			}
		})
	}
}

func TestPack77RejectsInvalidMessages(t *testing.T) {
	tests := []string{
		"",
		"123456789ABCDEFXYZ",
		"K1ABC RR73 W9XYZ <BH4GDF/P> -12",
		"K1ABC W9XYZ 0A NTX",
		"K1ABC W9XYZ 33A NTX",
		"K1ABC W9XYZ 1A BAD",
		"CQ BH4GDF ZZ99",
		"BH4GDF K1ABC -99",
		"TU; K1ABC W9XYZ 599 8000",
		"TU; K1ABC W9XYZ 519 0001",
		"BH4GDF <BAD*CALL>",
		"<BH4GDF/P> <K1ABC/P> 519999 PM00AA",
		"<BH4GDF/P> <K1ABC/P> 520123 PM00ZZ",
		"FREE TEXT TOO LONG",
		"HELLO@WORLD",
	}

	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			if _, _, _, ok := Pack77(msg); ok {
				t.Fatalf("Pack77(%q) succeeded, want failure", msg)
			}
		})
	}
}

func TestUnpack77RejectsInvalidC77(t *testing.T) {
	tests := []string{
		"",
		"101",
		"0000000000000000000000000000000000000000000000000000000000000000000000000000",
		"000000000000000000000000000000000000000000000000000000000000000000000000000X",
	}

	for _, c77 := range tests {
		t.Run(c77, func(t *testing.T) {
			if _, ok := Unpack77(c77); ok {
				t.Fatalf("Unpack77(%q) succeeded, want failure", c77)
			}
		})
	}
}

func TestC77BitsRoundTrip(t *testing.T) {
	bits, _, _, ok := Pack77("CQ BH4GDF PM00")
	if !ok {
		t.Fatal("Pack77 failed")
	}

	c77 := BitsToC77(bits)
	got, ok := C77ToBits(c77)
	if !ok {
		t.Fatalf("C77ToBits(%q) failed", c77)
	}
	if got != bits {
		t.Fatal("C77ToBits(BitsToC77(bits)) mismatch")
	}
}

func TestHashTables(t *testing.T) {
	ResetHashTables()
	t.Cleanup(ResetHashTables)

	if got := Hash10(0); got != "<...>" {
		t.Fatalf("Hash10 before save = %q, want placeholder", got)
	}
	for _, invalid := range []int{-1, maxHash10, maxHash22} {
		if got := LookupHash10(invalid); got != "" {
			t.Fatalf("LookupHash10(%d) = %q, want empty", invalid, got)
		}
	}

	SaveHashCall("BH4GDF/P")
	n10 := HashCall("BH4GDF/P", 10)
	n12 := HashCall("BH4GDF/P", 12)
	n22 := HashCall("BH4GDF/P", 22)
	if got := Hash10(n10); got != "BH4GDF/P" {
		t.Fatalf("Hash10(%d) = %q", n10, got)
	}
	if got := Hash12(n12); got != "BH4GDF/P" {
		t.Fatalf("Hash12(%d) = %q", n12, got)
	}
	if got := Hash22(n22); got != "BH4GDF/P" {
		t.Fatalf("Hash22(%d) = %q", n22, got)
	}

	SaveHashCall("K1ABC")
	if got := Hash22(HashCall("K1ABC", 22)); got != "<...>" {
		t.Fatalf("standard call was saved in hash table: %q", got)
	}
}

func TestPack28AndUnpackHelpers(t *testing.T) {
	tests := []string{"DE", "QRZ", "CQ", "CQ_123", "CQ DX", "BH4GDF", "K1ABC"}
	for _, call := range tests {
		t.Run(call, func(t *testing.T) {
			n28 := Pack28(call)
			got, ok := unpack28(n28)
			if !ok {
				t.Fatalf("unpack28(Pack28(%q)) failed", call)
			}
			want := call
			if want == "CQ DX" {
				want = "CQ_DX"
			}
			if got != want {
				t.Fatalf("unpack28(Pack28(%q)) = %q, want %q", call, got, want)
			}
		})
	}

	if isStdCall("NOAREA") {
		t.Fatal("isStdCall accepted callsign without area digit")
	}
	if n := Pack28("BAD*CALL"); n < packNTOKENS || n >= packNTOKENS+packMAX22 {
		t.Fatalf("Pack28(non-standard) = %d, want 22-bit hash range", n)
	}
}

func TestGridAndWSPRHelpers(t *testing.T) {
	if got, ok := toGrid4(0); !ok || got != "AA00" {
		t.Fatalf("toGrid4(0) = %q, %v", got, ok)
	}
	if _, ok := toGrid4(maxgrid4 + 1); ok {
		t.Fatal("toGrid4 accepted out-of-range grid")
	}
	if got, ok := toGrid6(0); !ok || got != "AA00AA" {
		t.Fatalf("toGrid6(0) = %q, %v", got, ok)
	}
	if _, ok := toGrid6(18662400); ok {
		t.Fatal("toGrid6 accepted out-of-range grid")
	}
	if got, ok := toGrid(0); !ok || got != "AA00AA" {
		t.Fatalf("toGrid(0) = %q, %v", got, ok)
	}
	if _, ok := toGrid(18 * 18 * 10 * 10 * 25 * 25); ok {
		t.Fatal("toGrid accepted out-of-range grid")
	}
	if got := wspr2Prefix(35); got != "Z" {
		t.Fatalf("wspr2Prefix(35) = %q, want Z", got)
	}
	if got, ok := wspr2Suffix(12959); !ok || got != "ZZ9" {
		t.Fatalf("wspr2Suffix(12959) = %q, %v", got, ok)
	}
	if _, ok := wspr2Suffix(12960); ok {
		t.Fatal("wspr2Suffix accepted out-of-range suffix")
	}
}
