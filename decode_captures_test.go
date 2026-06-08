package goft8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression test for WSJT-X 2.7.0 main-loop parity against the three
// reference captures. Yields below were measured against Fortran's
// dump_all_passes program (sync8 + ft8b + subtractft8 with AP CQ-only)
// on the go-ft8 research port and are expected to hold here.
//
// Run parameters must mirror the Fortran main loop:
//   - freq range 200..2600 Hz
//   - depth = 3 (BP + OSD(2))
//   - AP enabled, CQ-only
//
// See docs/context-handoff.md §"Pipeline parity" for provenance.
var captureFixtures = []struct {
	name    string
	file    string
	want    int
	freqMin int
	freqMax int
	mustSee []string // sample of decodes that must appear in the result set
}{
	{
		// MSHV-enhanced decoder yields 11 here (vs 10 in WSJT-X 2.7.0 port); the 11th
		// The extra signal "<...> RA6ABC KN96" is recovered via the 5th LLR
		// set (bmete) and finer 0.25 Hz frequency search.
		name:    "ft8_cap1",
		file:    "ft8_cap1.wav",
		want:    11,
		freqMin: 200,
		freqMax: 2600,
		mustSee: []string{
			"CQ PV8AJ FJ92",
			"KB7THX WB9VGJ RR73",
			"A61CK W3DQS -12",
		},
	},
	{
		name:    "ft8_cap2",
		file:    "ft8_cap2.wav",
		want:    15,
		freqMin: 200,
		freqMax: 2600,
		mustSee: []string{
			"CQ ZS4AW KG31",
			"CQ SV0TPN KM28",
			"CQ TN8GD JI75",
		},
	},
	{
		name:    "ft8_cap3",
		file:    "ft8_cap3.wav",
		want:    23,
		freqMin: 200,
		freqMax: 2600,
		mustSee: []string{
			"CQ KB3Z FN20",
			"CQ NH6D BL02",
			"CQ UR5QW KN77",
			"CQ SP4MSY KO13",
		},
	},
	{
		name:    "ft8_cap4",
		file:    "ft8_cap4.wav",
		want:    21,
		freqMin: 100,
		freqMax: 3000,
		mustSee: []string{
			"JS1JTN/P HL5BPF -10",
			"BG7HFE JI1CUL PM95",
			"JF2AIJ JH4IJG PM64",
		},
	},
}

func TestDecodeCaptures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping capture regression in -short mode")
	}

	for _, tc := range captureFixtures {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.file)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("fixture not present: %s", path)
			}

			freqMin, freqMax := tc.freqMin, tc.freqMax
			if freqMin == 0 && freqMax == 0 {
				freqMin, freqMax = 200, 2600
			}
			decodes, err := DecodeWAV(
				path,
				WithFreqRange(freqMin, freqMax),
				WithDepth(DepthDeep),
				WithAPEnabled(true),
				WithCQOnlyAP(true),
			)
			if err != nil {
				t.Fatalf("DecodeWAV: %v", err)
			}

			for _, d := range decodes {
				t.Logf("pass=%d  %+3d dB  dt=%+5.2f  f=%7.1f  %s",
					d.Pass, d.SNR, d.DT, d.Freq, d.Message)
			}

			if got := len(decodes); got != tc.want {
				t.Errorf("decode count: got %d, want %d", got, tc.want)
			}

			seen := make(map[string]bool, len(decodes))
			for _, d := range decodes {
				seen[strings.TrimSpace(d.Message)] = true
			}
			for _, msg := range tc.mustSee {
				if !seen[msg] {
					t.Errorf("missing expected decode: %q", msg)
				}
			}
		})
	}
}
