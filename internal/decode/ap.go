// ap.go implements a-priori (AP) decoding support.
//
// Port of the AP injection block in ft8b.f90 lines 300–401 (ncontest=0 only).

package decode

import (
	"strings"

	"github.com/bh4gdf/goft8/internal/protocol"
	ft8params "github.com/bh4gdf/goft8/params"
)

// ── AP message constants (±1 form) ──────────────────────────────────────
//
// Port of ft8b.f90 lines 39–46 after the 2*x-1 conversion (lines 52–58).
//
// mcq encodes "CQ" in the c28 field (29 bits, ±1 form):
//
//	data mcq/0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0/
//	mcq = 2*mcq - 1
var mcq = [29]int{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, +1, -1, -1,
}

// mrrr encodes "RRR" in the r19 field (19 bits, ±1 form):
//
//	data mrrr/0,1,1,1,1,1,1,0,1,0,0,1,0,0,1,0,0,0,1/
//	mrrr = 2*mrrr - 1
var mrrr = [19]int{
	-1, +1, +1, +1, +1, +1, +1, -1, +1, -1, -1, +1, -1, -1, +1, -1, -1, -1, +1,
}

// m73 encodes "73" in the r19 field (19 bits, ±1 form):
//
//	data m73/0,1,1,1,1,1,1,0,1,0,0,1,0,1,0,0,0,0,1/
//	m73 = 2*m73 - 1
var m73 = [19]int{
	-1, +1, +1, +1, +1, +1, +1, -1, +1, -1, -1, +1, -1, +1, -1, -1, -1, -1, +1,
}

// mrr73 encodes "RR73" in the r19 field (19 bits, ±1 form):
//
//	data mrr73/0,1,1,1,1,1,1,0,0,1,1,1,0,1,0,1,0,0,1/
//	mrr73 = 2*mrr73 - 1
var mrr73 = [19]int{
	-1, +1, +1, +1, +1, +1, +1, -1, -1, +1, +1, +1, -1, +1, -1, +1, -1, -1, +1,
}

// ComputeAPSymbols computes the 58-element a-priori symbol array from
// the operator's callsign (mycall) and the DX callsign (hiscall).
//
// Port of subroutine ft8apset from wsjt-wsjtx/lib/ft8/ft8apset.f90.
//
// For a type-1 message, the first 58 bits of c77 are:
//
//	n28a (28 bits) | ipa=0 (1 bit) | n28b (28 bits) | ipb=0 (1 bit)
//
// The returned apsym values are in ±1 form. Sentinel value 99 indicates
// unknown: apsym[0]==99 means mycall is invalid, apsym[29]==99 means
// hiscall is unknown.
func ComputeAPSymbols(mycall, hiscall string) [58]int {
	var apsym [58]int

	// Sentinel defaults (ft8apset.f90 lines 11-13)
	apsym[0] = 99
	apsym[29] = 99

	mc := strings.TrimSpace(strings.ToUpper(mycall))
	if len(mc) < 3 {
		return apsym
	}

	// Pack mycall
	n28a := protocol.Pack28(mc)

	// Write n28a as 28 bits (MSB first) into apsym[0:27], ipa=0 into apsym[28]
	for i := 0; i < 28; i++ {
		bit := (n28a >> uint(27-i)) & 1
		apsym[i] = 2*bit - 1
	}
	apsym[28] = -1 // ipa=0 → 2*0-1 = -1

	// Pack hiscall if provided
	hc := strings.TrimSpace(strings.ToUpper(hiscall))
	if len(hc) < 3 {
		apsym[29] = 99 // sentinel: unknown dxcall
		return apsym
	}

	n28b := protocol.Pack28(hc)
	for i := 0; i < 28; i++ {
		bit := (n28b >> uint(27-i)) & 1
		apsym[29+i] = 2*bit - 1
	}
	apsym[57] = -1 // ipb=0 → 2*0-1 = -1

	return apsym
}

// Naptypes2 maps [nQSOProgress][apIdx] → iaptype.
// Port of MSHV decoderft8.cpp static const int Naptypes2[6][4].
//
// nQSOProgress meanings:
//
//	0 = Calling CQ
//	1 = Rx1 (received reply)
//	2 = Rx2 (received report)
//	3 = Rx3 (received RRR/73/RR73)
//	4 = Tx3 (sending report)
//	5 = Tx4/Tx5/Tx6 (signoff)
var Naptypes2 = [6][4]int{
	{1, 2, 0, 0}, // 0: CQ → MyCall
	{2, 3, 0, 0}, // 1: MyCall → MyCall+DxCall
	{2, 3, 0, 0}, // 2: MyCall → MyCall+DxCall
	{3, 4, 5, 6}, // 3: MyCall+DxCall → RRR → 73 → RR73
	{3, 4, 5, 6}, // 4: same as 3
	{3, 1, 2, 0}, // 5: MyCall+DxCall → CQ → MyCall
}

// Nappasses2 maps nQSOProgress → number of AP pass pairs to try.
// Each pair consists of one llra pass + one llrc pass.
var Nappasses2 = [6]int{2, 2, 2, 4, 4, 3}

