// This example demonstrates rendering a chat template that includes tool
// definitions, which is how function calling is formatted for LLMs.
package main

import (
	"fmt"
	"log"

	"github.com/ardanlabs/jinja"
)

// This is a simplified Qwen3-style chat template with tool calling support.
const chatTemplate = `{%- if tools %}
<|im_start|>system
{%- if messages[0].role == 'system' %}
{{ messages[0].content }}

{% endif -%}
# Tools

You may call one or more functions to assist with the user query.

You are provided with function signatures within <tools></tools> XML tags:
<tools>
{%- for tool in tools %}
{{ tool | tojson }}
{%- endfor %}
</tools>

For each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:
<tool_call>
{"name": <function-name>, "arguments": <args-json-object>}
</tool_call><|im_end|>
{%- else %}
{%- if messages[0].role == 'system' %}
<|im_start|>system
{{ messages[0].content }}<|im_end|>
{%- endif %}
{%- endif %}
{%- for message in messages %}
{%- if message.role == 'user' or (message.role == 'system' and not loop.first) %}
<|im_start|>{{ message.role }}
{{ message.content }}<|im_end|>
{%- elif message.role == 'assistant' %}
<|im_start|>assistant
{{ message.content }}<|im_end|>
{%- endif %}
{%- endfor %}
{%- if add_generation_prompt %}
<|im_start|>assistant
{%- endif -%}`

func main() {

	// Compile the template once.
	tmpl, err := jinja.Compile(chatTemplate)
	if err != nil {
		log.Fatal(err)
	}

	// Render a conversation with tool definitions.
	result, err := tmpl.Render(map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "system",
				"content": "You are a helpful weather assistant.",
			},
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
								"description": "The city and country",
							},
						},
						"required": []any{"location"},
					},
				},
			},
		},
		"add_generation_prompt": true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}
