// pack77.go — Message packing for FT8.
//
// Port of pack77 from wsjt-wsjtx/lib/77bit/packjt77.f90 and
// MSHV's pack_unpack_msg77.cpp.
//
// Supports all major FT8 message types matching MSHV priority order.

package goft8

import (
	"math/big"
	"strconv"
	"strings"
)

// ARRL Field Day sections (NSEC=86, matching MSHV csec_77).
var csecArr = [86]string{
	"AB ", "AK ", "AL ", "AR ", "AZ ", "BC ", "CO ", "CT ", "DE ", "EB ",
	"EMA", "ENY", "EPA", "EWA", "GA ", "GH ", "IA ", "ID ", "IL ", "IN ",
	"KS ", "KY ", "LA ", "LAX", "NS ", "MB ", "MDC", "ME ", "MI ", "MN ",
	"MO ", "MS ", "MT ", "NC ", "ND ", "NE ", "NFL", "NH ", "NL ", "NLI",
	"NM ", "NNJ", "NNY", "TER", "NTX", "NV ", "OH ", "OK ", "ONE", "ONN",
	"ONS", "OR ", "ORG", "PAC", "PR ", "QC ", "RI ", "SB ", "SC ", "SCV",
	"SD ", "SDG", "SF ", "SFL", "SJV", "SK ", "SNJ", "STX", "SV ", "TN ",
	"UT ", "VA ", "VI ", "VT ", "WCF", "WI ", "WMA", "WNY", "WPA", "WTX",
	"WV ", "WWA", "WY ", "DX ", "PE ", "NB ",
}

// writeBits writes a value into a bit string at the given position.
func writeBits(bits *[77]int8, pos, nbits, value int) {
	for i := nbits - 1; i >= 0; i-- {
		bits[pos+i] = int8(value & 1)
		value >>= 1
	}
}

// grid4ToInt converts a 4-char Maidenhead grid to an integer.
func grid4ToInt(grid string) (int, bool) {
	if len(grid) != 4 {
		return 0, false
	}
	g := strings.ToUpper(grid)
	j1 := int(g[0] - 'A')
	j2 := int(g[1] - 'A')
	j3 := int(g[2] - '0')
	j4 := int(g[3] - '0')
	if j1 < 0 || j1 > 17 || j2 < 0 || j2 > 17 || j3 < 0 || j3 > 9 || j4 < 0 || j4 > 9 {
		return 0, false
	}
	return j1*18*10*10 + j2*10*10 + j3*10 + j4, true
}

// grid6ToInt converts a 6-char Maidenhead grid to an integer.
func grid6ToInt(grid string) (int, bool) {
	if len(grid) != 6 {
		return 0, false
	}
	g := strings.ToUpper(grid)
	j1 := int(g[0] - 'A')
	j2 := int(g[1] - 'A')
	j3 := int(g[2] - '0')
	j4 := int(g[3] - '0')
	j5 := int(g[4] - 'A')
	j6 := int(g[5] - 'A')
	if j1 < 0 || j1 > 17 || j2 < 0 || j2 > 17 || j3 < 0 || j3 > 9 || j4 < 0 || j4 > 9 || j5 < 0 || j5 > 23 || j6 < 0 || j6 > 23 {
		return 0, false
	}
	return j1*18*10*10*24*24 + j2*10*10*24*24 + j3*10*24*24 + j4*24*24 + j5*24 + j6, true
}

