package goft8

import (
	"errors"
	"strings"
)

// MsgType identifies the structural class of a decoded FT8 message.
type MsgType int

const (
	MsgUnknown   MsgType = iota
	MsgCQ                // "CQ [call] [grid]"
	MsgStandard          // "CALL1 CALL2 GRID"
	MsgReport            // "CALL1 CALL2 [-NN | R-NN | +NN | R+NN]"
	MsgRRR               // "CALL1 CALL2 RRR"
	MsgRR73              // "CALL1 CALL2 RR73"
	Msg73                // "CALL1 CALL2 73"
	MsgFreeText          // 13-character free text
	MsgTelemetry         // 18-hex-character telemetry
)

// Message holds the parsed fields of a decoded FT8 message.
type Message struct {
	Raw    string  // original text
	Type   MsgType // structural class
	Call1  string  // first callsign or "CQ" marker
	Call2  string  // second callsign
	Grid   string  // 4-char grid, if present
	Report string  // signal report, if present
}

// ParseMessage attempts to parse a decoded FT8 message into fields.
// Returns nil and a non-nil error on malformed input.
func ParseMessage(raw string) (*Message, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("goft8: empty message")
	}

	m := &Message{Raw: raw}

	if isTelemetry(trimmed) {
		m.Type = MsgTelemetry
		m.Call1 = trimmed
		return m, nil
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return nil, errors.New("goft8: no tokens")
	}

	if fields[0] == "CQ" {
		m.Type = MsgCQ
		m.Call1 = "CQ"
		switch len(fields) {
		case 1:
			// bare "CQ" — unusual but accept
		case 2:
			m.Call2 = fields[1]
		case 3:
			// "CQ <qualifier> CALL" (e.g. "CQ DX W1AW") or "CQ CALL GRID"
			if isGrid(fields[2]) {
				m.Call2 = fields[1]
				m.Grid = fields[2]
			} else {
				m.Call2 = fields[2]
			}
		case 4:
			// "CQ <qualifier> CALL GRID"
			m.Call2 = fields[2]
			if isGrid(fields[3]) {
				m.Grid = fields[3]
			}
		default:
			return m, nil
		}
		return m, nil
	}

	if len(fields) >= 2 {
		m.Call1 = fields[0]
		m.Call2 = fields[1]
	}

	if len(fields) == 2 {
		m.Type = MsgStandard
		return m, nil
	}

	if len(fields) >= 3 {
		tok := fields[2]
		switch tok {
		case "RRR":
			m.Type = MsgRRR
			return m, nil
		case "RR73":
			m.Type = MsgRR73
			return m, nil
		case "73":
			m.Type = Msg73
			return m, nil
		}
		if isReport(tok) {
			m.Type = MsgReport
			m.Report = tok
			return m, nil
		}
		if isGrid(tok) {
			m.Type = MsgStandard
			m.Grid = tok
			return m, nil
		}
	}

	// Fall back to free-text classification when nothing else fits.
	m.Type = MsgFreeText
	m.Call1 = ""
	m.Call2 = ""
	return m, nil
}

// isGrid reports whether s looks like a Maidenhead 4-character grid
// square (e.g. "FN31"). 6-char extended grids are also accepted.
func isGrid(s string) bool {
	if len(s) != 4 && len(s) != 6 {
		return false
	}
	if s[0] < 'A' || s[0] > 'R' || s[1] < 'A' || s[1] > 'R' {
		return false
	}
	if s[2] < '0' || s[2] > '9' || s[3] < '0' || s[3] > '9' {
		return false
	}
	if len(s) == 6 {
		if s[4] < 'A' || s[4] > 'X' || s[5] < 'A' || s[5] > 'X' {
			return false
		}
	}
	return true
}

// isReport reports whether s is an FT8 signal report token such as
// "-12", "+03", "R-08", or "R+15".
func isReport(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] == 'R' {
		return isReport(s[1:])
	}
	if s[0] != '+' && s[0] != '-' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isTelemetry reports whether s is an 18-hex-character telemetry blob.
func isTelemetry(s string) bool {
	if len(s) != 18 {
		return false
	}
	for i := 0; i < 18; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
