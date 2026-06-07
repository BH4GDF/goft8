package protocol

import "testing"

func FuzzPack77(f *testing.F) {
	for _, seed := range []string{
		"CQ BH4GDF PM00",
		"BH4GDF K1ABC -12",
		"BH4GDF K1ABC RR73",
		"ABCDEF1234567890AB",
		"<A/B> K1ABC 73",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, msg string) {
		ResetHashTables()
		_, _, _, _ = Pack77(msg)
	})
}
