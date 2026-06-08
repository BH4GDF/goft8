package ldpc

/*
#include "mshv_decode.h"
*/
import "C"
import (
	ft8params "github.com/bh4gdf/goft8/params"
)

func decodeLDPCCGO(llr [ft8params.LDPCn]float64, maxOSD, norder int, apmask [ft8params.LDPCn]int8) (DecodeResult, bool) {
	const n = ft8params.LDPCn
	const k = ft8params.LDPCk

	var cllr [n]C.double
	for i := 0; i < n; i++ {
		cllr[i] = C.double(llr[i])
	}

	var capmask [n]C.int
	for i := 0; i < n; i++ {
		capmask[i] = C.int(apmask[i])
	}

	var cmsg91 [k]C.int
	var ccw [n]C.int
	var cnhard C.int
	var cdmin C.double

	C.mshv_decode174_91(
		&cllr[0],
		C.int(maxOSD),
		C.int(norder),
		&capmask[0],
		&cmsg91[0],
		&ccw[0],
		&cnhard,
		&cdmin,
	)

	nhard := int(cnhard)
	if nhard < 0 {
		return DecodeResult{NHardErrors: nhard}, false
	}

	var result DecodeResult
	result.NHardErrors = nhard
	result.Dmin = float64(cdmin)
	if maxOSD < 0 || nhard == 0 {
		result.DecoderType = 1
	} else {
		result.DecoderType = 2
	}
	for i := 0; i < k; i++ {
		result.Message91[i] = int8(cmsg91[i])
	}
	for i := 0; i < n; i++ {
		result.Codeword[i] = int8(ccw[i])
	}
	return result, true
}
