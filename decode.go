// decode.go implements the FT8 decode pipeline.
//
// Port of subroutine ft8b from wsjt-wsjtx/lib/ft8/ft8b.f90
// and the iterative loop from wsjt-wsjtx/lib/ft8_decode.f90 lines 160–239.

package goft8

import (
	"github.com/bh4gdf/goft8/internal/decode"
	"github.com/bh4gdf/goft8/internal/encode"
	"github.com/bh4gdf/goft8/internal/ldpc"
	"github.com/bh4gdf/goft8/internal/protocol"
	ft8params "github.com/bh4gdf/goft8/params"
	"math"
	"strings"
	"sync"
)

// ── Type definitions ────────────────────────────────────────────────────────

// DecodeParams holds tunable parameters for the FT8 decoder.
type DecodeParams struct {
	// Depth controls the OSD search depth: 1=BP only, 2=BP+OSD(0), 3=BP+OSD(2).
	Depth int
	// APEnabled enables a-priori (AP) decoding passes.
	APEnabled bool
	// APCQOnly restricts AP decoding to CQ-only a-priori information.
	APCQOnly bool
	// APWidth is the frequency window (Hz) within which AP types ≥3 are applied.
	APWidth float64
	// MyCall is the operator's callsign (used for AP types 2–6). Empty = not set.
	MyCall string
	// DxCall is the DX station's callsign (used for AP types 3–6). Empty = not set.
	DxCall string
	// NfQSO is the nominal QSO frequency (Hz) for AP frequency guard.
	// AP types ≥3 are only tried if |f1 − NfQSO| ≤ APWidth.
	// Set to 0 to disable the frequency guard.
	NfQSO float64
	// MaxPasses is the number of subtraction passes for DecodeIterative (default 3).
	MaxPasses int
	// UseF32LDPC selects the float32 LDPC decoder variant (matching Fortran precision).
	UseF32LDPC bool
	// QSOProgress controls AP type dispatch (0–5). 0=calling CQ, 1=Rx1, 2=Rx2,
	// 3=Rx3 (report), 4=Tx3 (send report), 5=Tx4/Tx5/Tx6 (signoff).
	// Default 0 matches current behavior.
	QSOProgress int
	// Metric selects the bit-metric/hard-sync variant. 1=default, 2=aggressive.
	// In MSHV pass 1 uses 1, passes 2–3 use 2. Default 0 means auto (1).
	Metric int
	// ContestType selects contest-mode AP layout. 0=standard, 2=NA/EU VHF,
	// 3=ARRL Field Day, 4=ARRL RTTY RU. Default 0.
	ContestType int
	// Interval selects 3-interval decoding mode. 0=normal, 1=first, 2=second, 3=third.
	// Default 0 disables 3-interval logic.
	Interval int
	// Workers enables goroutine concurrency. 0 = serial, >0 = parallel with that many workers.
	Workers int
	// OnCandidate is called synchronously each time a new signal is
	// successfully decoded during iterative decoding. Keep it fast.
	OnCandidate func(DecodeCandidate)
	// AdaptiveOSD enables adaptive OSD depth based on signal priority.
	// When true, OSD depth is dynamically adjusted: non-AP passes use
	// ndeep=3 (order-1 + pre1 + pre2), AP passes near QSO frequency use
	// ndeep=4 (order-2 + pre1 + pre2). Only effective when Depth >= 3.
	// Matches MSHV ws300rc1 adaptive OSD strategy.
	AdaptiveOSD bool
}

// DecodeCandidate is the result of decoding one FT8 signal candidate.
type DecodeCandidate struct {
	// Message is the decoded text (up to 37 characters).
	Message string
	// Freq is the refined carrier frequency estimate (Hz).
	Freq float64
	// DT is the time offset relative to the nominal start of the 15-second period (seconds).
	DT float64
	// SNR is the estimated signal-to-noise ratio (dB, 2500 Hz bandwidth).
	SNR float64
	// NHardErrors is the number of hard-decision bit errors after decoding.
	NHardErrors int
	// Tones holds the 79 channel tone indices (0–7) for subtracting the signal.
	Tones [ft8params.NN]int
	// APType indicates the a-priori decoding type used (0 = no AP).
	APType int
	// Pass is the 1-based iterative-decode pass on which this signal was
	// recovered. Zero means unset (e.g., a direct DecodeSingle call).
	Pass int
}

