// This example demonstrates basic Jinja template rendering including
// variable substitution, for loops, conditionals, and filters.
package main

import (
	"fmt"
	"log"

	"github.com/ardanlabs/jinja"
)

func main() {

	// Simple variable substitution.
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

	// -------------------------------------------------------------------------

	// For loop with a list.
	tmpl, err = jinja.Compile(`{%- for item in items -%}
{{ loop.index }}. {{ item }}
{% endfor -%}`)
	if err != nil {
		log.Fatal(err)
	}

	result, err = tmpl.Render(map[string]any{
		"items": []any{"Go", "Python", "Rust"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
	// Output:
	// 1. Go
	// 2. Python
	// 3. Rust

	// -------------------------------------------------------------------------

	// Conditionals and filters.
	tmpl, err = jinja.Compile(`{% if enabled %}Feature is ON{% else %}Feature is OFF{% endif %} — {{ data | tojson }}`)
	if err != nil {
		log.Fatal(err)
	}

	result, err = tmpl.Render(map[string]any{
		"enabled": true,
		"data":    map[string]any{"version": 1, "name": "jinja"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
	// Output: Feature is ON — {"name":"jinja","version":1}
}
