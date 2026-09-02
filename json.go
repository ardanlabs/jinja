package jinja

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// =============================================================================
// Encoding
// =============================================================================

const hexDigits = "0123456789abcdef"

// appendJSONString writes the JSON encoding of s to sb. The output matches
// encoding/json, so U+2028 and U+2029 are escaped and an invalid UTF-8 byte
// becomes U+FFFD. With escapeHTML set, '<', '>' and '&' become the escaped
// forms \u003c, \u003e and \u0026.
func appendJSONString(sb *strings.Builder, s string, escapeHTML bool) {
	sb.WriteByte('"')

	start := 0
	for i := 0; i < len(s); {
		c := s[i]

		if c < utf8.RuneSelf {
			if !jsonNeedsEscape(c, escapeHTML) {
				i++
				continue
			}

			if start < i {
				sb.WriteString(s[start:i])
			}

			switch c {
			case '"':
				sb.WriteString(`\"`)
			case '\\':
				sb.WriteString(`\\`)
			case '\b':
				sb.WriteString(`\b`)
			case '\f':
				sb.WriteString(`\f`)
			case '\n':
				sb.WriteString(`\n`)
			case '\r':
				sb.WriteString(`\r`)
			case '\t':
				sb.WriteString(`\t`)
			default:
				sb.WriteString(`\u00`)
				sb.WriteByte(hexDigits[c>>4])
				sb.WriteByte(hexDigits[c&0xF])
			}

			i++
			start = i
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if size == 1 {
			if start < i {
				sb.WriteString(s[start:i])
			}
			sb.WriteString("\uFFFD")
			i++
			start = i
			continue
		}

		// U+2028 and U+2029 are valid JSON but break JavaScript string
		// literals, so encoding/json escapes them.
		if r == '\u2028' || r == '\u2029' {
			if start < i {
				sb.WriteString(s[start:i])
			}
			sb.WriteString(`\u202`)
			sb.WriteByte(hexDigits[r&0xF])
			i += size
			start = i
			continue
		}

		i += size
	}

	if start < len(s) {
		sb.WriteString(s[start:])
	}

	sb.WriteByte('"')
}

func jsonNeedsEscape(c byte, escapeHTML bool) bool {
	if c < 0x20 || c == '"' || c == '\\' {
		return true
	}
	return escapeHTML && (c == '<' || c == '>' || c == '&')
}

// appendJSONFloat writes f the way encoding/json does. It uses 'e' notation
// outside [1e-6, 1e21) and trims the leading zero from a negative exponent.
func appendJSONFloat(sb *strings.Builder, f float64) {
	abs := math.Abs(f)

	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}

	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], f, format, -1, 64)

	if format == 'e' {
		if n := len(b); n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}

	sb.Write(b)
}

// =============================================================================
// Decoding
// =============================================================================

type jsonParser struct {
	s   string
	pos int
}

// parseJSON decodes s into a Value tree without an intermediate Go-typed
// tree. Object keys keep their source order, and an integer literal that fits
// in an int64 stays an integer so that tojson round-trips it exactly.
func parseJSON(s string) (Value, error) {
	p := jsonParser{s: s}

	p.skipSpace()
	v, err := p.parseValue(0)
	if err != nil {
		return Undefined(), err
	}

	p.skipSpace()
	if p.pos != len(p.s) {
		return Undefined(), fmt.Errorf("invalid character %q after top-level value", p.s[p.pos])
	}

	return v, nil
}

// maxJSONDepth bounds recursion so that deeply nested input cannot exhaust
// the stack, which matters because a TinyGo wasm build has a small one. The
// limit matches CPython, whose default recursion limit makes json.loads fail
// past about 1000 levels.
const maxJSONDepth = 1000

func (p *jsonParser) parseValue(depth int) (Value, error) {
	if depth > maxJSONDepth {
		return Undefined(), fmt.Errorf("exceeded max depth")
	}

	if p.pos >= len(p.s) {
		return Undefined(), fmt.Errorf("unexpected end of JSON input")
	}

	switch c := p.s[p.pos]; c {
	case '{':
		return p.parseObject(depth)

	case '[':
		return p.parseArray(depth)

	case '"':
		s, err := p.parseString()
		if err != nil {
			return Undefined(), err
		}
		return NewString(s), nil

	case 't':
		if err := p.expect("true"); err != nil {
			return Undefined(), err
		}
		return NewBool(true), nil

	case 'f':
		if err := p.expect("false"); err != nil {
			return Undefined(), err
		}
		return NewBool(false), nil

	case 'n':
		if err := p.expect("null"); err != nil {
			return Undefined(), err
		}
		return None(), nil

	default:
		if c == '-' || (c >= '0' && c <= '9') {
			return p.parseNumber()
		}
		return Undefined(), fmt.Errorf("invalid character %q looking for beginning of value", c)
	}
}

func (p *jsonParser) parseObject(depth int) (Value, error) {
	p.pos++ // consume '{'

	v := NewDict()
	d := v.AsDict()

	p.skipSpace()
	if p.pos < len(p.s) && p.s[p.pos] == '}' {
		p.pos++
		return v, nil
	}

	for {
		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != '"' {
			return Undefined(), fmt.Errorf("invalid character looking for beginning of object key string")
		}

		key, err := p.parseString()
		if err != nil {
			return Undefined(), err
		}

		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != ':' {
			return Undefined(), fmt.Errorf("invalid character after object key")
		}
		p.pos++

		p.skipSpace()
		val, err := p.parseValue(depth + 1)
		if err != nil {
			return Undefined(), err
		}
		d.Set(key, val)

		p.skipSpace()
		if p.pos >= len(p.s) {
			return Undefined(), fmt.Errorf("unexpected end of JSON input")
		}

		switch p.s[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return v, nil
		default:
			return Undefined(), fmt.Errorf("invalid character %q after object key:value pair", p.s[p.pos])
		}
	}
}

