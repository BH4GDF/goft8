// hash.go — Callsign hash tables for FT8 non-standard callsign recovery.
//
// Port of MSHV's hash table management (save_hash_call, hash10, hash12, hash22).
// Maintains bidirectional 10-bit, 12-bit, and 22-bit hash tables.

package goft8

import (
	"strings"
	"sync"
)

const (
	maxHash10 = 1024
	maxHash12 = 4096
	maxHash22 = 4194304
)

var (
	hashMu     sync.RWMutex
	hash10Table [maxHash10]string
	hash12Table [maxHash12]string
	hash22Table map[int]string // 22-bit hash space is too large for array
)

func init() {
	hash22Table = make(map[int]string)
}

// SaveHashCall saves a callsign to the hash tables (10, 12, and 22-bit).
// Call this after successfully packing or decoding a non-standard callsign.
func SaveHashCall(callsign string) {
	cs := strings.TrimSpace(strings.ToUpper(callsign))
	if cs == "" || isStdCall(cs) {
		return
	}

	hashMu.Lock()
	defer hashMu.Unlock()

	n10 := hashCall(cs, 10)
	n12 := hashCall(cs, 12)
	n22 := hashCall(cs, 22)

	if n10 >= 0 && n10 < maxHash10 {
		hash10Table[n10] = cs
	}
	if n12 >= 0 && n12 < maxHash12 {
		hash12Table[n12] = cs
	}
	if n22 >= 0 && n22 < maxHash22 {
		hash22Table[n22] = cs
	}
}

// LookupHash10 looks up a 10-bit hash index and returns the callsign if known.
func LookupHash10(n10 int) string {
	if n10 < 0 || n10 >= maxHash10 {
		return ""
	}
	hashMu.RLock()
	defer hashMu.RUnlock()
	return hash10Table[n10]
}

// LookupHash12 looks up a 12-bit hash index and returns the callsign if known.
func LookupHash12(n12 int) string {
	if n12 < 0 || n12 >= maxHash12 {
		return ""
	}
	hashMu.RLock()
	defer hashMu.RUnlock()
	return hash12Table[n12]
}

// LookupHash22 looks up a 22-bit hash index and returns the callsign if known.
func LookupHash22(n22 int) string {
	if n22 < 0 || n22 >= maxHash22 {
		return ""
	}
	hashMu.RLock()
	defer hashMu.RUnlock()
	return hash22Table[n22]
}

// Hash10 looks up a 10-bit hash index and returns the callsign if known.
// Returns "<...>" if not found.
func Hash10(n10 int) string {
	if cs := LookupHash10(n10); cs != "" {
		return cs
	}
	return "<...>"
}

// Hash12 looks up a 12-bit hash index and returns the callsign if known.
// Returns "<...>" if not found.
func Hash12(n12 int) string {
	if cs := LookupHash12(n12); cs != "" {
		return cs
	}
	return "<...>"
}

// Hash22 looks up a 22-bit hash index and returns the callsign if known.
// Returns "<...>" if not found.
func Hash22(n22 int) string {
	if cs := LookupHash22(n22); cs != "" {
		return cs
	}
	return "<...>"
}

// ResetHashTables clears all hash tables.
func ResetHashTables() {
	hashMu.Lock()
	defer hashMu.Unlock()
	for i := range hash10Table {
		hash10Table[i] = ""
	}
	for i := range hash12Table {
		hash12Table[i] = ""
	}
	hash22Table = make(map[int]string)
}
