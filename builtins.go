package jinja

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// cachedBuiltins is a read-only scope populated once with all built-in
// globals and filters. It is shared across every render — no code path
// writes to the builtins scope (only chained reads through parent
// links and lookupFilter), so a single shared instance is safe and
// avoids ~20 allocations per render.
var cachedBuiltins = func() *scope {
	s := newScope(nil)
	registerGlobals(s)
	registerFilters(s)
	return s
}()

// =============================================================================
// Global functions
// =============================================================================

func registerGlobals(s *scope) {
	s.set("namespace", NewCallable("namespace", builtinNamespace))
	s.set("raise_exception", NewCallable("raise_exception", builtinRaiseException))
	s.set("range", NewCallable("range", builtinRange))
	s.set("joiner", NewCallable("joiner", builtinJoiner))
	s.set("dict", NewCallable("dict", builtinDict))
	s.set("cycler", NewCallable("cycler", builtinCycler))
	s.set("strftime_now", NewCallable("strftime_now", builtinStrftimeNow))
}

func builtinNamespace(args []Value, kwargs map[string]Value) (Value, error) {
	return builtinDict(args, kwargs)
}

func builtinRaiseException(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return Undefined(), fmt.Errorf("raise_exception: missing message")
	}
	return Undefined(), fmt.Errorf("%s", printValue(args[0]))
}

func builtinRange(args []Value, kwargs map[string]Value) (Value, error) {
	var start, stop, step int64
	switch len(args) {
	case 1:
		start = 0
		stop = toInt64(args[0])
		step = 1
	case 2:
		start = toInt64(args[0])
		stop = toInt64(args[1])
		step = 1
	case 3:
		start = toInt64(args[0])
		stop = toInt64(args[1])
		step = toInt64(args[2])
	default:
		return Undefined(), fmt.Errorf("range: expected 1-3 arguments, got %d", len(args))
	}

	if step == 0 {
		return Undefined(), fmt.Errorf("range: step must not be zero")
	}

	var items []Value
	if step > 0 {
		for i := start; i < stop; i += step {
			items = append(items, NewInt(i))
			// A step beyond the integer limit is also beyond stop.
			if i > math.MaxInt64-step {
				break
			}
		}
	} else {
		for i := start; i > stop; i += step {
			items = append(items, NewInt(i))
			if i < math.MinInt64-step {
				break
			}
		}
	}

	return NewList(items), nil
}

func builtinJoiner(args []Value, kwargs map[string]Value) (Value, error) {
	sep := ", "
	if len(args) > 0 {
		sep = printValue(args[0])
	}
	if v, ok := kwargs["sep"]; ok {
		sep = printValue(v)
	}

	first := true
	return NewCallable("joiner", func(args []Value, kwargs map[string]Value) (Value, error) {
		if first {
			first = false
			return NewString(""), nil
		}
		return NewString(sep), nil
	}), nil
}

func builtinDict(args []Value, kwargs map[string]Value) (Value, error) {
	d := NewDict()
	dict := d.AsDict()
	if len(args) > 1 {
		return Undefined(), fmt.Errorf("dict: expected at most one positional argument")
	}
	if len(args) == 1 {
		if !args[0].IsDict() && !args[0].IsList() && !args[0].IsString() {
			return Undefined(), fmt.Errorf("dict: positional argument must be iterable")
		}
		if args[0].IsDict() {
			for _, key := range args[0].AsDict().Keys {
				dict.Set(key, args[0].AsDict().Data[key])
			}
		} else {
			for _, pair := range iterableItems(args[0]) {
				items := iterableItems(pair)
				if len(items) != 2 {
					return Undefined(), fmt.Errorf("dict: iterable elements must have length 2")
				}
				dict.Set(printValue(items[0]), items[1])
			}
		}
	}
	for k, v := range kwargs {
		dict.Set(k, v)
	}
	return d, nil
}

func builtinStrftimeNow(args []Value, kwargs map[string]Value) (Value, error) {
	format := "%Y-%m-%d"
	if len(args) > 0 {
		format = printValue(args[0])
	}

	// Convert Python strftime format to Go time format.
	now := time.Now()
	goFmt := strftimeToGo(format)
	return NewString(now.Format(goFmt)), nil
}

// strftimeToGo converts a Python strftime format string to a Go time format.
func strftimeToGo(format string) string {
	r := strings.NewReplacer(
		"%Y", "2006",
		"%m", "01",
		"%d", "02",
		"%H", "15",
		"%M", "04",
		"%S", "05",
		"%B", "January",
		"%b", "Jan",
		"%A", "Monday",
		"%a", "Mon",
		"%p", "PM",
		"%I", "03",
		"%Z", "MST",
		"%%", "%",
	)
	return r.Replace(format)
}

func builtinCycler(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return Undefined(), fmt.Errorf("cycler: expected at least one argument")
	}

	idx := 0
	items := make([]Value, len(args))
	copy(items, args)

	d := NewDict()
	dict := d.AsDict()
	dict.Set("current", items[0])
	dict.Set("next", NewCallable("cycler.next", func(a []Value, kw map[string]Value) (Value, error) {
		v := items[idx]
		idx = (idx + 1) % len(items)
		dict.Set("current", items[idx])
		return v, nil
	}))
	dict.Set("reset", NewCallable("cycler.reset", func(a []Value, kw map[string]Value) (Value, error) {
		idx = 0
		dict.Set("current", items[0])
		return None(), nil
	}))

	return d, nil
}

