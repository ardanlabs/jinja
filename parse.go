package jinja

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// =============================================================================
// Expression tokenizer
// =============================================================================

type tokKind int

const (
	tokName     tokKind = iota // identifier or keyword
	tokInt                     // integer literal
	tokFloat                   // float literal
	tokString                  // string literal
	tokOp                      // operator (+, -, ==, etc)
	tokComma                   // ,
	tokDot                     // .
	tokColon                   // :
	tokLParen                  // (
	tokRParen                  // )
	tokLBracket                // [
	tokRBracket                // ]
	tokLBrace                  // {
	tokRBrace                  // }
	tokPipe                    // |
	tokTilde                   // ~
	tokPower                   // **
	tokFloorDiv                // //
	tokEOF
)

type tok struct {
	kind tokKind
	val  string
}

// decodeEscape decodes the backslash escape whose first character after the
// backslash is s[i], appends the result to b, and returns the index of the
// first byte after the escape.
//
// Jinja2 decodes string literals with Python's "unicode-escape" codec, so this
// follows Python string-literal semantics:
//
//   - \\ \' \" \a \b \f \n \r \t \v are the usual control characters.
//   - \xNN, \uNNNN and \UNNNNNNNN take exactly 2, 4 and 8 hex digits and denote
//     a Unicode code point, written as UTF-8 (so "\xe9" is "é", two bytes, not
//     the single byte 0xE9).
//   - \o, \oo and \ooo (octal, so \0 is NUL) also denote a code point.
//   - A backslash before a line break (LF, CRLF or CR) is a line continuation:
//     both are dropped. Jinja2 normalises line breaks to LF before decoding.
//   - Any other escape, including \8, \9 and \N{NAME}, is kept as written,
//     backslash included. Python decodes \N{NAME} by Unicode character name;
//     this engine has no name table, so it is left literal. A backslash before
//     a non-ASCII character is also kept as written (Jinja2 would turn "\é"
//     into the text \xe9, an artefact of its ASCII round trip).
//
// A truncated \x, \u or \U escape, or a \U beyond U+10FFFF, is an error, as it
// is in Jinja2 ("truncated \xXX escape"). llama.cpp's template engine is
// stricter still: it decodes \x but rejects \u, \U, octal and unknown
// escapes, so no template that works there reaches these errors. A surrogate code point (\ud800-\udfff) cannot be held in a Go string
// and is written as U+FFFD.
func decodeEscape(s string, i int, b *strings.Builder) (int, error) {
	c := s[i]
	switch c {
	case '\\', '\'', '"':
		b.WriteByte(c)
	case 'a':
		b.WriteByte('\a')
	case 'b':
		b.WriteByte('\b')
	case 'f':
		b.WriteByte('\f')
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case 'v':
		b.WriteByte('\v')
	case '\n':
		// Line continuation.
	case '\r':
		// Line continuation; a CRLF counts as one line break.
		if i+1 < len(s) && s[i+1] == '\n' {
			return i + 2, nil
		}
	case 'x', 'u', 'U':
		form := `\xXX`
		switch c {
		case 'u':
			form = `\uXXXX`
		case 'U':
			form = `\UXXXXXXXX`
		}
		start := i + 1
		end := start + len(form) - 2 // len(form) - 2 is the hex digit count
		if end > len(s) || !isHex(s[start:end]) {
			return 0, fmt.Errorf("string literal: truncated %s escape", form)
		}
		cp, _ := strconv.ParseUint(s[start:end], 16, 32)
		if cp > unicode.MaxRune {
			return 0, fmt.Errorf("string literal: illegal Unicode character \\%c%s", c, s[start:end])
		}
		b.WriteRune(rune(cp))
		return end, nil
	case '0', '1', '2', '3', '4', '5', '6', '7':
		end := i + 1
		for end < len(s) && end < i+3 && s[end] >= '0' && s[end] <= '7' {
			end++
		}
		cp, _ := strconv.ParseUint(s[i:end], 8, 32)
		b.WriteRune(rune(cp))
		return end, nil
	default:
		b.WriteByte('\\')
		b.WriteByte(c)
	}
	return i + 1, nil
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func tokenizeExpr(s string) ([]tok, error) {
	var tokens []tok
	i := 0

	for i < len(s) {
		ch := s[i]

		// Skip whitespace.
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// String literals.
		if ch == '\'' || ch == '"' {
			quote := ch
			i++
			var b strings.Builder
			for i < len(s) && s[i] != quote {
				if s[i] == '\\' && i+1 < len(s) {
					next, err := decodeEscape(s, i+1, &b)
					if err != nil {
						return nil, err
					}
					i = next
					continue
				}
				b.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++ // skip closing quote
			}
			tokens = append(tokens, tok{kind: tokString, val: b.String()})
			continue
		}

		// Numbers.
		if ch >= '0' && ch <= '9' {
			start := i
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			if i < len(s) && s[i] == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
				i++
				for i < len(s) && s[i] >= '0' && s[i] <= '9' {
					i++
				}
				// Handle scientific notation.
				if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
					i++
					if i < len(s) && (s[i] == '+' || s[i] == '-') {
						i++
					}
					for i < len(s) && s[i] >= '0' && s[i] <= '9' {
						i++
					}
				}
				tokens = append(tokens, tok{kind: tokFloat, val: s[start:i]})
			} else {
				tokens = append(tokens, tok{kind: tokInt, val: s[start:i]})
			}
			continue
		}

		// Identifiers and keywords.
		if ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			start := i
			for i < len(s) && (s[i] == '_' || (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= '0' && s[i] <= '9')) {
				i++
			}
			tokens = append(tokens, tok{kind: tokName, val: s[start:i]})
			continue
		}

		// Two-character operators.
		if i+1 < len(s) {
			two := s[i : i+2]
			switch two {
			case "**":
				tokens = append(tokens, tok{kind: tokPower, val: two})
				i += 2
				continue
			case "//":
				tokens = append(tokens, tok{kind: tokFloorDiv, val: two})
				i += 2
				continue
			case "==", "!=", "<=", ">=":
				tokens = append(tokens, tok{kind: tokOp, val: two})
				i += 2
				continue
			}
		}

		// Single-character tokens.
		switch ch {
		case ',':
			tokens = append(tokens, tok{kind: tokComma, val: ","})
		case '.':
			tokens = append(tokens, tok{kind: tokDot, val: "."})
		case ':':
			tokens = append(tokens, tok{kind: tokColon, val: ":"})
		case '(':
			tokens = append(tokens, tok{kind: tokLParen, val: "("})
		case ')':
			tokens = append(tokens, tok{kind: tokRParen, val: ")"})
		case '[':
			tokens = append(tokens, tok{kind: tokLBracket, val: "["})
		case ']':
			tokens = append(tokens, tok{kind: tokRBracket, val: "]"})
		case '{':
			tokens = append(tokens, tok{kind: tokLBrace, val: "{"})
		case '}':
			tokens = append(tokens, tok{kind: tokRBrace, val: "}"})
		case '|':
			tokens = append(tokens, tok{kind: tokPipe, val: "|"})
		case '~':
			tokens = append(tokens, tok{kind: tokTilde, val: "~"})
		case '+', '-', '*', '/', '%', '<', '>':
			tokens = append(tokens, tok{kind: tokOp, val: string(ch)})
		case '=':
			tokens = append(tokens, tok{kind: tokOp, val: "="})
		default:
			// skip unknown chars
		}
		i++
	}

	tokens = append(tokens, tok{kind: tokEOF, val: ""})
	return tokens, nil
}

