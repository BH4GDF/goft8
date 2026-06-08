package ldpc

import (
	"testing"

	ft8params "github.com/bh4gdf/goft8/params"
)

var (
	benchmarkLDPCResult DecodeResult
	benchmarkLDPCOK     bool
	benchmarkMessage91  [ft8params.LDPCk]int8
	benchmarkCodeword   [ft8params.LDPCn]int8
)

func benchmarkLLR(strength float64) [ft8params.LDPCn]float64 {
	vector := testLDPCVector(strength)
	benchmarkMessage91 = vector.message91
	benchmarkCodeword = vector.codeword
	return vector.llr
}

func BenchmarkDecodeLDPCCleanBP(b *testing.B) {
	llr := benchmarkLLR(4.0)
	var apmask [ft8params.LDPCn]int8
	benchmarkLDPCResult, benchmarkLDPCOK = DecodeLDPC(llr, ft8params.LDPCk, -1, 0, apmask)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkLDPCResult, benchmarkLDPCOK = DecodeLDPC(llr, ft8params.LDPCk, -1, 0, apmask)
		if !benchmarkLDPCOK {
			b.Fatal("DecodeLDPC failed for clean codeword")
		}
	}
}

func BenchmarkDecodeLDPCF32CleanBP(b *testing.B) {
	llr := benchmarkLLR(4.0)
	var apmask [ft8params.LDPCn]int8
	benchmarkLDPCResult, benchmarkLDPCOK = DecodeLDPCF32(llr, ft8params.LDPCk, -1, 0, apmask)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkLDPCResult, benchmarkLDPCOK = DecodeLDPCF32(llr, ft8params.LDPCk, -1, 0, apmask)
		if !benchmarkLDPCOK {
			b.Fatal("DecodeLDPCF32 failed for clean codeword")
		}
	}
}

func BenchmarkOSDDecodeOrder0Clean(b *testing.B) {
	llr := benchmarkLLR(4.0)
	var apmask [ft8params.LDPCn]int8
	benchmarkMessage91, benchmarkCodeword, benchmarkLDPCResult.NHardErrors, benchmarkLDPCOK = osdDecode(llr, ft8params.LDPCk, apmask, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg, cw, nhard, ok := osdDecode(llr, ft8params.LDPCk, apmask, 0)
		if !ok {
			b.Fatal("osdDecode order-0 failed for clean codeword")
		}
		benchmarkMessage91 = msg
		benchmarkCodeword = cw
		benchmarkLDPCResult.NHardErrors = nhard
	}
}

func BenchmarkOSDDecodeOrder3Clean(b *testing.B) {
	llr := benchmarkLLR(4.0)
	var apmask [ft8params.LDPCn]int8
	benchmarkMessage91, benchmarkCodeword, benchmarkLDPCResult.NHardErrors, benchmarkLDPCOK = osdDecode(llr, ft8params.LDPCk, apmask, 3)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg, cw, nhard, ok := osdDecode(llr, ft8params.LDPCk, apmask, 3)
		if !ok {
			b.Fatal("osdDecode order-3 failed for clean codeword")
		}
		benchmarkMessage91 = msg
		benchmarkCodeword = cw
		benchmarkLDPCResult.NHardErrors = nhard
	}
}
