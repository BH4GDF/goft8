package ldpc

import (
	"math"
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

func TestDecodeLDPCStrongCodeword(t *testing.T) {
	vector := testLDPCVector(4.0)
	var apmask [ft8params.LDPCn]int8

	got, ok := DecodeLDPC(vector.llr, ft8params.LDPCk, -1, 0, apmask)
	if !ok {
		t.Fatal("DecodeLDPC failed for strong valid codeword")
	}
	assertLDPCDecode(t, got, vector, 1, 0, 0)
}

func TestDecodeLDPCF32StrongCodeword(t *testing.T) {
	vector := testLDPCVector(4.0)
	var apmask [ft8params.LDPCn]int8

	got, ok := DecodeLDPCF32(vector.llr, ft8params.LDPCk, -1, 0, apmask)
	if !ok {
		t.Fatal("DecodeLDPCF32 failed for strong valid codeword")
	}
	assertLDPCDecode(t, got, vector, 1, 0, 0)
}

func TestDecodeLDPCRejectsBadCRCCodeword(t *testing.T) {
	vector := testLDPCVector(4.0)
	var badMessage [ft8params.LDPCk]int8
	copy(badMessage[:], vector.message91[:])
	badMessage[0] ^= 1

	cw := EncodeLDPCNoCRC(badMessage)
	llr := llrFromCodeword(cw, 4.0)
	var apmask [ft8params.LDPCn]int8

	got, ok := DecodeLDPC(llr, ft8params.LDPCk, -1, 0, apmask)
	if ok {
		t.Fatalf("DecodeLDPC accepted codeword with invalid embedded CRC: %+v", got)
	}
	if got.NHardErrors != -1 {
		t.Fatalf("NHardErrors = %d, want -1 on failure", got.NHardErrors)
	}
}

func TestOSDDecodeRepairsHardDecisionError(t *testing.T) {
	vector := testLDPCVector(4.0)
	llr := vector.llr
	llr[0] = -0.2
	var apmask [ft8params.LDPCn]int8

	msg, cw, nHard, ok := osdDecode(llr, ft8params.LDPCk, apmask, 0)
	if !ok {
		t.Fatal("osdDecode order-0 failed to repair one unreliable hard decision")
	}
	if msg != vector.message91 {
		t.Fatalf("message mismatch after OSD repair")
	}
	if cw != vector.codeword {
		t.Fatalf("codeword mismatch after OSD repair")
	}
	if nHard != 1 {
		t.Fatalf("nHard = %d, want 1", nHard)
	}
}

func TestDecodeLDPCRepairsWeakHardDecision(t *testing.T) {
	vector := testLDPCVector(4.0)
	llr := vector.llr
	llr[0] = -0.2
	var apmask [ft8params.LDPCn]int8

	got, ok := DecodeLDPC(llr, ft8params.LDPCk, -1, 0, apmask)
	if !ok {
		t.Fatal("DecodeLDPC failed to repair one unreliable hard decision")
	}
	assertLDPCDecode(t, got, vector, 1, 1, 0.2)
}

func TestOSDDecodeOrder3CleanCodeword(t *testing.T) {
	vector := testLDPCVector(4.0)
	var apmask [ft8params.LDPCn]int8

	msg, cw, nHard, ok := osdDecode(vector.llr, ft8params.LDPCk, apmask, 3)
	if !ok {
		t.Fatal("osdDecode order-3 failed for strong valid codeword")
	}
	if msg != vector.message91 {
		t.Fatalf("message mismatch")
	}
	if cw != vector.codeword {
		t.Fatalf("codeword mismatch")
	}
	if nHard != 0 {
		t.Fatalf("nHard = %d, want 0", nHard)
	}
}

func TestArgsortAscIntoSortsLargeInput(t *testing.T) {
	arr := make([]float64, 2048)
	for i := range arr {
		arr[i] = math.Sin(float64(i)*17.0) + math.Cos(float64(i)*3.0)
	}
	indx := make([]int, len(arr))

	got := argsortAscInto(indx, arr)

	if len(got) != len(arr) {
		t.Fatalf("len = %d, want %d", len(got), len(arr))
	}
	if len(got) > 0 && &got[0] != &indx[0] {
		t.Fatal("argsortAscInto did not reuse provided index buffer")
	}
	seen := make([]bool, len(arr))
	for i, idx := range got {
		if idx < 0 || idx >= len(arr) {
			t.Fatalf("index[%d] = %d out of range", i, idx)
		}
		if seen[idx] {
			t.Fatalf("index[%d] = %d appears more than once", i, idx)
		}
		seen[idx] = true
		if i > 0 && arr[got[i-1]] > arr[idx] {
			t.Fatalf("order violation at %d: %g > %g", i, arr[got[i-1]], arr[idx])
		}
	}
}

type ldpcTestVector struct {
	message91 [ft8params.LDPCk]int8
	codeword  [ft8params.LDPCn]int8
	llr       [ft8params.LDPCn]float64
}

func testLDPCVector(strength float64) ldpcTestVector {
	var msg [77]int8
	for i := range msg {
		if i%5 == 0 || i%11 == 0 {
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
	return ldpcTestVector{
		message91: message91,
		codeword:  cw,
		llr:       llrFromCodeword(cw, strength),
	}
}

func llrFromCodeword(cw [ft8params.LDPCn]int8, strength float64) [ft8params.LDPCn]float64 {
	var llr [ft8params.LDPCn]float64
	for i, bit := range cw {
		if bit == 1 {
			llr[i] = strength
		} else {
			llr[i] = -strength
		}
	}
	return llr
}

func assertLDPCDecode(t *testing.T, got DecodeResult, want ldpcTestVector, decoderType, nHard int, dmin float64) {
	t.Helper()

	if got.Message91 != want.message91 {
		t.Fatalf("Message91 mismatch")
	}
	if got.Codeword != want.codeword {
		t.Fatalf("Codeword mismatch")
	}
	if got.DecoderType != decoderType {
		t.Fatalf("DecoderType = %d, want %d", got.DecoderType, decoderType)
	}
	if got.NHardErrors != nHard {
		t.Fatalf("NHardErrors = %d, want %d", got.NHardErrors, nHard)
	}
	if math.Abs(got.Dmin-dmin) > 1e-6 {
		t.Fatalf("Dmin = %g, want %g", got.Dmin, dmin)
	}
}
