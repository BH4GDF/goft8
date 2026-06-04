// ldpc.go implements the (174,91) LDPC decoder with BP and OSD.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90,
// subroutine osd174_91 from wsjt-wsjtx/lib/ft8/osd174_91.f90, and
// subroutine platanh from wsjt-wsjtx/lib/platanh.f90.

package ldpc

import ft8params "github.com/bh4gdf/goft8/params"

import (
	"math"
	"strconv"
	"sync"
)

// ────────────────────────────────────────────────────────────────────────────
// Fast tanh lookup table for BP hot loop
// ────────────────────────────────────────────────────────────────────────────

const (
	tanhLUTSize   = 512
	tanhLUTXMax   = 8.0
	tanhLUTScale  = float64(tanhLUTSize) / (2.0 * tanhLUTXMax) // maps [-XMax,+XMax] → [0,Size]
	tanhLUTXMin   = -tanhLUTXMax
)

var tanhLUT [tanhLUTSize + 1]float64

func init() {
	for i := 0; i <= tanhLUTSize; i++ {
		x := tanhLUTXMin + float64(i)/tanhLUTScale
		tanhLUT[i] = math.Tanh(x)
	}
}

// fastTanh returns an approximation of tanh(x) via linear interpolation on a
// 512-entry lookup table covering [-8, +8]. Typical error < 3e-4.
func fastTanh(x float64) float64 {
	if x < tanhLUTXMin {
		return -1.0
	}
	if x > tanhLUTXMax {
		return 1.0
	}
	f := (x - tanhLUTXMin) * tanhLUTScale
	idx := int(f)
	if idx >= tanhLUTSize {
		return 1.0
	}
	frac := f - float64(idx)
	return tanhLUT[idx] + frac*(tanhLUT[idx+1]-tanhLUT[idx])
}

// ────────────────────────────────────────────────────────────────────────────
// Pre-computed index tables for BP loop (eliminates inner linear scans)
// ────────────────────────────────────────────────────────────────────────────

var (
	bpOnce    sync.Once
	// tovIdx[bit][kk] = index i in LDPCNm row of the check node such that
	// LDPCMn[bit][kk]-1 == check for that row entry.  Eliminates the
	// inner "for kk { if LDPCMn[bit][kk]-1 == j }" scan.
	tovIdx    [ft8params.LDPCn][ft8params.LDPCncw]int
	// prodSkip[bit][kk] = row index i in LDPCNm[check] to SKIP (the entry
	// that refers back to variable node `bit`).  All other entries are
	// multiplied.  Eliminates the "if LDPCNm[chk][i]-1 != bit" branch.
	prodSkip  [ft8params.LDPCn][ft8params.LDPCncw]int
)