// =============================================================================
// Expression parser (recursive descent)
// =============================================================================

type exprParser struct {
	tokens []tok
	pos    int
}

func newExprParser(tokens []tok) *exprParser {
	return &exprParser{tokens: tokens, pos: 0}
}

func (p *exprParser) peek() tok {
	if p.pos >= len(p.tokens) {
		return tok{kind: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *exprParser) advance() tok {
	t := p.peek()
	p.pos++
	return t
}

func (p *exprParser) expect(kind tokKind, val string) (tok, error) {
	t := p.advance()
	if t.kind != kind {
		return t, fmt.Errorf("expected %q but got %q", val, t.val)
	}
	if val != "" && t.val != val {
		return t, fmt.Errorf("expected %q but got %q", val, t.val)
	}
	return t, nil
}

func (p *exprParser) matchName(name string) bool {
	if p.peek().kind == tokName && p.peek().val == name {
		p.advance()
		return true
	}
	return false
}

// parseExpr is the public entry point.
func parseExpr(s string) (expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &litExpr{val: None()}, nil
	}

	tokens, err := tokenizeExpr(s)
	if err != nil {
		return nil, err
	}
	p := newExprParser(tokens)
	e, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("unexpected token %q after expression", p.peek().val)
	}

	return e, nil
}

