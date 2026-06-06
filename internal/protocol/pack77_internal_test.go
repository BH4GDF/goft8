package protocol

import "testing"

func TestPack77RoundTripCommonMessageTypes(t *testing.T) {
	tests := []string{
		"CQ BH4GDF PM00",
		"BH4GDF BH4HKZ -10",
		"BH4GDF KL0I RR73",
		"HELLO WORLD",
		"123456789ABCDEF123",
	}

	for _, msg := range tests {
		got, ok := Pack77RoundTrip(msg)
		if !ok {
			t.Fatalf("Pack77RoundTrip(%q) failed", msg)
		}
		if got != msg {
			t.Fatalf("Pack77RoundTrip(%q) = %q", msg, got)
		}
	}
}

func TestC77ToBitsRejectsInvalidInput(t *testing.T) {
	if _, ok := C77ToBits("ABC"); ok {
		t.Fatal("short C77 input accepted")
	}
	if _, ok := C77ToBits("ZZZZZZZZZZZZZZZZZZZZ"); ok {
		t.Fatal("invalid hex C77 input accepted")
	}
}

func TestGridConversionValidation(t *testing.T) {
	if n, ok := grid4ToInt("PM00"); !ok || n < 0 {
		t.Fatalf("grid4ToInt(PM00) = %d, %v", n, ok)
	}
	if _, ok := grid4ToInt("ZZ99"); ok {
		t.Fatal("grid4ToInt accepted invalid field")
	}
	if n, ok := grid6ToInt("PM00AA"); !ok || n < 0 {
		t.Fatalf("grid6ToInt(PM00AA) = %d, %v", n, ok)
	}
	if _, ok := grid6ToInt("PM00ZZ"); ok {
		t.Fatal("grid6ToInt accepted invalid subsquare")
	}
}
