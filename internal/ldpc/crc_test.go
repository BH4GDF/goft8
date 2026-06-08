package ldpc

import (
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestComputeCRC14IsDeterministic(t *testing.T) {
	var bits [77]int8
	for i := range bits {
		if i%3 == 0 {
			bits[i] = 1
		}
	}

	got := ComputeCRC14(bits)
	if got != ComputeCRC14(bits) {
		t.Fatal("ComputeCRC14 returned non-deterministic result")
	}
	if got >= 1<<14 {
		t.Fatalf("CRC = %d, want 14-bit value", got)
	}
}

func TestCRC14CodewordRoundTrip(t *testing.T) {
	var msg [77]int8
	for i := range msg {
		if i%5 == 0 || i%7 == 0 {
			msg[i] = 1
		}
	}

	crc := ComputeCRC14(msg)
	var message91 [ft8params.LDPCk]int8
	copy(message91[:77], msg[:])
	for i := 0; i < 14; i++ {
		message91[77+i] = int8((crc >> uint(13-i)) & 1)
	}

	cw := EncodeLDPCNoCRC(message91)
	if !checkCRC14Codeword(cw) {
		t.Fatal("encoded codeword failed CRC check")
	}

	cw[0] ^= 1
	if checkCRC14Codeword(cw) {
		t.Fatal("corrupted codeword passed CRC check")
	}
}
