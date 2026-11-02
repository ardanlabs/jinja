// Package jinja implements a Jinja2-compatible template engine.
package jinja

import (
	"fmt"
	"strings"
	"unicode"
)

// segKind identifies the type of a scanned template segment.
type segKind int

const (
	segText    segKind = iota // Raw text between tags.
	segComment                // {# ... #}
	segExpr                   // {{ ... }}
	segStmt                   // {% ... %}
)

// Whitespace control signs. A sign follows a tag's opener or precedes its
// closer: {%- -%} strips whitespace, {%+ +%} keeps it.
const (
	signNone  byte = 0
	signMinus byte = '-'
	signPlus  byte = '+'
)

// segment represents a single scanned piece of a template.
type segment struct {
	kind segKind
	text string // Content between delimiters (tags) or raw text.
	line int    // 1-based line number where the segment starts.
	col  int    // 1-based column number where the segment starts.
}

// scanner holds the state of one scan. It works the way the Jinja2 lexer does:
// whitespace control is applied while the source is read, so a tag can only ever
// affect the text touching it. If the neighbour of a tag is another tag, there is
// no text between them to strip.
//
// lstrip_blocks and trim_blocks are always on, because that is what HuggingFace's
// apply_chat_template uses and chat templates are written against it. Both apply
// to block and comment tags and never to expressions, and an explicit sign on
// that side of the tag takes precedence.
type scanner struct {
	src  string
	pos  int
	line int
	col  int

	// lineStarting records that the last tag closer ended in a newline, so the
	// text after it starts a line. It is what lets lstrip_blocks see indentation
	// whose newline trim_blocks has already consumed.
	lineStarting bool

	segs []segment
}

// scan splits source into a flat slice of segments representing the text,
// comments, expressions, and statements that make up a Jinja template. Text
// segments come back with whitespace control already applied, and the contents
// of a raw block come back as plain text.
func scan(source string) ([]segment, error) {
	s := scanner{
		src:          normalizeNewlines(source),
		line:         1,
		col:          1,
		lineStarting: true,
	}

	for {
		open := s.nextOpener()
		if open < 0 {
			break
		}
		if err := s.scanTag(open); err != nil {
			return nil, err
		}
	}

	s.emitText(s.src[s.pos:])
	return s.segs, nil
}

// normalizeNewlines converts every line ending to "\n" and removes one trailing
// newline, matching Jinja2's defaults of newline_sequence="\n" and
// keep_trailing_newline=False.
func normalizeNewlines(source string) string {
	if strings.IndexByte(source, '\r') >= 0 {
		source = strings.ReplaceAll(source, "\r\n", "\n")
		source = strings.ReplaceAll(source, "\r", "\n")
	}
	return strings.TrimSuffix(source, "\n")
}

// isSpace reports whether Python considers r to be whitespace, which is the set
// behind both str.rstrip and the \s used by the Jinja2 lexer. It is Go's set plus
// the four information separators.
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1C && r <= 0x1F)
}

// skipSpace returns the offset of the first non-whitespace byte at or after i.
func (s *scanner) skipSpace(i int) int {
	return len(s.src) - len(strings.TrimLeftFunc(s.src[i:], isSpace))
}

// signAt returns the whitespace control sign at offset i.
func (s *scanner) signAt(i int) byte {
	if i < len(s.src) && (s.src[i] == signMinus || s.src[i] == signPlus) {
		return s.src[i]
	}
	return signNone
}

// nextOpener returns the offset of the next tag opener, or -1 if there is none.
func (s *scanner) nextOpener() int {
	for i := s.pos; ; {
		j := strings.IndexByte(s.src[i:], '{')
		if j < 0 || i+j+1 >= len(s.src) {
			return -1
		}
		i += j + 1

		switch s.src[i] {
		case '{', '%', '#':
			return i - 1
		}
	}
}

// advanceTo moves the scanner to offset pos, keeping the line and column current.
func (s *scanner) advanceTo(pos int) {
	run := s.src[s.pos:pos]
	if n := strings.Count(run, "\n"); n > 0 {
		s.line += n
		s.col = len(run) - strings.LastIndexByte(run, '\n')
	} else {
		s.col += len(run)
	}
	s.pos = pos
}

// emitText appends a text segment starting at the current position.
func (s *scanner) emitText(text string) {
	if text == "" {
		return
	}
	s.segs = append(s.segs, segment{kind: segText, text: text, line: s.line, col: s.col})
}

// trimBefore applies a tag's opening whitespace control to the text in front of
// it. A minus strips all trailing whitespace and a plus keeps everything.
// Otherwise, for tags subject to lstrip_blocks, indentation is removed when
// nothing but whitespace sits between the start of the line and the tag.
func (s *scanner) trimBefore(text string, sign byte, lstrip bool) string {
	switch {
	case sign == signMinus:
		return strings.TrimRightFunc(text, isSpace)
	case sign == signPlus || !lstrip:
		return text
	}

	start := strings.LastIndexByte(text, '\n') + 1
	if (start > 0 || s.lineStarting) && strings.TrimRightFunc(text[start:], isSpace) == "" {
		return text[:start]
	}
	return text
}

