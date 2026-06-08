package goft8

import "testing"

func TestDecodeFoxHoundPlaceholder(t *testing.T) {
	got := DecodeFoxHound(make([]float32, AudioSamplesPerCycle), FoxHoundParams{Mode: ModeHound})
	if got != nil {
		t.Fatalf("DecodeFoxHound() = %v, want nil placeholder result", got)
	}
}