func initBPTables() {
	bpOnce.Do(func() {
		// Build tovIdx: for each variable node `bit` and check index `kk`,
		// find which row `i` in LDPCMn[bit][kk]'s Nm row corresponds to `bit`.
		for bit := 0; bit < ft8params.LDPCn; bit++ {
			for kk := 0; kk < ft8params.LDPCncw; kk++ {
				chk := LDPCMn[bit][kk] - 1 // 0-indexed check node
				for i := 0; i < LDPCNrw[chk]; i++ {
					if LDPCNm[chk][i]-1 == bit {
						tovIdx[bit][kk] = i
						break
					}
				}
			}
		}
		// Build prodSkip: for each variable node `bit` and check index `kk`,
		// find which row index in LDPCNm[check] to skip during the product.
		for bit := 0; bit < ft8params.LDPCn; bit++ {
			for kk := 0; kk < ft8params.LDPCncw; kk++ {
				chk := LDPCMn[bit][kk] - 1
				for i := 0; i < LDPCNrw[chk]; i++ {
					if LDPCNm[chk][i]-1 == bit {
						prodSkip[bit][kk] = i
						break
					}
				}
			}
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// DecodeResult holds the output of DecodeLDPC.
// ────────────────────────────────────────────────────────────────────────────

// DecodeResult holds the output of DecodeLDPC.
type DecodeResult struct {
	Message91   [ft8params.LDPCk]int8
	Codeword    [ft8params.LDPCn]int8
	NHardErrors int
	Dmin        float64
	DecoderType int // 1=BP, 2=OSD
}

// ────────────────────────────────────────────────────────────────────────────
// platanh — piecewise-linear atanh approximation
// ────────────────────────────────────────────────────────────────────────────

// platanh is the "protected" atanh used in the BP check→variable message update.
//
// Port of subroutine platanh from wsjt-wsjtx/lib/platanh.f90.
func platanh(x float64) float64 {
	sign := 1.0
	z := x
	if x < 0 {
		sign = -1.0
		z = -x
	}
	if z <= 0.664 {
		return x / 0.83
	} else if z <= 0.9217 {
		return sign * (z - 0.4064) / 0.322
	} else if z <= 0.9951 {
		return sign * (z - 0.8378) / 0.0524
	} else if z <= 0.9998 {
		return sign * (z - 0.9914) / 0.0012
	}
	return sign * 7.0
}

// ────────────────────────────────────────────────────────────────────────────
// Generator matrix (for OSD and encoding)
// ────────────────────────────────────────────────────────────────────────────

var (
	ldpcGenOnce sync.Once
	ldpcGen     [ft8params.LDPCm][ft8params.LDPCk]int8
)

// ldpcGenerator returns the (83×91) generator matrix G such that
// parity[i] = (G[i] · message) mod 2.
// Built once from the hex strings in ldpc_parity.go.
func ldpcGenerator() *[ft8params.LDPCm][ft8params.LDPCk]int8 {
	ldpcGenOnce.Do(func() {
		for row, hex := range ldpcGeneratorHex {
			col := 0
			for j, ch := range hex {
				ibmax := 4
				if j == 22 {
					ibmax = 3 // last nibble: only top 3 bits used (91 = 22*4+3)
				}
				nib, _ := strconv.ParseInt(string(ch), 16, 64)
				for jj := 0; jj < ibmax; jj++ {
					if col >= ft8params.LDPCk {
						break
					}
					// Fortran: btest(istr, 4-jj_1based) → bit (3-jj_0based).
					// For the last nibble, this gives the TOP 3 bits (3,2,1).
					bit := int8((nib >> uint(3-jj)) & 1)
					ldpcGen[row][col] = bit
					col++
				}
			}
		}
	})
	return &ldpcGen
}

// EncodeLDPCNoCRC encodes a 91-bit message into a 174-bit codeword
// without recomputing CRC.
//
// Port of subroutine encode174_91_nocrc from wsjt-wsjtx/lib/ft8/encode174_91.f90.
func EncodeLDPCNoCRC(message91 [ft8params.LDPCk]int8) [ft8params.LDPCn]int8 {
	gen := ldpcGenerator()
	var cw [ft8params.LDPCn]int8
	for i := 0; i < ft8params.LDPCk; i++ {
		cw[i] = message91[i]
	}
	for i := 0; i < ft8params.LDPCm; i++ {
		sum := 0
		for j := 0; j < ft8params.LDPCk; j++ {
			if message91[j] == 1 {
				sum ^= int(gen[i][j])
			}
		}
		cw[ft8params.LDPCk+i] = int8(sum)
	}
	return cw
}

// ────────────────────────────────────────────────────────────────────────────
// DecodeLDPC — hybrid BP/OSD decoder
// ────────────────────────────────────────────────────────────────────────────

// DecodeLDPC is the hybrid BP/OSD decoder for the FT8 (174,91) code.
//
//	maxOSD < 0: BP only
//	maxOSD = 0: BP then one OSD call with channel LLRs
//	maxOSD > 0: BP then up to maxOSD OSD calls with accumulated LLR sums
//
// ndeep controls OSD search depth (passed to osdDecode):
//
//	0=order-0, 1=order-1, 2=order-1+pre1, 3=order-1+pre1+pre2,
//	4=order-2+pre1+pre2, 5=order-3+pre, 6=order-4+pre.
//
// Port of subroutine decode174_91 from wsjt-wsjtx/lib/ft8/decode174_91.f90.
func DecodeLDPC(llr [ft8params.LDPCn]float64, keff, maxOSD, ndeep int, apmask [ft8params.LDPCn]int8) (DecodeResult, bool) {
	const (
		n             = ft8params.LDPCn
		m             = ft8params.LDPCm
		k             = ft8params.LDPCk
		ncw           = ft8params.LDPCncw
		maxIterations = 30
	)

	if maxOSD > 3 {
		maxOSD = 3
	}

	initBPTables()

	// Fortran: real llr(174). Truncate incoming float64 LLRs to float32
	// precision so BP and OSD operate on the same values Fortran does.
	for i := 0; i < n; i++ {
		llr[i] = float64(float32(llr[i]))
	}

	// decode174_91.f90 lines 28–37: set up OSD pass count and zsave.
	nosd := 0
	var zsave [3][n]float64
	switch {
	case maxOSD == 0:
		nosd = 1
		zsave[0] = llr
	case maxOSD > 0:
		nosd = maxOSD
	}

	var (
		tov     [n][ncw]float64
		toc     [m][7]float64
		tanhtoc [m][7]float64
	)

	// decode174_91.f90 lines 43–47: initialize messages to checks.
	for j := 0; j < m; j++ {
		for i := 0; i < LDPCNrw[j]; i++ {
			toc[j][i] = llr[LDPCNm[j][i]-1]
		}
	}

	ncnt := 0
	nclast := 0
	var zsum [n]float64

	// Hoist loop-local arrays outside the iteration loop to avoid repeated
	// stack allocation / stack-growth checks.
	var zn [n]float64
	var cw [n]int8

	// decode174_91.f90 lines 52–135: BP iterations.
	for iter := 0; iter <= maxIterations; iter++ {
		// Update bit LLR estimates (decode174_91.f90 lines 54–60).
		for i := 0; i < n; i++ {
			if apmask[i] != 1 {
				sum := llr[i]
				for kk := 0; kk < ncw; kk++ {
					sum += tov[i][kk]
				}
				zn[i] = sum
			} else {
				zn[i] = llr[i]
			}
		}

		// decode174_91.f90 lines 61–64: accumulate zsum, save for OSD.
		for i := 0; i < n; i++ {
			zsum[i] += zn[i]
		}
		if iter > 0 && iter <= maxOSD {
			zsave[iter-1] = zsum
		}

		// Hard decision (decode174_91.f90 lines 67–68).
		for i := 0; i < n; i++ {
			if zn[i] > 0 {
				cw[i] = 1
			} else {
				cw[i] = 0
			}
		}

		// Syndrome check (decode174_91.f90 lines 69–73).
		ncheck := 0
		for j := 0; j < m; j++ {
			s := 0
			for i := 0; i < LDPCNrw[j]; i++ {
				s += int(cw[LDPCNm[j][i]-1])
			}
			if s%2 != 0 {
				ncheck++
			}
		}

		// decode174_91.f90 lines 74–88: valid codeword → check CRC.
		if ncheck == 0 {
			var m96 [96]int8
			copy(m96[:77], cw[:77])
			copy(m96[82:96], cw[77:91])
			nHard := 0
			for i := 0; i < n; i++ {
				if float64(2*int(cw[i])-1)*llr[i] < 0 {
					nHard++
				}
			}
			if crc14Bits(m96[:]) == 0 {
				var msg91 [k]int8
				copy(msg91[:], cw[:k])
				// Compute dmin (decode174_91.f90 lines 83–85).
				var hdec [n]int8
				for i := 0; i < n; i++ {
					if llr[i] >= 0 {
						hdec[i] = 1
					}
				}
				dmin := 0.0
				for i := 0; i < n; i++ {
					if hdec[i] != cw[i] {
						dmin += math.Abs(llr[i])
					}
				}
				return DecodeResult{
					Message91:   msg91,
					Codeword:    cw,
					NHardErrors: nHard,
					Dmin:        dmin,
					DecoderType: 1,
				}, true
			}
		}

		// Early stopping (decode174_91.f90 lines 91–104).
		if iter > 0 {
			nd := ncheck - nclast
			if nd < 0 {
				ncnt = 0
			} else {
				ncnt++
			}
			if ncnt >= 5 && iter >= 10 && ncheck > 15 {
				break
			}
		}
		nclast = ncheck

		// Variable→check messages (decode174_91.f90 lines 108–118).
		// Uses pre-computed tovIdx to eliminate inner linear scan.
		for j := 0; j < m; j++ {
			for i := 0; i < LDPCNrw[j]; i++ {
				bit := LDPCNm[j][i] - 1
				v := zn[bit]
				// Find which kk index in LDPCMn[bit] points back to check j.
				// tovIdx[bit][kk] gives the Nm-row index; we need the kk where
				// LDPCMn[bit][kk]-1 == j.  Search over the 3 entries (still
				// branchless compared to inner loop with conditional subtract).
				for kk := 0; kk < ncw; kk++ {
					if LDPCMn[bit][kk]-1 == j {
						v -= tov[bit][kk]
						break
					}
				}
				toc[j][i] = v
			}
		}

		// Check→variable messages (decode174_91.f90 lines 121–133).
		// Use fastTanh LUT instead of math.Tanh.
		for j := 0; j < m; j++ {
			for i := 0; i < 7; i++ {
				tanhtoc[j][i] = fastTanh(-toc[j][i] / 2.0)
			}
		}

		// Check→variable product with pre-computed skip index.
		for bit := 0; bit < n; bit++ {
			for kk := 0; kk < ncw; kk++ {
				chk := LDPCMn[bit][kk] - 1
				skip := prodSkip[bit][kk]
				prod := 1.0
				nrw := LDPCNrw[chk]
				for i := 0; i < nrw; i++ {
					if i != skip {
						prod *= tanhtoc[chk][i]
					}
				}
				tov[bit][kk] = 2.0 * platanh(-prod)
			}
		}
	}

	// OSD passes (decode174_91.f90 lines 137–148).
	for i := 0; i < nosd; i++ {
		var zIn [n]float64
		if maxOSD == 0 {
			zIn = llr
		} else {
			zIn = zsave[i]
		}
		msg91, cw, nHard, ok := osdDecode(zIn, keff, apmask, ndeep)
		if ok && nHard > 0 {
			// Compute dmin (decode174_91.f90 lines 141–144).
			var hdec [n]int8
			for j := 0; j < n; j++ {
				if llr[j] >= 0 {
					hdec[j] = 1
				}
			}
			dmin := 0.0
			for j := 0; j < n; j++ {
				if hdec[j] != cw[j] {
					dmin += math.Abs(llr[j])
				}
			}
			return DecodeResult{
				Message91:   msg91,
				Codeword:    cw,
				NHardErrors: nHard,
				Dmin:        dmin,
				DecoderType: 2,
			}, true
		}
	}

	return DecodeResult{NHardErrors: -1}, false
}

// ────────────────────────────────────────────────────────────────────────────
// OSD decoder
// ────────────────────────────────────────────────────────────────────────────

// osdDecode is an ordered-statistics decoder for the (174,91) code.
//
// Port of subroutine osd174_91 from wsjt-wsjtx/lib/ft8/osd174_91.f90.
func osdDecode(llr [ft8params.LDPCn]float64, keff int, apmask [ft8params.LDPCn]int8, ndeep int) ([ft8params.LDPCk]int8, [ft8params.LDPCn]int8, int, bool) {
	const (
		n = ft8params.LDPCn
		k = ft8params.LDPCk
		m = ft8params.LDPCm
	)

	// osd174_91.f90 lines 37–65: build generator matrix (cached).
	gen := osdFullGenerator(keff)

	// osd174_91.f90 lines 67–68: rx = llr, apmaskr = apmask
	// Fortran uses float32 (real) throughout — truncate to match.
	var rx [n]float64
	for i := 0; i < n; i++ {
		rx[i] = float64(float32(llr[i]))
	}
	apmaskr := apmask

	// osd174_91.f90 lines 71–72: hard decisions.
	var hdec [n]int8
	for i := 0; i < n; i++ {
		if rx[i] >= 0 {
			hdec[i] = 1
		}
	}

	// osd174_91.f90 lines 75–76: sort by decreasing |LLR| (reliability).
	// Fortran sorts float32 absrx values. Truncate to float32 before sorting
	// to ensure identical tie-breaking (42 tied groups exist in typical LLRs).
	absrx := make([]float64, n)
	for i := range absrx {
		absrx[i] = float64(float32(math.Abs(rx[i])))
	}
	indx := argsortAsc(absrx) // indx[0]=least reliable

	// osd174_91.f90 lines 79–82: reorder generator matrix by decreasing reliability.
	var genmrb [ft8params.LDPCk][n]int8
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		ridx := indx[n-1-i] // Fortran: indx(N+1-i)
		for row := 0; row < k; row++ {
			genmrb[row][i] = gen[row][ridx]
		}
		indices[i] = ridx
	}

	// osd174_91.f90 lines 86–107: Gaussian elimination.
	for id := 0; id < k; id++ {
		found := false
		for icol := id; icol < k+20 && icol < n; icol++ {
			if genmrb[id][icol] == 1 {
				if icol != id {
					// Swap columns id and icol.
					for r := 0; r < k; r++ {
						genmrb[r][id], genmrb[r][icol] = genmrb[r][icol], genmrb[r][id]
					}
					indices[id], indices[icol] = indices[icol], indices[id]
				}
				// Eliminate column id from other rows.
				rowID := genmrb[id][:]
				for ii := 0; ii < k; ii++ {
					if ii != id && genmrb[ii][id] == 1 {
						rowII := genmrb[ii][:]
						for c := 0; c < n; c++ {
							rowII[c] ^= rowID[c]
						}
					}
				}
				found = true
				break
			}
		}
		// Fortran: if no pivot found for this diagonal, continues to next id
		// (does NOT break out of the outer loop).
		_ = found
	}

	// osd174_91.f90 lines 117–121: reorder hdec, absrx, apmaskr.
	var hdecR [n]int8
	var absR [n]float64
	var apmaskR [n]int8
	for i := 0; i < n; i++ {
		hdecR[i] = hdec[indices[i]]
		absR[i] = absrx[indices[i]]
		apmaskR[i] = apmaskr[indices[i]]
	}

	// osd174_91.f90 lines 123–128: order-0 codeword.
	var m0 [k]int8
	copy(m0[:], hdecR[:k])

	c0 := mrbEncode91(m0, genmrb)
	nhardMin := 0
	for i := 0; i < n; i++ {
		if c0[i] != hdecR[i] {
			nhardMin++
		}
	}
	dmin := 0.0
	for i := 0; i < n; i++ {
		if c0[i] != hdecR[i] {
			dmin += absR[i]
		}
	}
	bestCW := c0

	// osd174_91.f90 line 134: if(ndeep.eq.0) goto 998
	if ndeep == 0 {
		return osdFinish(bestCW, indices, nhardMin)
	}

	if ndeep > 6 {
		ndeep = 6
	}

	// osd174_91.f90 lines 136–177: search parameters from ndeep.
	var nord, npre1, npre2, nt, ntheta, ntau int
	switch ndeep {
	case 1:
		nord, npre1, npre2, nt, ntheta = 1, 0, 0, 40, 12
	case 2:
		nord, npre1, npre2, nt, ntheta = 1, 1, 0, 40, 10
	case 3:
		nord, npre1, npre2, nt, ntheta, ntau = 1, 1, 1, 40, 12, 14
	case 4:
		nord, npre1, npre2, nt, ntheta, ntau = 2, 1, 1, 40, 12, 17
	case 5:
		nord, npre1, npre2, nt, ntheta, ntau = 3, 1, 1, 40, 12, 15
	default: // ndeep=6
		nord, npre1, npre2, nt, ntheta, ntau = 4, 1, 1, 95, 12, 15
	}

	// osd174_91.f90 lines 179–228: order-1..nord combinatorial flip search.
	for iorder := 1; iorder <= nord; iorder++ {
		var misub [k]int8
		for i := k - iorder; i < k; i++ {
			misub[i] = 1
		}
		iflag := k - iorder

		for iflag >= 0 {
			iend := 0
			if iorder == nord && npre1 == 0 {
				iend = iflag
			}

			// baseCE = encode(m0 XOR misub) = c0 XOR sum(genmrb[set_bits][:]).
			var baseCE [n]int8
			copy(baseCE[:], c0[:])
			if iorder == 1 {
				row := genmrb[iflag][:]
				for c := 0; c < n; c++ {
					baseCE[c] ^= row[c]
				}
			} else {
				for bit := 0; bit < k; bit++ {
					if misub[bit] == 1 {
						row := genmrb[bit][:]
						for c := 0; c < n; c++ {
							baseCE[c] ^= row[c]
						}
					}
				}
			}

			var d1 float64
			var e2sub [m]int8
			for n1 := iflag; n1 >= iend; n1-- {
				var mi [k]int8
				copy(mi[:], misub[:])
				mi[n1] = 1

				// Skip if any flipped bit overlaps an AP-pinned position.
				skip := false
				for j := 0; j < k; j++ {
					if apmaskR[j] == 1 && mi[j] == 1 {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				// me = m0 XOR mi
				var me [k]int8
				for j := 0; j < k; j++ {
					me[j] = m0[j] ^ mi[j]
				}

				var e2 [m]int8
				var nd1kpt int
				if n1 == iflag {
					for j := 0; j < m; j++ {
						e2sub[j] = baseCE[k+j] ^ hdecR[k+j]
					}
					copy(e2[:], e2sub[:])
					nd1kpt = 1
					for j := 0; j < nt && j < m; j++ {
						nd1kpt += int(e2sub[j])
					}
					d1 = 0
					for j := 0; j < k; j++ {
						d1 += float64(me[j]^hdecR[j]) * absR[j]
					}
				} else {
					for j := 0; j < m; j++ {
						e2[j] = e2sub[j] ^ genmrb[n1][k+j]
					}
					nd1kpt = 2
					for j := 0; j < nt && j < m; j++ {
						nd1kpt += int(e2[j])
					}
				}

				if nd1kpt <= ntheta {
					var ce [n]int8
					if n1 < k && misub[n1] == 1 {
						ce = baseCE
					} else {
						copy(ce[:], baseCE[:])
						row := genmrb[n1][:]
						for c := 0; c < n; c++ {
							ce[c] ^= row[c]
						}
					}
					dd := 0.0
					nhard := 0
					if n1 == iflag {
						dd = d1
						for j := 0; j < m; j++ {
							dd += float64(e2sub[j]) * absR[k+j]
						}
						for j := 0; j < n; j++ {
							nxor := ce[j] ^ hdecR[j]
							nhard += int(nxor)
						}
					} else {
						dd = d1 + float64(ce[n1]^hdecR[n1])*absR[n1]
						for j := 0; j < m; j++ {
							dd += float64(e2[j]) * absR[k+j]
						}
						for j := 0; j < n; j++ {
							nxor := ce[j] ^ hdecR[j]
							nhard += int(nxor)
						}
					}
					if dd < dmin {
						dmin = dd
						bestCW = ce
						nhardMin = nhard
					}
				}
			}
			iflag = nextpat91(misub[:], k, iorder)
		}
	}

	// osd174_91.f90 lines 230–279: npre2 hash-based pair-flip search.
	if npre2 == 1 {
		// Build hash table (osd174_91.f90 lines 231–239).
		hashFP := make(map[int][]osdPairEntry)
		for i1 := k - 1; i1 >= 0; i1-- {
			for i2 := i1 - 1; i2 >= 0; i2-- {
				ipat := 0
				for t := 0; t < ntau && t < m; t++ {
					bit := genmrb[i1][k+t] ^ genmrb[i2][k+t]
					if bit == 1 {
						ipat |= 1 << uint(ntau-1-t)
					}
				}
				hashFP[ipat] = append(hashFP[ipat], osdPairEntry{i1: i1, i2: i2})
			}
		}

		// osd174_91.f90 lines 245–278: run through order-nord patterns.
		var misub2 [k]int8
		for i := k - nord; i < k; i++ {
			misub2[i] = 1
		}
		iflag2 := k - nord

		for iflag2 >= 0 {
			var me [k]int8
			for j := 0; j < k; j++ {
				me[j] = m0[j] ^ misub2[j]
			}
			ce := mrbEncode91(me, genmrb)
			var e2sub2 [m]int8
			for j := 0; j < m; j++ {
				e2sub2[j] = ce[k+j] ^ hdecR[k+j]
			}

			// Precompute wtBase and skipBase for misub2.
			wtBase := 0
			skipBase := false
			for j := 0; j < k; j++ {
				wtBase += int(misub2[j])
				if apmaskR[j] == 1 && misub2[j] == 1 {
					skipBase = true
					break
				}
			}

			for i2t := 0; i2t <= ntau && i2t <= m; i2t++ {
				ipat := 0
				for t := 0; t < ntau && t < m; t++ {
					bit := e2sub2[t]
					if t == i2t && i2t > 0 {
						bit ^= 1
					}
					if bit == 1 {
						ipat |= 1 << uint(ntau-1-t)
					}
				}

				entries := hashFP[ipat]
				for _, ent := range entries {
					in1, in2 := ent.i1, ent.i2

					// mi = misub2 OR {in1, in2}; incremental wt.
					delta1 := 0
					if misub2[in1] == 0 {
						delta1 = 1
					}
					delta2 := 0
					if misub2[in2] == 0 {
						delta2 = 1
					}
					wt := wtBase + delta1 + delta2
					if wt < nord+npre1+npre2 {
						continue
					}

					// AP mask check.
					skip := skipBase
					if !skip && misub2[in1] == 0 && apmaskR[in1] == 1 {
						skip = true
					}
					if !skip && misub2[in2] == 0 && apmaskR[in2] == 1 {
						skip = true
					}
					if skip {
						continue
					}

					// Incremental encoding: ce2 = ce XOR rows of flipped bits.
					var ce2 [n]int8
					row1 := genmrb[in1][:]
					row2 := genmrb[in2][:]
					if misub2[in1] == 0 && misub2[in2] == 0 {
						for j := 0; j < n; j++ {
							ce2[j] = ce[j] ^ row1[j] ^ row2[j]
						}
					} else if misub2[in1] == 0 {
						for j := 0; j < n; j++ {
							ce2[j] = ce[j] ^ row1[j]
						}
					} else if misub2[in2] == 0 {
						for j := 0; j < n; j++ {
							ce2[j] = ce[j] ^ row2[j]
						}
					} else {
						ce2 = ce
					}

					dd := 0.0
					nhard := 0
					for j := 0; j < n; j++ {
						nxor := ce2[j] ^ hdecR[j]
						dd += float64(nxor) * absR[j]
						nhard += int(nxor)
					}
					if dd < dmin {
						dmin = dd
						bestCW = ce2
						nhardMin = nhard
					}
				}
			}
			iflag2 = nextpat91(misub2[:], k, nord)
		}
	}

	return osdFinish(bestCW, indices, nhardMin)
}

// osdFinish re-orders the codeword, checks CRC, and returns the result.
// Port of osd174_91.f90 lines 281–292.
func osdFinish(bestCW [ft8params.LDPCn]int8, indices []int, nhardMin int) ([ft8params.LDPCk]int8, [ft8params.LDPCn]int8, int, bool) {
	const (
		n = ft8params.LDPCn
		k = ft8params.LDPCk
	)

	// Re-order to natural bit order (osd174_91.f90 line 283).
	var cwOut [n]int8
	for i := 0; i < n; i++ {
		cwOut[indices[i]] = bestCW[i]
	}

	// Check CRC (osd174_91.f90 lines 286–290).
	var msg91 [k]int8
	copy(msg91[:], cwOut[:k])
	if !checkCRC14Codeword(cwOut) {
		return [k]int8{}, [n]int8{}, -nhardMin, false
	}
	return msg91, cwOut, nhardMin, true
}

// ────────────────────────────────────────────────────────────────────────────
// OSD generator matrix (cached)
// ────────────────────────────────────────────────────────────────────────────

var (
	osdGenOnce sync.Once
	osdGen     [ft8params.LDPCk][ft8params.LDPCn]int8
)

// osdFullGenerator builds the full systematic generator matrix [k][n]
// for the OSD decoder. Cached via sync.Once.
//
// Port of osd174_91.f90 lines 37–65: the "if(first)" block.
func osdFullGenerator(keff int) *[ft8params.LDPCk][ft8params.LDPCn]int8 {
	osdGenOnce.Do(func() {
		gen := ldpcGenerator()
		// Build full systematic generator: I(k) | G^T
		for row := 0; row < ft8params.LDPCk; row++ {
			osdGen[row][row] = 1 // identity part
			for p := 0; p < ft8params.LDPCm; p++ {
				osdGen[row][ft8params.LDPCk+p] = gen[p][row]
			}
		}
	})
	return &osdGen
}

// ────────────────────────────────────────────────────────────────────────────
// OSD helper types and functions
// ────────────────────────────────────────────────────────────────────────────

type osdPairEntry struct {
	i1, i2 int
}

// mrbEncode91 encodes a k-bit message using the generator matrix genmrb.
//
// Port of subroutine mrbencode91 from wsjt-wsjtx/lib/ft8/osd174_91.f90 lines 295–305.
// Uses genmrb directly (row-major) for sequential memory access instead of
// the transposed g2 used in the Fortran original.
func mrbEncode91(me [ft8params.LDPCk]int8, genmrb [ft8params.LDPCk][ft8params.LDPCn]int8) [ft8params.LDPCn]int8 {
	var cw [ft8params.LDPCn]int8
	for i := 0; i < ft8params.LDPCk; i++ {
		if me[i] == 1 {
			row := genmrb[i][:]
			for c := 0; c < ft8params.LDPCn; c++ {
				cw[c] ^= row[c]
			}
		}
	}
	return cw
}

// nextpat91 generates the next test error pattern of weight iorder among k positions.
// mi is modified in place. Returns the 0-indexed position of the lowest set bit,
// or -1 when all patterns have been exhausted.
//
// Port of subroutine nextpat91 from wsjt-wsjtx/lib/ft8/osd174_91.f90 lines 307–334.
func nextpat91(mi []int8, k, iorder int) int {
	ind := -1
	for i := 0; i < k-1; i++ {
		if mi[i] == 0 && mi[i+1] == 1 {
			ind = i
		}
	}
	if ind < 0 {
		return -1
	}

	var ms [ft8params.LDPCk]int8
	copy(ms[:], mi[:ind])
	ms[ind] = 1
	// ms[ind+1] stays 0.
	if ind+1 < k {
		s := 0
		for _, v := range ms[:k] {
			s += int(v)
		}
		nz := iorder - s
		for i := k - nz; i < k; i++ {
			ms[i] = 1
		}
	}
	copy(mi, ms[:k])

	for i := 0; i < k; i++ {
		if mi[i] == 1 {
			return i
		}
	}
	return -1
}

// argsortAsc returns indices that sort arr[0..n-1] in ascending order.
//
// Numerical Recipes quicksort with insertion sort for small partitions
// (threshold M=7). The OSD decoder is sensitive to the exact tie-breaking
// order of this sort — the algorithm must match the Fortran implementation
// to produce identical decode results for marginal signals.
func argsortAsc(arr []float64) []int {
	const (
		m      = 7
		nstack = 50
	)
	n := len(arr)
	indx := make([]int, n)
	for i := range indx {
		indx[i] = i
	}

	jstack := 0
	l := 0
	ir := n - 1
	istack := make([]int, nstack)

	for {
		if ir-l < m {
			// Insertion sort for small partitions.
			for j := l + 1; j <= ir; j++ {
				indxt := indx[j]
				a := arr[indxt]
				i := j - 1
				for i >= l {
					if arr[indx[i]] <= a {
						break
					}
					indx[i+1] = indx[i]
					i--
				}
				indx[i+1] = indxt
			}
			if jstack == 0 {
				return indx
			}
			ir = istack[jstack-1]
			l = istack[jstack-2]
			jstack -= 2
		} else {
			k := (l + ir) / 2
			indx[k], indx[l+1] = indx[l+1], indx[k]

			if arr[indx[l+1]] > arr[indx[ir]] {
				indx[l+1], indx[ir] = indx[ir], indx[l+1]
			}
			if arr[indx[l]] > arr[indx[ir]] {
				indx[l], indx[ir] = indx[ir], indx[l]
			}
			if arr[indx[l+1]] > arr[indx[l]] {
				indx[l+1], indx[l] = indx[l], indx[l+1]
			}

			i := l + 1
			j := ir
			indxt := indx[l]
			a := arr[indxt]

			for {
				// Fortran: 3 continue; i=i+1; if(arr(indx(i)).lt.a) goto 3
				for i++; arr[indx[i]] < a; i++ {
				}
				// Fortran: 4 continue; j=j-1; if(arr(indx(j)).gt.a) goto 4
				for j--; arr[indx[j]] > a; j-- {
				}
				if j < i {
					break
				}
				indx[i], indx[j] = indx[j], indx[i]
			}

			indx[l] = indx[j]
			indx[j] = indxt
			jstack += 2
			if jstack > nstack {
				panic("indexx: NSTACK too small")
			}
			if ir-i+1 >= j-l {
				istack[jstack-1] = ir
				istack[jstack-2] = i
				ir = j - 1
			} else {
				istack[jstack-1] = j - 1
				istack[jstack-2] = l
				l = i
			}
		}
	}
}
