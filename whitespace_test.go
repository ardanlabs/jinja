package jinja_test

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ardanlabs/jinja"
)

// whitespaceData is the context for the whitespace tests. It must stay in step
// with the context in scripts/render_snippets.py.
func whitespaceData() map[string]any {
	return map[string]any{
		"x":     "Max",
		"y":     "",
		"n":     3,
		"yes":   true,
		"no":    false,
		"items": []any{"a", "b"},
		"ml":    "l1\nl2",
	}
}

// whitespaceCase is a template and the bytes Jinja2 renders for it.
type whitespaceCase struct {
	name string
	tmpl string
	want string
}

// whitespaceCases pins Jinja2's whitespace control. Every want value was
// produced by Jinja2 3.1.6, and TestWhitespaceParity re-checks the table against
// Jinja2 wherever uv is installed.
var whitespaceCases = []whitespaceCase{
	// The reported shape.
	{"if with trim markers then an expression", "{%- if x -%}A: {{ x }}{%- endif %}|", "A: Max|"},
	{"the same inside a template line", "[{%- if x -%}Reasoning Effort: {{ x }}{%- endif -%}]", "[Reasoning Effort: Max]"},
	// A marker reaches only the adjacent text.
	{"{%- after an expression keeps earlier text", "a {{ x }}{%- if x %}b{% endif %}", "a Maxb"},
	{"{%- after a comment keeps earlier text", "a {# c #}{%- if x %}b{% endif %}", "a b"},
	{"{%- after a block keeps earlier text", "a {% set v = 1 %}{%- if x %}b{% endif %}", "a b"},
	{"{{- after an expression keeps earlier text", "a {{ x }}{{- x }}", "a MaxMax"},
	{"{{- after a comment keeps earlier text", "a {# c #}{{- x }}", "a Max"},
	{"{{- after a block keeps earlier text", "a {% set v = 1 %}{{- x }}", "a Max"},
	{"{#- after an expression keeps earlier text", "a {{ x }}{#- c #}", "a Max"},
	{"{#- after a comment keeps earlier text", "a {# c #}{#- d #}", "a "},
	{"{#- after a block keeps earlier text", "a {% set v = 1 %}{#- c #}", "a "},
	{"-%} before an expression keeps later text", "{% if x -%}{{ x }} b{% endif %}", "Max b"},
	{"-%} before a comment keeps later text", "{% if x -%}{# c #} b{% endif %}", " b"},
	{"-%} before a block keeps later text", "{% set v = 1 -%}{% set w = 2 %} b", " b"},
	{"-}} before an expression keeps later text", "{{ x -}}{{ x }} b", "MaxMax b"},
	{"-}} before a comment keeps later text", "{{ x -}}{# c #} b", "Max b"},
	{"-}} before a block keeps later text", "{{ x -}}{% set v = 1 %} b", "Max b"},
	{"-#} before an expression keeps later text", "{# c -#}{{ x }} b", "Max b"},
	{"-#} before a comment keeps later text", "{# c -#}{# d #} b", " b"},
	{"-#} before a block keeps later text", "{# c -#}{% set v = 1 %} b", " b"},
	{"marker in a loop body", "{% for i in items -%}{{ i }} {%- endfor %}|", "ab|"},
	{"empty branch between markers", " a {%- if no -%}  {%- endif -%} b ", " ab "},
	// A marker strips all adjacent whitespace.
	{"{%- over spaces", "a   {%- set v = 1 %}b", "ab"},
	{"{%- over a newline and tabs", "a\n\t\t{%- set v = 1 %}b", "ab"},
	{"{{- over mixed whitespace", "a \t\n\n {{- x }}", "aMax"},
	{"{#- over mixed whitespace", "a \t\n\n {#- c #}b", "ab"},
	{"-%} over mixed whitespace", "{% set v = 1 -%} \t\n\n b", "b"},
	{"-}} over mixed whitespace", "{{ x -}} \t\n\n b", "Maxb"},
	{"-#} over mixed whitespace", "{# c -#} \t\n\n b", "b"},
	{"whitespace-only text between two markers", "{{ x -}} \n\t {{- x }}", "MaxMax"},
	{"text with interior whitespace survives", "{{ x -}}  a  b  {{- x }}", "Maxa  bMax"},
	{"whitespace before a marker at the start", " \n {%- set v = 1 %}b", "b"},
	{"whitespace after a marker at the end", "a{% set v = 1 -%} \n ", "a"},
	// Plus disables the defaults.
	{"{%+ keeps indentation", "a\n  {%+ set v = 1 %}b", "a\n  b"},
	{"{#+ keeps indentation", "a\n  {#+ c #}b", "a\n  b"},
	{"+%} keeps the newline", "{% set v = 1 +%}\nb", "\nb"},
	{"+#} keeps the newline", "{# c +#}\nb", "\nb"},
	{"{{+ is accepted and changes nothing", "a {{+ x }} b", "a Max b"},
	{"{%+ and -%} together", "a\n  {%+ set v = 1 -%}\n  b", "a\n  b"},
	{"{%- and +%} together", "a\n  {%- set v = 1 +%}\n  b", "a\n  b"},
	{"the sign is not part of the statement", "{%+ if x +%}y{%+ endif +%}", "y"},
	// Lstrip_blocks and trim_blocks defaults.
	{"block indentation stripped", "a\n   {% set v = 1 %}b", "a\nb"},
	{"comment indentation stripped", "a\n   {# c #}b", "a\nb"},
	{"expression indentation kept", "a\n   {{ x }}b", "a\n   Maxb"},
	{"indentation at the template start", "   {% set v = 1 %}b", "b"},
	{"tabs are indentation", "a\n\t\t{% set v = 1 %}b", "a\nb"},
	{"text before the tag on its line blocks lstrip", "a\n b {% set v = 1 %}c", "a\n b c"},
	{"a tag before the tag on its line blocks lstrip", "{{ x }}  {% set v = 1 %}b", "Max  b"},
	{"newline after a block removed", "{% set v = 1 %}\nb", "b"},
	{"only one newline removed after a block", "{% set v = 1 %}\n\nb", "\nb"},
	{"newline after a comment removed", "{# c #}\nb", "b"},
	{"newline after an expression kept", "{{ x }}\nb", "Max\nb"},
	{"indented block after a block", "{% if x %}\n   {% set v = 1 %}\n   b\n{% endif %}\nc", "   b\nc"},
	{"lstrip sees through trim_blocks", "{% set v = 1 %}\n  {% set w = 2 %}\nb", "b"},
	// Newlines at the source level.
	{"single trailing newline dropped", "a\n", "a"},
	{"only one trailing newline dropped", "a\n\n", "a\n"},
	{"trailing newline after an expression dropped", "{{ x }}\n", "Max"},
	{"only a newline", "\n", ""},
	{"CRLF normalised", "a\r\nb", "a\nb"},
	{"CR normalised", "a\rb", "a\nb"},
	{"trailing CRLF dropped", "a\r\n", "a"},
	{"trailing CR dropped", "a\r", "a"},
	{"CRLF indentation stripped", "a\r\n  {% set v = 1 %}b", "a\nb"},
	{"CR indentation stripped", "a\r  {% set v = 1 %}b", "a\nb"},
	{"CRLF after a block removed", "{% set v = 1 %}\r\nb", "b"},
	{"CR in a string literal normalised", "{{ 'a\rb' }}", "a\nb"},
	// Whitespace is what Python says it is.
	{"form feed", "{{ x -}}\f b", "Maxb"},
	{"vertical tab", "a \u000b{%- set v = 1 %}b", "ab"},
	{"file separator", "{{ x -}}\u001cb", "Maxb"},
	{"unit separator", "a\u001f{{- x }}", "aMax"},
	{"next line", "{{ x -}}\u0085b", "Maxb"},
	{"no-break space", "{{ x -}}\u00a0b", "Maxb"},
	{"ogham space", "a\u1680{{- x }}", "aMax"},
	{"en quad", "{{ x -}}\u2000b", "Maxb"},
	{"hair space", "a\u200a{{- x }}", "aMax"},
	{"line separator", "{{ x -}}\u2028b", "Maxb"},
	{"paragraph separator", "a\u2029{{- x }}", "aMax"},
	{"narrow no-break space", "{{ x -}}\u202fb", "Maxb"},
	{"medium mathematical space", "a\u205f{{- x }}", "aMax"},
	{"ideographic space", "{{ x -}}\u3000b", "Maxb"},
	{"no-break space as indentation", "a\n\u00a0{% set v = 1 %}b", "a\nb"},
	{"form feed as indentation", "a\n\f{% set v = 1 %}b", "a\nb"},
	{"zero width space is not whitespace", "{{ x -}}\u200bb", "Max\u200bb"},
	{"byte order mark is not whitespace", "a\ufeff{{- x }}", "a\ufeffMax"},
	{"mongolian vowel separator is not whitespace", "a\u180e{{- x }}", "a\u180eMax"},
	// Raw blocks.
	{"markers in raw are text", "{% raw %}{{- x -}} {%- y -%} {#- z -#}{% endraw %}", "{{- x -}} {%- y -%} {#- z -#}"},
	{"whitespace in raw is kept", "{% raw %} {{ x }} \n {% y %} {% endraw %}", " {{ x }} \n {% y %} "},
	{"an unclosed tag in raw", "{% raw %}{{ {% endraw %}", "{{ "},
	{"a quote in raw", "{% raw %}it's{% endraw %}", "it's"},
	{"trim markers on both raw tags", "a {%- raw -%} b {%- endraw -%} c", "abc"},
	{"no trim_blocks after raw begin", "{% raw %}\n b{% endraw %}", "\n b"},
	{"trim_blocks after raw end", "{% raw %}b{% endraw %}\nc", "bc"},
	{"lstrip before raw begin", "a\n  {% raw %}b{% endraw %}", "a\nb"},
	{"lstrip before raw end", "{% raw %}b\n  {% endraw %}c", "b\nc"},
	{"plus before raw begin", "a\n  {%+ raw %}b{% endraw %}", "a\n  b"},
	{"plus before raw end", "{% raw %}b\n  {%+ endraw %}c", "b\n  c"},
	{"plus after raw end", "{% raw %}b{% endraw +%}\nc", "b\nc"},
	{"tight raw tags", "{%raw%}b{%endraw%}", "b"},
	{"spaced raw tags", "{%-   raw   -%} b {%-   endraw   -%}", "b"},
	{"raw between other tags", "{% if x %}{% raw %}{{ y }}{% endraw %}{% endif %}", "{{ y }}"},
	// Tags with nothing to do.
	{"empty text everywhere", "{{ x -}}{{- x }}", "MaxMax"},
	{"empty template", "", ""},
	{"only spaces", "   ", "   "},
	{"negative literal keeps the marker meaning", "a {{-1}} b", "a1 b"},
	{"closer lookalike in a string", "{{ '-}}' }} b", "-}} b"},
	{"closer lookalike in a block string", "{% if '-%}' %} b{% endif %}", " b"},
	{"comment with only markers", "a {#--#} b", "ab"},
	{"comment with one marker", "a {#-#} b", "a b"},
	{"comment with a plus marker", "a {#+#} b", "a  b"},
}

