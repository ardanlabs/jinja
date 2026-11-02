# jinja

A pure-Go Jinja template engine purpose-built for rendering LLM chat templates. It compiles and executes the Jinja templates embedded in GGUF model files, turning conversations and tool definitions into the exact token sequences each model expects. Zero dependencies, zero CGO.

Copyright 2026 Ardan Labs

hello@ardanlabs.com

## Project Status

[![Go Reference](https://pkg.go.dev/badge/github.com/ardanlabs/jinja.svg)](https://pkg.go.dev/github.com/ardanlabs/jinja)
[![Go Report Card](https://goreportcard.com/badge/github.com/ardanlabs/jinja)](https://goreportcard.com/report/github.com/ardanlabs/jinja)
[![go.mod Go version](https://img.shields.io/github/go-mod/go-version/ardanlabs/jinja)](https://github.com/ardanlabs/jinja)
[![Linux](https://github.com/ardanlabs/jinja/actions/workflows/linux.yml/badge.svg)](https://github.com/ardanlabs/jinja/actions/workflows/linux.yml)

## Install

```
go get github.com/ardanlabs/jinja
```

## Pendra fork

This is Pendra's fork of [ardanlabs/jinja](https://github.com/ardanlabs/jinja). It keeps the
**upstream module path** (`module github.com/ardanlabs/jinja`) on purpose, so it is consumed
through a `go.mod` `replace` rather than by importing a different path:

```
require github.com/ardanlabs/jinja v1.6.0

replace github.com/ardanlabs/jinja => github.com/pendra-ai/jinja vX.Y.Z
```

### Releases

Every merge to `main` cuts a release, matching the convention in `pendra-ai/audiocpp-go` and
`pendra-ai/stable-diffusion-go`. The version is computed from the merged PR's labels or the
commit subject:

| Signal | Bump |
|---|---|
| `release:major` label or `#major` in the subject | major |
| `release:minor` label or `#minor` | minor |
| *(default)* | patch |
| `release:skip` label, `[skip release]` or `#norelease` | no release |

A `workflow_dispatch` run is a dry run — it reports the version it would cut and tags nothing.

Pin a real `vX.Y.Z` in the `replace` above rather than a `v0.0.0-<timestamp>-<sha>`
pseudo-version.

### Version lineage

The fork's tags are its own — it inherited none from upstream. The line was seeded by hand at
**`v1.7.0`**, chosen to sit just above the upstream **`v1.6.0`** this fork is based on, so the
number itself hints at the upstream base. Releases continue from there (`v1.7.1`, `v1.8.0`, …).

> **Version collisions with upstream.** Because the version line deliberately tracks upstream's
> numbering, a tag cut here could one day collide with an upstream release of the same version,
> where one version string would mean two different trees. That is an accepted trade-off in
> favour of a readable, upstream-aligned number. The escape hatch, if it ever bites, is a
> distinguishing suffix such as `v1.7.0-pendra.1` — upstream would never publish it.


## Quick Start

The API has two steps: compile a template once, then render it with data as many times as needed. Compiled templates are safe for concurrent use.

```go
package main

import (
	"fmt"
	"log"

	"github.com/ardanlabs/jinja"
)

func main() {
	tmpl, err := jinja.Compile("Hello {{ name }}!")
	if err != nil {
		log.Fatal(err)
	}

	result, err := tmpl.Render(map[string]any{
		"name": "World",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
	// Output: Hello World!
}
```

## Chat Template Example

The primary use case is rendering LLM chat templates. Each model ships a Jinja template that formats conversations into the token layout the model was trained on.

```go
const chatTemplate = `{%- if messages[0].role == 'system' -%}
<|im_start|>system
{{ messages[0].content }}<|im_end|>
{%- endif %}
{%- for message in messages -%}
{%- if message.role != 'system' %}
<|im_start|>{{ message.role }}
{{ message.content }}<|im_end|>
{%- endif -%}
{%- endfor %}
{%- if add_generation_prompt %}
<|im_start|>assistant
{%- endif -%}`

tmpl, err := jinja.Compile(chatTemplate)
if err != nil {
	log.Fatal(err)
}

result, err := tmpl.Render(map[string]any{
	"messages": []any{
		map[string]any{"role": "system", "content": "You are a helpful assistant."},
		map[string]any{"role": "user", "content": "What is the capital of France?"},
		map[string]any{"role": "assistant", "content": "The capital of France is Paris."},
		map[string]any{"role": "user", "content": "What about Germany?"},
	},
	"add_generation_prompt": true,
})
```

See the [examples](examples/) directory for more, including tool calling.

## Supported Features

### Template Syntax

- Variable output: `{{ expr }}`
- Statements: `{% if %}`, `{% for %}`, `{% set %}`, `{% macro %}`, `{% block %}`
- Whitespace control, matching Jinja2 with `trim_blocks` and `lstrip_blocks` on (the settings HuggingFace uses for chat templates): `{%- ... -%}`, `{{- ... -}}`, `{#- ... -#}`, `{%+ ... +%}`, `{#+ ... +#}`
- Comments: `{# ... #}`
- String concatenation with `~`
- Inline if expressions: `{{ x if condition else y }}`
- Slice notation: `{{ items[::-1] }}`

### Filters

`abs` · `batch` · `capitalize` · `count` · `default` / `d` · `dictsort` · `escape` / `e` · `first` · `float` · `format` · `fromjson` · `indent` · `int` · `items` · `join` · `last` · `length` · `list` · `lower` · `map` · `max` · `min` · `reject` · `rejectattr` · `replace` · `reverse` · `round` · `safe` · `select` · `selectattr` · `sort` · `string` · `sum` · `title` · `tojson` · `trim` · `unique` · `upper` · `wordcount`

### Tests

`defined` · `undefined` · `none` · `boolean` · `integer` · `float` · `number` · `string` · `mapping` · `iterable` · `sequence` · `callable` · `true` · `false` · `odd` · `even` · `upper` · `lower` · `sameas` · `eq` · `ne` · `gt` · `ge` · `lt` · `le` · `in`

As in Jinja2, a test takes its argument in parentheses or as one bare value: `x is sameas(false)`, `x is sameas false`, `x is in [1, 2]`.

### Global Functions

`namespace` · `range` · `dict` · `joiner` · `cycler` · `raise_exception` · `strftime_now`

### String Methods

`.strip()` · `.split()` · `.startswith()` · `.endswith()` · `.upper()` · `.lower()` · `.replace()` · `.find()` · `.count()` · `.lstrip()` · `.rstrip()`

### Dict Methods

`.get()` · `.items()` · `.keys()` · `.values()` · `.update()`

## Tested Models

The test suite compiles and renders the chat templates from these models:

| Model                         | Family      |
| ----------------------------- | ----------- |
| Qwen3-8B                      | Qwen3       |
| gpt-oss-20b                   | GPT-OSS     |
| Qwen3-VL-30B-A3B-Instruct     | Qwen3-VL    |
| Qwen3.5-35B-A3B               | Qwen3.5     |
| Qwen2-Audio-7B                | Qwen2-Audio |
| gemma-4-26B-A4B-it            | Gemma 4     |
| Ministral-3-14B-Instruct-2512 | Mistral 3   |
| rnj-1-instruct                | RNJ-1       |
| LFM2.5-VL-1.6B                | LFM2.5      |

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full text.