// Pack77 encodes a text message into 77 bits.
//
// Returns the 77-bit array, i3 type, n3 subtype, and ok status.
// Priority order matches MSHV pack77 implementation.
func Pack77(msg string) ([77]int8, int, int, bool) {
	var bits [77]int8
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return bits, 0, 0, false
	}

	// Telemetry (i3=0, n3=5): 18 hex chars, no spaces.
	if packed, ok := packTelemetry(msg); ok {
		return packed, 0, 5, true
	}

	// DXpedition (i3=0, n3=1).
	if packed, ok := packDXpedition(msg); ok {
		return packed, 0, 1, true
	}

	// ARRL Field Day (i3=0, n3=3/4).
	if packed, n3, ok := packFieldDay(msg); ok {
		return packed, 0, n3, true
	}

	// Standard messages (i3=1/2).
	if packed, i3, ok := packStandardMessage(msg); ok {
		return packed, i3, 0, true
	}

	// ARRL RTTY Contest (i3=3).
	if packed, ok := packRTTYContest(msg); ok {
		return packed, 3, 0, true
	}

	// Non-standard callsign (i3=4).
	if packed, ok := packNonStandardCall(msg); ok {
		return packed, 4, 0, true
	}

	// EU VHF Contest (i3=5).
	if packed, ok := packEUVHF(msg); ok {
		return packed, 5, 0, true
	}

	// Free text (i3=0, n3=0).
	if packed, ok := packFreeText(msg); ok {
		return packed, 0, 0, true
	}

	return bits, 0, 0, false
}

// packTelemetry encodes a telemetry message (i3=0, n3=5).
// Format: 18 hexadecimal characters, no spaces.
func packTelemetry(msg string) ([77]int8, bool) {
	if strings.Contains(msg, " ") || len(msg) != 18 {
		return [77]int8{}, false
	}

	ntel := make([]int64, 3)
	for i := 0; i < 3; i++ {
		n, err := strconv.ParseInt(msg[i*6:(i+1)*6], 16, 64)
		if err != nil {
			return [77]int8{}, false
		}
		ntel[i] = n
	}

	if ntel[0] >= (1 << 23) {
		return [77]int8{}, false
	}

	var bits [77]int8
	writeBits(&bits, 0, 23, int(ntel[0]))
	writeBits(&bits, 23, 24, int(ntel[1]))
	writeBits(&bits, 47, 24, int(ntel[2]))
	writeBits(&bits, 71, 3, 5)
	writeBits(&bits, 74, 3, 0)

	return bits, true
}

// packDXpedition encodes a DXpedition message (i3=0, n3=1).
// Format: "CALL1 RR73; CALL2 <CALL3> ±NN"
func packDXpedition(msg string) ([77]int8, bool) {
	semi := strings.Index(msg, ";")
	if semi < 0 {
		return [77]int8{}, false
	}
	part1 := strings.TrimSpace(msg[:semi])
	part2 := strings.TrimSpace(msg[semi+1:])

	w1 := strings.Fields(part1)
	if len(w1) != 2 || w1[1] != "RR73" {
		return [77]int8{}, false
	}

	w2 := strings.Fields(part2)
	if len(w2) != 3 {
		return [77]int8{}, false
	}

	call1 := w1[0]
	call2 := w2[0]
	call3 := w2[1]
	rptStr := w2[2]

	if !strings.HasPrefix(call3, "<") || !strings.HasSuffix(call3, ">") {
		return [77]int8{}, false
	}
	call3 = strings.TrimPrefix(call3, "<")
	call3 = strings.TrimSuffix(call3, ">")

	rpt, err := strconv.Atoi(rptStr)
	if err != nil || rpt < -50 || rpt > 59 {
		return [77]int8{}, false
	}

	n5 := (rpt + 30) / 2
	if n5 < 0 {
		n5 = 0
	}
	if n5 > 31 {
		n5 = 31
	}

	if !isStdCall(call1) || !isStdCall(call2) {
		return [77]int8{}, false
	}

	n28a := pack28(call1)
	n28b := pack28(call2)
	if n28a < 0 || n28b < 0 {
		return [77]int8{}, false
	}

	n10 := hashCall(call3, 10) & 0x3FF
	SaveHashCall(call3)

	var bits [77]int8
	writeBits(&bits, 0, 28, n28a)
	writeBits(&bits, 28, 28, n28b)
	writeBits(&bits, 56, 10, n10)
	writeBits(&bits, 66, 5, n5)
	writeBits(&bits, 71, 3, 1)
	writeBits(&bits, 74, 3, 0)

	return bits, true
}