func renderWhitespace(t *testing.T, tmpl string) string {
	t.Helper()
	compiled, err := jinja.Compile(tmpl)
	if err != nil {
		t.Fatalf("compile %q: %v", tmpl, err)
	}
	got, err := compiled.Render(whitespaceData())
	if err != nil {
		t.Fatalf("render %q: %v", tmpl, err)
	}
	return got
}

func TestWhitespaceTable(t *testing.T) {
	for _, tt := range whitespaceCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderWhitespace(t, tt.tmpl); got != tt.want {
				t.Errorf("template %q\n got: %q\nwant: %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

// TestWhitespaceSyntaxErrors holds forms Jinja2 rejects at compile time.
func TestWhitespaceSyntaxErrors(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"there is no +}} closer", `{{ x +}}`, ""},
		{"a raw tag cannot end in +%}", `{% raw +%}a{% endraw %}`, `unknown statement "raw"`},
		{"endraw without raw", `{% endraw %}`, `unknown statement "endraw"`},
		{"raw without endraw", `a{% raw %}b`, "unclosed raw block at line 1, col 2"},
		{"raw without endraw after a newline", "a\n {% raw %}b{% endra %}", "unclosed raw block at line 2, col 2"},
		{"empty block", `a{% %}b`, ""},
		{"empty block with a minus", `a{%- %}b`, ""},
		{"empty block with a plus", `a{%+ %}b`, ""},
		{"empty block with markers on both sides", `a{%- -%}b`, ""},
		{"expression opened at the very end", "a{{", "unclosed expression tag at line 1, col 2"},
		{"statement opened at the very end", "a{%", "unclosed statement tag at line 1, col 2"},
		{"unclosed expression", "a\n  {{ x", "unclosed expression tag at line 2, col 3"},
		{"unclosed block", "a\r\n  {% if", "unclosed statement tag at line 2, col 3"},
		{"unclosed comment", "a\r  {# x", "unclosed comment tag at line 2, col 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jinja.Compile(tt.tmpl)
			if err == nil {
				t.Fatalf("compile %q: want an error", tt.tmpl)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("compile %q: error %q does not contain %q", tt.tmpl, err, tt.want)
			}
		})
	}
}

