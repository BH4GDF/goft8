package dsp

/*
#cgo pkg-config: fftw3
#include <fftw3.h>
#include <stdlib.h>
*/
import "C"
import (
	"runtime"
	"sync"
	"unsafe"
)

// fftwRealFFTInto computes a forward real-to-complex FFT via FFTW3.
// x has length n (n must be even). dst must have length >= n/2+1.
func fftwRealFFTInto(dst []complex128, x []float64, n int) {
	p := getFFTWRealPlan(n)
	defer putFFTWRealPlan(n, p)
	p.exec(dst, x)
}

// fftwCmplxFFTInto computes a forward complex-to-complex FFT via FFTW3.
func fftwCmplxFFTInto(dst, x []complex128) {
	n := len(x)
	p := getFFTWForwardPlan(n)
	defer putFFTWForwardPlan(n, p)
	p.exec(dst, x)
}

// fftwCmplxIFFTInto computes an inverse complex-to-complex FFT via FFTW3 (unnormalized).
func fftwCmplxIFFTInto(dst, x []complex128) {
	n := len(x)
	p := getFFTWBackwardPlan(n)
	defer putFFTWBackwardPlan(n, p)
	p.exec(dst, x)
}

// ── FFTW real-to-complex plan ─────────────────────────────────────────────

type fftwRealPlan struct {
	n      int
	plan   C.fftw_plan
	inPtr  unsafe.Pointer // fftw_malloc'd double[n]
	outPtr unsafe.Pointer // fftw_malloc'd fftw_complex[n/2+1]
}

var fftwPlanMu sync.Mutex

func newFFTWRealPlan(n int) *fftwRealPlan {
	inPtr := C.fftw_malloc(C.size_t(n * C.sizeof_double))
	outSize := (n/2 + 1) * C.sizeof_fftw_complex
	outPtr := C.fftw_malloc(C.size_t(outSize))
	fftwPlanMu.Lock()
	plan := C.fftw_plan_dft_r2c_1d(
		C.int(n),
		(*C.double)(inPtr),
		(*C.fftw_complex)(outPtr),
		C.FFTW_ESTIMATE,
	)
	fftwPlanMu.Unlock()
	p := &fftwRealPlan{
		n:      n,
		plan:   plan,
		inPtr:  inPtr,
		outPtr: outPtr,
	}
	runtime.SetFinalizer(p, (*fftwRealPlan).destroy)
	return p
}

func (p *fftwRealPlan) exec(dst []complex128, x []float64) {
	// Copy input into FFTW-aligned buffer.
	inBuf := unsafe.Slice((*float64)(p.inPtr), p.n)
	copy(inBuf, x)
	// Execute.
	C.fftw_execute_dft_r2c(p.plan,
		(*C.double)(p.inPtr),
		(*C.fftw_complex)(p.outPtr))
	// Copy output back to Go slice.
	outN := p.n/2 + 1
	outBuf := unsafe.Slice((*complex128)(p.outPtr), outN)
	copy(dst, outBuf)
}

func (p *fftwRealPlan) destroy() {
	if p.plan != nil {
		C.fftw_destroy_plan(p.plan)
		p.plan = nil
	}
	if p.inPtr != nil {
		C.fftw_free(p.inPtr)
		p.inPtr = nil
	}
	if p.outPtr != nil {
		C.fftw_free(p.outPtr)
		p.outPtr = nil
	}
}

var (
	fftwRealPlanPool3840   = newFixedPoolWithEvict[*fftwRealPlan](128, (*fftwRealPlan).destroy)
	fftwRealPlanPool192000 = newFixedPoolWithEvict[*fftwRealPlan](128, (*fftwRealPlan).destroy)
	fftwRealPlanPool3200   = newFixedPoolWithEvict[*fftwRealPlan](128, (*fftwRealPlan).destroy)
	fftwRealPlanPool180000 = newFixedPoolWithEvict[*fftwRealPlan](128, (*fftwRealPlan).destroy)
)

func getFFTWRealPlan(n int) *fftwRealPlan {
	switch n {
	case 3840:
		return fftwRealPlanPool3840.get(func() *fftwRealPlan { return newFFTWRealPlan(3840) })
	case 192000:
		return fftwRealPlanPool192000.get(func() *fftwRealPlan { return newFFTWRealPlan(192000) })
	case 3200:
		return fftwRealPlanPool3200.get(func() *fftwRealPlan { return newFFTWRealPlan(3200) })
	case 180000:
		return fftwRealPlanPool180000.get(func() *fftwRealPlan { return newFFTWRealPlan(180000) })
	default:
		return newFFTWRealPlan(n)
	}
}

func putFFTWRealPlan(n int, p *fftwRealPlan) {
	switch n {
	case 3840:
		fftwRealPlanPool3840.put(p)
	case 192000:
		fftwRealPlanPool192000.put(p)
	case 3200:
		fftwRealPlanPool3200.put(p)
	case 180000:
		fftwRealPlanPool180000.put(p)
	default:
		p.destroy()
	}
}

// ── FFTW complex-to-complex plan ──────────────────────────────────────────

type fftwCmplxPlan struct {
	n      int
	plan   C.fftw_plan
	inPtr  unsafe.Pointer // fftw_malloc'd fftw_complex[n]
	outPtr unsafe.Pointer // fftw_malloc'd fftw_complex[n]
}

