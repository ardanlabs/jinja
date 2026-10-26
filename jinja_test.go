package jinja_test

import (
	"context"
	"fmt"
	"maps"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/ardanlabs/jinja"
)

// =============================================================================
// Basic engine tests (no model dependency)
// =============================================================================

func TestBasicRender(t *testing.T) {
	tmpl, err := jinja.Compile("Hello {{ name }}!")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"name": "World",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}

func TestNumericEquality(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"distinct large integers", `{{ 9007199254740992 == 9007199254740993 }}`, "False"},
		{"large integer and rounded float", `{{ 9007199254740993 == 9007199254740992.0 }}`, "False"},
		{"large integer inequality", `{{ 9007199254740992 != 9007199254740993 }}`, "True"},
		{"equal integer and float", `{{ 1 == 1.0 }}`, "True"},
		{"nested numeric equality", `{{ [{'qfi': 1}] == [{'qfi': 1.0}] }}`, "True"},
		{"numeric membership", `{{ 1.0 in [1] }}`, "True"},
		{"boolean numeric equality", `{{ true == 1 }}`, "True"},
		{"equalto numeric equality", `{{ 1 is equalto(1.0) }}`, "True"},
		{"float before integer", `{{ 1.0 == 1 }}`, "True"},
		{"fractional float", `{{ -1 == -1.5 }}`, "False"},
		{"minimum integer and float", `{{ min_int == -9223372036854775808.0 }}`, "True"},
		{"float above integer range", `{{ min_int == 9223372036854775808.0 }}`, "False"},
		{"float below integer range", `{{ min_int == -9223372036854777856.0 }}`, "False"},
		{"maximum integer and rounded float", `{{ 9223372036854775807 == 9223372036854775808.0 }}`, "False"},
		{"integer and nan", `{{ min_int == nan }}`, "False"},
		{"nan and itself", `{{ nan == nan }}`, "False"},
		{"integer and infinity", `{{ min_int == inf }}`, "False"},
		{"float infinity", `{{ inf == inf }}`, "True"},
		// sameas tests identity, not numeric equality.
		{"integer and float are not identical", `{{ 1 is sameas(1.0) }}`, "False"},
		{"true and integer are not identical", `{{ true is sameas(1) }}`, "False"},
		{"false and float are not identical", `{{ false is sameas(0.0) }}`, "False"},
		{"equal lists are not identical", `{{ [1] is sameas([1.0]) }}`, "False"},
		{"equal dictionaries are not identical", `{{ {'qfi': 1} is sameas({'qfi': 1.0}) }}`, "False"},
		{"list identity", `{% set a = [1] %}{{ a is sameas(a) }}`, "True"},
		{"dictionary identity", `{% set a = {'qfi': 1} %}{{ a is sameas(a) }}`, "True"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tc.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(map[string]any{
				"min_int": int64(math.MinInt64),
				"nan":     math.NaN(),
				"inf":     math.Inf(1),
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNumericOrdering(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"distinct large integers", `{{ 9007199254740992 < 9007199254740993 }}`, "True"},
		{"large integer and rounded float", `{{ 9007199254740993 > 9007199254740992.0 }}`, "True"},
		{"float before large integer", `{{ 9007199254740992.0 < 9007199254740993 }}`, "True"},
		{"negative large integer", `{{ -9007199254740993 < -9007199254740992.0 }}`, "True"},
		{"non-strict large integer comparisons", `{{ 9007199254740993 <= 9007199254740992 }}|{{ 9007199254740992 >= 9007199254740993 }}`, "False|False"},
		{"fractional comparisons", `{{ -1 > -1.5 }}|{{ -1 < -0.5 }}|{{ 1 < 1.5 }}|{{ 1.5 > 1 }}`, "True|True|True|True"},
		{"integer boundaries", `{{ max_int < 9223372036854775808.0 }}|{{ min_int >= -9223372036854775808.0 }}|{{ min_int > -9223372036854777856.0 }}`, "True|True|True"},
		{"sort large integers", `{{ [9007199254740993, 9007199254740992] | sort }}`, "[9007199254740992, 9007199254740993]"},
		{"reverse sort large integers", `{{ [9007199254740992, 9007199254740993] | sort(reverse=true) }}`, "[9007199254740993, 9007199254740992]"},
		{"minimum large integer", `{{ [9007199254740993, 9007199254740992] | min }}`, "9007199254740992"},
		{"maximum large integer", `{{ [9007199254740992, 9007199254740993] | max }}`, "9007199254740993"},
		{"ordering tests", `{{ 9007199254740993 is gt(9007199254740992) }}|{{ 9007199254740992 is lt(9007199254740993) }}|{{ 9007199254740993 is le(9007199254740992) }}|{{ 9007199254740992 is ge(9007199254740993) }}`, "True|True|False|False"},
		{"nan on right", `{{ 1 < nan }}|{{ 1 > nan }}|{{ 1 <= nan }}|{{ 1 >= nan }}`, "False|False|False|False"},
		{"nan on left", `{{ nan < 1 }}|{{ nan > 1 }}|{{ nan <= 1 }}|{{ nan >= 1 }}`, "False|False|False|False"},
		{"float nan comparisons", `{{ 1.5 < nan }}|{{ nan > 1.5 }}|{{ nan <= nan }}|{{ nan >= nan }}`, "False|False|False|False"},
		{"nan ordering tests", `{{ 1 is gt(nan) }}|{{ nan is lt(1) }}|{{ 1 is le(nan) }}|{{ nan is ge(1) }}`, "False|False|False|False"},
		{"equal numeric values remain ordered", `{{ 1 <= 1.0 }}|{{ 1.0 >= 1 }}|{{ 1 < 1.0 }}|{{ 1.0 > 1 }}`, "True|True|False|False"},
		{"boolean numeric ordering", `{{ false < 0.5 }}|{{ 0.5 < true }}|{{ true >= 1 }}|{{ 0 <= false }}`, "True|True|True|True"},
		{"infinities", `{{ max_int < inf }}|{{ min_int > -inf }}`, "True|True"},
		{"small numeric and string controls", `{{ 2 < 3 }}|{{ 3.5 > 2.5 }}|{{ false < true }}|{{ 'apple' < 'banana' }}`, "True|True|True|True"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tc.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(map[string]any{
				"min_int": int64(math.MinInt64),
				"max_int": int64(math.MaxInt64),
				"nan":     math.NaN(),
				"inf":     math.Inf(1),
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNumericLiterals(t *testing.T) {
	for _, tc := range []struct {
		name           string
		source         string
		want           string
		wantCompileErr bool
	}{
		{"malformed exponent", `{{ 1.2e+ }}`, "", true},
		{"oversized integer", `{{ 999999999999999999999999 }}`, "", true},
		{"integer above supported range", `{{ 9223372036854775808 }}`, "", true},
		{"integer below supported range", `{{ -9223372036854775809 }}`, "", true},
		{"float overflow", `{{ 1.0e400 }}`, "", true},
		{"minimum integer", `{{ -9223372036854775808 }}`, "-9223372036854775808", false},
		{"minimum integer with filter", `{{ -9223372036854775808 | int }}`, "-9223372036854775808", false},
		{"parenthesized minimum integer with filter", `{{ (-9223372036854775808) | int }}`, "-9223372036854775808", false},
		{"minimum integer with chained filters", `{{ -9223372036854775808 | string | length }}`, "20", false},
		{"maximum integer", `{{ 9223372036854775807 }}`, "9223372036854775807", false},
		{"valid exponent", `{{ 1.25e+2 == 125.0 }}`, "True", false},
		{"small negative integer with filter", `{{ -42 | int }}`, "-42", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tc.source)
			if tc.wantCompileErr {
				if err == nil {
					t.Fatal("compile: got no error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(nil)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSequenceRepetition(t *testing.T) {
	for _, tc := range []struct {
		name    string
		source  string
		want    string
		wantErr bool
	}{
		{"string repetition", `{{ 'ab' * 3 }}`, "ababab", false},
		{"reversed string operands", `{{ 3 * '-' }}`, "---", false},
		{"negative string count", `{{ 'x' * -1 }}`, "", false},
		{"list repetition", `{{ [1, 0] * 2 }}`, "[1, 0, 1, 0]", false},
		{"reversed list operands", `{{ 2 * [1, 0] }}`, "[1, 0, 1, 0]", false},
		{"negative list count", `{{ [1] * -2 }}`, "[]", false},
		{"zero counts", `{{ 'x' * 0 }}|{{ [1] * 0 }}`, "|[]", false},
		{"boolean counts", `{{ 'x' * true }}|{{ false * [1] }}`, "x|[]", false},
		{"unicode repetition", `{{ 3 * 'é' }}`, "ééé", false},
		{"empty list copy independence", `{% set a = [] %}{% set b = a * 3 %}{% set ignored = b.append(1) %}{{ a }}|{{ b }}`, "[]|[1]", false},
		{"nonempty list copy independence", `{% set a = [2] %}{% set b = a * 2 %}{% set ignored = b.append(1) %}{{ a }}|{{ b }}`, "[2]|[2, 2, 1]", false},
		{"zero count copy independence", `{% set a = [2] %}{% set b = a * 0 %}{% set ignored = b.append(1) %}{{ a }}|{{ b }}`, "[2]|[1]", false},
		{"nested elements remain shared", `{% set a = [[]] %}{% set b = a * 2 %}{% set ignored = b[0].append(7) %}{{ a }}|{{ b }}`, "[[7]]|[[7], [7]]", false},
		{"empty string huge count", `{{ '' * 9223372036854775807 }}`, "", false},
		{"empty list huge count", `{{ [] * 9223372036854775807 }}`, "[]", false},
		{"string size overflow", `{{ 'NR' * 9223372036854775807 }}`, "", true},
		{"list size overflow", `{{ 9223372036854775807 * [1, 2] }}`, "", true},
		{"string allocation budget", `{{ 'x' * 67108865 }}`, "", true},
		{"list allocation budget", `{{ [1] * oversized_list_count }}`, "", true},
		{"numeric multiplication unchanged", `{{ 3 * 4 }}|{{ 2.5 * 4 == 10.0 }}`, "12|True", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tc.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			// Report a rendering panic as a failure without aborting other cases.
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("render: unexpected panic: %v", p)
				}
			}()
			got, err := tmpl.Render(map[string]any{
				"oversized_list_count": int64((64<<20)/unsafe.Sizeof(jinja.Value{})) + 1,
			})
			if tc.wantErr {
				if err == nil {
					t.Fatal("render: got no error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRangeIntegerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"positive overflow", `{{ range(9223372036854775805, 9223372036854775807, 3) | list }}`, "[9223372036854775805]"},
		{"negative overflow", `{{ range(-9223372036854775806, -9223372036854775808, -3) | list }}`, "[-9223372036854775806]"},
		{"maximum positive step", `{{ range(1, 2, 9223372036854775807) | list }}`, "[1]"},
		{"large negative step", `{{ range(-2, -3, -9223372036854775807) | list }}`, "[-2]"},
		{"minimum negative step", `{{ range(9223372036854775807, -9223372036854775808, -9223372036854775808) | list }}`, "[9223372036854775807, -1]"},
		{"positive exact stop", `{{ range(9223372036854775803, 9223372036854775807, 2) | list }}`, "[9223372036854775803, 9223372036854775805]"},
		{"negative exact stop", `{{ range(-9223372036854775804, -9223372036854775808, -2) | list }}`, "[-9223372036854775804, -9223372036854775806]"},
		{"positive empty range", `{{ range(9223372036854775807, 9223372036854775805, 3) | list }}`, "[]"},
		{"negative empty range", `{{ range(-9223372036854775808, -9223372036854775806, -3) | list }}`, "[]"},
		{"ordinary range", `{{ range(2, 9, 3) | list }}`, "[2, 5, 8]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate runaway allocation from a wrapped counter. A goroutine
			// timeout would leave the broken render running in this process.
			if os.Getenv("JINJA_RANGE_WORKER") != "1" {
				executable, err := os.Executable()
				if err != nil {
					t.Fatalf("test executable: %v", err)
				}
				t.Setenv("JINJA_RANGE_WORKER", "1")
				t.Setenv("GOMEMLIMIT", "16MiB")
				ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				defer cancel()
				pattern := "^TestRangeIntegerBoundaries$/^" + strings.ReplaceAll(tc.name, " ", "_") + "$"
				cmd := exec.CommandContext(ctx, executable, "-test.run="+pattern)
				output, err := cmd.CombinedOutput()
				if ctx.Err() != nil {
					t.Fatalf("render did not finish within 2 seconds; want %q", tc.want)
				}
				if err != nil {
					t.Fatalf("render subprocess: %v\n%s", err, output)
				}
				return
			}

			tmpl, err := jinja.Compile(tc.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(nil)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestForLoop(t *testing.T) {
	source := `{% for item in items %}{{ item }}{% if not loop.last %}, {% endif %}{% endfor %}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "a, b, c" {
		t.Errorf("expected 'a, b, c', got %q", result)
	}
}

func TestIfElse(t *testing.T) {
	source := `{% if x %}yes{% else %}no{% endif %}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{"x": true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "yes" {
		t.Errorf("expected 'yes', got %q", result)
	}

	result, err = tmpl.Render(map[string]any{"x": false})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "no" {
		t.Errorf("expected 'no', got %q", result)
	}
}

func TestTojsonFilter(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"default separators", `{{ data | tojson }}`, `{"name": "test", "values": [1, 2]}`},
		{"compact separators", `{{ data | tojson(separators=[',', ':']) }}`, `{"name":"test","values":[1,2]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			result, err := tmpl.Render(map[string]any{
				"data": map[string]any{"name": "test", "values": []int{1, 2}},
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("result: got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestListFilter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"dictionary keys in insertion order", `{{ {'z': 9, 'a': 1} | list | tojson }}`, `["z", "a"]`},
		{"empty dictionary", `{{ {} | list | tojson }}`, `[]`},
		{"list unchanged", `{{ [3, 1] | list | tojson }}`, `[3, 1]`},
		{"unicode string", `{{ 'café' | list | tojson }}`, `["c", "a", "f", "é"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tc.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(nil)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFromJSONFilter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		source  string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:   "from_json alias",
			source: `{% set config = data | from_json %}{{ config.name }}:{{ config.value }}`,
			input:  `{"name":"test","value":42}`,
			want:   "test:42",
		},
		{
			name:   "large integer precision and type",
			source: `{{ (data | fromjson).id }}:{{ (data | fromjson).id is integer }}`,
			input:  `{"id":9007199254740993}`,
			want:   "9007199254740993:True",
		},
		{
			name:   "nested large integer with alias",
			source: `{{ (data | from_json)[0].id }}`,
			input:  `[{"id":9007199254740993}]`,
			want:   "9007199254740993",
		},
		{
			name:   "object key order",
			source: `{% for key in data | fromjson %}{{ key }};{% endfor %}`,
			input:  `{"upf":1,"amf":2}`,
			want:   "upf;amf;",
		},
		{
			name:   "nested object key order",
			source: `{% for key in (data | fromjson)[0] %}{{ key }};{% endfor %}`,
			input:  `[{"z":1,"a":2}]`,
			want:   "z;a;",
		},
		{
			name:   "float type preserved",
			source: `{{ (data | fromjson) is float }}`,
			input:  `1.0`,
			want:   "True",
		},
		{
			name:   "integer boundaries",
			source: `{{ data | fromjson | tojson }}`,
			input:  `[-9223372036854775808,9223372036854775807]`,
			want:   "[-9223372036854775808, 9223372036854775807]",
		},
		{
			name:    "integer above supported range rejected",
			source:  `{{ data | fromjson }}`,
			input:   `9223372036854775808`,
			wantErr: true,
		},
		{
			name:    "integer below supported range rejected",
			source:  `{{ data | fromjson }}`,
			input:   `-9223372036854775809`,
			wantErr: true,
		},
		{
			name:   "duplicate key keeps its first position and last value",
			source: `{% set config = data | fromjson %}{% for key in config %}{{ key }}={{ config[key] }};{% endfor %}`,
			input:  `{"z":1,"a":2,"z":3}`,
			want:   "z=3;a=2;",
		},
		{
			name:   "scalars and empty containers",
			source: `{{ data | fromjson | tojson }}`,
			input:  `[null,true,false,"text",{},[]]`,
			want:   `[null, true, false, "text", {}, []]`,
		},
		{
			name:    "numeric overflow is an error, not a string",
			source:  `{{ data | fromjson | tojson }}`,
			input:   `{"id":1e400}`,
			wantErr: true,
		},
		{
			name:    "trailing data rejected",
			source:  `{{ data | fromjson }}`,
			input:   `{} {}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON rejected",
			source:  `{{ data | fromjson }}`,
			input:   `{"id":[1}`,
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tc.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(map[string]any{"data": tc.input})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("render: got %q without error, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("result: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUndefinedCallableErrorNamesExpression(t *testing.T) {
	tmpl, err := jinja.Compile(`{{ args.items() }}`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	_, err = tmpl.Render(nil)
	if err == nil {
		t.Fatal("render: expected error")
	}

	const want = "args.items is not callable: Undefined"
	if err.Error() != want {
		t.Errorf("error: got %q, want %q", err, want)
	}
}

func TestNamespace(t *testing.T) {
	source := `{%- set ns = namespace(found=false) -%}
{%- for item in items -%}
{%- if item == "target" -%}
{%- set ns.found = true -%}
{%- endif -%}
{%- endfor -%}
{{ ns.found }}`

	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"a", "target", "b"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	result = strings.TrimSpace(result)
	if result != "True" {
		t.Errorf("expected 'True', got %q", result)
	}
}

func TestStringMethods(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"strip", `{{ "  hello  ".strip() }}`, "hello"},
		{"split", `{{ "a,b,c".split(",") | join("-") }}`, "a-b-c"},
		{"startswith", `{{ "hello".startswith("hel") }}`, "True"},
		{"endswith", `{{ "hello".endswith("llo") }}`, "True"},
		{"upper", `{{ "hello".upper() }}`, "HELLO"},
		{"lower", `{{ "HELLO".lower() }}`, "hello"},
		{"replace", `{{ "hello world".replace("world", "jinja") }}`, "hello jinja"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			result, err := tmpl.Render(nil)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestDictMethods(t *testing.T) {
	source := `{%- for key, value in data.items() -%}{{ key }}={{ value }} {% endfor -%}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"data": map[string]any{"a": 1, "b": 2},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Keys are sorted for deterministic output.
	if !strings.Contains(result, "a=1") || !strings.Contains(result, "b=2") {
		t.Errorf("unexpected dict items output: %q", result)
	}
}

func TestFromJSONWithDictItems(t *testing.T) {
	source := `{%- set func = tool['function'] -%}
{%- set args = func['arguments'] -%}
{%- if args is string -%}
  {%- set args = args | from_json -%}
{%- endif -%}
{%- for key, val in args.items() -%}{{ key }}={{ val }}{%- endfor -%}`

	tests := []struct {
		name      string
		arguments any
		want      string
	}{
		{
			name:      "json string",
			arguments: `{"location":"New York City, NY"}`,
			want:      "location=New York City, NY",
		},
		{
			name:      "map",
			arguments: map[string]any{"location": "New York City, NY"},
			want:      "location=New York City, NY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			result, err := tmpl.Render(map[string]any{
				"tool": map[string]any{
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": tt.arguments,
					},
				},
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			if result != tt.want {
				t.Errorf("result: got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestNestedRangePreservesOuterLoop(t *testing.T) {
	source := `{%- for message in messages -%}
{%- if message['role'] == 'assistant' -%}
{%- set ep = namespace(idx=(loop.index0 - 1), done=false) -%}
{%- for _i in range(loop.index0) -%}
{%- if not ep.done and ep.idx >= 0 -%}
{%- set ep.done = true -%}
{%- endif -%}
{%- endfor -%}
{%- for tool in message['tool_calls'] -%}
{%- set args = tool['function']['arguments'] -%}
{%- if args is string -%}{%- set args = args | from_json -%}{%- endif -%}
{%- for key, val in args.items() -%}{{ key }}={{ val }}{%- endfor -%}
{%- endfor -%}
{%- endif -%}
{%- endfor -%}`

	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": "weather"},
			{
				"role": "assistant",
				"tool_calls": []map[string]any{
					{
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": map[string]any{"location": "New York City, NY"},
						},
					},
				},
			},
			{"role": "tool", "content": `{"temperature":"72°F"}`},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	const want = "location=New York City, NY"
	if result != want {
		t.Errorf("result: got %q, want %q", result, want)
	}
}

func TestInlineIf(t *testing.T) {
	source := `{{ "yes" if x else "no" }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{"x": true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "yes" {
		t.Errorf("expected 'yes', got %q", result)
	}
}

func TestIsTests(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"is defined", `{{ "yes" if x is defined else "no" }}`, "yes"},
		{"is not defined", `{{ "yes" if y is not defined else "no" }}`, "yes"},
		{"is string", `{{ "yes" if x is string else "no" }}`, "yes"},
		{"is none", `{{ "yes" if n is none else "no" }}`, "yes"},
		{"is true", `{{ "yes" if t is true else "no" }}`, "yes"},
		{"is false", `{{ "yes" if f is false else "no" }}`, "yes"},
	}

	data := map[string]any{
		"x": "hello",
		"n": nil,
		"t": true,
		"f": false,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			result, err := tmpl.Render(data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestWhitespaceControl(t *testing.T) {
	source := "A\n  {%- if true %}\nB\n  {%- endif %}\nC"
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(result, "A") && !strings.Contains(result, "B") && !strings.Contains(result, "C") {
		t.Errorf("whitespace control produced unexpected output: %q", result)
	}
}

func TestSlicing(t *testing.T) {
	source := `{{ items[::-1] | join(",") }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "c,b,a" {
		t.Errorf("expected 'c,b,a', got %q", result)
	}
}

func TestStringConcat(t *testing.T) {
	source := `{{ "Hello" ~ " " ~ "World" }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result)
	}
}

func TestLoopVariables(t *testing.T) {
	source := `{%- for item in items -%}{{ loop.index0 }}:{{ item }} {% endfor -%}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"x", "y", "z"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "0:x 1:y 2:z " {
		t.Errorf("expected '0:x 1:y 2:z ', got %q", result)
	}
}

// =============================================================================
// Chat template render tests with simple messages
// =============================================================================

func TestRender_Qwen3_8B_SimpleChat(t *testing.T) {
	template := chatTemplates["Qwen3-8B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3-8B output:\n%s", result)

	// Verify key structural elements.
	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
	if !strings.Contains(result, "user") {
		t.Error("output missing user role marker")
	}
	if !strings.Contains(result, "assistant") {
		t.Error("output missing assistant generation prompt")
	}
}

func TestRender_GPT_OSS_20B_SimpleChat(t *testing.T) {
	template := chatTemplates["gpt-oss-20b-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gpt-oss-20b output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Qwen35_35B_SimpleChat(t *testing.T) {
	template := chatTemplates["Qwen3.5-35B-A3B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3.5-35B output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

// =============================================================================
// Tool calling template tests
// =============================================================================

func TestRender_Qwen3_8B_ToolCalling(t *testing.T) {
	template := chatTemplates["Qwen3-8B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "What is the weather in London?",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a location",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The location to get weather for",
							},
						},
						"required": []any{"location"},
					},
				},
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3-8B tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
	if !strings.Contains(result, "London") {
		t.Error("output missing user message")
	}
}

func TestRender_GPT_OSS_20B_ToolCalling(t *testing.T) {
	template := chatTemplates["gpt-oss-20b-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "What is the weather in London?",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a location",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The location to get weather for",
							},
						},
						"required": []any{"location"},
					},
				},
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gpt-oss-20b tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
}

func TestRender_Qwen35_35B_ToolCalling(t *testing.T) {
	template := chatTemplates["Qwen3.5-35B-A3B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "What is the weather in London?",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a location",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The location to get weather for",
							},
						},
						"required": []any{"location"},
					},
				},
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3.5-35B tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
}

// =============================================================================
// Additional model template tests
// =============================================================================

func TestRender_Local_Gemma4_SimpleChat(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gemma-4 output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
	if !strings.Contains(result, "model") || !strings.Contains(result, "user") {
		t.Error("output missing role markers")
	}
}

func TestRender_Local_Ministral3_SimpleChat(t *testing.T) {
	template := chatTemplates["Ministral-3-14B-Instruct-2512-Q4_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Ministral-3 output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Local_RNJ1_SimpleChat(t *testing.T) {
	template := chatTemplates["rnj-1-instruct-Q6_K"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("rnj-1 output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Local_LFM25_SimpleChat(t *testing.T) {
	template := chatTemplates["LFM2.5-VL-1.6B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("LFM2.5-VL output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Local_Gemma4_ToolCalling(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "What is the weather in London?",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a location",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The location to get weather for",
							},
						},
						"required": []any{"location"},
					},
				},
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gemma-4 tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
	if !strings.Contains(result, "London") {
		t.Error("output missing user message")
	}
}

// TestRender_Local_Gemma4_MultiTurnToolCall exercises the exact scenario that
// fails in gonja: multi-turn conversation with assistant tool_calls followed by
// tool responses. The template uses message.get('reasoning'), message.get('tool_calls'),
// message.get('content'), etc.
func TestRender_Local_Gemma4_MultiTurnToolCall(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "What is the weather in London?",
			},
			map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []any{
					map[string]any{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": `{"location": "London"}`,
						},
					},
				},
			},
			map[string]any{
				"role":         "tool",
				"tool_call_id": "call_1",
				"name":         "get_weather",
				"content":      `{"temperature": 15, "condition": "cloudy"}`,
			},
			map[string]any{
				"role":    "assistant",
				"content": "The weather in London is 15°C and cloudy.",
			},
			map[string]any{
				"role":    "user",
				"content": "What about Paris?",
			},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a location",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The location to get weather for",
							},
						},
						"required": []any{"location"},
					},
				},
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gemma-4 multi-turn tool call output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
	if !strings.Contains(result, "London") {
		t.Error("output missing first user message")
	}
	if !strings.Contains(result, "Paris") {
		t.Error("output missing second user message")
	}
	if !strings.Contains(result, "tool_call") || !strings.Contains(result, "tool_response") {
		t.Error("output missing tool call/response markers")
	}
}

// TestDictGet verifies dict.get() works correctly — the exact method that
// fails in gonja with "unknown method 'get' for ”".
func TestDictGet(t *testing.T) {
	tests := []struct {
		name   string
		source string
		data   map[string]any
		want   string
	}{
		{
			name:   "get existing key",
			source: `{{ d.get('name') }}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "Alice",
		},
		{
			name:   "get missing key returns none",
			source: `{{ d.get('missing') }}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "",
		},
		{
			name:   "get missing key with default",
			source: `{{ d.get('missing', 'fallback') }}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "fallback",
		},
		{
			name:   "get in or chain",
			source: `{{ d.get('a') or d.get('b') or 'none' }}`,
			data:   map[string]any{"d": map[string]any{"b": "found_b"}},
			want:   "found_b",
		},
		{
			name:   "get in if condition",
			source: `{% if d.get('tool_calls') %}yes{% else %}no{% endif %}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "no",
		},
		{
			name:   "get truthy check",
			source: `{% if d.get('tool_calls') %}yes{% else %}no{% endif %}`,
			data:   map[string]any{"d": map[string]any{"tool_calls": []any{"call1"}}},
			want:   "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			result, err := tmpl.Render(tt.data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestDefaultFilterBoolean(t *testing.T) {
	tests := []struct {
		name   string
		source string
		data   map[string]any
		want   string
	}{
		{name: "falsey value without boolean remains", source: `{{ value | default('fallback') }}`, data: map[string]any{"value": ""}},
		{name: "falsey value with positional boolean uses default", source: `{{ value | default('fallback', true) }}`, data: map[string]any{"value": ""}, want: "fallback"},
		{name: "truthy value with positional boolean remains", source: `{{ value | default('fallback', true) }}`, data: map[string]any{"value": "actual"}, want: "actual"},
		{name: "falsey value with keyword boolean uses default", source: `{{ value | default('fallback', boolean=true) }}`, data: map[string]any{"value": false}, want: "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			got, err := tmpl.Render(tt.data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tt.want {
				t.Errorf("render: got %q, want %q", got, tt.want)
			}
		})
	}
}

// Beam's unmodified template comes from unsloth/Beam-1.0-501B-A23B-GGUF,
// revision 4cc80272381734e2b16b34c1af3362c7268a0ebc, chat_template.jinja.
// TestRequestsAgainstTemplates also compares it against Python Jinja2 for
// every captured request, including the Beam-specific tool round trip.
func TestRender_Beam_ToolRoundTrip(t *testing.T) {
	tmpl, err := jinja.Compile(chatTemplates["Beam-1.0-501B-A23B"])
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := buildRenderData(loadRequests(t)["kronk-req-beam"])
	data["allow_missing_thinking"] = false
	got, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	for _, fragment := range []string{
		"<|channel|>analysis<|message|>I can write the note and inspect the directory in parallel.<|end|>",
		"<|start|>assistant to=write_note<|channel|>analysis <|constrain|>xml<|message|>",
		"<rfl:parameter name=\"count\">\n3\n</rfl:parameter>",
		"<rfl:parameter name=\"options\">\n{\"active\": true, \"labels\": [\"café\", \"draft\"]}\n</rfl:parameter>",
		"<rfl:parameter name=\"text\">\n  café & <tag> literal \\n\nnext line  \n</rfl:parameter>",
		"<|end|><|start|>assistant to=bash<|channel|>analysis <|constrain|>xml<|message|>",
		"<rfl:parameter name=\"command\">\npwd\n</rfl:parameter><|call|>",
		"<|start|>write_note to=assistant<|channel|>analysis<|message|>Note saved.<|end|>",
		"<|start|>bash to=assistant<|channel|>analysis<|message|>/workspace<|end|>",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("output missing %q", fragment)
		}
	}
	if !strings.HasSuffix(got, "<|start|>assistant") {
		t.Errorf("output missing assistant generation prefix: %q", got)
	}
}

func TestRender_Beam_ReasoningOptions(t *testing.T) {
	tmpl, err := jinja.Compile(chatTemplates["Beam-1.0-501B-A23B"])
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, tc := range []struct {
		name    string
		kwargs  map[string]any
		want    string
		wantErr string
	}{
		{name: "default", want: "Reasoning effort: 512\n\n"},
		{name: "null", kwargs: map[string]any{"reasoning_effort": nil}, want: "Reasoning effort: 512\n\n"},
		{name: "minimal", kwargs: map[string]any{"reasoning_effort": "minimal"}, want: "Reasoning effort: 8\n\n"},
		{name: "low", kwargs: map[string]any{"reasoning_effort": "low"}, want: "Reasoning effort: 32\n\n"},
		{name: "medium", kwargs: map[string]any{"reasoning_effort": "medium"}, want: "Reasoning effort: 64\n\n"},
		{name: "high", kwargs: map[string]any{"reasoning_effort": "high"}, want: "Reasoning effort: 128\n\n"},
		{name: "xhigh", kwargs: map[string]any{"reasoning_effort": "xhigh"}, want: "Reasoning effort: 256\n\n"},
		{name: "max", kwargs: map[string]any{"reasoning_effort": "max"}, want: "Reasoning effort: 512\n\n"},
		{name: "integer", kwargs: map[string]any{"reasoning_effort": 17}, want: "Reasoning effort: 17\n\n"},
		{name: "invalid-none", kwargs: map[string]any{"reasoning_effort": "none"}, wantErr: "reasoning_effort must be an int or one of:"},
		{name: "missing-history-reasoning", kwargs: map[string]any{
			"allow_missing_thinking": false,
			"messages":               []any{map[string]any{"role": "assistant", "content": "Answer."}},
		}, wantErr: "is missing its reasoning"},
		{name: "preserved-history-reasoning", kwargs: map[string]any{
			"allow_missing_thinking": false,
			"messages":               []any{map[string]any{"role": "assistant", "content": "Answer.", "reasoning_content": "Thought."}},
		}, want: "<|channel|>analysis<|message|>Thought.<|end|>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{
				"messages":              []any{map[string]any{"role": "user", "content": "Hello."}},
				"add_generation_prompt": true,
			}
			maps.Copy(data, tc.kwargs)
			got, err := tmpl.Render(data)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("render: got %v, want error containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("output missing %q: %q", tc.want, got)
			}
		})
	}
}

func TestRender_Beam_BuiltinBash(t *testing.T) {
	tmpl, err := jinja.Compile(chatTemplates["Beam-1.0-501B-A23B"])
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, tc := range []struct {
		name        string
		command     map[string]any
		wantBuiltin bool
	}{
		{"unconstrained", map[string]any{"type": "string"}, true},
		{"constrained", map[string]any{"type": "string", "enum": []any{"pwd"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tmpl.Render(map[string]any{
				"messages":              []any{map[string]any{"role": "user", "content": "Check the directory."}},
				"add_generation_prompt": true,
				"tools": []any{map[string]any{"type": "function", "function": map[string]any{
					"name": "bash", "description": "Custom shell description.",
					"parameters": map[string]any{"type": "object", "properties": map[string]any{"command": tc.command}},
				}}},
			})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			system, rest, ok := strings.Cut(got, "<|end|>")
			if !ok {
				t.Fatal("output missing system message terminator")
			}
			if inSystem := strings.Contains(system, "type bash ="); inSystem != tc.wantBuiltin {
				t.Errorf("bash in system: got %v, want %v", inSystem, tc.wantBuiltin)
			}
			if inDeveloper := strings.Contains(rest, "type bash ="); inDeveloper == tc.wantBuiltin {
				t.Errorf("bash in developer: got %v, want %v", inDeveloper, !tc.wantBuiltin)
			}
			if tc.wantBuiltin && !strings.Contains(system, "Execute a bash command.") {
				t.Error("builtin bash missing trained description")
			}
			if !tc.wantBuiltin && !strings.Contains(rest, "Custom shell description.") {
				t.Error("constrained bash missing supplied description")
			}
		})
	}
}

// TestDeepSeekV41Primitives covers the Jinja operations used by the upstream
// template without coupling their regression coverage to its full prompt.
func TestDeepSeekV41Primitives(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"copy mapping", `{% set a = {'x': 1} %}{% set b = dict(a, x=2) %}{{ a.x }} {{ b.x }}`, "1 2", false},
		{"dynamic keyword", `{% set key = 'x' %}{{ dict({'a': 1}, **{key: 2}).x }}`, "2", false},
		{"macro keyword", `{% macro show(x) %}{{ x }}{% endmacro %}{{ show(**{'x': 3}) }}`, "3", false},
		{"filter keyword", `{{ ['a', 'b'] | join(**{'d': '-'}) }}`, "a-b", false},
		{"invalid expansion", `{{ dict(**[]) }}`, "", true},
		{"duplicate expansion", `{{ dict(x=1, **{'x': 2}) }}`, "", true},
		{"invalid copy", `{{ dict(1) }}`, "", true},
		{"too many copies", `{{ dict({}, {}) }}`, "", true},
		{"stable attribute sort", `{{ [{'n': 2, 's': 'a'}, {'n': 1, 's': 'b'}, {'n': 2, 's': 'c'}] | sort(attribute='n') | join(attribute='s') }}`, "bac", false},
		{"reverse attribute sort", `{{ [{'n': 1, 's': 'a'}, {'n': 2, 's': 'b'}] | sort(attribute='n', reverse=true) | join(attribute='s') }}`, "ba", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.input)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := tmpl.Render(nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("render: got error %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegression(t *testing.T) {
	for name, content := range chatTemplates {
		t.Run(name, func(t *testing.T) {
			_, err := jinja.Compile(content)
			if err != nil {
				t.Fatalf("compile - %v", err)
			}
		})
	}
}

var chatTemplates map[string]string

func init() {
	files, err := filepath.Glob("testdata/templates/*.jinja")
	if err != nil {
		panic(fmt.Sprintf("error: can't read testdata/templates - %v", err))
	}

	chatTemplates = make(map[string]string)
	suffixLen := len(".jinja")

	for _, fileName := range files {
		data, err := os.ReadFile(fileName)
		if err != nil {
			panic(fmt.Sprintf("error: %q - read: %v", fileName, err))
		}

		name := filepath.Base(fileName)
		name = name[:len(name)-suffixLen]
		chatTemplates[name] = string(data)
	}
}