// pythonWhitespace is every code point Python treats as whitespace, which is what
// Jinja2's trim markers strip. TestWhitespaceParity checks it against Python.
var pythonWhitespace = []rune{
	0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x1C, 0x1D, 0x1E, 0x1F, 0x20, 0x85, 0xA0, 0x1680,
	0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009,
	0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000,
}

// TestWhitespaceCodePoints runs every code point past a trim marker on each side
// of a tag and past lstrip_blocks.
func TestWhitespaceCodePoints(t *testing.T) {
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) || r == '{' || r == '\r' {
			continue
		}

		text := string(r)
		kept, stripped := text, ""
		if !slices.Contains(pythonWhitespace, r) {
			stripped = text
		}

		tests := []struct{ tmpl, want string }{
			{"a" + text + "{{- x }}", "a" + stripped + "Max"},
			{"{{ x -}}" + text + "|", "Max" + stripped + "|"},
			{"\n" + text + "{% set v = 1 %}|", "\n" + stripped + "|"},
			{"a" + text + "{{ x }}", "a" + kept + "Max"},
		}

		// A newline is not indentation: it ends the line lstrip_blocks looks at.
		if r == '\n' {
			tests[2].want = "\n\n|"
		}

		for _, tt := range tests {
			if got := renderWhitespace(t, tt.tmpl); got != tt.want {
				t.Fatalf("U+%04X: template %q\n got: %q\nwant: %q", r, tt.tmpl, got, tt.want)
			}
		}
	}
}