// ApplyAP injects a-priori information into the LLR and mask arrays.
//
// Port of ft8b.f90 lines 300–401 and MSHV decoderft8.cpp.
// Supports contest types 0–4 (standard, EU VHF, Field Day, RTTY RU).
//
// Parameters:
//   - llrz: soft symbols (174 LLRs) — modified in place
//   - apmask: AP mask (174 bits) — modified in place; 1 = position has AP info
//   - iaptype: AP type (1–6)
//   - apsym: AP symbols (±1 form); apsym[0:28] = mycall c28, apsym[29:57] = dxcall c28
//   - apmag: magnitude of AP LLRs
//   - contestType: 0=standard, 2=NA/EU VHF, 3=ARRL Field Day, 4=ARRL RTTY RU
func ApplyAP(llrz *[ft8params.LDPCn]float64, apmask *[ft8params.LDPCn]int8, iaptype int, apsym [58]int, apmag float64, contestType int) {
	// ft8b.f90 line 270–271: apmask=0; iaptype=0  (caller zeroes for non-AP passes)
	// Here we always zero apmask first, then fill per iaptype.
	for i := range apmask {
		apmask[i] = 0
	}

	// ft8b.f90 lines 300–314: iaptype=1 — CQ
	// ncontest=0: llrz(1:29)=apmag*mcq(1:29)
	if iaptype == 1 {
		// Fortran: apmask(1:29)=1; llrz(1:29)=apmag*mcq(1:29)
		for i := 0; i < 29; i++ {
			apmask[i] = 1
			llrz[i] = apmag * float64(mcq[i])
		}
		// Fortran: apmask(75:77)=1; llrz(75:76)=apmag*(-1); llrz(77)=apmag*(+1)
		apmask[74] = 1
		apmask[75] = 1
		apmask[76] = 1
		llrz[74] = apmag * (-1)
		llrz[75] = apmag * (-1)
		llrz[76] = apmag * (+1)
	}

	// iaptype=2 — MyCall,???,???
	// contestType variants from MSHV decoderft8.cpp lines 1038–1102.
	if iaptype == 2 {
		switch contestType {
		case 2: // EU VHF: 28 bits + i3=2 (bits 71-73)
			for i := 0; i < 28; i++ {
				apmask[i] = 1
				llrz[i] = apmag * float64(apsym[i])
			}
			apmask[71] = 1
			apmask[72] = 1
			apmask[73] = 1
			llrz[71] = apmag * (-1)
			llrz[72] = apmag * (+1)
			llrz[73] = apmag * (-1)
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (-1)
			llrz[76] = apmag * (-1)
		case 3: // ARRL Field Day: 28 bits + i3=0,n3=4 (bits 74-76)
			for i := 0; i < 28; i++ {
				apmask[i] = 1
				llrz[i] = apmag * float64(apsym[i])
			}
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (-1)
			llrz[76] = apmag * (-1)
		case 4: // ARRL RTTY RU: mycall shifted by 1 bit (itu prefix)
			for i := 1; i < 29; i++ {
				apmask[i] = 1
				llrz[i] = apmag * float64(apsym[i-1])
			}
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (+1)
			llrz[76] = apmag * (+1)
		default: // standard QSO (contestType 0 or 1)
			for i := 0; i < 29; i++ {
				apmask[i] = 1
				llrz[i] = apmag * float64(apsym[i])
			}
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (-1)
			llrz[76] = apmag * (+1)
		}
	}

	// iaptype=3 — MyCall,DxCall,???
	// contestType variants from MSHV decoderft8.cpp lines 1115–1165.
	if iaptype == 3 {
		switch contestType {
		case 3: // ARRL Field Day: 56 bits (2×28) + i3=0,n3=4
			for i := 0; i < 56; i++ {
				apmask[i] = 1
			}
			for i := 0; i < 28; i++ {
				llrz[i] = apmag * float64(apsym[i])
				llrz[i+28] = apmag * float64(apsym[i+29]) // skip ipa bit
			}
			apmask[71] = 1
			apmask[72] = 1
			apmask[73] = 1
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (-1)
			llrz[76] = apmag * (-1)
		case 4: // ARRL RTTY RU: mycall shifted by 1 bit
			for i := 0; i < 57; i++ {
				if i > 0 {
					apmask[i] = 1
				}
			}
			for i := 0; i < 28; i++ {
				llrz[i+1] = apmag * float64(apsym[i]) // mycall shifted
			}
			for i := 29; i < 58; i++ {
				llrz[i] = apmag * float64(apsym[i]) // dxcall unchanged
			}
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (+1)
			llrz[76] = apmag * (+1)
		default: // standard QSO and EU VHF (contestType 0,1,2)
			for i := 0; i < 58; i++ {
				apmask[i] = 1
				llrz[i] = apmag * float64(apsym[i])
			}
			apmask[74] = 1
			apmask[75] = 1
			apmask[76] = 1
			llrz[74] = apmag * (-1)
			llrz[75] = apmag * (-1)
			llrz[76] = apmag * (+1)
		}
	}

	// ft8b.f90 lines 382–389: iaptype=4,5,6 — MyCall,DxCall,RRR|73|RR73
	// ncontest=0 (ncontest.le.5): apmask(1:77)=1
	if iaptype >= 4 && iaptype <= 6 {
		for i := 0; i < 77; i++ {
			apmask[i] = 1
		}
		// Fortran: llrz(1:58)=apmag*apsym
		for i := 0; i < 58; i++ {
			llrz[i] = apmag * float64(apsym[i])
		}
		// Fortran: llrz(59:77)=apmag*mrrr|m73|mrr73
		var msg *[19]int
		switch iaptype {
		case 4:
			msg = &mrrr
		case 5:
			msg = &m73
		case 6:
			msg = &mrr73
		}
		for i := 0; i < 19; i++ {
			llrz[58+i] = apmag * float64(msg[i])
		}
	}
}