// parseExpression handles the lowest precedence: inline if/else.
func (p *exprParser) parseExpression() (expr, error) {
	e, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	// Handle inline if: expr if condition [else expr]
	if p.peek().kind == tokName && p.peek().val == "if" {
		p.advance()
		cond, err := p.parseOr()
		if err != nil {
			return nil, err
		}

		var falseExpr expr
		if p.matchName("else") {
			falseExpr, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}

		return &condExpr{
			trueExpr:  e,
			condition: cond,
			falseExpr: falseExpr,
		}, nil
	}

	return e, nil
}

func (p *exprParser) parseOr() (expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.peek().kind == tokName && p.peek().val == "or" {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &binExpr{op: "or", left: left, right: right}
	}

	return left, nil
}

func (p *exprParser) parseAnd() (expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.peek().kind == tokName && p.peek().val == "and" {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &binExpr{op: "and", left: left, right: right}
	}

	return left, nil
}

func (p *exprParser) parseNot() (expr, error) {
	if p.peek().kind == tokName && p.peek().val == "not" {
		p.advance()
		operand, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &unaryExpr{op: "not", operand: operand}, nil
	}

	return p.parseComparison()
}

func (p *exprParser) parseComparison() (expr, error) {
	left, err := p.parseConcat()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()

		// Standard comparison operators.
		if t.kind == tokOp {
			switch t.val {
			case "==", "!=", "<", ">", "<=", ">=":
				p.advance()
				right, err := p.parseConcat()
				if err != nil {
					return nil, err
				}
				left = &binExpr{op: t.val, left: left, right: right}
				continue
			}
		}

		// "in" operator.
		if t.kind == tokName && t.val == "in" {
			p.advance()
			right, err := p.parseConcat()
			if err != nil {
				return nil, err
			}
			left = &binExpr{op: "in", left: left, right: right}
			continue
		}

		// "not in" operator.
		if t.kind == tokName && t.val == "not" && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].kind == tokName && p.tokens[p.pos+1].val == "in" {
			p.advance() // not
			p.advance() // in
			right, err := p.parseConcat()
			if err != nil {
				return nil, err
			}
			left = &binExpr{op: "not in", left: left, right: right}
			continue
		}

		// "is" / "is not" test.
		if t.kind == tokName && t.val == "is" {
			p.advance()
			negated := false
			if p.peek().kind == tokName && p.peek().val == "not" {
				p.advance()
				negated = true
			}

			// Parse test name.
			testTok := p.advance()
			testName := testTok.val

			// Parse optional arguments.
			testArgs, err := p.parseTestArgs(testTok)
			if err != nil {
				return nil, err
			}

			left = &testExpr{
				expr:    left,
				name:    testName,
				negated: negated,
				args:    testArgs,
			}

			// As in Jinja2, a filter here applies to the test's result. A bare
			// argument stops short of it, so `x is sameas y | string` is
			// `(x is sameas y) | string`.
			for p.peek().kind == tokPipe {
				left, err = p.parseFilter(left)
				if err != nil {
					return nil, err
				}
			}
			continue
		}

		break
	}

	return left, nil
}

// parseTestArgs parses what follows a test's name, mirroring Jinja2's
// Parser.parse_test: a parenthesised argument list, or one bare argument, which
// is a primary expression plus attribute access, subscripts and calls, with no
// operators or filters.
func (p *exprParser) parseTestArgs(testTok tok) ([]expr, error) {
	if p.peek().kind == tokLParen {
		p.advance()
		var args []expr
		for p.peek().kind != tokRParen && p.peek().kind != tokEOF {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().kind == tokComma {
				p.advance()
			}
		}
		if _, err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		return args, nil
	}

	if testTok.kind != tokName || !startsBareTestArg(p.peek()) {
		return nil, nil
	}

	// Only `is` straight after the test's name is an error, as in Jinja2.
	// `x is sameas y is true` is a test of the first test's result.
	if t := p.peek(); t.kind == tokName && t.val == "is" {
		return nil, fmt.Errorf("cannot chain multiple tests with is")
	}

	arg, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	arg, err = p.parseTrailers(arg, false)
	if err != nil {
		return nil, err
	}

	return []expr{arg}, nil
}

// startsBareTestArg reports whether t begins a test's bare argument.
//
// Jinja2 excludes only `else`, `or` and `and`, and takes any other name as the
// argument. `if`, `in` and `not` are excluded here too, because they carry on
// the expression (`x is defined if y else z`, `x is defined not in y`) and that
// already worked. Taking them as arguments would quietly turn `not in` into
// `in`, since tests here ignore arguments they do not use. An opening paren is
// not listed because it starts the parenthesised form.
func startsBareTestArg(t tok) bool {
	switch t.kind {
	case tokString, tokInt, tokFloat, tokLBracket, tokLBrace:
		return true
	case tokName:
		switch t.val {
		case "and", "or", "else", "if", "in", "not":
			return false
		}
		return true
	}
	return false
}