// gridTag is one tag in the whitespace grid.
type gridTag struct {
	src       string // The tag's source.
	output    string // What the tag renders.
	left      string // Opening sign.
	right     string // Closing sign.
	lstrip    bool   // Subject to lstrip_blocks.
	trimBlock bool   // Subject to trim_blocks.
}

// gridSigns returns every whitespace control sign legal on a tag.
var gridSigns = []string{"", "-", "+"}

// gridPads is the spacing inside a tag, which the grid rotates through.
var gridPads = []string{" ", "", "\n"}

// gridTags returns a tag for every legal pair of signs around body.
func gridTags(start, body, end, output string, rights []string, block, trimBlock bool) []gridTag {
	var tags []gridTag
	for _, left := range gridSigns {
		for _, right := range rights {
			pad := gridPads[len(tags)%len(gridPads)]
			tags = append(tags, gridTag{
				src:       start + left + pad + body + pad + right + end,
				output:    output,
				left:      left,
				right:     right,
				lstrip:    block,
				trimBlock: trimBlock,
			})
		}
	}
	return tags
}

// gridPairs returns every pair of tags the grid places around a run of text:
// each mix of expression, block and comment, an if statement, and a raw block.
func gridPairs() [][2]gridTag {
	single := slices.Concat(
		gridTags("{{", "x", "}}", "Max", []string{"", "-"}, false, false),
		gridTags("{%", "set v = 1", "%}", "", gridSigns, true, true),
		gridTags("{#", "c", "#}", "", gridSigns, true, true),
	)

	var pairs [][2]gridTag
	for _, first := range single {
		for _, second := range single {
			pairs = append(pairs, [2]gridTag{first, second})
		}
	}

	ifs := gridTags("{%", "if yes", "%}", "", gridSigns, true, true)
	endifs := gridTags("{%", "endif", "%}", "", gridSigns, true, true)
	for _, first := range ifs {
		for _, second := range endifs {
			pairs = append(pairs, [2]gridTag{first, second})
		}
	}

	raws := gridTags("{%", "raw", "%}", "", []string{"", "-"}, true, false)
	endraws := gridTags("{%", "endraw", "%}", "", gridSigns, true, true)
	for _, first := range raws {
		for _, second := range endraws {
			pairs = append(pairs, [2]gridTag{first, second})
		}
	}

	return pairs
}

