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