// packFieldDay encodes an ARRL Field Day message (i3=0, n3=3/4).
// Format: "CALL1 CALL2 [R] NTXCLASS SECTION"
func packFieldDay(msg string) ([77]int8, int, bool) {
	parts := strings.Fields(msg)
	if len(parts) < 4 || len(parts) > 5 {
		return [77]int8{}, 0, false
	}

	call1 := parts[0]
	call2 := parts[1]
	if !isStdCall(call1) || !isStdCall(call2) {
		return [77]int8{}, 0, false
	}

	// Parse section (last part).
	section := parts[len(parts)-1]
	isec := -1
	for i, s := range csecArr {
		if strings.TrimSpace(s) == section {
			isec = i + 1
			break
		}
	}
	if isec == -1 {
		return [77]int8{}, 0, false
	}

	// Check R prefix.
	ir := 0
	idx := 2
	if len(parts) == 5 {
		if parts[2] != "R" {
			return [77]int8{}, 0, false
		}
		ir = 1
		idx = 3
	}

	// Parse NTXCLASS (e.g., "16A").
	ntxclass := parts[idx]
	if len(ntxclass) < 2 || len(ntxclass) > 3 {
		return [77]int8{}, 0, false
	}
	nclass := int(ntxclass[len(ntxclass)-1] - 'A')
	if nclass < 0 || nclass > 25 {
		return [77]int8{}, 0, false
	}
	ntx, err := strconv.Atoi(ntxclass[:len(ntxclass)-1])
	if err != nil || ntx < 1 || ntx > 32 {
		return [77]int8{}, 0, false
	}

	n3 := 3
	intx := ntx - 1
	if intx >= 16 {
		n3 = 4
		intx = ntx - 17
	}

	n28a := pack28(call1)
	n28b := pack28(call2)

	var bits [77]int8
	writeBits(&bits, 0, 28, n28a)
	writeBits(&bits, 28, 28, n28b)
	writeBits(&bits, 56, 1, ir)
	writeBits(&bits, 57, 4, intx)
	writeBits(&bits, 61, 3, nclass)
	writeBits(&bits, 64, 7, isec)
	writeBits(&bits, 71, 3, n3)
	writeBits(&bits, 74, 3, 0)

	return bits, n3, true
}

