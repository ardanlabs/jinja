package jinja_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/ardanlabs/jinja"
)

// TestNetworkProvisioning compares the self-contained provisioning plan and its
// language features against Python, without loading model request data.
func TestNetworkProvisioning(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv is required for the Python Jinja2 reference")
	}

	source, err := os.ReadFile("testdata/provisioning/5g-netprov.jinja")
	if err != nil {
		t.Fatal(err)
	}

	// Source is the template to exercise. Data supplies its input variables:
	// strings, numbers, booleans, nil, []any lists, and map[string]any mappings.
	// Expected output comes from Python, not from Data. The complete plan
	// declares its own inventory, so it intentionally needs an empty context.
	cases := []struct {
		Name    string         `json:"name"`
		Source  string         `json:"source"`
		Data    map[string]any `json:"data"`
		WantErr bool           `json:"-"`
	}{
		{Name: "complete plan", Source: string(source)},
		{Name: "raw delimiters", Source: `{% raw %}sed 's/{{//g'{% endraw %}`},
		{Name: "whitespace overrides", Source: "a\n  {%+ if true %}b{% endif %}\nc{% if true +%}\n  d{% endif %}\n"},
		{Name: "nested dict closing braces", Source: `{{ {'snssai': {'sst': 1}}}}`},
		{Name: "tuple assignment and repr", Source: `{% set a, b = '001', '01' %}{{ (a,) }} {{ (a,b,'x')[:2] }} {{ (a,b)[2:1] }} {{ [a,b] == (a,b) }}`},
		{Name: "literals", Source: `{{ 1e-3 }} {{ 3e9|int }} {{ 'mnc.' 'mcc.' 'gprs' }} {{ '5G\x20\u004e\122' }}`},
		{Name: "power and types", Source: `{{ 2**3**2 }} {{ 2**10 is integer }} {{ 1.0 }} {{ none }}`},
		{Name: "comparison chains", Source: `{{ 0 < 85 < 80 }} {{ 5 < 3 < raise_exception('must not run') }} {{ '10.2' < '9.4' }} {{ [10,2] < [10,4] }}`},
		{Name: "operator precedence", Source: `{{ -3|abs }} {{ 3 + 1 is integer }} {{ +true }} {{ -true }}`},
		{Name: "signed arithmetic", Source: `{{ -7//3 }} {{ 7%-3 }} {{ 5.5%2 }} {{ 10//0.5 }}`},
		{Name: "floating floor boundary", Source: `{{ a//b }} {{ -a//b }} {{ a%-b }} {{ -0.0//b }}`, Data: map[string]any{"a": 0.9, "b": 0.1}},
		{Name: "numeric indexing", Source: `{{ [['n2','n3']].0.1 }}|{{ ['a','b'][2/2] }}|{{ 'abcdef'[5/2:] }}`},
		{Name: "slice bounds and none", Source: `{{ [1,2,3,4][100:-100:-2] }} {{ 'Évry-1'[100:-100:-2] }} {{ 'abc'[none:none:-1] }}`},
		{Name: "slice evaluation order", Source: `{% set ns=namespace(n='') %}{% macro b(k,v) %}{% set ns.n=ns.n~k %}{{ v }}{% endmacro %}{{ 'ABCDEF'[b('a',0)|int:b('b',6)|int:b('c',2)|int] }} {{ ns.n }}`},
		{Name: "slice zero step", Source: `{{ 'abc'[::0] }}`, WantErr: true},
		{Name: "slice bound errors", Source: `{{ 'abc'[raise_exception('bad bound'):] }}`, WantErr: true},
		{Name: "slice bounds controls", Source: `{{ text[-100:100] }} {{ text[100:200] }} {{ text[none:none:-2] }} {{ text[0:100:2] }}`, Data: map[string]any{"text": "Évry-1"}},
		{Name: "runtime slice index type", Source: `{{ text[1.5:] }}`, Data: map[string]any{"text": "Évry-1"}, WantErr: true},
		{Name: "macro defaults and extra arguments", Source: `{% macro m(parts=['a','b'], separator='-') %}{{ parts|join(separator) }} {{ varargs|join(',') }} {{ kwargs.get('critical') }}{% endmacro %}{{ m(['x','y'], ':', 'z', critical=true) }}`},
		{Name: "loop local assignments", Source: `{% set v='standard' %}{% for dense in [true,false] %}{% if dense %}{% set v='dense' %}{% endif %}{{ v }};{% endfor %}{{ v }}`},
		{Name: "filtered tuple loop", Source: `{% for d,e in [('a',true),('b',false),('c',true)] if e %}{{ loop.index }}/{{ loop.length }}:{{ d }};{% else %}empty{% endfor %}`},
		{Name: "empty filtered loop", Source: `{% for d,e in [('a',false)] if e %}unexpected{% else %}empty{% endfor %}`},
		{Name: "recursive loop", Source: `{% for n in [{'name':'a','children':[{'name':'b','children':[]}]}] recursive %}{{ loop.depth0 }}:{{ n.name }};{% if n.children %}{{ loop(n.children) }}{% endif %}{% endfor %}`},
		{Name: "loop changed", Source: `{% for x in ['a','a','b','a'] %}{{ loop.changed(x) }};{% endfor %}`},
		{Name: "filter block", Source: `{% filter replace('-', '_')|upper %}metro-edge{% endfilter %}`},
		{Name: "mapping methods and order", Source: `{% for k,v in dict(upf=4,amf=3,smf=2).items() %}{{ k }}={{ v }};{% endfor %}{{ {'items':['x'],'vendor':'v'}.items()|list|length }} {{ dict([('qfi',9),('arp',8)],qfi=1) }}`},
		{Name: "joiner and namespace", Source: `{% set n=namespace({'limit':12}) %}{{ n.limit }}:{% set j=joiner() %}{% for x in ['a','b'] %}{{ j() }}{{ x }}{% endfor %}`},
		{Name: "sequence and case tests", Source: `{{ ['abc' is sequence, {} is sequence, '001' is upper, '001' is lower, 8 is divisibleby(4)] }}`},
		{Name: "mapping iteration filters", Source: `{% set d={'upf':4,'amf':3,'smf':2} %}{{ d|join(',') }} {{ d|first }} {{ d|last }} {{ 'Évry'|first }}`},
		{Name: "list copy and unique", Source: `{% set a=['IMS','ims','internet'] %}{% set b=a|list %}{{ b is sameas(a) }} {{ a|unique|join(',') }}`},
		{Name: "map and nested attribute filters", Source: `{% set a=[{'p':{'d':'ims','enabled':true}},{'p':{'enabled':false}}] %}{{ ['amf','upf']|map('upper')|join(',') }} {{ a|map(attribute='p.d',default='internet')|join(',') }} {{ a|selectattr('p.enabled')|list|length }} {{ a|rejectattr('p.enabled')|list|length }}`},
		{Name: "filter options", Source: `{{ '0xff'|int(base=16) }} {{ 'auto'|int(default=9) }} {{ 'auto'|float(default=23.5) }} {{ ''|default(default_value='internet',boolean=true) }} {{ 1.21|round(1,'ceil') }} {{ 1.29|round(1,'floor') }} {{ 2.5|round }}`},
		{Name: "filter option controls", Source: `{{ '42'|int(default=9) }} {{ '2.75'|float(default=23.5) }} {{ 2.675|round(2) }} {{ ['IMS','ims']|unique(case_sensitive=true)|join(',') }} {{ [{'n':'a','c':40},{'n':'b','c':100}]|min(attribute='c') }}`},
		{Name: "capacity filters", Source: `{{ [{'mbps':10000},{'mbps':25000}]|sum(attribute='mbps',start=1000) }} {{ []|sum(start=10000) }} {{ ([{'n':'a','c':40},{'n':'b','c':100}]|max(attribute='c')).n }} {{ [1,2,3]|batch(2,0)|map('list')|list }}`},
		{Name: "string methods", Source: `{{ "operator's metro EDGE lab".title() }} {{ 'NR RADIO'.capitalize() }} {{ '  ims  '.strip(none) }} {{ '\x1cims\x1f'.strip() }} {{ '　東京-edge　'.lstrip().rstrip() }}`},
		{Name: "bounded string methods", Source: `{{ 'NR-cell-07'.startswith(('LTE','NR')) }} {{ 'c1NR-metro'.startswith('NR',2,4) }} {{ 'NR-cell-07'.endswith('cell',0,7) }} {{ 'n78/n78/n41'.count('n78',3,7) }} {{ '東京-nr-nr'.find('nr',4) }} {{ 'NR'.find('',3) }}`},
		{Name: "bounded string controls", Source: `{{ text.find('nr',-2) }} {{ text.find('absent') }} {{ text.count('',100) }} {{ text.startswith('',100) }} {{ text.endswith('nr',0,-1) }}`, Data: map[string]any{"text": "東京-nr-nr"}},
		{Name: "string split and selection", Source: `{{ 'ims.mnc001.mcc001.gprs'.split('.',1) }} {{ '  interface   n3  '.split(none,1) }} {{ 'MetroEdgeLab'|select('upper')|join }} {{ 'transport-down,loss-of-sync'|wordcount }}`},
		{Name: "percent formatting", Source: `{{ 'cell-%02d' % 7 }}`},
		{Name: "json options", Source: `{{ {'site':'東京📡','load':0.0}|tojson(ensure_ascii=true,sort_keys=true) }} {{ {'power':1.0}|tojson }}`},
		{Name: "nested json options", Source: `{{ {'z':{'b':1.0,'a':['東京','📡']},'a':[]}|tojson(indent=2,sort_keys=true,ensure_ascii=true) }}`},
		{Name: "indent options", Source: `{{ 'a\n\nb'|indent(2,first=true,blank=true) }}|{{ 'a\nb'|indent('# ',first=true) }}`},
		{
			Name:   "snssai macro uses defaults",
			Source: `{% macro label(parts=['01','000001'], separator='-') %}{{ parts|join(separator) }}{% endmacro %}{{ label() }}`,
		},
		{
			Name:   "snssai macro accepts named provisioning inputs",
			Source: `{% macro label(parts=['01','000001'], separator='-') %}{{ parts|join(separator) }}{% endmacro %}{{ label(separator=separator, parts=parts) }}`,
			Data:   map[string]any{"parts": []any{"02", "000042"}, "separator": ":"},
		},
		{
			Name:   "nf macro collects only extra arguments",
			Source: `{% macro group(name) %}{{ name }}:{{ varargs|join(',') }}:{{ kwargs|length }}:{{ kwargs.get('critical', false) }}{% endmacro %}{{ group(name, first, second, critical=critical) }}|{{ group(name=name) }}`,
			Data:   map[string]any{"name": "control-plane", "first": "amf", "second": "smf", "critical": true},
		},
		{
			Name:    "macro rejects duplicate parameter binding",
			Source:  `{% macro label(parts) %}{{ parts|join('-') }}{% endmacro %}{{ label(parts, parts=parts) }}`,
			Data:    map[string]any{"parts": []any{"02", "000042"}},
			WantErr: true,
		},
		{
			Name:    "macro rejects unexpected keyword",
			Source:  `{% macro label(parts) %}{{ parts|join('-') }}{% endmacro %}{{ label(parts, unknown=true) }}`,
			Data:    map[string]any{"parts": []any{"02", "000042"}},
			WantErr: true,
		},
		{
			Name:   "admission comparison evaluates middle operand once",
			Source: `{% set trace=namespace(calls=0) %}{% macro measured(v) %}{% set trace.calls=trace.calls+1 %}{{ v }}{% endmacro %}{{ floor <= measured(utilization)|int < alarm }}:{{ trace.calls }}`,
			Data:   map[string]any{"floor": 0, "utilization": 65, "alarm": 80},
		},
		{
			Name:   "admission comparison stops after failed lower bound",
			Source: `{{ floor <= utilization < raise_exception('upper bound must not run') }}`,
			Data:   map[string]any{"floor": 0, "utilization": -1},
		},
		{
			Name:   "topology preserves siblings and parent loop metadata",
			Source: `{% for node in topology recursive %}[{{ loop.depth0 }}:{{ loop.index }}/{{ loop.length }}:{{ node.name }}{% if node.children %}{{ loop(node.children) }}{% endif %}:{{ loop.depth0 }}:{{ loop.index }}]{% endfor %}`,
			Data: map[string]any{"topology": []any{
				map[string]any{"name": "metro", "children": []any{
					map[string]any{"name": "edge-a", "children": []any{}},
					map[string]any{"name": "edge-b", "children": []any{
						map[string]any{"name": "site-1", "children": []any{}},
					}},
				}},
				map[string]any{"name": "suburban", "children": []any{}},
			}},
		},
		{
			Name:   "topology handles empty inventory",
			Source: `{% for node in topology recursive %}{{ node.name }}{% if node.children %}{{ loop(node.children) }}{% endif %}{% else %}no sites{% endfor %}`,
			Data:   map[string]any{"topology": []any{}},
		},
		{
			Name:   "region and zone change detection",
			Source: `{% for site in sites %}{{ loop.changed(site.region, site.zone) }};{% endfor %}`,
			Data: map[string]any{"sites": []any{
				map[string]any{"region": "metro", "zone": "a"},
				map[string]any{"region": "metro", "zone": "a"},
				map[string]any{"region": "metro", "zone": "b"},
				map[string]any{"region": "suburban", "zone": "b"},
				map[string]any{"region": "metro", "zone": "a"},
			}},
		},
		{
			Name:   "nested acceptance loops isolate assignments and share namespace",
			Source: `{% set count=namespace(n=0) %}{% set label='outer' %}{% for site in sites %}{% set label=site %}{% for check in checks %}{% set label=check %}{% set count.n=count.n+1 %}{{ label }}-{{ count.n }};{% endfor %}{{ label }};{% endfor %}{{ label }}:{{ count.n }}`,
			Data:   map[string]any{"sites": []any{"metro-1", "sub-2"}, "checks": []any{"registration", "dns", "time-sync"}},
		},
		{
			Name:   "standby list copy preserves nested identity",
			Source: `{% set copy=zones|list %}{{ copy is sameas(zones) }}:{{ copy.0 is sameas(zones.0) }}:{{ copy|length }}`,
			Data:   map[string]any{"zones": []any{[]any{"edge-a", "edge-b"}, []any{"edge-c"}}},
		},
		{
			Name:   "policy defaults distinguish missing null and empty",
			Source: `{{ slices|map(attribute='policy.dnn', default='internet')|list }}`,
			Data: map[string]any{"slices": []any{
				map[string]any{"policy": map[string]any{}},
				map[string]any{"policy": map[string]any{"dnn": nil}},
				map[string]any{"policy": map[string]any{"dnn": ""}},
				map[string]any{"policy": map[string]any{"dnn": "ims"}},
			}},
		},
		{
			Name:   "dnn fallback distinguishes undefined and false values",
			Source: `{{ missing|default('internet') }};{% for value in values %}{{ value|default('internet') }}:{{ value|default('internet', true) }};{% endfor %}`,
			Data:   map[string]any{"values": []any{nil, false, 0, "", []any{}, "ims"}},
		},
		{
			Name:   "backhaul sum accepts positional and keyword options",
			Source: `{{ links|sum('mbps', reserved) }}:{{ links|sum(attribute='mbps', start=reserved) }}`,
			Data:   map[string]any{"links": []any{map[string]any{"mbps": 1200}, map[string]any{"mbps": 3400}}, "reserved": 75},
		},
		{
			Name:   "empty backhaul sum preserves reservation",
			Source: `{{ links|sum(attribute='mbps', start=reserved) }}`,
			Data:   map[string]any{"links": []any{}, "reserved": 75},
		},
		{
			Name:   "carrier batch exact multiple needs no padding",
			Source: `{{ carriers|batch(2, 0)|map('list')|list }}`,
			Data:   map[string]any{"carriers": []any{1, 3, 5, 7}},
		},
		{
			Name:   "carrier batch empty inventory stays empty",
			Source: `{{ carriers|batch(2, 0)|map('list')|list }}`,
			Data:   map[string]any{"carriers": []any{}},
		},
		{
			Name:   "dnn catalog preserves first occurrence order",
			Source: `{{ dnns|unique|join(',') }}:{{ dnns|unique(case_sensitive=true)|join(',') }}`,
			Data:   map[string]any{"dnns": []any{"Internet", "IMS", "internet", "ims", "iot", "IMS"}},
		},
		{
			Name:   "cell formatting accepts multiple substitutions",
			Source: `{{ '%s-cell-%02d' % (rat, index) }}:{{ '100%% ready' % () }}`,
			Data:   map[string]any{"rat": "NR", "index": 12},
		},
		{
			Name:   "cell integer formatting accepts fractional numeric input",
			Source: `{{ 'cell-%02d' % index }}`,
			Data:   map[string]any{"index": 7.9},
		},
		{
			Name:   "vendor string bounds use unicode character positions",
			Source: `{{ name.find('NR', start, stop) }}:{{ name.startswith(('LTE','NR'), start, stop) }}:{{ name.endswith('NR', start, stop) }}`,
			Data:   map[string]any{"name": "東京NR-edge-NR", "start": 2, "stop": 4},
		},
		{
			Name:   "cli split preserves remainder after first whitespace split",
			Source: `{{ command.split(none, 1) }}`,
			Data:   map[string]any{"command": "\tinterface   n3  "},
		},
		{
			Name:   "numeric conversion options accept positional arguments",
			Source: `{{ reference|int(9, 16) }}:{{ invalid|int(9) }}:{{ invalid|float(23.5) }}`,
			Data:   map[string]any{"reference": "0x2a", "invalid": "auto"},
		},
		{Name: "macro accepts kwargs referenced in inactive branch", Source: `{% macro m() %}{% if false %}{{ kwargs }}{% endif %}ok{% endmacro %}{{ m(extra=true) }}`},
		{Name: "macro keyword name in string does not enable kwargs", Source: `{% macro m() %}{{ 'kwargs' }}{% endmacro %}{{ m(extra=true) }}`, WantErr: true},
		{Name: "mixed integer and float percent formatting", Source: `{{ '%02d/%.2f/%+04d/%%' % (7.9, 2.75, -3.8) }}`},
	}
	for i := range cases {
		if cases[i].Data == nil {
			cases[i].Data = map[string]any{}
		}
	}
	input, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("uv", "run", "--quiet", "--with", "jinja2>=3.1", "python", "scripts/render_provisioning.py")
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Python reference: %v\n%s", err, stderr.String())
	}
	var reference []struct {
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(output, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference) != len(cases) {
		t.Fatalf("reference count: got %d, want %d", len(reference), len(cases))
	}
	for i, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if (reference[i].Error != "") != tc.WantErr {
				t.Fatalf("Python: got error %q, want error %v", reference[i].Error, tc.WantErr)
			}
			tmpl, err := jinja.Compile(tc.Source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			for range 2 {
				got, err := tmpl.Render(tc.Data)
				if (err != nil) != tc.WantErr {
					t.Fatalf("render: got error %v, want error %v", err, tc.WantErr)
				}
				if !tc.WantErr && got != reference[i].Output {
					t.Errorf("output differs from Python\n%s", diffSummary(got, reference[i].Output))
				}
			}
		})
	}
}
