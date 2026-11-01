package jinja_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ardanlabs/jinja"
)

// bareTestArgData is the context for the bare test argument tables. Each
// expected value below was checked against Jinja2 3.1.6, except where a case
// says it deliberately differs.
func bareTestArgData() map[string]any {
	return map[string]any{
		"n":    1,
		"m":    2,
		"s":    "a",
		"yes":  true,
		"no":   false,
		"nil":  nil,
		"list": []any{1, 2},
		"dict": map[string]any{"a": 1, "b": map[string]any{"c": 2}, "l": []any{1, 2}},
	}
}

func TestBareTestArgument(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		// Literals.
		{"false", `{{ no is sameas false }}`, "True"},
		{"False", `{{ no is sameas False }}`, "True"},
		{"true", `{{ yes is sameas true }}`, "True"},
		{"True", `{{ yes is sameas True }}`, "True"},
		{"none", `{{ nil is sameas none }}`, "True"},
		{"None", `{{ nil is sameas None }}`, "True"},
		{"true is not 1", `{{ 1 is sameas true }}`, "False"},
		{"false is not 0", `{{ 0 is sameas false }}`, "False"},
		{"undefined is not false", `{{ missing is sameas false }}`, "False"},
		{"not sameas undefined", `{{ missing is not sameas false }}`, "True"},
		{"string", `{{ s is eq 'a' }}`, "True"},
		{"double quoted string", `{{ s is eq "a" }}`, "True"},
		{"integer", `{{ n is eq 1 }}`, "True"},
		{"float", `{{ 1.5 is eq 1.5 }}`, "True"},
		{"gt", `{{ m is gt 1 }}`, "True"},

		// Names and postfix.
		{"name", `{{ n is eq m }}`, "False"},
		{"undefined name", `{{ n is eq missing }}`, "False"},
		{"dotted", `{{ n is eq dict.a }}`, "True"},
		{"dotted twice", `{{ m is eq dict.b.c }}`, "True"},
		{"indexed", `{{ n is eq list[0] }}`, "True"},
		{"string key", `{{ n is eq dict['a'] }}`, "True"},
		{"dotted then indexed", `{{ m is eq dict.l[1] }}`, "True"},
		{"sliced", `{{ list is eq list[0:] }}`, "True"},
		{"call", `{% macro f() %}a{% endmacro %}{{ s is eq f() }}`, "True"},
		{"method call", `{{ 'A' is eq s.upper() }}`, "True"},
		{"literal with postfix", `{{ n is eq [1, 2][0] }}`, "True"},

		// List and dict literals.
		{"in list", `{{ n is in [1, 2] }}`, "True"},
		{"in name", `{{ n is in list }}`, "True"},
		{"not in list", `{{ n is not in [1, 2] }}`, "False"},
		{"list", `{{ list is eq [1, 2] }}`, "True"},
		{"empty list", `{{ list is eq [] }}`, "False"},
		{"dict", `{{ {'a': 1} is eq {'a': 1} }}`, "True"},
		{"filter inside a list", `{{ s is in [s|upper, s] }}`, "True"},

		// not, and, or, if...else.
		{"is not", `{{ no is not sameas false }}`, "False"},
		{"not prefix", `{{ not no is sameas false }}`, "False"},
		{"and", `{{ no is sameas false and n is eq 1 }}`, "True"},
		{"or", `{{ no is sameas true or n is eq 1 }}`, "True"},
		{"or then and", `{{ no is sameas true or n is eq 1 and m is eq 2 }}`, "True"},
		{"inline if", `{{ 'y' if no is sameas false else 'n' }}`, "y"},
		{"inline else", `{{ 'y' if no is sameas true else 'n' }}`, "n"},
		{"inline if without else", `[{{ 'y' if no is sameas true }}]`, "[]"},
		{"test before inline if", `{{ no is sameas false if yes else 'n' }}`, "True"},
		{"in else branch", `{{ 'n' if no else yes is sameas true }}`, "True"},
		{"comparison after", `{{ no is sameas false == true }}`, "True"},
		{"nested in parens", `{{ (no is sameas false) and (n is eq 1) }}`, "True"},

		// Filters apply to the test's result, not to the bare argument.
		{"filter on the result", `{{ no is sameas false | string }}`, "True"},
		{"filter is not on the argument", `{{ 'False' is eq no | string }}`, "False"},
		{"parenthesised filter is on the argument", `{{ 'False' is eq(no | string) }}`, "True"},
		{"filter then comparison", `{{ no is sameas false | string == 'True' }}`, "True"},
		{"filter chain", `{{ no is sameas false | string | upper }}`, "TRUE"},
		{"filter with arguments", `{{ no is sameas false | replace('True', 'y') }}`, "y"},
		{"is not with filter", `{{ no is not sameas false | string }}`, "False"},
		{"filter on the left", `{{ s | upper is eq 'A' }}`, "True"},
		{"filter after attribute", `{{ s is eq dict.a | string }}`, "False"},
		{"no-argument test with filter", `{{ n is defined | string }}`, "True"},
		{"parenthesised test with filter", `{{ n is eq(1) | string }}`, "True"},

		// A test may follow a bare argument: only `is` straight after the
		// test name is an error in Jinja2.
		{"test of a test", `{{ n is eq m is sameas false }}`, "True"},
		{"test of a parenthesised test", `{{ n is eq(m) is sameas false }}`, "True"},
		{"test of a filtered test", `{{ n is eq 1 | string is string }}`, "True"},

		// Forms that already worked.
		{"parenthesised", `{{ no is sameas(false) }}`, "True"},
		{"parenthesised with a space", `{{ no is sameas (false) }}`, "True"},
		{"parenthesised is not", `{{ no is not sameas(false) }}`, "False"},
		{"parenthesised argument is not extended", `{{ n is eq(1) and yes }}`, "True"},
		{"no argument", `{{ n is defined }}`, "True"},
		{"no argument and", `{{ n is defined and yes }}`, "True"},
		{"no argument or", `{{ missing is defined or yes }}`, "True"},
		{"no argument inline else", `{{ 'y' if missing is defined else 'n' }}`, "n"},
		{"true test", `{{ yes is true }}`, "True"},
		{"none test", `{{ nil is none }}`, "True"},
		{"not none test", `{{ nil is not none }}`, "False"},

		// Jinja2 reads these keywords as the bare argument and then fails. They
		// are kept here as the operators they already were: taking them as an
		// argument would quietly turn `not in` into `in`, since tests here
		// ignore arguments they do not use.
		{"keeps inline if after a test", `{{ n is defined if yes else 'n' }}`, "True"},
		{"keeps in after a test", `{{ n is defined in [false] }}`, "False"},
		{"keeps not in after a test", `{{ n is defined not in [true] }}`, "False"},

		// Statements.
		{"if", `{% if no is sameas false %}y{% else %}n{% endif %}`, "y"},
		{"elif", `{% if no is sameas true %}a{% elif no is sameas false %}b{% endif %}`, "b"},
		{"if not", `{% if not no is sameas false %}y{% else %}n{% endif %}`, "n"},
		{"if with dict", `{% if {'a': {'b': 1}} is eq {'a': {'b': 1}} %}y{% endif %}`, "y"},
		{"for filter", `{% for i in [1, 2, 3] if i is gt 1 %}{{ i }}{% endfor %}`, "23"},
		{"for filter in list", `{% for i in [1, 2, 3] if i is in [1, 3] %}{{ i }}{% endfor %}`, "13"},
		{"for filter with recursive", `{% for i in [1, 2] if i is eq 1 recursive %}{{ i }}{% endfor %}`, "1"},
		{"in loop body", `{% for i in list %}{{ i is eq 1 }},{% endfor %}`, "True,False,"},
		{"loop variable", `{% for i in list %}{% if loop.index is eq i %}y{% endif %}{% endfor %}`, "yy"},
		{"set", `{% set v = no is sameas false %}{{ v }}`, "True"},
		{"set inline if", `{% set v = 'y' if no is sameas false else 'n' %}{{ v }}`, "y"},
		{"set namespace attribute", `{% set ns = namespace(t=false) %}{% set ns.t = ns.t or no is sameas false %}{{ ns.t }}`, "True"},
		{"set with equals in a string", `{% set v = s is eq 'a=b' %}{{ v }}`, "False"},
		{"macro default", `{% macro f(a=no is sameas false, b=2) %}{{ a }}{{ b }}{% endmacro %}{{ f() }}`, "True2"},

		// Inside other expressions.
		{"list item", `{{ [no is sameas false, 1] | length }}`, "2"},
		{"filter argument", `{{ 'x' | replace('x', no is sameas false) }}`, "True"},
		{"dict value", `{{ {'k': no is sameas false}.k }}`, "True"},
		{"subscript", `{{ ['a', 'b'][n is eq 2 | int] }}`, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.tmpl)
			if err != nil {
				t.Fatalf("compile %q: %v", tt.tmpl, err)
			}

			got, err := tmpl.Render(bareTestArgData())
			if err != nil {
				t.Fatalf("render %q: %v", tt.tmpl, err)
			}

			if got != tt.want {
				t.Errorf("%s = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestBareTestArgumentErrors(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"chained after a no-argument test", `{{ n is defined is true }}`, "cannot chain"},
		{"chained after is not", `{{ n is not defined is true }}`, "cannot chain"},
		{"chained in a statement", `{% if n is defined is true %}{% endif %}`, "cannot chain"},
		{"two arguments", `{{ n is eq m 1 }}`, "unexpected token"},
		{"two names", `{{ n is eq m s }}`, "unexpected token"},
		{"argument then not", `{{ n is sameas not m }}`, "unexpected token"},
		{"unary minus is not an argument", `{{ n is eq -1 }}`, "unexpected token"},

		// Jinja2 accepts these: it reads a test tighter than any operator, so the
		// operator applies to the test's result, and a bare comma makes a tuple.
		// Here they are errors, as they are after any test, because a bare
		// argument must not take the operator in and change the test's meaning.
		{"tuple is not an argument", `{{ n is eq 1, 2 }}`, "unexpected token"},
		{"arithmetic is not an argument", `{{ n is eq 1 + 1 }}`, "unexpected token"},
		{"concat is not an argument", `{{ n is eq 'a' ~ 'b' }}`, "unexpected token"},

		// Malformed arguments.
		{"unclosed list", `{% if n is in [1, 2 %}{% endif %}`, `expected "]"`},
		{"unclosed dict", `{% if n is eq {'a': 1 %}{% endif %}`, `expected "}"`},
		{"unclosed subscript", `{{ n is eq list[0 }}`, `expected "]"`},
		{"dangling dot", `{{ n is eq dict. }}`, "expected attribute name"},
		{"non-name test keeps rejecting an argument", `{{ n is 'eq' 1 }}`, "unexpected token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jinja.Compile(tt.tmpl)
			if err == nil {
				t.Fatalf("compile %q: expected an error", tt.tmpl)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("compile %q: error %q does not contain %q", tt.tmpl, err, tt.want)
			}
		})
	}
}