// packStandardMessage encodes a standard FT8 message (i3=1 or i3=2).
// Format: CALL1 [ipa] CALL2 [ipb] [R] GRID|REPORT|RRR|RR73|73
func packStandardMessage(msg string) ([77]int8, int, bool) {
	parts := strings.Fields(msg)
	if len(parts) < 2 {
		return [77]int8{}, 0, false
	}

	i3 := 1

	// Parse first callsign.
	call1 := parts[0]

	// Handle /R and /P suffixes for call1.
	ipa := 0
	if strings.HasSuffix(call1, "/R") {
		ipa = 1
		call1 = strings.TrimSuffix(call1, "/R")
	} else if strings.HasSuffix(call1, "/P") {
		ipa = 1
		call1 = strings.TrimSuffix(call1, "/P")
		i3 = 2 // /P uses i3=2
	}

	// Parse second callsign.
	call2 := parts[1]

	// Handle /R and /P suffixes for call2.
	ipb := 0
	if strings.HasSuffix(call2, "/R") {
		ipb = 1
		call2 = strings.TrimSuffix(call2, "/R")
	} else if strings.HasSuffix(call2, "/P") {
		ipb = 1
		call2 = strings.TrimSuffix(call2, "/P")
		i3 = 2
	}

	// Only proceed if both callsigns are standard format or special tokens.
	if call1 != "CQ" && call1 != "DE" && call1 != "QRZ" && !isStdCall(call1) {
		return [77]int8{}, 0, false
	}
	if call2 != "CQ" && call2 != "DE" && call2 != "QRZ" && !isStdCall(call2) {
		return [77]int8{}, 0, false
	}

	// Encode callsigns.
	n28a := pack28(call1)
	n28b := pack28(call2)
	if n28a < 0 || n28b < 0 {
		return [77]int8{}, 0, false
	}

	var bits [77]int8
	writeBits(&bits, 0, 28, n28a)
	writeBits(&bits, 28, 1, ipa)
	writeBits(&bits, 29, 28, n28b)
	writeBits(&bits, 57, 1, ipb)

	// Parse remaining parts.
	ir := 0
	igrid4 := 0

	if len(parts) == 2 {
		// "CALL1 CALL2" (no grid/report).
		igrid4 = maxgrid4 + 1
	} else if len(parts) >= 3 {
		// Check for R prefix on grid/report.
		startIdx := 2
		rest := strings.Join(parts[2:], " ")

		if parts[2] == "R" && len(parts) >= 4 {
			// "CALL1 CALL2 R GRID" or "CALL1 CALL2 R -10"
			ir = 1
			startIdx = 3
			rest = strings.Join(parts[3:], " ")
		} else if len(parts) == 3 {
			// Could be "R-10", "R+10", "RRR", "RR73", "73", "-10", "+10", "GRID"
			p2 := parts[2]
			if strings.HasPrefix(p2, "R-") || strings.HasPrefix(p2, "R+") {
				ir = 1
				rest = p2[1:] // strip leading 'R'
			}
		}

		last := parts[len(parts)-1]
		if startIdx == len(parts)-1 {
			if g, ok := grid4ToInt(last); ok {
				// Grid locator.
				igrid4 = g
				goto doneGrid
			}
		}

		// Check for report or signoff.
		switch rest {
		case "RRR":
			igrid4 = maxgrid4 + 2
		case "RR73":
			igrid4 = maxgrid4 + 3
		case "73":
			igrid4 = maxgrid4 + 4
		default:
			// Try to parse as report: "-NN", "+NN"
			if rpt, err := strconv.Atoi(rest); err == nil && rpt >= -50 && rpt <= 50 {
				irpt := rpt + 35
				if irpt < 5 {
					irpt += 101
				}
				igrid4 = maxgrid4 + irpt
			} else {
				return [77]int8{}, 0, false
			}
		}
	}
doneGrid:

	writeBits(&bits, 58, 1, ir)
	writeBits(&bits, 59, 15, igrid4)
	writeBits(&bits, 74, 3, i3)

	return bits, i3, true
}

// packNonStandardCall encodes a message with a non-standard callsign (i3=4).
func packNonStandardCall(msg string) ([77]int8, bool) {
	parts := strings.Fields(msg)
	if len(parts) < 2 || len(parts) > 3 {
		return [77]int8{}, false
	}

	var n12, n58, iflip, nrpt, icq int

	if len(parts) == 2 {
		if parts[0] == "CQ" {
			icq = 1
			call2 := strings.TrimPrefix(parts[1], "<")
			call2 = strings.TrimSuffix(call2, ">")
			n58 = packNonStdCall58(call2)
			if n58 < 0 {
				return [77]int8{}, false
			}
			n12 = 0
			SaveHashCall(call2)
		} else {
			if strings.HasPrefix(parts[1], "<") && strings.HasSuffix(parts[1], ">") {
				iflip = 0
				call2 := strings.TrimPrefix(parts[1], "<")
				call2 = strings.TrimSuffix(call2, ">")
				n58 = packNonStdCall58(call2)
				if n58 < 0 {
					return [77]int8{}, false
				}
				n12 = hashCall(parts[0], 12) & 0xFFF
				SaveHashCall(call2)
			} else if strings.HasPrefix(parts[0], "<") && strings.HasSuffix(parts[0], ">") {
				iflip = 1
				call1 := strings.TrimPrefix(parts[0], "<")
				call1 = strings.TrimSuffix(call1, ">")
				n58 = packNonStdCall58(call1)
				if n58 < 0 {
					return [77]int8{}, false
				}
				n12 = hashCall(parts[1], 12) & 0xFFF
				SaveHashCall(call1)
			} else {
				return [77]int8{}, false
			}
		}
	} else if len(parts) == 3 {
		if strings.HasPrefix(parts[1], "<") && strings.HasSuffix(parts[1], ">") {
			iflip = 0
			call2 := strings.TrimPrefix(parts[1], "<")
			call2 = strings.TrimSuffix(call2, ">")
			n58 = packNonStdCall58(call2)
			if n58 < 0 {
				return [77]int8{}, false
			}
			n12 = hashCall(parts[0], 12) & 0xFFF
			SaveHashCall(call2)
		} else if strings.HasPrefix(parts[0], "<") && strings.HasSuffix(parts[0], ">") {
			iflip = 1
			call1 := strings.TrimPrefix(parts[0], "<")
			call1 = strings.TrimSuffix(call1, ">")
			n58 = packNonStdCall58(call1)
			if n58 < 0 {
				return [77]int8{}, false
			}
			n12 = hashCall(parts[1], 12) & 0xFFF
			SaveHashCall(call1)
		} else {
			return [77]int8{}, false
		}

		switch parts[2] {
		case "RRR":
			nrpt = 1
		case "RR73":
			nrpt = 2
		case "73":
			nrpt = 3
		default:
			return [77]int8{}, false
		}
	}

	var bits [77]int8
	writeBits(&bits, 0, 12, n12)
	writeBits(&bits, 12, 58, n58)
	writeBits(&bits, 70, 1, iflip)
	writeBits(&bits, 71, 2, nrpt)
	writeBits(&bits, 73, 1, icq)
	writeBits(&bits, 74, 3, 4)

	return bits, true
}