func (p *jsonParser) parseArray(depth int) (Value, error) {
	p.pos++ // consume '['

	p.skipSpace()
	if p.pos < len(p.s) && p.s[p.pos] == ']' {
		p.pos++
		return NewList(nil), nil
	}

	var items []Value
	for {
		p.skipSpace()
		item, err := p.parseValue(depth + 1)
		if err != nil {
			return Undefined(), err
		}
		items = append(items, item)

		p.skipSpace()
		if p.pos >= len(p.s) {
			return Undefined(), fmt.Errorf("unexpected end of JSON input")
		}

		switch p.s[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return NewList(items), nil
		default:
			return Undefined(), fmt.Errorf("invalid character %q after array element", p.s[p.pos])
		}
	}
}

func (p *jsonParser) parseNumber() (Value, error) {
	start := p.pos

	if p.pos < len(p.s) && p.s[p.pos] == '-' {
		p.pos++
	}

	if !p.scanIntPart() {
		return Undefined(), fmt.Errorf("invalid number literal")
	}

	isFloat := false

	if p.pos < len(p.s) && p.s[p.pos] == '.' {
		isFloat = true
		p.pos++
		if !p.scanDigits() {
			return Undefined(), fmt.Errorf("invalid number literal")
		}
	}

	if p.pos < len(p.s) && (p.s[p.pos] == 'e' || p.s[p.pos] == 'E') {
		isFloat = true
		p.pos++
		if p.pos < len(p.s) && (p.s[p.pos] == '+' || p.s[p.pos] == '-') {
			p.pos++
		}
		if !p.scanDigits() {
			return Undefined(), fmt.Errorf("invalid number literal")
		}
	}

	lit := p.s[start:p.pos]

	if !isFloat {
		if n, err := strconv.ParseInt(lit, 10, 64); err == nil {
			return NewInt(n), nil
		}
	}

	f, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		return Undefined(), fmt.Errorf("number %s is out of range", lit)
	}

	return NewFloat(f), nil
}

// scanIntPart consumes the integer part of a number. JSON forbids a leading
// zero before another digit. RFC 8259 section 6.
func (p *jsonParser) scanIntPart() bool {
	start := p.pos
	if !p.scanDigits() {
		return false
	}
	return p.pos-start == 1 || p.s[start] != '0'
}

// scanDigits consumes one or more digits and reports whether it found any.
// Fraction and exponent digits may have leading zeros.
func (p *jsonParser) scanDigits() bool {
	start := p.pos
	for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
		p.pos++
	}
	return p.pos > start
}

func (p *jsonParser) parseString() (string, error) {
	p.pos++ // consume '"'
	start := p.pos

	// Fast path. Return a slice of the source when there is nothing to unescape.
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		switch {
		case c == '"':
			s := p.s[start:p.pos]
			p.pos++
			return s, nil
		case c == '\\':
			return p.parseStringSlow(start)
		case c < 0x20:
			return "", fmt.Errorf("invalid character in string literal")
		}
		p.pos++
	}

	return "", fmt.Errorf("unexpected end of JSON input")
}

func (p *jsonParser) parseStringSlow(start int) (string, error) {
	var sb strings.Builder
	sb.Grow(len(p.s) - start)
	sb.WriteString(p.s[start:p.pos])

	for p.pos < len(p.s) {
		c := p.s[p.pos]

		switch {
		case c == '"':
			p.pos++
			return sb.String(), nil

		case c < 0x20:
			return "", fmt.Errorf("invalid character in string literal")

		case c != '\\':
			sb.WriteByte(c)
			p.pos++
			continue
		}

		p.pos++
		if p.pos >= len(p.s) {
			return "", fmt.Errorf("unexpected end of JSON input")
		}

		switch e := p.s[p.pos]; e {
		case '"', '\\', '/':
			sb.WriteByte(e)
			p.pos++
		case 'b':
			sb.WriteByte('\b')
			p.pos++
		case 'f':
			sb.WriteByte('\f')
			p.pos++
		case 'n':
			sb.WriteByte('\n')
			p.pos++
		case 'r':
			sb.WriteByte('\r')
			p.pos++
		case 't':
			sb.WriteByte('\t')
			p.pos++
		case 'u':
			p.pos++
			r, err := p.parseHex4()
			if err != nil {
				return "", err
			}

			if utf16.IsSurrogate(r) {
				if p.pos+1 < len(p.s) && p.s[p.pos] == '\\' && p.s[p.pos+1] == 'u' {
					save := p.pos
					p.pos += 2
					r2, err := p.parseHex4()
					if err != nil {
						return "", err
					}
					if dec := utf16.DecodeRune(r, r2); dec != utf8.RuneError {
						sb.WriteRune(dec)
						continue
					}
					p.pos = save
				}
				r = utf8.RuneError
			}

			sb.WriteRune(r)

		default:
			return "", fmt.Errorf("invalid escape character %q in string literal", e)
		}
	}

	return "", fmt.Errorf("unexpected end of JSON input")
}

func (p *jsonParser) parseHex4() (rune, error) {
	if p.pos+4 > len(p.s) {
		return 0, fmt.Errorf("invalid \\u escape in string literal")
	}

	var r rune
	for range 4 {
		c := p.s[p.pos]
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, fmt.Errorf("invalid \\u escape in string literal")
		}
		p.pos++
	}

	return r, nil
}

func (p *jsonParser) skipSpace() {
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) expect(lit string) error {
	if !strings.HasPrefix(p.s[p.pos:], lit) {
		return fmt.Errorf("invalid character looking for beginning of value")
	}
	p.pos += len(lit)
	return nil
}
