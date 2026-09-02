package jinja

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// jsonStringCorpus covers every escaping decision appendJSONString makes.
func jsonStringCorpus() []string {
	corpus := []string{
		"",
		"plain",
		`quote " backslash \ slash /`,
		"tab\there",
		"nl\nrn\r",
		"bell\b form\f",
		"del\x7f",
		"html < > & mix",
		"line sep   para sep  ",
		"café 世界 \U0001F600",
		"trailing invalid \xff",
		"\xff leading invalid",
		"mid \xc3 invalid",
		"\xed\xa0\x80", // encoded surrogate, invalid UTF-8
	}

	for c := range 0x20 {
		corpus = append(corpus, "ctl"+string(rune(c))+"end")
	}

	return corpus
}

func TestAppendJSONStringMatchesEncodingJSON(t *testing.T) {
	for _, s := range jsonStringCorpus() {
		for _, escapeHTML := range []bool{false, true} {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(escapeHTML)
			if err := enc.Encode(s); err != nil {
				t.Fatalf("encode %q: %v", s, err)
			}
			want := strings.TrimSuffix(buf.String(), "\n")

			var sb strings.Builder
			appendJSONString(&sb, s, escapeHTML)

			if got := sb.String(); got != want {
				t.Errorf("appendJSONString(%q, %v)\ngot  %s\nwant %s", s, escapeHTML, got, want)
			}
		}
	}
}

func TestAppendJSONFloatMatchesEncodingJSON(t *testing.T) {
	floats := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.5, 1e-6, 9.99e-7, 1e-7, 1e-9,
		1e20, 1e21, 1.5e21, 1e100, -1e-300, 5e-324, math.MaxFloat64,
		3.141592653589793, 9007199254740992,
	}

	for _, f := range floats {
		want, err := json.Marshal(f)
		if err != nil {
			t.Fatalf("marshal %v: %v", f, err)
		}

		var sb strings.Builder
		appendJSONFloat(&sb, f)

		if got := sb.String(); got != string(want) {
			t.Errorf("appendJSONFloat(%v) got %s want %s", f, got, want)
		}
	}
}

// jsonValueCorpus builds Value trees that exercise every encoder branch.
func jsonValueCorpus() []Value {
	nested := NewDict()
	nested.AsDict().Set("zebra", NewString("last <&>"))
	nested.AsDict().Set("alpha", NewList([]Value{NewInt(1), NewFloat(2.5), None()}))
	nested.AsDict().Set("empty_list", NewList(nil))
	nested.AsDict().Set("empty_dict", NewDict())

	deep := NewDict()
	deep.AsDict().Set("outer", NewList([]Value{nested, NewBool(true)}))

	return []Value{
		None(),
		Undefined(),
		NewBool(false),
		NewInt(-42),
		NewFloat(0.125),
		NewString("plain <tag> & \"quoted\""),
		NewList(nil),
		NewDict(),
		nested,
		deep,
	}
}

// valueToGo mirrors the helper that builtins.go used before the hand-written
// encoder replaced it. Kept here only to build the reference output.
func valueToGo(v Value) any {
	switch v.kind {
	case KindBool:
		return v.AsBool()
	case KindInt:
		return v.AsInt()
	case KindFloat:
		return v.AsFloat()
	case KindString:
		return v.AsString()
	case KindList:
		out := make([]any, v.AsList().Len())
		for i, item := range v.AsList().Items {
			out[i] = valueToGo(item)
		}
		return out
	case KindDict:
		d := v.AsDict()
		out := make(map[string]any, d.Len())
		for _, key := range d.Keys {
			out[key] = valueToGo(d.Data[key])
		}
		return out
	}
	return nil
}

func TestEncodeValueJSONIndentMatchesMarshalIndent(t *testing.T) {
	for _, v := range jsonValueCorpus() {
		for _, width := range []int{0, 1, 2, 4} {
			pad := strings.Repeat(" ", width)

			want, err := json.MarshalIndent(valueToGo(v), "", pad)
			if err != nil {
				t.Fatalf("marshal indent: %v", err)
			}

			var sb strings.Builder
			if err := encodeValueJSON(&sb, v, pad, 0, true); err != nil {
				t.Fatalf("encodeValueJSON: %v", err)
			}

			if got := sb.String(); got != string(want) {
				t.Errorf("indent=%d\ngot\n%s\nwant\n%s", width, got, want)
			}
		}
	}
}