func (p *exprParser) parseConcat() (expr, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}

	for p.peek().kind == tokTilde {
		p.advance()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &binExpr{op: "~", left: left, right: right}
	}

	return left, nil
}

func (p *exprParser) parseAddSub() (expr, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}

	for p.peek().kind == tokOp && (p.peek().val == "+" || p.peek().val == "-") {
		op := p.advance().val
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		left = &binExpr{op: op, left: left, right: right}
	}

	return left, nil
}

func (p *exprParser) parseMulDiv() (expr, error) {
	left, err := p.parsePower()
	if err != nil {
		return nil, err
	}

	for {
		t := p.peek()
		if t.kind == tokOp && (t.val == "*" || t.val == "/" || t.val == "%") {
			op := p.advance().val
			right, err := p.parsePower()
			if err != nil {
				return nil, err
			}
			left = &binExpr{op: op, left: left, right: right}
			continue
		}
		if t.kind == tokFloorDiv {
			p.advance()
			right, err := p.parsePower()
			if err != nil {
				return nil, err
			}
			left = &binExpr{op: "//", left: left, right: right}
			continue
		}
		break
	}

	return left, nil
}

func (p *exprParser) parsePower() (expr, error) {
	base, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	if p.peek().kind == tokPower {
		p.advance()
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &binExpr{op: "**", left: base, right: exp}, nil
	}

	return base, nil
}

func (p *exprParser) parseUnary() (expr, error) {
	if p.peek().kind == tokOp && (p.peek().val == "-" || p.peek().val == "+") {
		op := p.advance().val
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &unaryExpr{op: op, operand: operand}, nil
	}

	return p.parsePostfix()
}

func (p *exprParser) parsePostfix() (expr, error) {
	e, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	return p.parseTrailers(e, true)
}

// parseTrailers applies the attribute accesses, subscripts and calls that
// follow e, and any filters when withFilters is set.
func (p *exprParser) parseTrailers(e expr, withFilters bool) (expr, error) {
	var err error

	for {
		t := p.peek()

		// Attribute access: e.attr
		if t.kind == tokDot {
			p.advance()
			attrTok := p.advance()
			if attrTok.kind != tokName && attrTok.kind != tokInt {
				return nil, fmt.Errorf("expected attribute name after '.', got %q", attrTok.val)
			}

			// Check if this is a method call: e.method(args)
			if p.peek().kind == tokLParen {
				p.advance()
				args, kwargs, err := p.parseCallArgs()
				if err != nil {
					return nil, err
				}
				e = &callExpr{
					fn:     &attrExpr{obj: e, attr: attrTok.val},
					args:   args,
					kwargs: kwargs,
				}
			} else {
				e = &attrExpr{obj: e, attr: attrTok.val}
			}
			continue
		}

		// Subscript / slice: e[key] or e[start:stop]
		if t.kind == tokLBracket {
			p.advance()

			// Check for slice: e[start:stop:step]
			var startExpr, stopExpr, stepExpr expr
			isSlice := false

			if p.peek().kind == tokColon {
				isSlice = true
			} else {
				startExpr, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
				if p.peek().kind == tokColon {
					isSlice = true
				}
			}

			if isSlice {
				if p.peek().kind == tokColon {
					p.advance()
				}
				if p.peek().kind != tokRBracket && p.peek().kind != tokColon {
					stopExpr, err = p.parseExpression()
					if err != nil {
						return nil, err
					}
				}
				if p.peek().kind == tokColon {
					p.advance()
					if p.peek().kind != tokRBracket {
						stepExpr, err = p.parseExpression()
						if err != nil {
							return nil, err
						}
					}
				}
				if _, err := p.expect(tokRBracket, "]"); err != nil {
					return nil, err
				}
				e = &sliceExpr{obj: e, start: startExpr, stop: stopExpr, step: stepExpr}
			} else {
				if _, err := p.expect(tokRBracket, "]"); err != nil {
					return nil, err
				}
				e = &itemExpr{obj: e, key: startExpr}
			}
			continue
		}

		// Function call: e(args)
		if t.kind == tokLParen {
			p.advance()
			args, kwargs, err := p.parseCallArgs()
			if err != nil {
				return nil, err
			}
			e = &callExpr{fn: e, args: args, kwargs: kwargs}
			continue
		}

		// Filter: e | filtername or e | filtername(args)
		if withFilters && t.kind == tokPipe {
			e, err = p.parseFilter(e)
			if err != nil {
				return nil, err
			}
			continue
		}

		break
	}

	return e, nil
}