// TestMuseSparkOpenGolden renders the Muse Spark Open template, which is what
// needs bare test arguments (`d is sameas false`), against a conversation whose
// namespaced tools reach those tests. Only a function that is explicitly not
// deferred is listed from the deferred namespace.
//
// tools.rendered.txt is that template's output from Jinja2 3.1.6, using the
// environment and context in scripts/render_python.py plus current_date. The
// request lives here because testdata/requests is rendered through every
// template, and the others have no namespaced tools. Its keys are sorted because
// decoding it into a Go map loses their order.
func TestMuseSparkOpenGolden(t *testing.T) {
	const dir = "testdata/golden/Muse-Spark-Open-816B-A42B/"

	raw, err := os.ReadFile(dir + "tools.request.json")
	if err != nil {
		t.Fatalf("read request: %v", err)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var req map[string]any
	if err := dec.Decode(&req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	normalizeToolCallArguments(req)

	want, err := os.ReadFile(dir + "tools.rendered.txt")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	tmpl, err := jinja.Compile(chatTemplates["Muse-Spark-Open-816B-A42B"])
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := buildRenderData(req)
	data["current_date"] = "2026-11-01"

	got, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if got != string(want) {
		t.Errorf("rendered output diverges from Jinja2\n%s", diffSummary(got, string(want)))
	}
}
