package jinja_test

import (
	"testing"

	"github.com/ardanlabs/jinja"
)

// Jinja2 concatenates adjacent string literals in parse_primary, which is also
// what makes a Python-style """triple-quoted""" string work: it lexes as "",
// "...", "". Nemotron 3.5 Content Safety's chat template relies on it. Every
// expected value is what Jinja2 3.1 renders for the same source.
func TestAdjacentStringLiterals(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"two", `{{ "a" "b" }}`, "ab"},
		{"mixed quotes", `{{ "a" 'b' "c" }}`, "abc"},
		{"no whitespace", `{{ "a""b" }}`, "ab"},
		{"newline between", "{{ \"a\"\n\"b\" }}", "ab"},
		{"tabs and spaces between", "{{ 'a' \t  'b' }}", "ab"},
		{"triple double quote", `{{ """abc""" }}`, "abc"},
		{"triple multiline in set", "{% set x = \"\"\"multi\nline\"\"\" %}{{ x }}", "multi\nline"},
		{"triple single quote", `{{ '''abc''' }}`, "abc"},
		{"triple with inner quote", `{{ """say 'hi'""" }}`, "say 'hi'"},
		{"adjacent across lines", "{% set x = \"one \"\n    \"two \"\n    \"three\" %}{{ x }}", "one two three"},
		{"in parens with concat", `{{ ("a" "b") ~ "c" }}`, "abc"},
		{"filter applies to whole", `{{ "a" "b" | upper }}`, "AB"},
		{"in comparison", `{% if "x" "y" == "xy" %}ok{% endif %}`, "ok"},
		{"as call argument", `{{ ["a", "b"] | join("-" "+") }}`, "a-+b"},
		{"as dict key", `{{ {"k" "ey": 1}["key"] }}`, "1"},
		{"escapes preserved", `{{ "a\n" "b" }}`, "a\nb"},
		{"empty strings", `{{ "" "" }}`, ""},
		{"single empty", `{{ "" }}`, ""},
		{"list items stay separate", `{{ ["a" "b", "c"] | length }}`, "2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tt.src, err)
			}
			got, err := tmpl.Render(nil)
			if err != nil {
				t.Fatalf("render %q: %v", tt.src, err)
			}
			if got != tt.want {
				t.Errorf("%q = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

// A string literal next to a non-string operand is still a syntax error.
func TestAdjacentStringLiteralNegative(t *testing.T) {
	for _, src := range []string{
		`{{ "a" 1 }}`,
		`{{ "a" b }}`,
		`{{ 1 "a" }}`,
		`{{ b "a" }}`,
		`{{ "a" "b" "c" ) }}`,
	} {
		if _, err := jinja.Compile(src); err == nil {
			t.Errorf("Compile(%q) succeeded, want an error", src)
		}
	}
}