// =============================================================================
// Filter functions
// =============================================================================

func registerFilters(s *scope) {
	filters := map[string]func([]Value, map[string]Value) (Value, error){
		"tojson":     filterTojson,
		"from_json":  filterFromjson,
		"fromjson":   filterFromjson,
		"items":      filterItems,
		"join":       filterJoin,
		"default":    filterDefault,
		"d":          filterDefault,
		"trim":       filterTrim,
		"lower":      filterLower,
		"upper":      filterUpper,
		"title":      filterTitle,
		"capitalize": filterCapitalize,
		"first":      filterFirst,
		"last":       filterLast,
		"length":     filterLength,
		"count":      filterLength,
		"reverse":    filterReverse,
		"sort":       filterSort,
		"unique":     filterUnique,
		"list":       filterList,
		"int":        filterInt,
		"float":      filterFloatConv,
		"string":     filterString,
		"safe":       filterSafe,
		"replace":    filterReplace,
		"round":      filterRound,
		"abs":        filterAbs,
		"map":        filterMap,
		"select":     filterSelect,
		"reject":     filterReject,
		"selectattr": filterSelectattr,
		"rejectattr": filterRejectattr,
		"indent":     filterIndent,
		"wordcount":  filterWordcount,
		"escape":     filterEscape,
		"e":          filterEscape,
		"dictsort":   filterDictsort,
		"max":        filterMax,
		"min":        filterMin,
		"sum":        filterSum,
		"batch":      filterBatch,
	}

	for name, fn := range filters {
		s.set("__filter_"+name, NewCallable(name, fn))
	}
}

// lookupFilter retrieves a filter callable from the scope chain.
func lookupFilter(s *scope, name string) (Value, bool) {
	return s.get("__filter_" + name)
}

// =============================================================================
// Test evaluation
// =============================================================================

func evalTest(name string, val Value, args []Value) (bool, error) {
	switch name {
	case "defined":
		return !val.IsUndefined(), nil
	case "undefined":
		return val.IsUndefined(), nil
	case "none":
		return val.IsNone(), nil
	case "boolean":
		return val.IsBool(), nil
	case "integer":
		return val.IsInt(), nil
	case "float":
		return val.IsFloat(), nil
	case "number":
		return val.IsNumber(), nil
	case "string":
		return val.IsString(), nil
	case "mapping":
		return val.IsDict(), nil
	case "iterable":
		return val.IsList() || val.IsDict() || val.IsString(), nil
	case "sequence":
		return val.IsList() || val.IsString() || val.IsDict(), nil
	case "callable":
		return val.IsCallable(), nil
	case "true":
		return val.IsBool() && val.AsBool(), nil
	case "false":
		return val.IsBool() && !val.AsBool(), nil
	case "eq", "equalto", "==":
		if len(args) < 1 {
			return false, fmt.Errorf("test %q requires an argument", name)
		}
		return val.Equals(args[0]), nil
	case "ne", "!=":
		if len(args) < 1 {
			return false, fmt.Errorf("test %q requires an argument", name)
		}
		return !val.Equals(args[0]), nil
	case "gt", "greaterthan", ">":
		if len(args) < 1 {
			return false, nil
		}
		order, ordered := compareValues(val, args[0])
		return ordered && order > 0, nil
	case "ge", ">=":
		if len(args) < 1 {
			return false, nil
		}
		order, ordered := compareValues(val, args[0])
		return ordered && order >= 0, nil
	case "lt", "lessthan", "<":
		if len(args) < 1 {
			return false, nil
		}
		order, ordered := compareValues(val, args[0])
		return ordered && order < 0, nil
	case "le", "<=":
		if len(args) < 1 {
			return false, nil
		}
		order, ordered := compareValues(val, args[0])
		return ordered && order <= 0, nil
	case "in":
		if len(args) < 1 {
			return false, nil
		}
		return containsValue(args[0], val), nil
	case "odd":
		if val.IsInt() {
			return val.AsInt()%2 != 0, nil
		}
		return false, nil
	case "even":
		if val.IsInt() {
			return val.AsInt()%2 == 0, nil
		}
		return false, nil
	case "divisibleby":
		if len(args) == 0 || toFloat64(args[0]) == 0 {
			return false, fmt.Errorf("divisibleby requires a nonzero divisor")
		}
		if val.IsInt() && args[0].IsInt() {
			return val.AsInt()%args[0].AsInt() == 0, nil
		}
		return math.Mod(toFloat64(val), toFloat64(args[0])) == 0, nil
	case "upper":
		if val.IsString() {
			s := val.AsString()
			return s == strings.ToUpper(s) && strings.ContainsFunc(s, unicode.IsUpper), nil
		}
		return false, nil
	case "lower":
		if val.IsString() {
			s := val.AsString()
			return s == strings.ToLower(s) && strings.ContainsFunc(s, unicode.IsLower), nil
		}
		return false, nil
	case "sameas":
		if len(args) < 1 {
			return false, nil
		}
		if val.kind != args[0].kind {
			return false, nil
		}
		if val.IsList() || val.IsDict() {
			return val.v == args[0].v, nil
		}
		return val.Equals(args[0]), nil
	}

	return false, fmt.Errorf("unknown test %q", name)
}

// =============================================================================
// Filter implementations
// =============================================================================

