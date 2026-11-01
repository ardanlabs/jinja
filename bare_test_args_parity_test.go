package jinja_test

import (
	"testing"

	"github.com/ardanlabs/jinja"
)

// parityData is the context for TestBareTestArgumentParity. It is separate from
// bareTestArgData so that the two tables can change independently.
func parityData() map[string]any {
	return map[string]any{
		"n":     1,
		"m":     2,
		"s":     "a",
		"yes":   true,
		"no":    false,
		"nil":   nil,
		"list":  []any{1, 2},
		"words": []any{"a", "b"},
		"dict":  map[string]any{"a": 1, "b": map[string]any{"c": 2}, "l": []any{1, 2}},
	}
}

// TestBareTestArgumentParity holds cases beyond TestBareTestArgument that were
// found by rendering the same templates with the same context through Jinja2
// 3.1.6 (the environment in scripts/render_python.py: sandboxed, trim_blocks,
// lstrip_blocks, loopcontrols) and keeping those where this library agrees. The
// expected values are Jinja2's output. They cover spacing and trim markers around
// a test, names that start with an operator keyword, postfix after a bare
// argument, tests nested in other expressions and in statements.
func TestBareTestArgumentParity(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"FALSE is a name, not the literal", `{{ no is sameas FALSE }}`, "False"},
		{"NONE is a name, not the literal", `{{ nil is sameas NONE }}`, "False"},
		{"mixed case false is a name", `{{ no is sameas fAlSe }}`, "False"},
		{"none is not false", `{{ nil is sameas false }}`, "False"},
		{"test name used as argument is an undefined name", `{{ n is eq eq }}`, "False"},
		{"tight braces", `{{n is eq 1}}`, "True"},
		{"tight trim markers", `x {{-n is eq 1-}} y`, "xTruey"},
		{"tight string argument", `{{ s is eq'a' }}`, "True"},
		{"tight list argument", `{{ n is in[1,2] }}`, "True"},
		{"tight if tag", `{%if n is eq 1%}y{%else%}z{%endif%}`, "y"},
		{"tight if tag with trim markers", `a {%-if n is eq 1-%} y {%-endif-%} b`, "ayb"},
		{"keyword glued to a string", `{{ s is eq 'a'if yes else 'q' }}`, "True"},
		{"keyword glued to a number", `{{ 2 is eq 1if yes else 'q' }}`, "False"},
		{"name starting with not", `{% set not_a = 'a' %}{{ s is eq not_a }}`, "True"},
		{"name starting with if", `{% set iffy = 'a' %}{{ s is eq iffy }}`, "True"},
		{"name starting with in", `{% set in_x = 'a' %}{{ s is eq in_x }}`, "True"},
		{"name starting with and", `{% set andy = 'a' %}{{ s is eq andy }}`, "True"},
		{"name argument then and", `{% set a = 'a' %}{{ s is eq a and yes }}`, "True"},
		{"name argument then or", `{% set a = 'a' %}{{ s is eq a or no }}`, "True"},
		{"name argument then if", `{% set a = 'a' %}{{ s is eq a if yes else 'q' }}`, "True"},
		{"name argument then filter chain", `{{ 'a' is eq s| upper| lower }}`, "true"},
		{"space before call parens", `{{ 'A' is eq s.upper () }}`, "True"},
		{"spaced attribute", `{{ 2 is eq dict .b .c }}`, "True"},
		{"space before subscript", `{{ 2 is eq dict.b ['c'] }}`, "True"},
		{"space before attribute after subscript", `{{ 2 is eq dict['b'] .c }}`, "True"},
		{"slice then index", `{{ 1 is eq dict.l[0:1][0] }}`, "True"},
		{"negative index after slice", `{{ 2 is eq dict.l[1:][-1] }}`, "True"},
		{"string literal subscript", `{{ 'x' is eq 'xy'[0] }}`, "True"},
		{"closing braces in a string", `{{ s is eq '}}' }}`, "False"},
		{"closing block tag in a string", `{% if s is eq '%}' %}y{% else %}n{% endif %}`, "n"},
		{"is not with a name", `{{ n is not eq m }}`, "True"},
		{"not before is not", `{{ not n is not eq m }}`, "False"},
		{"is not in then and", `{{ n is not in dict.l and yes }}`, "False"},
		{"is in with a string", `{{ s is in 'abc' }}`, "True"},
		{"is in with a dict", `{{ s is in dict }}`, "True"},
		{"ge equal", `{{ n is ge 1 }}`, "True"},
		{"filter on a name argument result", `{{ n is eq m | upper }}`, "FALSE"},
		{"filter on an is not result", `{{ n is not eq m | string }}`, "True"},
		{"test in the else branch", `{{ 'a' if no else n is eq 1 }}`, "True"},
		{"tests in condition and else", `{{ 'a' if n is eq 2 else m is eq 2 }}`, "True"},
		{"test in both branches", `{{ n is eq 1 if n is eq 1 else n is eq 2 }}`, "True"},
		{"filtered branch value", `{{ 'a' | upper if n is eq 1 else 'b' }}`, "A"},
		{"filtered else value", `{{ 'a' if n is eq 2 else 'b' | upper }}`, "B"},
		{"nested in a subscript", `{{ words[n is eq 2 | int] }}`, "a"},
		{"nested in a slice bound", `{{ words[0 : n is eq 1 | int] }}`, "['a']"},
		{"nested in a namespace", `{{ namespace(a=n is eq 1, b=n is eq 2).b }}`, "False"},
		{"nested in a join filter", `{{ list | join(s is eq 'a') }}`, "1True2"},
		{"nested in a list of lists", `{{ [[n is eq 1, 2], [3]] }}`, "[[True, 2], [3]]"},
		{"dict with spaced closing braces", `{{ {'k': n is eq {'a': 1} } }}`, "{'k': False}"},
		{"elif with a filter", `{% if n is eq 2 %}a{% elif n is eq 1 | string %}b{% endif %}`, "b"},
		{"elif else chain", `{% if n is eq 3 %}a{% elif n is eq 2 %}b{% else %}c{% endif %}`, "c"},
		{"nested ifs", `{% if n is eq 1 %}{% if m is eq 3 %}y{% else %}z{% endif %}{% endif %}`, "z"},
		{"and not in if", `{% if n is eq 1 and not m is eq 3 %}y{% else %}z{% endif %}`, "y"},
		{"for filter with not", `{% for i in list if not i is eq 1 %}{{ i }}{% endfor %}`, "2"},
		{"for filter that matches nothing falls to else", `{% for i in list if i is gt 5 %}{{ i }}{% else %}none{% endfor %}`, "none"},
		{"for filter with a name argument and recursive", `{% for i in list if i is eq n recursive %}{{ i }}{% endfor %}`, "1"},
		{"for filter on the loop variable with sameas", `{% for i in [true, false, 1, 0] if i is sameas true %}[{{ i }}]{% endfor %}`, "[True]"},
		{"loop last separator", `{% for i in list %}{{ loop.index }}{% if loop.last is sameas false %}, {% endif %}{% endfor %}`, "1, 2"},
		{"continue guarded by a bare test", `{% for i in list %}{% if i is eq 1 %}{% continue %}{% endif %}{{ i }}{% endfor %}`, "2"},
		{"set block", `{% set v %}{{ n is eq 1 }}{% endset %}{{ v }}`, "True"},
		{"namespace attribute with and", `{% set ns = namespace(t=false) %}{% set ns.t = ns.t is sameas false and n is eq 1 %}{{ ns.t }}`, "True"},
		{"namespace attribute in a loop", `{% set ns = namespace(t=false) %}{% for i in list %}{% set ns.t = ns.t or i is eq 2 %}{% endfor %}{{ ns.t }}`, "True"},
		{"namespace counter in a for filter", `{% set ns = namespace(c=0) %}{% for i in list if i is gt 0 %}{% set ns.c = ns.c + 1 %}{% endfor %}{{ ns.c }}`, "2"},
		{"macro default with a filter", `{% macro f(a=n is eq 1 | string, b=2) %}{{ a }}{{ b }}{% endmacro %}{{ f() }}`, "True2"},
		{"macro argument tested for none", `{% macro f(a) %}{{ a is sameas none }}{% endmacro %}{{ f(none) }}{{ f(0) }}`, "TrueFalse"},
		{"macro call with a keyword bare test", `{% macro f(x) %}{{ x }}{% endmacro %}{{ f(x=n is eq 1 | string) }}`, "True"},
		{"caller result with a filter", `{% macro f() %}{{ caller() is eq 'x' | string }}{% endmacro %}{% call f() %}x{% endcall %}`, "True"},
		{"call block argument", `{% macro f() %}{{ caller(1) }}{% endmacro %}{% call(a) f() %}{{ a is eq 1 }}{% endcall %}`, "True"},
		{"newlines between tokens", "{{ n\n   is\n   eq\n   1 }}", "True"},
		{"newline before a filter", "{{ n is eq\n 1 | string }}", "True"},
		{"lstrip blocks", "x\n  {% if n is eq 1 %}\n  y\n  {% endif %}\nz", "x\n  y\nz"},
		{"trim marker both sides", "a\n{%- if n is eq 1 -%}\n  y\n{%- endif -%}\nb", "ayb"},
		{"trim marker on elif", "{% for i in list %}\n  {%- if i is eq 1 -%}\n    one\n  {%- elif i is eq 2 -%}\n    two\n  {%- endif -%}\n{% endfor %}", "onetwo"},
		{"indented elif keeps text", "{% for i in list %}\n  {% if i is eq 1 %}\n    one\n  {% elif i is eq 2 %}\n    two\n  {% endif %}\n{% endfor %}", "    one\n    two\n"},
		{"namespace flag first-item pattern", "{% set ns = namespace(f=true) %}\n{% for i in list %}\n{% if ns.f is sameas true %}\nfirst\n{% set ns.f = false %}\n{% else %}\nrest\n{% endif %}\n{% endfor %}", "first\nrest\n"},
		{"raw block containing a test", `{% raw %}{{ n is eq 1 }}{% endraw %}`, "{{ n is eq 1 }}"},
		{"macro trim", "{%- macro f(a=no is sameas false) -%}\n{{ a }}\n{%- endmacro -%}\n[{{ f() }}]", "[True]"},
		{"multi-test macro dispatch", `{% macro f(x) %}{% if x is sameas none %}N{% elif x is sameas false %}F{% elif x is sameas true %}T{% else %}O{% endif %}{% endmacro %}{{ f(none) }}{{ f(false) }}{{ f(true) }}{{ f(0) }}{{ f(1) }}{{ f('') }}{{ f() }}`, "NFTOOOO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.tmpl)
			if err != nil {
				t.Fatalf("compile %q: %v", tt.tmpl, err)
			}

			got, err := tmpl.Render(parityData())
			if err != nil {
				t.Fatalf("render %q: %v", tt.tmpl, err)
			}

			if got != tt.want {
				t.Errorf("%q = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

// TestBareTestArgumentParityErrors lists templates that Jinja2 3.1.6 rejects,
// either when compiling or when rendering, and that must still fail here.
func TestBareTestArgumentParityErrors(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
	}{
		{"two bare arguments", `{{ n is eq m s }}`},
		{"bare argument then a number", `{{ n is eq 1 2 }}`},
		{"is straight after a bare name", `{{ n is eq m is }}`},
		{"test name missing", `{{ n is }}`},
		{"test name missing after not", `{{ n is not }}`},
		{"number as test name", `{{ n is 1 }}`},
		{"unclosed paren", `{{ no is eq(false }}`},
		{"stray paren", `{{ no is eq false) }}`},
		{"unclosed bracket argument", `{{ n is in [1, 2 }}`},
		{"empty filter name", `{{ n is eq 1 | }}`},
		{"trailing dot", `{{ n is eq dict. }}`},
		{"missing endif", `{% if n is eq 1 %}y`},
		{"chained after is not", `{{ n is not defined is true }}`},
		{"unknown test name", `{{ n is eq1 }}`},
		{"unknown test name glued to its argument", `{{ no is sameasfalse }}`},
		{"else is not an argument", `{{ n is eq else }}`},
		{"and is not an argument", `{{ n is eq and yes }}`},
		{"or is not an argument", `{{ n is eq or yes }}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.tmpl)
			if err != nil {
				return
			}

			if _, err := tmpl.Render(parityData()); err == nil {
				t.Errorf("%q compiled and rendered, want an error", tt.tmpl)
			}
		})
	}
}
