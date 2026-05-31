package goft8

import (
	"testing"
)

func TestDecodeStreamCallbackOrder(t *testing.T) {
	audio, err := ReadWAVMono12k("testdata/ft8_cap4.wav")
	if err != nil {
		t.Skipf("missing test fixture: %v", err)
	}
	if len(audio) > AudioSamplesPerCycle {
		audio = audio[:AudioSamplesPerCycle]
	}

	var callbacks []string
	dec := NewDecoder(WithFreqRange(100, 3000))

	results, err := dec.DecodeStream(audio, func(d Decoded) {
		callbacks = append(callbacks, d.Message)
		t.Logf("callback: pass=%d msg=%q", d.Pass, d.Message)
	})
	if err != nil {
		t.Fatalf("decode stream failed: %v", err)
	}

	if len(callbacks) == 0 {
		t.Fatal("expected callbacks")
	}
	if len(callbacks) != len(results) {
		t.Fatalf("callback count %d != result count %d", len(callbacks), len(results))
	}

	// 回调顺序应该和最终结果顺序一致
	for i := range results {
		if callbacks[i] != results[i].Message {
			t.Fatalf("order mismatch at %d: callback=%q result=%q", i, callbacks[i], results[i].Message)
		}
	}
	t.Logf("DecodeStream: %d callbacks, %d results", len(callbacks), len(results))
}