func filterTojson(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}

	// Check both encoding paths before recursively converting containers.
	if containsCycle(args[0], nil) {
		return Undefined(), fmt.Errorf("tojson: circular reference detected")
	}

	itemSeparator := ", "
	keySeparator := ": "
	indent := builtinArgument(args, kwargs, 2, "indent", None())
	var padding *string
	if !indent.IsNone() && !indent.IsUndefined() {
		pad := ""
		if indent.IsString() {
			pad = indent.AsString()
		} else {
			pad = strings.Repeat(" ", max(0, int(toInt64(indent))))
		}
		padding = &pad
		itemSeparator = ","
	}
	if separators := builtinArgument(args, kwargs, 3, "separators", None()); !separators.IsNone() {
		if !separators.IsList() || separators.AsList().Len() != 2 {
			return Undefined(), fmt.Errorf("tojson: separators must contain two strings")
		}

		items := separators.AsList().Items
		if !items[0].IsString() || !items[1].IsString() {
			return Undefined(), fmt.Errorf("tojson: separators must contain two strings")
		}
		itemSeparator = items[0].AsString()
		keySeparator = items[1].AsString()
	}

	var sb strings.Builder
	sortKeys := builtinArgument(args, kwargs, 4, "sort_keys", NewBool(false)).IsTruthy()
	if err := encodeValueJSON(&sb, args[0], itemSeparator, keySeparator, padding, sortKeys, 0); err != nil {
		return Undefined(), err
	}
	out := sb.String()
	if builtinArgument(args, kwargs, 1, "ensure_ascii", NewBool(false)).IsTruthy() {
		var ascii strings.Builder
		for _, r := range out {
			if r < 127 {
				ascii.WriteRune(r)
			} else if r <= 0xffff {
				fmt.Fprintf(&ascii, "\\u%04x", r)
			} else {
				r -= 0x10000
				fmt.Fprintf(&ascii, "\\u%04x\\u%04x", 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			}
		}
		out = ascii.String()
	}
	return NewString(out), nil
}

// Only references to ancestors form cycles; siblings may share a container.
func containsCycle(v Value, ancestors []any) bool {
	switch v.kind {
	case KindList:
		list := v.AsList()
		if slices.Contains(ancestors, any(list)) {
			return true
		}
		ancestors = append(ancestors, list)
		for _, item := range list.Items {
			if containsCycle(item, ancestors) {
				return true
			}
		}

	case KindDict:
		d := v.AsDict()
		if slices.Contains(ancestors, any(d)) {
			return true
		}
		ancestors = append(ancestors, d)
		for _, key := range d.Keys {
			if containsCycle(d.Data[key], ancestors) {
				return true
			}
		}
	}
	return false
}