// parseFilter parses `| filtername` or `| filtername(args)` applied to e. The
// next token must be the pipe.
func (p *exprParser) parseFilter(e expr) (expr, error) {
	p.advance()
	filterTok := p.advance()
	if filterTok.kind != tokName {
		return nil, fmt.Errorf("expected filter name after '|', got %q", filterTok.val)
	}

	var args []expr
	var kwargs []kwarg
	if p.peek().kind == tokLParen {
		p.advance()
		var err error
		args, kwargs, err = p.parseCallArgs()
		if err != nil {
			return nil, err
		}
	}

	return &filterExpr{
		expr:   e,
		name:   filterTok.val,
		args:   args,
		kwargs: kwargs,
	}, nil
}

func (p *exprParser) parseAtom() (expr, error) {
	t := p.peek()

	// None / null
	if t.kind == tokName && (t.val == "None" || t.val == "none" || t.val == "null") {
		p.advance()
		return &litExpr{val: None()}, nil
	}

	// True / true
	if t.kind == tokName && (t.val == "True" || t.val == "true") {
		p.advance()
		return &litExpr{val: NewBool(true)}, nil
	}

	// False / false
	if t.kind == tokName && (t.val == "False" || t.val == "false") {
		p.advance()
		return &litExpr{val: NewBool(false)}, nil
	}

	// Integer literal.
	if t.kind == tokInt {
		p.advance()
		n, _ := strconv.ParseInt(t.val, 10, 64)
		return &litExpr{val: NewInt(n)}, nil
	}

	// Float literal.
	if t.kind == tokFloat {
		p.advance()
		f, _ := strconv.ParseFloat(t.val, 64)
		return &litExpr{val: NewFloat(f)}, nil
	}

	// String literal.
	if t.kind == tokString {
		p.advance()
		return &litExpr{val: NewString(t.val)}, nil
	}

	// Identifier.
	if t.kind == tokName {
		p.advance()
		return &nameExpr{name: t.val}, nil
	}

	// Parenthesized expression or tuple.
	if t.kind == tokLParen {
		p.advance()
		if p.peek().kind == tokRParen {
			p.advance()
			return &listExpr{items: nil}, nil
		}
		e, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		// Check if tuple: (a, b, ...)
		if p.peek().kind == tokComma {
			items := []expr{e}
			for p.peek().kind == tokComma {
				p.advance()
				if p.peek().kind == tokRParen {
					break
				}
				item, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				items = append(items, item)
			}
			if _, err := p.expect(tokRParen, ")"); err != nil {
				return nil, err
			}
			return &listExpr{items: items}, nil
		}

		if _, err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		return e, nil
	}

	// List literal: [a, b, ...]
	if t.kind == tokLBracket {
		p.advance()
		var items []expr
		for p.peek().kind != tokRBracket && p.peek().kind != tokEOF {
			item, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			if p.peek().kind == tokComma {
				p.advance()
			}
		}
		if _, err := p.expect(tokRBracket, "]"); err != nil {
			return nil, err
		}
		return &listExpr{items: items}, nil
	}

	// Dict literal: {k: v, ...}
	if t.kind == tokLBrace {
		p.advance()
		var pairs []dictPair
		for p.peek().kind != tokRBrace && p.peek().kind != tokEOF {
			key, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokColon, ":"); err != nil {
				return nil, err
			}
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, dictPair{key: key, value: value})
			if p.peek().kind == tokComma {
				p.advance()
			}
		}
		if _, err := p.expect(tokRBrace, "}"); err != nil {
			return nil, err
		}
		return &dictExpr{pairs: pairs}, nil
	}

	// Negative number: handle case where we get here with "-" as op token.
	if t.kind == tokOp && t.val == "-" {
		p.advance()
		operand, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		return &unaryExpr{op: "-", operand: operand}, nil
	}

	return nil, fmt.Errorf("unexpected token %q (kind=%d)", t.val, t.kind)
}

func (p *exprParser) parseCallArgs() ([]expr, []kwarg, error) {
	var args []expr
	var kwargs []kwarg

	for p.peek().kind != tokRParen && p.peek().kind != tokEOF {
		// Check for kwarg: name=value
		if p.peek().kind == tokName && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].kind == tokOp && p.tokens[p.pos+1].val == "=" {
			name := p.advance().val
			p.advance() // skip =
			val, err := p.parseExpression()
			if err != nil {
				return nil, nil, err
			}
			kwargs = append(kwargs, kwarg{name: name, value: val})
		} else {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, arg)
		}

		if p.peek().kind == tokComma {
			p.advance()
		}
	}

	if _, err := p.expect(tokRParen, ")"); err != nil {
		return nil, nil, err
	}

	return args, kwargs, nil
}

