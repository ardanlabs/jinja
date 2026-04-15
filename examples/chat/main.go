// This example demonstrates rendering an LLM chat template. Chat templates
// are Jinja templates embedded in model files that format a conversation
// into the token sequence expected by a specific model.
package main

import (
	"fmt"
	"log"

	"github.com/ardanlabs/jinja"
)

// This is a simplified Qwen-style chat template for demonstration.
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

func main() {

	// Compile the chat template once.
	tmpl, err := jinja.Compile(chatTemplate)
	if err != nil {
		log.Fatal(err)
	}

	// Render with a multi-turn conversation.
	result, err := tmpl.Render(map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "system",
				"content": "You are a helpful assistant.",
			},
			map[string]any{
				"role":    "user",
				"content": "What is the capital of France?",
			},
			map[string]any{
				"role":    "assistant",
				"content": "The capital of France is Paris.",
			},
			map[string]any{
				"role":    "user",
				"content": "What about Germany?",
			},
		},
		"add_generation_prompt": true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}