// encodeValueJSON writes the JSON encoding of v directly into sb without
// going through an intermediate Go-typed tree.
//
// The output matches Python json.dumps's defaults so that prompts rendered by
// this engine are byte-for-byte identical to those produced by HuggingFace's
// reference Jinja2 implementation:
//
//   - Item separator is ", " (with a trailing space), key/value separator is
//     ": " — Python's default when no indent is given.
//   - HTML metacharacters '&', '<', '>' are NOT escaped to \u0026 / \u003c /
//     \u003e (Go's encoding/json escapes these by default; Python does not).
//
// Both differences would otherwise change the tokenization of every tool
// definition that ends up in a chat prompt.
func encodeValueJSON(sb *strings.Builder, v Value, itemSeparator, keySeparator string, padding *string, sortKeys bool, depth int) error {
	newline := func(level int) {
		if padding != nil {
			sb.WriteByte('\n')
			sb.WriteString(strings.Repeat(*padding, level))
		}
	}
	switch v.kind {
	case KindUndefined, KindNone, KindCallable:
		sb.WriteString("null")
		return nil

	case KindBool:
		if v.AsBool() {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
		return nil

	case KindInt:
		var buf [20]byte
		sb.Write(strconv.AppendInt(buf[:0], v.AsInt(), 10))
		return nil

	case KindFloat:
		f := v.AsFloat()
		switch {
		case math.IsNaN(f):
			sb.WriteString("NaN")
		case math.IsInf(f, 1):
			sb.WriteString("Infinity")
		case math.IsInf(f, -1):
			sb.WriteString("-Infinity")
		default:
			sb.WriteString(pythonFloat(f))
		}
		return nil

	case KindString:
		return encodeJSONString(sb, v.AsString())

	case KindList:
		sb.WriteByte('[')
		for i, item := range v.AsList().Items {
			if i > 0 {
				sb.WriteString(itemSeparator)
			}
			newline(depth + 1)
			if err := encodeValueJSON(sb, item, itemSeparator, keySeparator, padding, sortKeys, depth+1); err != nil {
				return err
			}
		}
		if v.AsList().Len() > 0 {
			newline(depth)
		}
		sb.WriteByte(']')
		return nil

	case KindDict:
		sb.WriteByte('{')
		d := v.AsDict()
		keys := d.Keys
		if sortKeys {
			keys = slices.Clone(keys)
			slices.Sort(keys)
		}
		for i, key := range keys {
			if i > 0 {
				sb.WriteString(itemSeparator)
			}
			newline(depth + 1)
			if err := encodeJSONString(sb, key); err != nil {
				return err
			}
			sb.WriteString(keySeparator)
			if err := encodeValueJSON(sb, d.Data[key], itemSeparator, keySeparator, padding, sortKeys, depth+1); err != nil {
				return err
			}
		}
		if len(keys) > 0 {
			newline(depth)
		}
		sb.WriteByte('}')
		return nil
	}

	sb.WriteString("null")
	return nil
}

// encodeJSONString writes the JSON encoding of s without HTML-escaping (no
// \u0026 / \u003c / \u003e), matching Python json.dumps's default behaviour.
func encodeJSONString(sb *strings.Builder, s string) error {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			sb.WriteByte('\\')
			sb.WriteRune(r)
		case '\b':
			sb.WriteString("\\b")
		case '\f':
			sb.WriteString("\\f")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		default:
			if r < 0x20 {
				fmt.Fprintf(sb, "\\u%04x", r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return nil
}

func filterFromjson(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 || !args[0].IsString() {
		return Undefined(), nil
	}

	dec := json.NewDecoder(strings.NewReader(args[0].AsString()))
	dec.UseNumber()
	result, err := decodeJSONValue(dec)
	if err != nil {
		return Undefined(), fmt.Errorf("fromjson: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		if err != nil {
			return Undefined(), fmt.Errorf("fromjson: %w", err)
		}
		return Undefined(), fmt.Errorf("fromjson: trailing data")
	}

	return result, nil
}

// decodeJSONValue builds ordered containers and converts numeric tokens without
// the rounding or string fallback of decoding through plain Go values.
func decodeJSONValue(dec *json.Decoder) (Value, error) {
	token, err := dec.Token()
	if err != nil {
		return Undefined(), err
	}

	switch token := token.(type) {
	case json.Number:
		if !strings.ContainsAny(string(token), ".eE") {
			n, err := token.Int64()
			if err != nil {
				return Undefined(), fmt.Errorf("integer %q: %w", token, err)
			}
			return NewInt(n), nil
		}
		n, err := token.Float64()
		if err != nil {
			return Undefined(), fmt.Errorf("float %q: %w", token, err)
		}
		return NewFloat(n), nil

	case json.Delim:
		switch token {
		case '{':
			result := NewDict()
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return Undefined(), err
				}
				value, err := decodeJSONValue(dec)
				if err != nil {
					return Undefined(), err
				}
				result.AsDict().Set(key.(string), value)
			}
			if _, err := dec.Token(); err != nil {
				return Undefined(), err
			}
			return result, nil

		case '[':
			var items []Value
			for dec.More() {
				value, err := decodeJSONValue(dec)
				if err != nil {
					return Undefined(), err
				}
				items = append(items, value)
			}
			if _, err := dec.Token(); err != nil {
				return Undefined(), err
			}
			return NewList(items), nil
		}
		return Undefined(), fmt.Errorf("unexpected JSON delimiter %q", token)

	default:
		return FromGoValue(token), nil
	}
}

func filterItems(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 || args[0].IsUndefined() {
		return NewList(nil), nil
	}

	// Python jinja2's |items filter raises TemplateError when applied to
	// anything that isn't a mapping, and several chat templates rely on
	// that error to bail out of an unsupported branch (e.g. when a
	// tool_call's `arguments` is a JSON string instead of an object).
	// Silently returning an empty list here let those templates render
	// invalid prompts. The wording deliberately matches Python jinja2 so
	// templates that surface this string get an identical message.
	if !args[0].IsDict() {
		return Undefined(), fmt.Errorf("can only get item pairs from a mapping")
	}

	d := args[0].AsDict()
	items := make([]Value, 0, d.Len())
	for _, key := range d.Keys {
		pair := NewList([]Value{NewString(key), d.Data[key]})
		items = append(items, pair)
	}

	return NewList(items), nil
}

func filterJoin(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}

	sep := ""
	if len(args) > 1 {
		sep = printValue(args[1])
	}
	if s, ok := kwargs["d"]; ok {
		sep = printValue(s)
	}

	attr := ""
	if len(args) > 2 && !args[2].IsNone() {
		attr = printValue(args[2])
	}
	if a, ok := kwargs["attribute"]; ok {
		attr = printValue(a)
	}

	items := iterableItems(args[0])
	parts := make([]string, len(items))
	for i, item := range items {
		if attr != "" {
			item = builtinAttribute(item, attr, Undefined())
		}
		parts[i] = printValue(item)
	}

	return NewString(strings.Join(parts, sep)), nil
}

func filterDefault(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return Undefined(), nil
	}

	val := args[0]
	defVal := NewString("")
	if len(args) > 1 {
		defVal = args[1]
	}
	if v, ok := kwargs["default_value"]; ok {
		defVal = v
	}

	boolean := false
	if len(args) > 2 && args[2].IsBool() {
		boolean = args[2].AsBool()
	} else if b, ok := kwargs["boolean"]; ok && b.IsBool() {
		boolean = b.AsBool()
	}

	if val.IsUndefined() {
		return defVal, nil
	}

	if boolean && !val.IsTruthy() {
		return defVal, nil
	}

	return val, nil
}

func filterTrim(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	s := printValue(args[0])
	if len(args) > 1 {
		chars := printValue(args[1])
		s = strings.Trim(s, chars)
	} else {
		s = strings.TrimSpace(s)
	}
	return NewString(s), nil
}

func filterLower(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	return NewString(strings.ToLower(printValue(args[0]))), nil
}

func filterUpper(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	return NewString(strings.ToUpper(printValue(args[0]))), nil
}

func filterTitle(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}

	s := printValue(args[0])
	runes := []rune(s)
	inWord := false
	for i, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				runes[i] = unicode.ToUpper(r)
				inWord = true
			} else {
				runes[i] = unicode.ToLower(r)
			}
		} else {
			inWord = false
		}
	}
	return NewString(string(runes)), nil
}

