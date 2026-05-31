package goft8

import (
	"testing"

	"github.com/bh4gdf/goft8/internal/encode"
	"github.com/bh4gdf/goft8/internal/protocol"
)

func BenchmarkGenFT8CWave(b *testing.B) {
	msg := "CQ BH4GDF PM00"
	bits, _, _, ok := protocol.Pack77(msg)
	if !ok {
		b.Fatal("pack failed")
	}
	itone := encode.GenFT8Tones(bits)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encode.GenFT8CWave(itone, 1500)
	}
}

func BenchmarkEncoderEncode(b *testing.B) {
	enc := NewEncoder(WithTxFreq(1500))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enc.Encode("CQ BH4GDF PM00")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncoderEncodeMulti2(b *testing.B) {
	enc := NewEncoder()
	msgs := []MessageFreq{
		{Message: "CQ BH4GDF PM00", Freq: 1200},
		{Message: "BH4GDF BH4HKZ -10", Freq: 1800},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enc.EncodeMulti(msgs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