// closeTag moves past a tag closer that ends at offset end, applying the tag's
// closing sign to the text after it. A minus consumes all following whitespace.
func (s *scanner) closeTag(end int, sign byte) {
	if sign == signMinus {
		end = s.skipSpace(end)
	}

	s.advanceTo(end)
	s.lineStarting = s.src[end-1] == '\n'
}

// closeBlock is closeTag for tags subject to trim_blocks: with no sign, it also
// consumes a single newline.
func (s *scanner) closeBlock(end int, sign byte) {
	if sign == signNone && end < len(s.src) && s.src[end] == '\n' {
		end++
	}

	s.closeTag(end, sign)
}

// scanTag scans the tag whose opener is at offset open, along with the text in
// front of it.
func (s *scanner) scanTag(open int) error {
	var kind segKind
	var closer string

	switch s.src[open+1] {
	case '{':
		kind, closer = segExpr, "}}"
	case '%':
		kind, closer = segStmt, "%}"
	case '#':
		kind, closer = segComment, "#}"
	}

	sign := s.signAt(open + 2)
	s.emitText(s.trimBefore(s.src[s.pos:open], sign, kind != segExpr))
	s.advanceTo(open)

	// A raw tag closed by +%} is not a raw directive to the Jinja2 lexer, so it
	// falls through to the parser and is rejected as an unknown statement.
	if kind == segStmt {
		rawSign, rawEnd, ok := s.matchRawTag(open, "raw")
		if ok && rawSign != signPlus {
			return s.scanRaw(rawSign, rawEnd)
		}
	}

	start := open + 2
	if sign != signNone {
		start++
	}

	line, col := s.line, s.col

	contentEnd, closeSign, end, ok := s.findCloser(start, kind, closer)
	if !ok {
		tagKind := "expression"
		switch kind {
		case segStmt:
			tagKind = "statement"
		case segComment:
			tagKind = "comment"
		}
		return fmt.Errorf("unclosed %s tag at line %d, col %d", tagKind, line, col)
	}

	s.segs = append(s.segs, segment{kind: kind, text: s.src[start:contentEnd], line: line, col: col})

	if kind == segExpr {
		s.closeTag(end, closeSign)
	} else {
		s.closeBlock(end, closeSign)
	}
	return nil
}

// findCloser scans forward from offset i for the end of a tag, ignoring closers
// inside quoted strings. It returns the offset where the tag's content ends, the
// closing sign, and the offset just past the closer.
func (s *scanner) findCloser(i int, kind segKind, closer string) (contentEnd int, sign byte, end int, ok bool) {
	src := s.src

	closes := func(i int) bool {
		return i+1 < len(src) && src[i] == closer[0] && src[i+1] == closer[1]
	}

	for i < len(src) {
		ch := src[i]

		switch {
		case ch == '\'' || ch == '"':
			// Comments are raw text until #}, so quotes mean nothing there.
			if kind != segComment {
				i = skipString(src, i)
				continue
			}
		case ch == closer[0] && closes(i):
			return i, signNone, i + 2, true

		// Expressions have no +}} closer, so there a plus stays in the content
		// and the expression parser reports it.
		case (ch == signMinus || (ch == signPlus && kind != segExpr)) && closes(i+1):
			return i, ch, i + 3, true
		}

		i++
	}

	return 0, signNone, 0, false
}

// skipString returns the offset just past the quoted string starting at offset i.
func skipString(src string, i int) int {
	quote := src[i]

	for i++; i < len(src) && src[i] != quote; i++ {
		if src[i] == '\\' {
			i++
		}
	}

	return min(i+1, len(src))
}

// matchRawTag reports whether a {% tag holding only word starts at offset open,
// returning the tag's closing sign and the offset just past its closer. Raw tags
// are matched here rather than by keyword because their contents must not be
// scanned as template syntax.
func (s *scanner) matchRawTag(open int, word string) (byte, int, bool) {
	i := open + 2
	if s.signAt(i) != signNone {
		i++
	}

	i = s.skipSpace(i)
	if !strings.HasPrefix(s.src[i:], word) {
		return signNone, 0, false
	}
	i = s.skipSpace(i + len(word))

	sign := s.signAt(i)
	if sign != signNone {
		i++
	}
	if !strings.HasPrefix(s.src[i:], "%}") {
		return signNone, 0, false
	}

	return sign, i + 2, true
}

// scanRaw scans a raw block whose opening tag ends at offset end. The contents
// become a single text segment. The opening tag gets no trim_blocks, which is how
// the Jinja2 lexer treats it, but the rest of the whitespace control is as usual.
func (s *scanner) scanRaw(sign byte, end int) error {
	line, col := s.line, s.col
	s.closeTag(end, sign)

	for i := s.pos; ; {
		j := strings.Index(s.src[i:], "{%")
		if j < 0 {
			return fmt.Errorf("unclosed raw block at line %d, col %d", line, col)
		}
		open := i + j

		closeSign, closeEnd, ok := s.matchRawTag(open, "endraw")
		if !ok {
			i = open + 2
			continue
		}

		s.emitText(s.trimBefore(s.src[s.pos:open], s.signAt(open+2), true))
		s.advanceTo(open)
		s.closeBlock(closeEnd, closeSign)
		return nil
	}
}