func filterCapitalize(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	s := printValue(args[0])
	if len(s) == 0 {
		return NewString(""), nil
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return NewString(string(runes)), nil
}

func filterFirst(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return Undefined(), nil
	}
	items := iterableItems(args[0])
	if len(items) == 0 {
		return Undefined(), nil
	}
	return items[0], nil
}

func filterLast(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return Undefined(), nil
	}
	items := iterableItems(args[0])
	if len(items) == 0 {
		return Undefined(), nil
	}
	return items[len(items)-1], nil
}

func filterLength(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewInt(0), nil
	}

	v := args[0]
	switch {
	case v.IsList():
		return NewInt(int64(v.AsList().Len())), nil
	case v.IsDict():
		return NewInt(int64(v.AsDict().Len())), nil
	case v.IsString():
		return NewInt(int64(len([]rune(v.AsString())))), nil
	}

	return NewInt(0), nil
}

func filterReverse(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	v := args[0]
	if v.IsString() {
		runes := []rune(v.AsString())
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return NewString(string(runes)), nil
	}

	if v.IsList() {
		src := v.AsList().Items
		items := make([]Value, len(src))
		for i, item := range src {
			items[len(src)-1-i] = item
		}
		return NewList(items), nil
	}

	return v, nil
}

func filterSort(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 || !args[0].IsList() {
		return NewList(nil), nil
	}

	src := args[0].AsList().Items
	items := make([]Value, len(src))
	copy(items, src)

	reverse := false
	if r, ok := kwargs["reverse"]; ok && r.IsBool() {
		reverse = r.AsBool()
	}

	attribute := ""
	if a, ok := kwargs["attribute"]; ok {
		attribute = printValue(a)
	}

	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if attribute != "" {
			if left.IsDict() {
				left, _ = left.AsDict().Get(attribute)
			}
			if right.IsDict() {
				right, _ = right.AsDict().Get(attribute)
			}
		}
		order, ordered := compareValues(left, right)
		if !ordered {
			return false
		}
		if reverse {
			return order > 0
		}
		return order < 0
	})

	return NewList(items), nil
}

func filterUnique(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	src := iterableItems(args[0])
	caseSensitive := builtinArgument(args, kwargs, 1, "case_sensitive", NewBool(false)).IsTruthy()
	attribute := builtinArgument(args, kwargs, 2, "attribute", None())
	var seen []Value
	var items []Value
	for _, item := range src {
		key := item
		if !attribute.IsNone() {
			key = builtinAttribute(item, printValue(attribute), Undefined())
		}
		if !caseSensitive && key.IsString() {
			key = NewString(strings.ToLower(key.AsString()))
		}
		if !slices.ContainsFunc(seen, key.Equals) {
			seen = append(seen, key)
			items = append(items, item)
		}
	}

	return NewList(items), nil
}

func filterList(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	v := args[0]
	if v.IsList() {
		return NewList(slices.Clone(v.AsList().Items)), nil
	}
	if v.IsDict() {
		keys := v.AsDict().OrderedKeys()
		items := make([]Value, len(keys))
		for i, key := range keys {
			items[i] = NewString(key)
		}
		return NewList(items), nil
	}
	if v.IsString() {
		runes := []rune(v.AsString())
		items := make([]Value, len(runes))
		for i, r := range runes {
			items[i] = NewString(string(r))
		}
		return NewList(items), nil
	}

	if v.IsUndefined() {
		return NewList(nil), nil
	}
	return Undefined(), fmt.Errorf("list: value is not iterable")
}

func filterInt(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewInt(0), nil
	}
	def := builtinArgument(args, kwargs, 1, "default", NewInt(0))
	v := args[0]
	if v.IsInt() || v.IsBool() {
		return NewInt(toInt64(v)), nil
	}
	if v.IsString() {
		s := strings.TrimSpace(v.AsString())
		base := int(toInt64(builtinArgument(args, kwargs, 2, "base", NewInt(10))))
		// Unlike Go's explicit-base parser, Python accepts matching prefixes.
		sign, digits := "", s
		if strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
			sign, digits = digits[:1], digits[1:]
		}
		prefix := map[int]string{2: "0b", 8: "0o", 16: "0x"}[base]
		if prefix != "" && strings.HasPrefix(strings.ToLower(digits), prefix) {
			s = sign + digits[2:]
		}
		if n, err := strconv.ParseInt(s, base, 64); err == nil {
			return NewInt(n), nil
		}
	}
	f, ok := builtinFloat(v)
	if ok && !math.IsNaN(f) && !math.IsInf(f, 0) && f >= -0x1p63 && f < 0x1p63 {
		return NewInt(int64(f)), nil
	}
	return def, nil
}

func filterFloatConv(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewFloat(0), nil
	}
	if f, ok := builtinFloat(args[0]); ok {
		return NewFloat(f), nil
	}
	return builtinArgument(args, kwargs, 1, "default", NewFloat(0)), nil
}

func filterString(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	return NewString(printValue(args[0])), nil
}

func filterSafe(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	return args[0], nil
}

func filterReplace(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) < 3 {
		return NewString(""), fmt.Errorf("replace: expected at least 3 arguments")
	}
	s := printValue(args[0])
	old := printValue(args[1])
	newStr := printValue(args[2])

	count := -1
	if len(args) > 3 {
		count = int(toInt64(args[3]))
	}
	if c, ok := kwargs["count"]; ok {
		count = int(toInt64(c))
	}

	return NewString(strings.Replace(s, old, newStr, count)), nil
}