func newFFTWForwardPlan(n int) *fftwCmplxPlan {
	return newFFTWCmplxPlan(n, C.FFTW_FORWARD)
}

func newFFTWBackwardPlan(n int) *fftwCmplxPlan {
	return newFFTWCmplxPlan(n, C.FFTW_BACKWARD)
}

func newFFTWCmplxPlan(n int, sign C.int) *fftwCmplxPlan {
	size := C.size_t(n * C.sizeof_fftw_complex)
	inPtr := C.fftw_malloc(size)
	outPtr := C.fftw_malloc(size)
	fftwPlanMu.Lock()
	plan := C.fftw_plan_dft_1d(
		C.int(n),
		(*C.fftw_complex)(inPtr),
		(*C.fftw_complex)(outPtr),
		sign,
		C.FFTW_ESTIMATE,
	)
	fftwPlanMu.Unlock()
	p := &fftwCmplxPlan{
		n:      n,
		plan:   plan,
		inPtr:  inPtr,
		outPtr: outPtr,
	}
	runtime.SetFinalizer(p, (*fftwCmplxPlan).destroy)
	return p
}

func (p *fftwCmplxPlan) exec(dst, x []complex128) {
	inBuf := unsafe.Slice((*complex128)(p.inPtr), p.n)
	copy(inBuf, x)
	C.fftw_execute_dft(p.plan,
		(*C.fftw_complex)(p.inPtr),
		(*C.fftw_complex)(p.outPtr))
	outBuf := unsafe.Slice((*complex128)(p.outPtr), p.n)
	copy(dst, outBuf)
}

func (p *fftwCmplxPlan) destroy() {
	if p.plan != nil {
		C.fftw_destroy_plan(p.plan)
		p.plan = nil
	}
	if p.inPtr != nil {
		C.fftw_free(p.inPtr)
		p.inPtr = nil
	}
	if p.outPtr != nil {
		C.fftw_free(p.outPtr)
		p.outPtr = nil
	}
}

var (
	fftwForwardPlanPool3840   = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwForwardPlanPool1920   = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwForwardPlanPool96000  = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwForwardPlanPool3200   = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwForwardPlanPool180000 = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)

	fftwBackwardPlanPool3840   = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwBackwardPlanPool1920   = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwBackwardPlanPool96000  = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwBackwardPlanPool3200   = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
	fftwBackwardPlanPool180000 = newFixedPoolWithEvict[*fftwCmplxPlan](128, (*fftwCmplxPlan).destroy)
)

func getFFTWForwardPlan(n int) *fftwCmplxPlan {
	switch n {
	case 3840:
		return fftwForwardPlanPool3840.get(func() *fftwCmplxPlan { return newFFTWForwardPlan(3840) })
	case 1920:
		return fftwForwardPlanPool1920.get(func() *fftwCmplxPlan { return newFFTWForwardPlan(1920) })
	case 96000:
		return fftwForwardPlanPool96000.get(func() *fftwCmplxPlan { return newFFTWForwardPlan(96000) })
	case 3200:
		return fftwForwardPlanPool3200.get(func() *fftwCmplxPlan { return newFFTWForwardPlan(3200) })
	case 180000:
		return fftwForwardPlanPool180000.get(func() *fftwCmplxPlan { return newFFTWForwardPlan(180000) })
	default:
		return newFFTWForwardPlan(n)
	}
}

func putFFTWForwardPlan(n int, p *fftwCmplxPlan) {
	switch n {
	case 3840:
		fftwForwardPlanPool3840.put(p)
	case 1920:
		fftwForwardPlanPool1920.put(p)
	case 96000:
		fftwForwardPlanPool96000.put(p)
	case 3200:
		fftwForwardPlanPool3200.put(p)
	case 180000:
		fftwForwardPlanPool180000.put(p)
	default:
		p.destroy()
	}
}

func getFFTWBackwardPlan(n int) *fftwCmplxPlan {
	switch n {
	case 3840:
		return fftwBackwardPlanPool3840.get(func() *fftwCmplxPlan { return newFFTWBackwardPlan(3840) })
	case 1920:
		return fftwBackwardPlanPool1920.get(func() *fftwCmplxPlan { return newFFTWBackwardPlan(1920) })
	case 96000:
		return fftwBackwardPlanPool96000.get(func() *fftwCmplxPlan { return newFFTWBackwardPlan(96000) })
	case 3200:
		return fftwBackwardPlanPool3200.get(func() *fftwCmplxPlan { return newFFTWBackwardPlan(3200) })
	case 180000:
		return fftwBackwardPlanPool180000.get(func() *fftwCmplxPlan { return newFFTWBackwardPlan(180000) })
	default:
		return newFFTWBackwardPlan(n)
	}
}

func putFFTWBackwardPlan(n int, p *fftwCmplxPlan) {
	switch n {
	case 3840:
		fftwBackwardPlanPool3840.put(p)
	case 1920:
		fftwBackwardPlanPool1920.put(p)
	case 96000:
		fftwBackwardPlanPool96000.put(p)
	case 3200:
		fftwBackwardPlanPool3200.put(p)
	case 180000:
		fftwBackwardPlanPool180000.put(p)
	default:
		p.destroy()
	}
}
