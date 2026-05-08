// Package fastjson decodes the official rinha-de-backend-2026 fraud-score
// payload directly into the values needed for vectorization, avoiding the
// reflection and per-field allocations of encoding/json.
//
// The parser accepts arbitrary key order and ignores unknown keys, but it
// rejects features outside the contract (string escapes, scientific
// notation, multi-line whitespace inside numbers). On any rejection the
// caller is expected to fall back to encoding/json.
package fastjson

import (
	"errors"
	"strconv"
	"unsafe"
)

var (
	ErrSyntax     = errors.New("fastjson: syntax error")
	ErrUnexpected = errors.New("fastjson: unexpected token")
)

type parser struct {
	data []byte
	pos  int
}

func (p *parser) skipWS() {
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return
		}
		p.pos++
	}
}

func (p *parser) eof() bool { return p.pos >= len(p.data) }

func (p *parser) peek() byte {
	if p.pos >= len(p.data) {
		return 0
	}
	return p.data[p.pos]
}

func (p *parser) expect(b byte) error {
	p.skipWS()
	if p.eof() || p.data[p.pos] != b {
		return ErrUnexpected
	}
	p.pos++
	return nil
}

// parseString returns a slice into the underlying buffer for the contents
// of the next JSON string. Backslash escapes are rejected — the official
// payload uses plain ASCII for all string fields.
func (p *parser) parseString() ([]byte, error) {
	p.skipWS()
	if p.eof() || p.data[p.pos] != '"' {
		return nil, ErrUnexpected
	}
	p.pos++
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == '"' {
			s := p.data[start:p.pos]
			p.pos++
			return s, nil
		}
		if c == '\\' {
			return nil, ErrSyntax
		}
		p.pos++
	}
	return nil, ErrSyntax
}

// parseNumber parses a JSON number into float64. Scientific notation is
// rejected to keep the hot path simple — the official contract never
// emits it. The captured byte slice is forwarded to strconv.ParseFloat
// to match encoding/json bit-for-bit; the unsafe.String conversion lets
// strconv read directly from the body buffer without allocating.
func (p *parser) parseNumber() (float64, error) {
	p.skipWS()
	start := p.pos
	if p.pos < len(p.data) && p.data[p.pos] == '-' {
		p.pos++
	}
	digitStart := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c < '0' || c > '9' {
			break
		}
		p.pos++
	}
	if p.pos == digitStart {
		return 0, ErrSyntax
	}
	if p.pos < len(p.data) && p.data[p.pos] == '.' {
		p.pos++
		fracStart := p.pos
		for p.pos < len(p.data) {
			c := p.data[p.pos]
			if c < '0' || c > '9' {
				break
			}
			p.pos++
		}
		if p.pos == fracStart {
			return 0, ErrSyntax
		}
	}
	if p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == 'e' || c == 'E' {
			return 0, ErrSyntax
		}
	}
	raw := p.data[start:p.pos]
	return strconv.ParseFloat(unsafeBytesToString(raw), 64)
}

func unsafeBytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

// parseInt parses a non-negative integer (no sign). The contract uses
// signed-but-non-negative values for installments and tx_count_24h.
func (p *parser) parseInt() (int, error) {
	p.skipWS()
	start := p.pos
	var n int
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		p.pos++
	}
	if p.pos == start {
		return 0, ErrSyntax
	}
	return n, nil
}

func (p *parser) parseBool() (bool, error) {
	p.skipWS()
	if p.pos+4 <= len(p.data) &&
		p.data[p.pos] == 't' &&
		p.data[p.pos+1] == 'r' &&
		p.data[p.pos+2] == 'u' &&
		p.data[p.pos+3] == 'e' {
		p.pos += 4
		return true, nil
	}
	if p.pos+5 <= len(p.data) &&
		p.data[p.pos] == 'f' &&
		p.data[p.pos+1] == 'a' &&
		p.data[p.pos+2] == 'l' &&
		p.data[p.pos+3] == 's' &&
		p.data[p.pos+4] == 'e' {
		p.pos += 5
		return false, nil
	}
	return false, ErrUnexpected
}

func (p *parser) consumeNull() bool {
	p.skipWS()
	if p.pos+4 <= len(p.data) &&
		p.data[p.pos] == 'n' &&
		p.data[p.pos+1] == 'u' &&
		p.data[p.pos+2] == 'l' &&
		p.data[p.pos+3] == 'l' {
		p.pos += 4
		return true
	}
	return false
}

// skipValue advances past any JSON value, supporting nested objects and arrays.
func (p *parser) skipValue() error {
	p.skipWS()
	if p.eof() {
		return ErrSyntax
	}
	c := p.data[p.pos]
	switch {
	case c == '{':
		return p.skipObject()
	case c == '[':
		return p.skipArray()
	case c == '"':
		_, err := p.parseString()
		return err
	case c == 't' || c == 'f':
		_, err := p.parseBool()
		return err
	case c == 'n':
		if !p.consumeNull() {
			return ErrSyntax
		}
		return nil
	case c == '-' || (c >= '0' && c <= '9'):
		_, err := p.parseNumber()
		return err
	}
	return ErrSyntax
}

func (p *parser) skipObject() error {
	if err := p.expect('{'); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		if _, err := p.parseString(); err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		if err := p.skipValue(); err != nil {
			return err
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

func (p *parser) skipArray() error {
	if err := p.expect('['); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == ']' {
		p.pos++
		return nil
	}
	for {
		if err := p.skipValue(); err != nil {
			return err
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect(']')
	}
}