func filterRound(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewFloat(0), nil
	}

	precision := int64(0)
	if len(args) > 1 {
		precision = toInt64(args[1])
	}
	if p, ok := kwargs["precision"]; ok {
		precision = toInt64(p)
	}

	f := toFloat64(args[0])
	pow := math.Pow(10, float64(precision))
	method := printValue(builtinArgument(args, kwargs, 2, "method", NewString("common")))
	switch method {
	case "ceil":
		return NewFloat(math.Ceil(f*pow) / pow), nil
	case "floor":
		return NewFloat(math.Floor(f*pow) / pow), nil
	case "common":
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return NewFloat(f), nil
		}
		if precision >= 0 && precision <= 323 {
			// Decimal formatting rounds the actual binary value, not f*10^n.
			n, err := strconv.ParseFloat(strconv.FormatFloat(f, 'f', int(precision), 64), 64)
			if err != nil {
				return Undefined(), fmt.Errorf("round: %w", err)
			}
			return NewFloat(n), nil
		}
		if precision > 323 {
			return NewFloat(f), nil
		}
		if precision < -308 {
			return NewFloat(math.Copysign(0, f)), nil
		}
		return NewFloat(math.RoundToEven(f*pow) / pow), nil
	default:
		return Undefined(), fmt.Errorf("round: method must be common, ceil or floor")
	}
}

func filterAbs(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewInt(0), nil
	}

	v := args[0]
	if v.IsInt() {
		n := v.AsInt()
		if n < 0 {
			n = -n
		}
		return NewInt(n), nil
	}
	if v.IsFloat() {
		return NewFloat(math.Abs(v.AsFloat())), nil
	}

	return NewInt(0), nil
}

func filterMap(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	src := iterableItems(args[0])

	// map(attribute="name") form
	if attr, ok := kwargs["attribute"]; ok {
		attrName := printValue(attr)
		defVal := Undefined()
		if d, ok := kwargs["default"]; ok {
			defVal = d
		}

		items := make([]Value, len(src))
		for i, item := range src {
			items[i] = builtinAttribute(item, attrName, defVal)
		}
		return NewList(items), nil
	}

	if len(args) < 2 {
		return Undefined(), fmt.Errorf("map requires a filter argument")
	}
	s := newScope(nil)
	registerFilters(s)
	filter, ok := lookupFilter(s, printValue(args[1]))
	if !ok {
		return Undefined(), fmt.Errorf("unknown filter %q", printValue(args[1]))
	}
	items := make([]Value, len(src))
	for i, item := range src {
		callArgs := append([]Value{item}, args[2:]...)
		v, err := filter.AsCallable().Fn(callArgs, kwargs)
		if err != nil {
			return Undefined(), err
		}
		items[i] = v
	}
	return NewList(items), nil
}

func filterSelect(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	list := NewList(iterableItems(args[0])).AsList()
	testName := ""
	if len(args) > 1 && args[1].IsString() {
		testName = args[1].AsString()
	}

	var testArgs []Value
	if len(args) > 2 {
		testArgs = args[2:]
	}

	var items []Value
	for _, item := range list.Items {
		if testName == "" {
			if item.IsTruthy() {
				items = append(items, item)
			}
		} else {
			result, err := evalTest(testName, item, testArgs)
			if err != nil {
				return Undefined(), err
			}
			if result {
				items = append(items, item)
			}
		}
	}

	return NewList(items), nil
}

func filterReject(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	list := NewList(iterableItems(args[0])).AsList()
	testName := ""
	if len(args) > 1 && args[1].IsString() {
		testName = args[1].AsString()
	}

	var testArgs []Value
	if len(args) > 2 {
		testArgs = args[2:]
	}

	var items []Value
	for _, item := range list.Items {
		if testName == "" {
			if !item.IsTruthy() {
				items = append(items, item)
			}
		} else {
			result, err := evalTest(testName, item, testArgs)
			if err != nil {
				return Undefined(), err
			}
			if !result {
				items = append(items, item)
			}
		}
	}

	return NewList(items), nil
}

func filterSelectattr(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) < 2 {
		return NewList(nil), nil
	}

	list := NewList(iterableItems(args[0])).AsList()
	attrName := printValue(args[1])
	testName := "truthy"
	if len(args) > 2 {
		testName = printValue(args[2])
	}

	var testArgs []Value
	if len(args) > 3 {
		testArgs = args[3:]
	}

	var items []Value
	for _, item := range list.Items {
		attrVal := builtinAttribute(item, attrName, Undefined())

		if testName == "truthy" {
			if attrVal.IsTruthy() {
				items = append(items, item)
			}
		} else {
			result, err := evalTest(testName, attrVal, testArgs)
			if err != nil {
				return Undefined(), err
			}
			if result {
				items = append(items, item)
			}
		}
	}

	return NewList(items), nil
}

func filterRejectattr(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) < 2 {
		return NewList(nil), nil
	}

	list := NewList(iterableItems(args[0])).AsList()
	attrName := printValue(args[1])
	testName := "truthy"
	if len(args) > 2 {
		testName = printValue(args[2])
	}

	var testArgs []Value
	if len(args) > 3 {
		testArgs = args[3:]
	}

	var items []Value
	for _, item := range list.Items {
		attrVal := builtinAttribute(item, attrName, Undefined())

		if testName == "truthy" {
			if !attrVal.IsTruthy() {
				items = append(items, item)
			}
		} else {
			result, err := evalTest(testName, attrVal, testArgs)
			if err != nil {
				return Undefined(), err
			}
			if !result {
				items = append(items, item)
			}
		}
	}

	return NewList(items), nil
}