// CandidateFreq is a {frequency, DT} pair to try decoding.
// Identical to decode.Candidate from sync8.go.
type CandidateFreq = decode.Candidate

// DefaultDecodeParams returns sensible defaults matching WSJT-X ndepth=2.
func DefaultDecodeParams() DecodeParams {
	return DecodeParams{
		Depth:     2,
		APWidth:   25.0,
		MaxPasses: 5,
	}
}

// ── DecodeSingle ────────────────────────────────────────────────────────────

// DecodeSingle attempts to decode a single FT8 signal at the given frequency
// and time offset.
//
// Port of subroutine ft8b from wsjt-wsjtx/lib/ft8/ft8b.f90.
func DecodeSingle(
	dd []float32,
	ds *decode.Downsampler,
	f1 float64,
	xdt float64,
	newdat bool,
	params DecodeParams,
	xbase float64,
) (DecodeCandidate, bool) {
	twopi := 2.0 * math.Pi
	ndepth := params.Depth
	if ndepth < 1 {
		ndepth = 2
	}

	// ── Step 1: decode.Downsample to baseband (ft8b.f90 line 105) ──────────
	cd0 := ds.Downsample(dd, &newdat, f1)
	maxMag := 0.0
	for _, c := range cd0 {
		mag := math.Sqrt(real(c)*real(c) + imag(c)*imag(c))
		if mag > maxMag {
			maxMag = mag
		}
	}

	// ── Step 2: DT search ±10 samples (ft8b.f90 lines 108–116) ─────
	i0 := int(math.Round((xdt + 0.5) * ft8params.Fs2)) // ft8b.f90 line 108: i0=nint((xdt+0.5)*fs2)
	var ctwk [32]complex128
	smax := 0.0
	ibest := 0
	for idt := i0 - 10; idt <= i0+10; idt++ {
		sync := decode.Sync8d(cd0, idt, ctwk, 0)
		if sync > smax {
			smax = sync
			ibest = idt
		}
	}

	// ── Step 3: Frequency search ±2.5 Hz (ft8b.f90 lines 119–133) ──
	// MSHV: ±10 × 0.25 Hz = ±2.5 Hz, step 0.25 Hz (finer than WSJT-X 0.5 Hz)
	smax = 0.0
	delfbest := 0.0
	for ifr := -10; ifr <= 10; ifr++ {
		delf := float64(ifr) * 0.25
		dphi := twopi * delf * ft8params.Dt2
		phi := 0.0
		for i := 0; i < 32; i++ {
			sin, cos := math.Sincos(phi)
			ctwk[i] = complex(cos, sin)
			phi += dphi
		}
		sync := decode.Sync8d(cd0, ibest, ctwk, 1)
		if sync > smax {
			smax = sync
			delfbest = delf
		}
	}

	// ── Step 4: Frequency refinement (ft8b.f90 lines 134–137) ───────
	a := [5]float64{-delfbest, 0, 0, 0, 0}
	decode.TwkFreq1Into(cd0, cd0, ft8params.Fs2, a)
	f1 = f1 + delfbest

	// ── Step 5: Re-downsample at refined frequency (ft8b.f90 line 140)
	noNewdat := false
	cd0 = ds.Downsample(dd, &noNewdat, f1)

	// ── Step 6: Final DT search ±4 samples (ft8b.f90 lines 143–152)
	var ss [9]float64
	for idt := -4; idt <= 4; idt++ {
		ss[idt+4] = decode.Sync8d(cd0, ibest+idt, ctwk, 0)
	}
	smax = ss[0]
	imax := 0
	for i := 1; i < 9; i++ {
		if ss[i] > smax {
			smax = ss[i]
			imax = i
		}
	}
	ibest = imax - 4 + ibest
	xdt = float64(ibest-1) * ft8params.Dt2

	// ── Step 7: Symbol spectra (ft8b.f90 lines 154–161) ─────────────
	cs, s8 := decode.ComputeSymbolSpectra(cd0, ibest)

	// ── Step 8: Hard sync check (ft8b.f90 lines 163–180) ────────────
	// MSHV: syncmin=8 when ndepth<=2, syncmin=6 otherwise.
	// imetric parameter is reserved for future bit-metric variants (currently no-op).
	nsync := decode.HardSync(&s8)
	hsyncMin := 6
	if ndepth <= 2 {
		hsyncMin = 8
	}
	_ = params.Metric // reserved
	if nsync < hsyncMin {
		return DecodeCandidate{}, false
	}

	// ── Step 9: Soft metrics (ft8b.f90 lines 182–239) ───────────────
	// MSHV: 5 metric sets including bmete (best of a/b/c per position).
	bmeta, bmetb, bmetc, bmetd, bmete := decode.ComputeSoftMetrics(&cs)

	// Fortran: real llr(174), scalefac — multiply in float32 precision to
	// match. ldpc.DecodeLDPC re-truncates at entry for all other call sites.
	var llra, llrb, llrc, llrd, llre [ft8params.LDPCn]float64
	sf32 := float32(ft8params.ScaleFac)
	for i := 0; i < ft8params.LDPCn; i++ {
		llra[i] = float64(sf32 * float32(bmeta[i]))
		llrb[i] = float64(sf32 * float32(bmetb[i]))
		llrc[i] = float64(sf32 * float32(bmetc[i]))
		llrd[i] = float64(sf32 * float32(bmetd[i]))
		llre[i] = float64(sf32 * float32(bmete[i]))
	}

	// apmag = max(|llra|) * 1.1 (MSHV uses 1.1, WSJT-X uses 1.01)
	apmag := 0.0
	for i := 0; i < ft8params.LDPCn; i++ {
		if v := math.Abs(llra[i]); v > apmag {
			apmag = v
		}
	}
	apmag *= 1.1

	// ── Step 10: Decode passes (ft8b.f90 lines 254–462) ─────────────
	// Compute AP symbols from callsigns (ft8apset.f90)
	apsym := decode.ComputeAPSymbols(params.MyCall, params.DxCall)

	// Pass count: 5 regular + AP passes (MSHV ws300rc1 adds pass 5 with llre).
	// AP passes use llra/llrc alternating, up to 4 AP passes (9 total).
	npasses := 5
	if params.APEnabled {
		if params.APCQOnly {
			npasses = 6
		} else {
			qsoProg := params.QSOProgress
			if qsoProg < 0 || qsoProg > 5 {
				qsoProg = 0
			}
			npasses = 5 + decode.Nappasses2[qsoProg]
		}
	}

	for ipass := 1; ipass <= npasses; ipass++ {
		// Select LLR set (ft8b.f90 lines 266–269 + MSHV pass 5 + MSHV AP alternation)
		var llrz [ft8params.LDPCn]float64
		switch ipass {
		case 1:
			llrz = llra
		case 2:
			llrz = llrb
		case 3:
			llrz = llrc
		case 4:
			llrz = llrd
		case 5:
			llrz = llre // MSHV ws300rc1: best-of-a/b/c metric
		default:
			// AP passes: alternate llra (odd offset) and llrc (even offset).
			// MSHV: if mod(ipass-5,2)==1 → llra; if mod(ipass-5,2)==0 → llrc
			if (ipass-5)%2 == 1 {
				llrz = llra
			} else {
				llrz = llrc
			}
		}

		var apmask [ft8params.LDPCn]int8
		iaptype := 0

		// AP injection (ft8b.f90 lines 274–401, MSHV ws300rc1)
		if ipass > 5 {
			if params.APCQOnly {
				iaptype = 1
			} else {
				qsoProg := params.QSOProgress
				if qsoProg < 0 || qsoProg > 5 {
					qsoProg = 0
				}
				apIdx := (ipass - 6) / 2
				if apIdx < 4 {
					iaptype = decode.Naptypes2[qsoProg][apIdx]
				}
				if iaptype == 0 {
					continue
				}
			}

			// Guard: skip iaptype≥2 if mycall is unknown (ft8b.f90 line 296)
			if iaptype >= 2 && apsym[0] > 1 {
				continue
			}
			// Guard: skip iaptype≥3 if dxcall is unknown (ft8b.f90 line 298)
			if iaptype >= 3 && apsym[29] > 1 {
				continue
			}

			// Frequency guard for AP types ≥3 (ft8b.f90 line 293)
			if iaptype >= 3 && params.NfQSO > 0 && math.Abs(f1-params.NfQSO) > params.APWidth {
				continue
			}

			// Apply AP (contestType 0=standard, 2=EU VHF, 3=Field Day, 4=RTTY RU)
			decode.ApplyAP(&llrz, &apmask, iaptype, apsym, apmag, params.ContestType)
		}

		// OSD depth control (ft8b.f90 lines 403–412)
		// Fortran: maxosd=2 (default), then sequential if statements:
		//   ndepth=1 → maxosd=-1 (BP only)
		//   ndepth=2 → maxosd=0
		//   ndepth=3 → maxosd stays at 2 (the conditional block is a no-op)
		norder := 2
		maxosd := 2
		if ndepth == 1 {
			maxosd = -1 // BP only
		} else if ndepth == 2 {
			maxosd = 0 // uncoupled BP+OSD
		}
		// ndepth >= 3: maxosd stays at 2 (default)

		// Adaptive OSD depth (MSHV ws300rc1 strategy).
		// Non-AP passes: ndeep=3 (order-1 + pre1 + pre2) — deeper than default ndeep=2.
		// AP passes near QSO frequency: ndeep=4 (order-2 + pre1 + pre2).
		if params.AdaptiveOSD && ndepth >= 3 && maxosd >= 0 {
			norder = 3 // MSHV baseline for non-AP passes
			if ipass > 5 && iaptype > 0 {
				// AP pass: use deeper search when signal is near QSO frequency.
				if params.NfQSO > 0 && math.Abs(f1-params.NfQSO) <= params.APWidth {
					norder = 4 // MSHV QSO-priority
				}
			}
		}

		// LDPC decode (ft8b.f90 lines 413–418)
		var result ldpc.DecodeResult
		var ok bool
		if params.UseF32LDPC {
			result, ok = ldpc.DecodeLDPCF32(llrz, ft8params.LDPCk, maxosd, norder, apmask)
		} else {
			result, ok = ldpc.DecodeLDPC(llrz, ft8params.LDPCk, maxosd, norder, apmask)
		}
		if !ok {
			continue
		}
		if result.NHardErrors < 0 || result.NHardErrors > 36 {
			continue
		}

		// Reject all-zero codeword (ft8b.f90 line 423)
		allZero := true
		for _, b := range result.Codeword {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}

		// Extract message bits and validate (i3, n3) (ft8b.f90 lines 424–428)
		var msgBits [77]int8
		copy(msgBits[:], result.Message91[:77])
		c77 := protocol.BitsToC77(msgBits)

		// Parse i3 and n3 from bit positions 72–77 (ft8b.f90 lines 425–428)
		n3 := int(c77[71]-'0')<<2 | int(c77[72]-'0')<<1 | int(c77[73]-'0')
		i3 := int(c77[74]-'0')<<2 | int(c77[75]-'0')<<1 | int(c77[76]-'0')
		if i3 > 5 || (i3 == 0 && n3 > 6) {
			continue
		}
		if i3 == 0 && n3 == 2 {
			continue
		}

		// Unpack message (ft8b.f90 line 429)
		msg, unpkOK := protocol.Unpack77(c77)
		if !unpkOK {
			continue
		}

		// Generate tones for subtraction/SNR (ft8b.f90 line 432)
		itone := encode.GenFT8Tones(msgBits)

		// SNR estimation using spectrum baseline (ft8b.f90 / MSHV)
		xsig := 0.0
		for i := 0; i < ft8params.NN; i++ {
			s88 := s8[itone[i]][i] * 0.001
			xsig += s88 * s88
		}
		// Natural alignment with MSHV: input scaling, downsample input,
		// and sync8d correlation now match MSHV's scaling chain.
		// No empirical calibration factor needed.
		xbaseCal := xbase
		ratio := xsig/xbaseCal - 1.0
		// Protect against NaN/negative like MSHV's db() (arg ≤ 1.259e-10 → -99 dB)
		if ratio <= 1.259e-10 {
			ratio = 1.259e-10
		}
		xsnr := 10.0*math.Log10(ratio) - 36.0
		if nsync <= 10 && xsnr < -25.0 {
			return DecodeCandidate{}, false
		}
		if xsnr < -25.0 {
			xsnr = -25.0
		}
		if xsnr > 49.0 {
			xsnr = 49.0
		}

		return DecodeCandidate{
			Message:     msg,
			Freq:        f1,
			DT:          xdt,
			SNR:         xsnr,
			NHardErrors: result.NHardErrors,
			Tones:       itone,
			APType:      iaptype,
		}, true
	}

	return DecodeCandidate{}, false
}