func TestParseJSONMatchesUnmarshal(t *testing.T) {
	inputs := []string{
		`null`, `true`, `false`, `0`, `1`, `-1`, `12345678901234567890`,
		`1.5`, `1e3`, `1E+3`, `1e-3`, `-2.5e10`, `0.0`, `1.05`, `1e007`,
		`""`, `"plain"`, `"esc \" \\ \/ \b \f \n \r \t"`,
		`"é 世 😀"`, `"\ud800"`, `"\ud800x"`, `"\udc00"`,
		`[]`, `[1,2,3]`, `[[1],[2,[3]]]`, `{}`, `{"a":1}`,
		`{"b":1,"a":2,"c":[true,null,"s"]}`,
		"  \t\r\n {\"a\" : [ 1 , 2 ] } \n ",
		`{"dup":1,"dup":2}`,
	}

	for _, in := range inputs {
		var want any
		if err := json.Unmarshal([]byte(in), &want); err != nil {
			t.Fatalf("reference unmarshal %s: %v", in, err)
		}

		got, err := parseJSON(in)
		if err != nil {
			t.Errorf("parseJSON(%s): %v", in, err)
			continue
		}

		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("remarshal: %v", err)
		}

		var sb strings.Builder
		if err := encodeValueJSON(&sb, got, "", 0, true); err != nil {
			t.Fatalf("encode: %v", err)
		}

		// The reference tree turns every number into a float64, so compare
		// through a float-normalized re-encode of the parsed value.
		gotJSON := compactJSON(t, sb.String())
		if gotJSON != compactJSON(t, string(wantJSON)) {
			t.Errorf("parseJSON(%s)\ngot  %s\nwant %s", in, gotJSON, wantJSON)
		}
	}
}

// compactJSON round-trips through encoding/json so that integer and float
// spellings of the same number normalize to one form.
func compactJSON(t *testing.T, s string) string {
	t.Helper()

	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("compact %s: %v", s, err)
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("compact marshal: %v", err)
	}

	return string(b)
}

// TestParseJSONNegativeZero documents a deliberate difference from
// encoding/json, which decodes -0 as float64 and re-encodes it as "-0".
// Python json.loads("-0") returns the integer 0, and this package follows
// Python.
func TestParseJSONNegativeZero(t *testing.T) {
	v, err := parseJSON(`-0`)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}

	if !v.IsInt() || v.AsInt() != 0 {
		t.Errorf("got %v want int 0", v)
	}
}

func TestParseJSONKeepsIntegersAndKeyOrder(t *testing.T) {
	v, err := parseJSON(`{"b":9007199254740993,"a":1.0,"c":3}`)
	if err != nil {
		t.Fatalf("parseJSON: %v", err)
	}

	var sb strings.Builder
	if err := encodeValueJSON(&sb, v, "", 0, false); err != nil {
		t.Fatalf("encode: %v", err)
	}

	const want = `{"b": 9007199254740993, "a": 1, "c": 3}`
	if got := sb.String(); got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestParseJSONRejectsBadInput(t *testing.T) {
	bad := []string{
		``, `  `, `nul`, `tru`, `[`, `]`, `{`, `}`, `{"a"}`, `{"a":}`,
		`{"a":1,}`, `[1,]`, `[1 2]`, `{a:1}`, `01`, `-01`, `1.`, `.5`,
		`1e`, `1e+`, `+1`, `"unterminated`, `"bad \q"`, `"bad \u12"`,
		`"raw` + "\n" + `newline"`, `1 2`, `{} {}`, `"a" "b"`,
	}

	for _, in := range bad {
		if _, err := parseJSON(in); err == nil {
			t.Errorf("parseJSON(%q) accepted invalid input", in)
		}
	}
}

func TestParseJSONDepthLimit(t *testing.T) {
	ok := strings.Repeat("[", maxJSONDepth) + strings.Repeat("]", maxJSONDepth)
	if _, err := parseJSON(ok); err != nil {
		t.Errorf("parseJSON at max depth: %v", err)
	}

	tooDeep := strings.Repeat("[", maxJSONDepth+2) + strings.Repeat("]", maxJSONDepth+2)
	if _, err := parseJSON(tooDeep); err == nil {
		t.Error("parseJSON accepted input past max depth")
	}
}
