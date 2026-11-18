package jinja_test

import (
	"testing"

	"github.com/ardanlabs/jinja"
)

func TestNamespaceMethodCollisions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"items assignment", `{% set ns = namespace(items=[]) %}{% set ns.items = ns.items + ['cat'] %}{{ ns.items | join(' ') }}`, "cat"},
		{"keys assignment", `{% set ns = namespace(keys=[]) %}{% set ns.keys = ns.keys + ['cat'] %}{{ ns.keys | join(' ') }}`, "cat"},
		{"values assignment", `{% set ns = namespace(values=[]) %}{% set ns.values = ns.values + ['cat'] %}{{ ns.values | join(' ') }}`, "cat"},
		{"loop mutation", `{% set ns = namespace(items=[], keys=[], values=[]) %}{% for word in ['cat', 'dog'] %}{% set ns.items = ns.items + [word] %}{% set ns.keys = ns.keys + [word] %}{% set ns.values = ns.values + [word] %}{% endfor %}{{ ns.items|join(' ') }}|{{ ns.keys|join(' ') }}|{{ ns.values|join(' ') }}`, "cat dog|cat dog|cat dog"},
		{"positional mapping and alias", `{% set ns = namespace({'items': ['cat']}) %}{% set it = ns %}{% set it.items = it.items + ['dog'] %}{{ ns.items|join(' ') }}`, "cat dog"},
		{"missing methods are undefined", `{% set ns = namespace() %}{{ ns.items is undefined }}|{{ ns.keys is undefined }}|{{ ns.values is undefined }}`, "True|True|True"},
		{"dictionary items control", `{% set d = {'items': 'cat'} %}{{ d.items is callable }}|{{ d.items()|length }}|{{ d['items'] }}`, "True|1|cat"},
		{"dictionary keys control", `{% set d = {'keys': 'cat'} %}{{ d.keys is callable }}|{{ d.keys()|join(' ') }}|{{ d['keys'] }}`, "True|keys|cat"},
		{"dictionary values control", `{% set d = {'values': 'cat'} %}{{ d.values is callable }}|{{ d.values()|join(' ') }}|{{ d['values'] }}`, "True|cat|cat"},
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