// =============================================================================
// Template parser (segments -> AST nodes)
// =============================================================================

func parseTemplate(segs []segment) ([]node, error) {
	p := &templateParser{segs: segs, pos: 0}
	nodes, err := p.parseUntil()
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

type templateParser struct {
	segs []segment
	pos  int
}

func (p *templateParser) done() bool {
	return p.pos >= len(p.segs)
}

func (p *templateParser) peek() *segment {
	if p.done() {
		return nil
	}
	return &p.segs[p.pos]
}

func (p *templateParser) advance() *segment {
	s := p.peek()
	p.pos++
	return s
}

// parseUntil parses nodes until we hit a statement starting with one of the
// stop keywords (e.g., "endif", "endfor", "elif", "else"). Returns the nodes
// collected so far. The stop keyword segment is NOT consumed.
func (p *templateParser) parseUntil(stopKeywords ...string) ([]node, error) {
	var nodes []node

	for !p.done() {
		seg := p.peek()

		if seg.kind == segStmt {
			keyword := stmtKeyword(seg.text)
			if slices.Contains(stopKeywords, keyword) {
				return nodes, nil
			}
		}

		switch seg.kind {
		case segText:
			p.advance()
			nodes = append(nodes, &textNode{text: seg.text})

		case segComment:
			p.advance()
			// Comments produce no output.

		case segExpr:
			p.advance()
			e, err := parseExpr(seg.text)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", seg.line, err)
			}
			nodes = append(nodes, &outputNode{expression: e})

		case segStmt:
			keyword := stmtKeyword(seg.text)
			switch keyword {
			case "if":
				n, err := p.parseIf()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, n)

			case "for":
				n, err := p.parseFor()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, n)

			case "set":
				n, err := p.parseSet()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, n)

			case "macro":
				n, err := p.parseMacro()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, n)

			case "call":
				n, err := p.parseCall()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, n)

			case "filter":
				n, err := p.parseFilterBlock()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, n)

			case "break":
				p.advance()
				nodes = append(nodes, &breakNode{})

			case "continue":
				p.advance()
				nodes = append(nodes, &continueNode{})

			case "generation":
				// generation/endgeneration are tokenizer hints; treat as no-op.
				p.advance()
				body, err := p.parseUntil("endgeneration")
				if err != nil {
					return nil, err
				}
				if p.done() || stmtKeyword(p.peek().text) != "endgeneration" {
					return nil, fmt.Errorf("line %d: unclosed generation block", seg.line)
				}
				p.advance()
				nodes = append(nodes, body...)

			default:
				return nil, fmt.Errorf("line %d: unknown statement %q", seg.line, keyword)
			}

		default:
			p.advance()
		}
	}

	return nodes, nil
}

func (p *templateParser) parseIf() (*ifNode, error) {
	n := &ifNode{}

	// Parse initial "if" branch.
	seg := p.advance()
	condStr := strings.TrimPrefix(strings.TrimSpace(seg.text), "if")
	cond, err := parseExpr(condStr)
	if err != nil {
		return nil, fmt.Errorf("line %d: if condition: %w", seg.line, err)
	}

	body, err := p.parseUntil("elif", "else", "endif")
	if err != nil {
		return nil, err
	}
	n.branches = append(n.branches, condBranch{condition: cond, body: body})

	// Parse elif / else branches.
	for !p.done() {
		seg := p.peek()
		keyword := stmtKeyword(seg.text)

		if keyword == "elif" {
			p.advance()
			condStr := strings.TrimPrefix(strings.TrimSpace(seg.text), "elif")
			cond, err := parseExpr(condStr)
			if err != nil {
				return nil, fmt.Errorf("line %d: elif condition: %w", seg.line, err)
			}
			body, err := p.parseUntil("elif", "else", "endif")
			if err != nil {
				return nil, err
			}
			n.branches = append(n.branches, condBranch{condition: cond, body: body})
			continue
		}

		if keyword == "else" {
			p.advance()
			body, err := p.parseUntil("endif")
			if err != nil {
				return nil, err
			}
			n.branches = append(n.branches, condBranch{condition: nil, body: body})
			continue
		}

		if keyword == "endif" {
			p.advance()
			return n, nil
		}

		break
	}

	return nil, fmt.Errorf("unclosed if block")
}