// packNonStdCall58 encodes an 11-char non-standard callsign into 58 bits.
func packNonStdCall58(callsign string) int {
	cs := strings.ToUpper(callsign)
	if len(cs) > 11 {
		cs = cs[:11]
	}

	var n58 int64
	for i := 0; i < 11; i++ {
		c := ' '
		if i < len(cs) {
			c = rune(cs[i])
		}
		idx := strings.IndexRune(c38set, c)
		if idx < 0 {
			return -1
		}
		n58 = n58*38 + int64(idx)
	}
	return int(n58)
}

// packRTTYContest encodes an ARRL RTTY Contest message (i3=3).
func packRTTYContest(msg string) ([77]int8, bool) {
	parts := strings.Fields(msg)
	if len(parts) < 4 {
		return [77]int8{}, false
	}

	itu := 0
	start := 0
	if strings.HasPrefix(msg, "TU; ") {
		itu = 1
		start = 1
	}

	if len(parts)-start < 4 {
		return [77]int8{}, false
	}

	p := parts[start:]
	call1 := p[0]
	call2 := p[1]

	if !isStdCall(call1) || !isStdCall(call2) {
		return [77]int8{}, false
	}

	n28a := pack28(call1)
	n28b := pack28(call2)
	if n28a < 0 || n28b < 0 {
		return [77]int8{}, false
	}

	// Find R prefix and report.
	ir := 0
	idx := 2
	if p[2] == "R" {
		ir = 1
		idx = 3
	}
	if len(p) <= idx {
		return [77]int8{}, false
	}

	// Parse report (5N9 format: 5 followed by digit 0-9 followed by 9).
	report := p[idx]
	if len(report) != 3 || report[0] != '5' || report[2] != '9' {
		return [77]int8{}, false
	}
	irpt := int(report[1] - '0' - 2)
	if irpt < 0 || irpt > 7 {
		return [77]int8{}, false
	}

	// Parse exchange (serial number or state).
	idx++
	if len(p) <= idx {
		return [77]int8{}, false
	}
	exch := p[idx]
	nexch := 0
	if n, err := strconv.Atoi(exch); err == nil && n >= 1 && n <= 7999 {
		nexch = n // serial number
	} else {
		// Try state/province abbreviation.
		for i, s := range cmult {
			if s == exch {
				nexch = 8000 + i + 1
				break
			}
		}
		if nexch == 0 {
			return [77]int8{}, false
		}
	}

	var bits [77]int8
	writeBits(&bits, 0, 1, itu)
	writeBits(&bits, 1, 28, n28a)
	writeBits(&bits, 29, 28, n28b)
	writeBits(&bits, 57, 1, ir)
	writeBits(&bits, 58, 3, irpt)
	writeBits(&bits, 61, 13, nexch)
	writeBits(&bits, 74, 3, 3)

	return bits, true
}