// IntervalState holds cross-interval state for MSHV 3-interval decoding.
type IntervalState struct {
	decodes []intervalDecode
	dd      []float32
}

type intervalDecode struct {
	tones [ft8params.NN]int
	freq  float64
	dt    float64
}

// DecodeInterval decodes one interval of a 3-interval FT8 sequence.
// interval: 1=first, 2=second, 3=third.
// state: nil for interval 1; returned state from previous interval for 2 and 3.
// Returns decoded candidates and updated state.
func DecodeInterval(audio []float32, interval int, state *IntervalState, params DecodeParams, freqMin, freqMax float64) ([]DecodeCandidate, *IntervalState) {
	if interval < 1 || interval > 3 {
		return nil, state
	}

	dd := make([]float32, len(audio))
	copy(dd, audio)

	// Interval 2: subtract interval-1 decodes, save subtracted audio.
	if interval == 2 {
		if state == nil || len(state.decodes) == 0 {
			return nil, state
		}
		for _, d := range state.decodes {
			if d.dt-0.5 < 0.396 {
				decode.SubtractFT8(dd, d.tones, d.freq, d.dt)
			}
		}
		state = &IntervalState{decodes: state.decodes}
		state.dd = make([]float32, len(dd))
		copy(state.dd, dd)
	}

	// Interval 3: restore interval-2 subtracted audio, subtract remaining.
	if interval == 3 {
		if state == nil || len(state.dd) == 0 {
			return nil, state
		}
		copy(dd, state.dd)
		for _, d := range state.decodes {
			decode.SubtractFT8(dd, d.tones, d.freq, d.dt)
		}
	}

	p := params
	p.Interval = interval
	results := DecodeIterative(dd, p, freqMin, freqMax)

	if state == nil {
		state = &IntervalState{}
	}
	for _, r := range results {
		state.decodes = append(state.decodes, intervalDecode{
			tones: r.Tones,
			freq:  r.Freq,
			dt:    r.DT + 0.5,
		})
	}
	return results, state
}