func filterIndent(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}

	s := printValue(args[0])
	width := builtinArgument(args, kwargs, 1, "width", NewInt(4))
	pad := ""
	if width.IsString() {
		pad = width.AsString()
	} else {
		pad = strings.Repeat(" ", max(0, int(toInt64(width))))
	}

	indentFirst := false
	if len(args) > 2 {
		indentFirst = args[2].IsTruthy()
	}
	if f, ok := kwargs["first"]; ok {
		indentFirst = f.IsTruthy()
	}

	blank := builtinArgument(args, kwargs, 3, "blank", NewBool(false)).IsTruthy()
	// Python splitlines normalizes all line boundaries, including CRLF.
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\v", "\n", "\f", "\n", "\x1c", "\n", "\x1d", "\n", "\x1e", "\n", "\u0085", "\n", "\u2028", "\n", "\u2029", "\n").Replace(s)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if i == 0 && !indentFirst {
			continue
		}
		if line != "" || blank || (i == 0 && indentFirst) {
			lines[i] = pad + line
		}
	}

	return NewString(strings.Join(lines, "\n")), nil
}

func filterWordcount(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewInt(0), nil
	}
	words := strings.FieldsFunc(printValue(args[0]), func(r rune) bool {
		return r != '_' && !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return NewInt(int64(len(words))), nil
}

func filterEscape(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewString(""), nil
	}
	s := printValue(args[0])
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return NewString(s), nil
}

func filterDictsort(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 || !args[0].IsDict() {
		return NewList(nil), nil
	}

	d := args[0].AsDict()
	reverse := false
	if r, ok := kwargs["reverse"]; ok && r.IsBool() {
		reverse = r.AsBool()
	}

	keys := make([]string, len(d.Keys))
	copy(keys, d.Keys)

	sort.SliceStable(keys, func(i, j int) bool {
		if reverse {
			return keys[i] > keys[j]
		}
		return keys[i] < keys[j]
	})

	items := make([]Value, len(keys))
	for i, key := range keys {
		items[i] = NewList([]Value{NewString(key), d.Data[key]})
	}

	return NewList(items), nil
}

func filterMax(args []Value, kwargs map[string]Value) (Value, error) {
	return filterExtremum(args, kwargs, 1)
}

func filterMin(args []Value, kwargs map[string]Value) (Value, error) {
	return filterExtremum(args, kwargs, -1)
}

func filterExtremum(args []Value, kwargs map[string]Value, direction int) (Value, error) {
	if len(args) == 0 {
		return Undefined(), nil
	}
	items := iterableItems(args[0])
	if len(items) == 0 {
		return Undefined(), nil
	}
	attribute := builtinArgument(args, kwargs, 2, "attribute", None())
	caseSensitive := builtinArgument(args, kwargs, 1, "case_sensitive", NewBool(false)).IsTruthy()
	key := func(item Value) Value {
		if !attribute.IsNone() {
			item = builtinAttribute(item, printValue(attribute), Undefined())
		}
		if !caseSensitive && item.IsString() {
			item = NewString(strings.ToLower(item.AsString()))
		}
		return item
	}
	best, bestKey := items[0], key(items[0])
	for _, item := range items[1:] {
		itemKey := key(item)
		if order, ordered := compareValues(itemKey, bestKey); ordered && order*direction > 0 {
			best, bestKey = item, itemKey
		}
	}
	return best, nil
}

func filterSum(args []Value, kwargs map[string]Value) (Value, error) {
	sum := builtinArgument(args, kwargs, 2, "start", NewInt(0))
	if len(args) == 0 {
		return sum, nil
	}
	attribute := builtinArgument(args, kwargs, 1, "attribute", None())
	for _, item := range iterableItems(args[0]) {
		if !attribute.IsNone() {
			item = builtinAttribute(item, printValue(attribute), Undefined())
		}
		if !(item.IsNumber() || item.IsBool()) || !(sum.IsNumber() || sum.IsBool()) {
			return Undefined(), fmt.Errorf("sum: expected numeric values")
		}
		sum = addValues(sum, item)
	}
	return sum, nil
}

func filterBatch(args []Value, kwargs map[string]Value) (Value, error) {
	if len(args) == 0 {
		return NewList(nil), nil
	}

	list := NewList(iterableItems(args[0])).AsList()
	linecount := int(toInt64(builtinArgument(args, kwargs, 1, "linecount", NewInt(0))))
	if linecount <= 0 {
		return NewList(nil), nil
	}

	var batches []Value
	for i := 0; i < list.Len(); i += linecount {
		end := min(i+linecount, list.Len())
		batch := make([]Value, end-i)
		copy(batch, list.Items[i:end])
		fill := builtinArgument(args, kwargs, 2, "fill_with", None())
		if !fill.IsNone() {
			for len(batch) < linecount {
				batch = append(batch, fill)
			}
		}
		batches = append(batches, NewList(batch))
	}

	return NewList(batches), nil
}

// =============================================================================
// Helpers
// =============================================================================

func builtinArgument(args []Value, kwargs map[string]Value, index int, name string, fallback Value) Value {
	if value, ok := kwargs[name]; ok {
		return value
	}
	if len(args) > index {
		return args[index]
	}
	return fallback
}

