package jinja_test

import (
	"testing"

	"github.com/ardanlabs/jinja"
)

// =============================================================================
// Simple benchmarks – basic variable substitution
// =============================================================================

func BenchmarkCompile_Simple(b *testing.B) {
	const source = "Hello {{ name }}!"
	for b.Loop() {
		if _, err := jinja.Compile(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRender_Simple(b *testing.B) {
	tmpl, err := jinja.Compile("Hello {{ name }}!")
	if err != nil {
		b.Fatal(err)
	}

	data := map[string]any{"name": "World"}

	b.ResetTimer()
	for b.Loop() {
		if _, err := tmpl.Render(data); err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// Chat template benchmarks – Qwen3 with tool calling (real-world)
// =============================================================================

var chatBenchData = map[string]any{
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

func BenchmarkCompile_Qwen3(b *testing.B) {
	source := chatTemplates["Qwen3-8B-Q8_0"]
	for b.Loop() {
		if _, err := jinja.Compile(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRender_Qwen3(b *testing.B) {
	tmpl, err := jinja.Compile(chatTemplates["Qwen3-8B-Q8_0"])
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := tmpl.Render(chatBenchData); err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// Complex template benchmarks – Gemma4 multi-turn tool calling (heaviest)
// =============================================================================

var gemmaMultiTurnData = map[string]any{
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

func BenchmarkCompile_Gemma4(b *testing.B) {
	source := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]
	for b.Loop() {
		if _, err := jinja.Compile(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRender_Gemma4(b *testing.B) {
	tmpl, err := jinja.Compile(chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"])
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := tmpl.Render(gemmaMultiTurnData); err != nil {
			b.Fatal(err)
		}
	}
}