// gridTexts is the text placed around the tags in the grid.
var gridTexts = []string{
	"", " ", "\t", "\n", "\n\n", " \n\t ", "  \n  ", "a", " a ", "\na\n", "a\n  ",
}

// gridCase is the template pre first mid second post.
type gridCase struct {
	pre    string
	first  gridTag
	mid    string
	second gridTag
	post   string
}

func (c gridCase) template() string {
	return c.pre + c.first.src + c.mid + c.second.src + c.post
}

// want works out the expected output from the rules as Jinja2 documents them,
// one side of a tag at a time. It is deliberately not how the scanner does it, so
// that the two can check each other. TestWhitespaceParity checks the scanner
// against Jinja2 on the same grid, which keeps this model honest.
func (c gridCase) want() string {
	const space = " \t\n"

	left := func(text string, tag gridTag, atStart bool) string {
		switch {
		case tag.left == "-":
			return strings.TrimRight(text, space)
		case tag.left == "+" || !tag.lstrip:
			return text
		}
		line := strings.LastIndexByte(text, '\n') + 1
		if (line > 0 || atStart) && strings.Trim(text[line:], " \t") == "" {
			return text[:line]
		}
		return text
	}

	right := func(text string, tag gridTag) string {
		switch {
		case tag.right == "-":
			return strings.TrimLeft(text, space)
		case tag.right == "" && tag.trimBlock:
			return strings.TrimPrefix(text, "\n")
		}
		return text
	}

	pre := left(c.pre, c.first, true)
	mid := right(left(c.mid, c.second, false), c.first)
	post := right(strings.TrimSuffix(c.post, "\n"), c.second)

	return pre + c.first.output + mid + c.second.output + post
}

// TestWhitespaceGrid renders every sign on every tag next to every kind of
// neighbour, through every combination of the texts in gridTexts.
func TestWhitespaceGrid(t *testing.T) {
	for _, pair := range gridPairs() {
		for _, pre := range gridTexts {
			for _, mid := range gridTexts {
				for _, post := range gridTexts {
					c := gridCase{pre: pre, first: pair[0], mid: mid, second: pair[1], post: post}
					if got, want := renderWhitespace(t, c.template()), c.want(); got != want {
						t.Fatalf("template %q\n got: %q\nwant: %q", c.template(), got, want)
					}
				}
			}
		}
	}
}