func (p *templateParser) parseFor() (*forNode, error) {
	seg := p.advance()
	n := &forNode{}

	// Parse: for [key,] val in iter [if filter] [recursive]
	forExpr := strings.TrimPrefix(strings.TrimSpace(seg.text), "for")
	forExpr = strings.TrimSpace(forExpr)

	// Find "in" keyword to split vars from iterable.
	inIdx := findKeyword(forExpr, "in")
	if inIdx < 0 {
		return nil, fmt.Errorf("line %d: for loop missing 'in'", seg.line)
	}

	varPart := strings.TrimSpace(forExpr[:inIdx])
	iterPart := strings.TrimSpace(forExpr[inIdx+2:])

	// Parse variables: key, val or just val
	vars := splitVars(varPart)
	switch len(vars) {
	case 1:
		n.valVar = vars[0]
	case 2:
		n.keyVar = vars[0]
		n.valVar = vars[1]
	default:
		return nil, fmt.Errorf("line %d: invalid for loop variables: %q", seg.line, varPart)
	}

	// Check for "if" filter clause and "recursive" keyword at the end.
	ifIdx := findKeyword(iterPart, "if")
	recIdx := findKeyword(iterPart, "recursive")

	if recIdx >= 0 {
		n.recursive = true
		iterPart = strings.TrimSpace(iterPart[:recIdx])
		// Re-check for "if" in the truncated part.
		ifIdx = findKeyword(iterPart, "if")
	}

	if ifIdx >= 0 {
		filterStr := strings.TrimSpace(iterPart[ifIdx+2:])
		iterPart = strings.TrimSpace(iterPart[:ifIdx])
		fe, err := parseExpr(filterStr)
		if err != nil {
			return nil, fmt.Errorf("line %d: for filter: %w", seg.line, err)
		}
		n.filterExpr = fe
	}

	iter, err := parseExpr(iterPart)
	if err != nil {
		return nil, fmt.Errorf("line %d: for iterable: %w", seg.line, err)
	}
	n.iter = iter

	body, err := p.parseUntil("else", "endfor")
	if err != nil {
		return nil, err
	}
	n.body = body

	// Check for else.
	if !p.done() && stmtKeyword(p.peek().text) == "else" {
		p.advance()
		elseBody, err := p.parseUntil("endfor")
		if err != nil {
			return nil, err
		}
		n.elseBody = elseBody
	}

	if p.done() || stmtKeyword(p.peek().text) != "endfor" {
		return nil, fmt.Errorf("unclosed for block")
	}
	p.advance()

	return n, nil
}

func (p *templateParser) parseSet() (node, error) {
	seg := p.advance()
	setExpr := strings.TrimPrefix(strings.TrimSpace(seg.text), "set")
	setExpr = strings.TrimSpace(setExpr)

	// Block set assignment: {% set var %}body{% endset %}
	if !strings.Contains(setExpr, "=") {
		target := strings.TrimSpace(setExpr)
		body, err := p.parseUntil("endset")
		if err != nil {
			return nil, err
		}
		if p.done() || stmtKeyword(p.peek().text) != "endset" {
			return nil, fmt.Errorf("line %d: unclosed set block", seg.line)
		}
		p.advance()
		return &setBlockNode{target: target, body: body}, nil
	}

	// Inline set: {% set var = expr %}
	before, after, ok := strings.Cut(setExpr, "=")
	if !ok {
		return nil, fmt.Errorf("line %d: set statement missing '='", seg.line)
	}

	target := strings.TrimSpace(before)
	valueStr := strings.TrimSpace(after)

	val, err := parseExpr(valueStr)
	if err != nil {
		return nil, fmt.Errorf("line %d: set value: %w", seg.line, err)
	}

	// Check for namespace: ns.attr = value
	if before, after, ok := strings.Cut(target, "."); ok {
		return &setNode{
			namespace: before,
			target:    after,
			value:     val,
		}, nil
	}

	return &setNode{target: target, value: val}, nil
}