// ── DecodeIterative ─────────────────────────────────────────────────────────

// DecodeIterative runs the full FT8 decode pipeline with iterative signal
// subtraction, matching WSJT-X's multi-pass approach.
//
// Port of the decode subroutine in wsjt-wsjtx/lib/ft8_decode.f90 lines 160–239.
func DecodeIterative(audio []float32, params DecodeParams, freqMin, freqMax float64) []DecodeCandidate {
	ndepth := params.Depth
	if ndepth < 1 {
		ndepth = 2
	}
	npass := 3
	if ndepth == 1 {
		npass = 2
	}
	// MSHV 3-interval: interval 1 uses 5 passes (n4pas3int=true).
	if params.Interval == 1 {
		npass = 5
	}
	if params.MaxPasses > 0 && params.MaxPasses < npass {
		npass = params.MaxPasses
	}

	// Work on a copy so subtraction doesn't modify the caller's audio.
	var ddArr [ft8params.NMAX]float32
	n := len(audio)
	if n > ft8params.NMAX {
		n = ft8params.NMAX
	}
	copy(ddArr[:n], audio[:n])
	dd := ddArr[:]

	nfa := int(freqMin)
	nfb := int(freqMax)

	var results []DecodeCandidate
	seen := make(map[string]bool)
	ndecodes := 0
	prevPassDecodes := 0

	for ipass := 0; ipass < npass; ipass++ {
		// ft8_decode.f90 lines 176–178
		// MSHV: syncmin = 1.33 base, 1.56 when ndepth<=2, 1.96 for 3-interval first pass.
		syncmin := 1.33
		if ndepth <= 2 {
			syncmin = 1.56
		}
		if params.Interval == 1 {
			syncmin = 1.96
		}

		// ft8_decode.f90 lines 179–191
		ndeep := ndepth
		if ipass == 0 && ndepth == 3 {
			ndeep = 2 // lighter OSD on first pass
		}

		// Early termination (ft8_decode.f90 lines 185, 189)
		// Pass 2: skip if no decodes at all yet
		if ipass == 1 && ndecodes == 0 {
			break
		}
		// Pass 3: skip if pass 2 added nothing
		if ipass == 2 && prevPassDecodes == 0 {
			break
		}

		// Resolve effective worker count once per pass.
		nw := decode.NumWorkers(params.Workers)

		// decode.Sync8 candidate search (ft8_decode.f90 lines 193–195)
		maxcand := 600
		candidates, sbase := decode.Sync8(ddArr, ft8params.NMAX, nfa, nfb, syncmin, 0, maxcand, nw)

		metric := 1
		if ipass >= 1 {
			metric = 2 // MSHV: pass 2+ uses imetric=2
		}
		passParams := DecodeParams{
			Depth:       ndeep,
			APEnabled:   params.APEnabled,
			APCQOnly:    params.APCQOnly,
			APWidth:     params.APWidth,
			MyCall:      params.MyCall,
			DxCall:      params.DxCall,
			NfQSO:       params.NfQSO,
			QSOProgress: params.QSOProgress,
			Metric:      metric,
			ContestType: params.ContestType,
			Interval:    params.Interval,
			OnCandidate: params.OnCandidate,
		}

		passDecodes := 0
		if nw > 1 {
			// Parallel path: worker pool. dd is read-only in DecodeSingle,
			// so workers share it directly without copying.
			type decodeJob struct {
				cand  decode.Candidate
				xbase float64
				idx   int
			}
			type decodeWorkerResult struct {
				result DecodeCandidate
				ok     bool
				msg    string
			}

			decoded := make([]decodeWorkerResult, len(candidates))

			// Precompute the 192000-point FFT once and share across workers.
			sharedDS := decode.NewDownsampler()
			newdatShared := true
			sharedDS.Downsample(dd, &newdatShared, candidates[0].Freq)

			jobs := make(chan decodeJob, len(candidates))
			var wg sync.WaitGroup
			for w := 0; w < nw; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ds := decode.CloneFrom(sharedDS)
					for job := range jobs {
						res, ok := DecodeSingle(dd, ds, job.cand.Freq, job.cand.DT, false, passParams, job.xbase)
						if ok {
							decoded[job.idx] = decodeWorkerResult{
								result: res,
								ok:     true,
								msg:    strings.TrimSpace(res.Message),
							}
						}
					}
				}()
			}

			for i, cand := range candidates {
				// Compute xbase from spectrum baseline for this candidate frequency.
				freqBin := int(math.Round(cand.Freq / 3.125))
				if freqBin < 0 {
					freqBin = 0
				}
				if freqBin >= ft8params.NH1 {
					freqBin = ft8params.NH1 - 1
				}
				xbase := math.Pow(10.0, 0.1*(sbase[freqBin]-40.0))
				jobs <- decodeJob{cand: cand, xbase: xbase, idx: i}
			}
			close(jobs)
			wg.Wait()

			var toSubtract []decode.SubtractSignal
			for i := range candidates {
				dr := decoded[i]
				if !dr.ok {
					continue
				}
				msg := dr.msg
				if seen[msg] {
					continue
				}
				result := dr.result
				// Adjust DT for display (ft8_decode.f90 line 210: xdt=xdt-0.5)
				result.DT -= 0.5

				seen[msg] = true
				ndecodes++
				passDecodes++
				result.Pass = ipass + 1
				if passParams.OnCandidate != nil {
					passParams.OnCandidate(result)
				}
				results = append(results, result)

				// Collect signal for batch subtraction at end of pass.
				// (All DecodeSingle calls in this pass already ran on the same dd.)
				toSubtract = append(toSubtract, decode.SubtractSignal{
					Tones: result.Tones,
					Freq:  result.Freq,
					DT:    result.DT + 0.5,
				})
			}

			// Batch subtract all decoded signals from this pass.
			for _, sig := range toSubtract {
				decode.SubtractFT8(dd, sig.Tones, sig.Freq, sig.DT)
			}
		} else {
			// Serial path
			ds := decode.NewDownsampler()
			first := true
			for _, cand := range candidates {
				newdat := first
				first = false

				// Compute xbase from spectrum baseline for this candidate frequency.
				freqBin := int(math.Round(cand.Freq / 3.125))
				if freqBin < 0 {
					freqBin = 0
				}
				if freqBin >= ft8params.NH1 {
					freqBin = ft8params.NH1 - 1
				}
				xbase := math.Pow(10.0, 0.1*(sbase[freqBin]-40.0))

				result, ok := DecodeSingle(dd, ds, cand.Freq, cand.DT, newdat, passParams, xbase)
				if !ok {
					continue
				}

				msg := strings.TrimSpace(result.Message)
				if seen[msg] {
					continue
				}

				// Adjust DT for display (ft8_decode.f90 line 210: xdt=xdt-0.5)
				result.DT -= 0.5

				seen[msg] = true
				ndecodes++
				passDecodes++
				result.Pass = ipass + 1
				if passParams.OnCandidate != nil {
					passParams.OnCandidate(result)
				}
				results = append(results, result)

				// Subtract decoded signal (ft8_decode.f90 line ~207 via ft8b line 435)
				// Use unadjusted DT for subtraction (result.DT has been adjusted, add 0.5 back)
				decode.SubtractFT8(dd, result.Tones, result.Freq, result.DT+0.5)
			}
		}

		prevPassDecodes = passDecodes
	}

	return results
}

// ── decode.Sync8FindCandidates ─────────────────────────────────────────────────────

// decode.Sync8FindCandidates searches for potential FT8 signals using the
// spectrogram-based sync8 algorithm.
//
// Wrapper around the research decode.Sync8() function.
func Sync8FindCandidates(audio []float32, freqMin, freqMax int, syncmin float64, nfqso, maxcand int, workers int) []CandidateFreq {
	var dd [ft8params.NMAX]float32
	n := len(audio)
	if n > ft8params.NMAX {
		n = ft8params.NMAX
	}
	copy(dd[:n], audio[:n])
	cands, _ := decode.Sync8(dd, n, freqMin, freqMax, syncmin, nfqso, maxcand, workers)
	return cands
}