// packEUVHF encodes an EU VHF Contest message (i3=5).
// Format: "<CALL1> <CALL2> [R] 5NNNNNN GRID6"
func packEUVHF(msg string) ([77]int8, bool) {
	parts := strings.Fields(msg)
	if len(parts) < 4 || len(parts) > 5 {
		return [77]int8{}, false
	}

	if !strings.HasPrefix(parts[0], "<") || !strings.HasSuffix(parts[0], ">") ||
		!strings.HasPrefix(parts[1], "<") || !strings.HasSuffix(parts[1], ">") {
		return [77]int8{}, false
	}

	call1 := strings.TrimPrefix(parts[0], "<")
	call1 = strings.TrimSuffix(call1, ">")
	call2 := strings.TrimPrefix(parts[1], "<")
	call2 = strings.TrimSuffix(call2, ">")

	// R prefix.
	ir := 0
	idx := 2
	if parts[2] == "R" && len(parts) >= 5 {
		ir = 1
		idx = 3
	}

	if len(parts) <= idx {
		return [77]int8{}, false
	}
	nx, err := strconv.Atoi(parts[idx])
	if err != nil || nx < 520001 || nx > 594095 {
		return [77]int8{}, false
	}

	if len(parts) <= idx+1 {
		return [77]int8{}, false
	}
	grid6 := parts[idx+1]
	igrid6, ok := grid6ToInt(grid6)
	if !ok {
		return [77]int8{}, false
	}

	irpt := nx/10000 - 52
	iserial := nx % 10000
	if iserial > 2047 {
		iserial = 2047
	}

	n12 := hashCall(call1, 12) & 0xFFF
	n22 := hashCall(call2, 22) & 0x3FFFFF
	SaveHashCall(call1)
	SaveHashCall(call2)

	var bits [77]int8
	writeBits(&bits, 0, 12, n12)
	writeBits(&bits, 12, 22, n22)
	writeBits(&bits, 34, 1, ir)
	writeBits(&bits, 35, 3, irpt)
	writeBits(&bits, 38, 11, iserial)
	writeBits(&bits, 49, 25, igrid6)
	writeBits(&bits, 74, 3, 5)

	return bits, true
}

// packFreeText encodes a free-text message (i3=0, n3=0).
func packFreeText(msg string) ([77]int8, bool) {
	msg = strings.TrimSpace(msg)
	if len(msg) > 13 {
		return [77]int8{}, false
	}

	for _, c := range msg {
		if !strings.ContainsRune(freeTextChars, c) {
			return [77]int8{}, false
		}
	}

	n := big.NewInt(0)
	for i := 0; i < 13; i++ {
		c := ' '
		if i < len(msg) {
			c = rune(msg[i])
		}
		idx := strings.IndexRune(freeTextChars, c)
		if idx < 0 {
			return [77]int8{}, false
		}
		n.Mul(n, big.NewInt(42))
		n.Add(n, big.NewInt(int64(idx)))
	}

	var bits [77]int8
	for i := 70; i >= 0; i-- {
		bits[i] = int8(new(big.Int).And(n, big.NewInt(1)).Int64())
		n.Rsh(n, 1)
	}

	writeBits(&bits, 71, 3, 0)
	writeBits(&bits, 74, 3, 0)
	return bits, true
}

// C77ToBits converts a 77-character '0'/'1' string to a [77]int8 array.
func C77ToBits(c77 string) ([77]int8, bool) {
	var bits [77]int8
	if len(c77) != 77 {
		return bits, false
	}
	for i := 0; i < 77; i++ {
		if c77[i] == '0' {
			bits[i] = 0
		} else if c77[i] == '1' {
			bits[i] = 1
		} else {
			return bits, false
		}
	}
	return bits, true
}

// Pack77RoundTrip is a helper for testing.
func Pack77RoundTrip(msg string) (string, bool) {
	bits, _, _, ok := Pack77(msg)
	if !ok {
		return "", false
	}
	c77 := BitsToC77(bits)
	unpacked, ok2 := Unpack77(c77)
	return unpacked, ok2
}