func iterableItems(v Value) []Value {
	switch {
	case v.IsList():
		return v.AsList().Items
	case v.IsDict():
		items := make([]Value, len(v.AsDict().Keys))
		for i, key := range v.AsDict().Keys {
			items[i] = NewString(key)
		}
		return items
	case v.IsString():
		var items []Value
		for _, r := range v.AsString() {
			items = append(items, NewString(string(r)))
		}
		return items
	}
	return nil
}

func builtinAttribute(v Value, path string, fallback Value) Value {
	for _, part := range strings.Split(path, ".") {
		if v.IsDict() {
			v, _ = v.AsDict().Get(part)
		} else if index, err := strconv.Atoi(part); err == nil && index >= 0 {
			items := iterableItems(v)
			v = Undefined()
			if index < len(items) {
				v = items[index]
			}
		} else {
			v = Undefined()
		}
		if v.IsUndefined() {
			v = fallback
		}
	}
	return v
}

func builtinFloat(v Value) (float64, bool) {
	if v.IsNumber() || v.IsBool() {
		return toFloat64(v), true
	}
	if v.IsString() {
		f, err := strconv.ParseFloat(strings.TrimSpace(v.AsString()), 64)
		return f, err == nil || math.IsInf(f, 0)
	}
	return 0, false
}

func toInt64(v Value) int64 {
	switch {
	case v.IsInt():
		return v.AsInt()
	case v.IsFloat():
		return int64(v.AsFloat())
	case v.IsBool():
		if v.AsBool() {
			return 1
		}
		return 0
	case v.IsString():
		s := v.AsString()
		var n int64
		fmt.Sscanf(s, "%d", &n)
		return n
	}
	return 0
}

func toFloat64(v Value) float64 {
	switch {
	case v.IsFloat():
		return v.AsFloat()
	case v.IsInt():
		return float64(v.AsInt())
	case v.IsBool():
		if v.AsBool() {
			return 1
		}
		return 0
	case v.IsString():
		s := v.AsString()
		var f float64
		fmt.Sscanf(s, "%f", &f)
		return f
	}
	return 0
}

// compareValues returns the ordering and whether the values are ordered.
// A comparison involving a floating-point NaN is unordered.
func compareValues(a, b Value) (int, bool) {
	if a.IsString() && b.IsString() {
		return strings.Compare(a.AsString(), b.AsString()), true
	}
	if a.IsList() && b.IsList() {
		left, right := a.AsList().Items, b.AsList().Items
		for i := range min(len(left), len(right)) {
			if left[i].Equals(right[i]) {
				continue
			}
			return compareValues(left[i], right[i])
		}
		return cmp.Compare(len(left), len(right)), true
	}
	if (a.IsFloat() && math.IsNaN(a.AsFloat())) || (b.IsFloat() && math.IsNaN(b.AsFloat())) {
		return 0, false
	}

	aIntegral := a.IsInt() || a.IsBool()
	bIntegral := b.IsInt() || b.IsBool()
	switch {
	case aIntegral && bIntegral:
		return cmp.Compare(toInt64(a), toInt64(b)), true
	case aIntegral && b.IsFloat():
		return compareIntFloat(toInt64(a), b.AsFloat()), true
	case a.IsFloat() && bIntegral:
		return -compareIntFloat(toInt64(b), a.AsFloat()), true
	}

	af := toFloat64(a)
	bf := toFloat64(b)
	if af < bf {
		return -1, true
	}
	if af > bf {
		return 1, true
	}
	if a.IsString() && b.IsString() {
		return strings.Compare(a.AsString(), b.AsString()), true
	}
	return 0, true
}

// compareIntFloat compares without rounding i to a float. f must not be NaN.
func compareIntFloat(i int64, f float64) int {
	if f >= 0x1p63 {
		return -1
	}
	if f < -0x1p63 {
		return 1
	}
	if order := cmp.Compare(i, int64(f)); order != 0 {
		return order
	}
	// The integer parts match; only a fractional component can differ.
	return cmp.Compare(float64(i), f)
}

func containsValue(container, item Value) bool {
	if container.IsList() {
		for _, v := range container.AsList().Items {
			if v.Equals(item) {
				return true
			}
		}
		return false
	}
	if container.IsDict() && item.IsString() {
		return container.AsDict().Has(item.AsString())
	}
	if container.IsString() && item.IsString() {
		return strings.Contains(container.AsString(), item.AsString())
	}
	return false
}

// printValue converts a Value to its template output string. Unlike
// String() which returns a Python-style repr, printValue returns the
// raw text for template rendering (no quotes around strings, empty
// string for Undefined).
func printValue(v Value) string {
	switch v.kind {
	case KindUndefined:
		return ""
	case KindNone:
		return "None"
	case KindBool:
		if v.AsBool() {
			return "True"
		}
		return "False"
	case KindInt:
		return fmt.Sprintf("%d", v.AsInt())
	case KindFloat:
		return pythonFloat(v.AsFloat())
	case KindString:
		return v.AsString()
	case KindList:
		return v.String()
	case KindDict:
		return v.String()
	case KindCallable:
		return ""
	}
	return ""
}

func pythonFloat(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	format := byte('f')
	if a := math.Abs(f); a != 0 && (a < 1e-4 || a >= 1e16) {
		format = 'e'
	}
	s := strconv.FormatFloat(f, format, -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}