func (p *templateParser) parseMacro() (*macroNode, error) {
	seg := p.advance()
	macroExpr := strings.TrimPrefix(strings.TrimSpace(seg.text), "macro")
	macroExpr = strings.TrimSpace(macroExpr)

	// Parse name and params: name(param1, param2=default)
	before, after, ok := strings.Cut(macroExpr, "(")
	if !ok {
		return nil, fmt.Errorf("line %d: macro missing parameter list", seg.line)
	}

	name := strings.TrimSpace(before)
	paramStr := after
	paramStr = strings.TrimSuffix(paramStr, ")")

	var params []macroParam
	if strings.TrimSpace(paramStr) != "" {
		for part := range strings.SplitSeq(paramStr, ",") {
			part = strings.TrimSpace(part)
			if before, after, ok := strings.Cut(part, "="); ok {
				pName := strings.TrimSpace(before)
				defStr := strings.TrimSpace(after)
				defVal, err := parseExpr(defStr)
				if err != nil {
					return nil, fmt.Errorf("line %d: macro default: %w", seg.line, err)
				}
				params = append(params, macroParam{name: pName, defaultVal: defVal})
			} else {
				params = append(params, macroParam{name: part})
			}
		}
	}

	body, err := p.parseUntil("endmacro")
	if err != nil {
		return nil, err
	}

	if p.done() || stmtKeyword(p.peek().text) != "endmacro" {
		return nil, fmt.Errorf("unclosed macro block")
	}
	p.advance()

	return &macroNode{name: name, params: params, body: body}, nil
}

func (p *templateParser) parseCall() (*callNode, error) {
	seg := p.advance()
	callStr := strings.TrimPrefix(strings.TrimSpace(seg.text), "call")
	callStr = strings.TrimSpace(callStr)

	n := &callNode{}

	// Check for caller args: call(arg1, arg2) macro(...)
	if strings.HasPrefix(callStr, "(") {
		closeIdx := strings.Index(callStr, ")")
		if closeIdx < 0 {
			return nil, fmt.Errorf("line %d: unclosed call args", seg.line)
		}
		argStr := callStr[1:closeIdx]
		for a := range strings.SplitSeq(argStr, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				n.callerArgs = append(n.callerArgs, a)
			}
		}
		callStr = strings.TrimSpace(callStr[closeIdx+1:])
	}

	callExprVal, err := parseExpr(callStr)
	if err != nil {
		return nil, fmt.Errorf("line %d: call expression: %w", seg.line, err)
	}
	n.expr = callExprVal

	body, err := p.parseUntil("endcall")
	if err != nil {
		return nil, err
	}
	n.body = body

	if p.done() || stmtKeyword(p.peek().text) != "endcall" {
		return nil, fmt.Errorf("unclosed call block")
	}
	p.advance()

	return n, nil
}

func (p *templateParser) parseFilterBlock() (*filterBlockNode, error) {
	seg := p.advance()
	filterStr := strings.TrimPrefix(strings.TrimSpace(seg.text), "filter")
	filterStr = strings.TrimSpace(filterStr)

	fe, err := parseExpr(filterStr)
	if err != nil {
		return nil, fmt.Errorf("line %d: filter expression: %w", seg.line, err)
	}

	body, err := p.parseUntil("endfilter")
	if err != nil {
		return nil, err
	}

	if p.done() || stmtKeyword(p.peek().text) != "endfilter" {
		return nil, fmt.Errorf("unclosed filter block")
	}
	p.advance()

	return &filterBlockNode{filter: fe, body: body}, nil
}

// =============================================================================
// Helper functions
// =============================================================================

// breakNode and continueNode for loop control.
type breakNode struct{}

func (n *breakNode) nodeType() string { return "break" }

type continueNode struct{}

func (n *continueNode) nodeType() string { return "continue" }

// stmtKeyword extracts the first word from a statement segment text.
func stmtKeyword(text string) string {
	text = strings.TrimSpace(text)
	for i, r := range text {
		if !unicode.IsLetter(r) && r != '_' {
			return text[:i]
		}
	}
	return text
}

// findKeyword finds the position of a keyword in an expression string,
// respecting parentheses and quotes. Returns -1 if not found.
func findKeyword(s string, kw string) int {
	depth := 0
	inStr := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inStr != 0 {
			if ch == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if ch == inStr {
				inStr = 0
			}
			continue
		}

		if ch == '\'' || ch == '"' {
			inStr = ch
			continue
		}

		if ch == '(' || ch == '[' || ch == '{' {
			depth++
			continue
		}
		if ch == ')' || ch == ']' || ch == '}' {
			depth--
			continue
		}

		if depth == 0 && i+len(kw) <= len(s) {
			// Check if keyword matches and is bounded by non-identifier chars.
			if s[i:i+len(kw)] == kw {
				before := i == 0 || !isIdentChar(s[i-1])
				after := i+len(kw) >= len(s) || !isIdentChar(s[i+len(kw)])
				if before && after {
					return i
				}
			}
		}
	}

	return -1
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// splitVars splits a comma-separated variable list, trimming whitespace.
func splitVars(s string) []string {
	parts := strings.Split(s, ",")
	var vars []string
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v != "" {
			vars = append(vars, v)
		}
	}
	return vars
}
