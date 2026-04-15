package jinja_test

import (
	"strings"
	"testing"

	"github.com/ardanlabs/jinja"
)

// =============================================================================
// Basic engine tests (no model dependency)
// =============================================================================

func TestBasicRender(t *testing.T) {
	tmpl, err := jinja.Compile("Hello {{ name }}!")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"name": "World",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}

func TestForLoop(t *testing.T) {
	source := `{% for item in items %}{{ item }}{% if not loop.last %}, {% endif %}{% endfor %}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "a, b, c" {
		t.Errorf("expected 'a, b, c', got %q", result)
	}
}

func TestIfElse(t *testing.T) {
	source := `{% if x %}yes{% else %}no{% endif %}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{"x": true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "yes" {
		t.Errorf("expected 'yes', got %q", result)
	}

	result, err = tmpl.Render(map[string]any{"x": false})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "no" {
		t.Errorf("expected 'no', got %q", result)
	}
}

func TestTojsonFilter(t *testing.T) {
	source := `{{ data | tojson }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"data": map[string]any{"name": "test", "value": 42},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(result, `"name"`) || !strings.Contains(result, `"test"`) {
		t.Errorf("tojson output unexpected: %q", result)
	}
}

func TestNamespace(t *testing.T) {
	source := `{%- set ns = namespace(found=false) -%}
{%- for item in items -%}
{%- if item == "target" -%}
{%- set ns.found = true -%}
{%- endif -%}
{%- endfor -%}
{{ ns.found }}`

	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"a", "target", "b"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	result = strings.TrimSpace(result)
	if result != "True" {
		t.Errorf("expected 'True', got %q", result)
	}
}

func TestStringMethods(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"strip", `{{ "  hello  ".strip() }}`, "hello"},
		{"split", `{{ "a,b,c".split(",") | join("-") }}`, "a-b-c"},
		{"startswith", `{{ "hello".startswith("hel") }}`, "True"},
		{"endswith", `{{ "hello".endswith("llo") }}`, "True"},
		{"upper", `{{ "hello".upper() }}`, "HELLO"},
		{"lower", `{{ "HELLO".lower() }}`, "hello"},
		{"replace", `{{ "hello world".replace("world", "jinja") }}`, "hello jinja"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			result, err := tmpl.Render(nil)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestDictMethods(t *testing.T) {
	source := `{%- for key, value in data.items() -%}{{ key }}={{ value }} {% endfor -%}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"data": map[string]any{"a": 1, "b": 2},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Keys are sorted for deterministic output.
	if !strings.Contains(result, "a=1") || !strings.Contains(result, "b=2") {
		t.Errorf("unexpected dict items output: %q", result)
	}
}

func TestInlineIf(t *testing.T) {
	source := `{{ "yes" if x else "no" }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{"x": true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if result != "yes" {
		t.Errorf("expected 'yes', got %q", result)
	}
}

func TestIsTests(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"is defined", `{{ "yes" if x is defined else "no" }}`, "yes"},
		{"is not defined", `{{ "yes" if y is not defined else "no" }}`, "yes"},
		{"is string", `{{ "yes" if x is string else "no" }}`, "yes"},
		{"is none", `{{ "yes" if n is none else "no" }}`, "yes"},
		{"is true", `{{ "yes" if t is true else "no" }}`, "yes"},
		{"is false", `{{ "yes" if f is false else "no" }}`, "yes"},
	}

	data := map[string]any{
		"x": "hello",
		"n": nil,
		"t": true,
		"f": false,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			result, err := tmpl.Render(data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestWhitespaceControl(t *testing.T) {
	source := "A\n  {%- if true %}\nB\n  {%- endif %}\nC"
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(result, "A") && !strings.Contains(result, "B") && !strings.Contains(result, "C") {
		t.Errorf("whitespace control produced unexpected output: %q", result)
	}
}

func TestSlicing(t *testing.T) {
	source := `{{ items[::-1] | join(",") }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "c,b,a" {
		t.Errorf("expected 'c,b,a', got %q", result)
	}
}

func TestStringConcat(t *testing.T) {
	source := `{{ "Hello" ~ " " ~ "World" }}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result)
	}
}

func TestLoopVariables(t *testing.T) {
	source := `{%- for item in items -%}{{ loop.index0 }}:{{ item }} {% endfor -%}`
	tmpl, err := jinja.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := tmpl.Render(map[string]any{
		"items": []any{"x", "y", "z"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if result != "0:x 1:y 2:z " {
		t.Errorf("expected '0:x 1:y 2:z ', got %q", result)
	}
}

// =============================================================================
// Chat template compilation tests (verify templates from GGUF models compile)
// =============================================================================

func TestCompile_Qwen3_8B(t *testing.T) {
	template := chatTemplates["Qwen3-8B-Q8_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile Qwen3-8B template: %v", err)
	}
}

func TestCompile_GPT_OSS_20B(t *testing.T) {
	template := chatTemplates["gpt-oss-20b-Q8_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile gpt-oss-20b template: %v", err)
	}
}

func TestCompile_Qwen3_VL_30B(t *testing.T) {
	template := chatTemplates["Qwen3-VL-30B-A3B-Instruct-Q8_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile Qwen3-VL-30B template: %v", err)
	}
}

func TestCompile_Qwen35_35B(t *testing.T) {
	template := chatTemplates["Qwen3.5-35B-A3B-Q8_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile Qwen3.5-35B template: %v", err)
	}
}

func TestCompile_Qwen2_Audio(t *testing.T) {
	template := chatTemplates["Qwen2-Audio-7B.Q8_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile Qwen2-Audio template: %v", err)
	}
}

// =============================================================================
// Chat template render tests with simple messages
// =============================================================================

func TestRender_Qwen3_8B_SimpleChat(t *testing.T) {
	template := chatTemplates["Qwen3-8B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3-8B output:\n%s", result)

	// Verify key structural elements.
	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
	if !strings.Contains(result, "user") {
		t.Error("output missing user role marker")
	}
	if !strings.Contains(result, "assistant") {
		t.Error("output missing assistant generation prompt")
	}
}

func TestRender_GPT_OSS_20B_SimpleChat(t *testing.T) {
	template := chatTemplates["gpt-oss-20b-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gpt-oss-20b output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Qwen35_35B_SimpleChat(t *testing.T) {
	template := chatTemplates["Qwen3.5-35B-A3B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3.5-35B output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

// =============================================================================
// Tool calling template tests
// =============================================================================

func TestRender_Qwen3_8B_ToolCalling(t *testing.T) {
	template := chatTemplates["Qwen3-8B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
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

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3-8B tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
	if !strings.Contains(result, "London") {
		t.Error("output missing user message")
	}
}

func TestRender_GPT_OSS_20B_ToolCalling(t *testing.T) {
	template := chatTemplates["gpt-oss-20b-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
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
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gpt-oss-20b tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
}

func TestRender_Qwen35_35B_ToolCalling(t *testing.T) {
	template := chatTemplates["Qwen3.5-35B-A3B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
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

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Qwen3.5-35B tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
}

// =============================================================================
// Additional model template tests
// =============================================================================

func TestCompile_Local_Gemma4(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile gemma-4 template: %v", err)
	}
}

func TestCompile_Local_Ministral3(t *testing.T) {
	template := chatTemplates["Ministral-3-14B-Instruct-2512-Q4_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile Ministral-3 template: %v", err)
	}
}

func TestCompile_Local_RNJ1(t *testing.T) {
	template := chatTemplates["rnj-1-instruct-Q6_K"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile rnj-1 template: %v", err)
	}
}

func TestCompile_Local_LFM25(t *testing.T) {
	template := chatTemplates["LFM2.5-VL-1.6B-Q8_0"]
	_, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("failed to compile LFM2.5-VL template: %v", err)
	}
}

func TestRender_Local_Gemma4_SimpleChat(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
		"enable_thinking":       true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gemma-4 output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
	if !strings.Contains(result, "model") || !strings.Contains(result, "user") {
		t.Error("output missing role markers")
	}
}

func TestRender_Local_Ministral3_SimpleChat(t *testing.T) {
	template := chatTemplates["Ministral-3-14B-Instruct-2512-Q4_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("Ministral-3 output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Local_RNJ1_SimpleChat(t *testing.T) {
	template := chatTemplates["rnj-1-instruct-Q6_K"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("rnj-1 output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Local_LFM25_SimpleChat(t *testing.T) {
	template := chatTemplates["LFM2.5-VL-1.6B-Q8_0"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"add_generation_prompt": true,
	}

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("LFM2.5-VL output:\n%s", result)

	if !strings.Contains(result, "Hello!") {
		t.Error("output missing user message content")
	}
}

func TestRender_Local_Gemma4_ToolCalling(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
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

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gemma-4 tool calling output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
	if !strings.Contains(result, "London") {
		t.Error("output missing user message")
	}
}

// TestRender_Local_Gemma4_MultiTurnToolCall exercises the exact scenario that
// fails in gonja: multi-turn conversation with assistant tool_calls followed by
// tool responses. The template uses message.get('reasoning'), message.get('tool_calls'),
// message.get('content'), etc.
func TestRender_Local_Gemma4_MultiTurnToolCall(t *testing.T) {
	template := chatTemplates["gemma-4-26B-A4B-it-UD-Q8_K_XL"]

	tmpl, err := jinja.Compile(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := map[string]any{
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

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	t.Logf("gemma-4 multi-turn tool call output:\n%s", result)

	if !strings.Contains(result, "get_weather") {
		t.Error("output missing tool name")
	}
	if !strings.Contains(result, "London") {
		t.Error("output missing first user message")
	}
	if !strings.Contains(result, "Paris") {
		t.Error("output missing second user message")
	}
	if !strings.Contains(result, "tool_call") || !strings.Contains(result, "tool_response") {
		t.Error("output missing tool call/response markers")
	}
}

// TestDictGet verifies dict.get() works correctly — the exact method that
// fails in gonja with "unknown method 'get' for ”".
func TestDictGet(t *testing.T) {
	tests := []struct {
		name   string
		source string
		data   map[string]any
		want   string
	}{
		{
			name:   "get existing key",
			source: `{{ d.get('name') }}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "Alice",
		},
		{
			name:   "get missing key returns none",
			source: `{{ d.get('missing') }}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "",
		},
		{
			name:   "get missing key with default",
			source: `{{ d.get('missing', 'fallback') }}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "fallback",
		},
		{
			name:   "get in or chain",
			source: `{{ d.get('a') or d.get('b') or 'none' }}`,
			data:   map[string]any{"d": map[string]any{"b": "found_b"}},
			want:   "found_b",
		},
		{
			name:   "get in if condition",
			source: `{% if d.get('tool_calls') %}yes{% else %}no{% endif %}`,
			data:   map[string]any{"d": map[string]any{"name": "Alice"}},
			want:   "no",
		},
		{
			name:   "get truthy check",
			source: `{% if d.get('tool_calls') %}yes{% else %}no{% endif %}`,
			data:   map[string]any{"d": map[string]any{"tool_calls": []any{"call1"}}},
			want:   "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := jinja.Compile(tt.source)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			result, err := tmpl.Render(tt.data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

// chatTemplates maps model names to their chat templates from Hugging Face.
var chatTemplates = map[string]string{
	"Qwen3-8B-Q8_0": `{%- if tools %}
    {{- '<|im_start|>system\n' }}
    {%- if messages[0].role == 'system' %}
        {{- messages[0].content + '\n\n' }}
    {%- endif %}
    {{- "# Tools\n\nYou may call one or more functions to assist with the user query.\n\nYou are provided with function signatures within <tools></tools> XML tags:\n<tools>" }}
    {%- for tool in tools %}
        {{- "\n" }}
        {{- tool | tojson }}
    {%- endfor %}
    {{- "\n</tools>\n\nFor each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:\n<tool_call>\n{\"name\": <function-name>, \"arguments\": <args-json-object>}\n</tool_call><|im_end|>\n" }}
{%- else %}
    {%- if messages[0].role == 'system' %}
        {{- '<|im_start|>system\n' + messages[0].content + '<|im_end|>\n' }}
    {%- endif %}
{%- endif %}
{%- set ns = namespace(multi_step_tool=true, last_query_index=messages|length - 1) %}
{%- for message in messages[::-1] %}
    {%- set index = (messages|length - 1) - loop.index0 %}
    {%- if ns.multi_step_tool and message.role == "user" and message.content is string and not(message.content.startswith('<tool_response>') and message.content.endswith('</tool_response>')) %}
        {%- set ns.multi_step_tool = false %}
        {%- set ns.last_query_index = index %}
    {%- endif %}
{%- endfor %}
{%- for message in messages %}
    {%- if message.content is string %}
        {%- set content = message.content %}
    {%- else %}
        {%- set content = '' %}
    {%- endif %}
    {%- if (message.role == "user") or (message.role == "system" and not loop.first) %}
        {{- '<|im_start|>' + message.role + '\n' + content + '<|im_end|>' + '\n' }}
    {%- elif message.role == "assistant" %}
        {%- set reasoning_content = '' %}
        {%- if message.reasoning_content is string %}
            {%- set reasoning_content = message.reasoning_content %}
        {%- else %}
            {%- if '</think>' in content %}
                {%- set reasoning_content = content.split('</think>')[0].rstrip('\n').split('<think>')[-1].lstrip('\n') %}
                {%- set content = content.split('</think>')[-1].lstrip('\n') %}
            {%- endif %}
        {%- endif %}
        {%- if loop.index0 > ns.last_query_index %}
            {%- if loop.last or (not loop.last and reasoning_content) %}
                {{- '<|im_start|>' + message.role + '\n<think>\n' + reasoning_content.strip('\n') + '\n</think>\n\n' + content.lstrip('\n') }}
            {%- else %}
                {{- '<|im_start|>' + message.role + '\n' + content }}
            {%- endif %}
        {%- else %}
            {{- '<|im_start|>' + message.role + '\n' + content }}
        {%- endif %}
        {%- if message.tool_calls %}
            {%- for tool_call in message.tool_calls %}
                {%- if (loop.first and content) or (not loop.first) %}
                    {{- '\n' }}
                {%- endif %}
                {%- if tool_call.function %}
                    {%- set tool_call = tool_call.function %}
                {%- endif %}
                {{- '<tool_call>\n{"name": "' }}
                {{- tool_call.name }}
                {{- '", "arguments": ' }}
                {%- if tool_call.arguments is string %}
                    {{- tool_call.arguments }}
                {%- else %}
                    {{- tool_call.arguments | tojson }}
                {%- endif %}
                {{- '}\n</tool_call>' }}
            {%- endfor %}
        {%- endif %}
        {{- '<|im_end|>\n' }}
    {%- elif message.role == "tool" %}
        {%- if loop.first or (messages[loop.index0 - 1].role != "tool") %}
            {{- '<|im_start|>user' }}
        {%- endif %}
        {{- '\n<tool_response>\n' }}
        {{- content }}
        {{- '\n</tool_response>' }}
        {%- if loop.last or (messages[loop.index0 + 1].role != "tool") %}
            {{- '<|im_end|>\n' }}
        {%- endif %}
    {%- endif %}
{%- endfor %}
{%- if add_generation_prompt %}
    {{- '<|im_start|>assistant\n' }}
    {%- if enable_thinking is defined and enable_thinking is false %}
        {{- '<think>\n\n</think>\n\n' }}
    {%- endif %}
{%- endif %}`,

	"gpt-oss-20b-Q8_0": `{#-
  In addition to the normal inputs of ` + "`" + `messages` + "`" + ` and ` + "`" + `tools` + "`" + `, this template also accepts the
  following kwargs:
  - "builtin_tools": A list, can contain "browser" and/or "python".
  - "model_identity": A string that optionally describes the model identity.
  - "reasoning_effort": A string that describes the reasoning effort, defaults to "medium".
 #}

{#- Tool Definition Rendering ============================================== #}
{%- macro render_typescript_type(param_spec, required_params, is_nullable=false) -%}
    {%- if param_spec.type == "array" -%}
        {%- if param_spec['items'] -%}
            {%- if param_spec['items']['type'] == "string" -%}
                {{- "string[]" }}
            {%- elif param_spec['items']['type'] == "number" -%}
                {{- "number[]" }}
            {%- elif param_spec['items']['type'] == "integer" -%}
                {{- "number[]" }}
            {%- elif param_spec['items']['type'] == "boolean" -%}
                {{- "boolean[]" }}
            {%- else -%}
                {%- set inner_type = render_typescript_type(param_spec['items'], required_params) -%}
                {%- if inner_type == "object | object" or inner_type|length > 50 -%}
                    {{- "any[]" }}
                {%- else -%}
                    {{- inner_type + "[]" }}
                {%- endif -%}
            {%- endif -%}
            {%- if param_spec.nullable -%}
                {{- " | null" }}
            {%- endif -%}
        {%- else -%}
            {{- "any[]" }}
            {%- if param_spec.nullable -%}
                {{- " | null" }}
            {%- endif -%}
        {%- endif -%}
    {%- elif param_spec.type is defined and param_spec.type is iterable and param_spec.type is not string and param_spec.type is not mapping and param_spec.type[0] is defined -%}
        {#- Handle array of types like ["object", "object"] from Union[dict, list] #}
        {%- if param_spec.type | length > 1 -%}
            {{- param_spec.type | join(" | ") }}
        {%- else -%}
            {{- param_spec.type[0] }}
        {%- endif -%}
    {%- elif param_spec.oneOf -%}
        {#- Handle oneOf schemas - check for complex unions and fallback to any #}
        {%- set has_object_variants = false -%}
        {%- for variant in param_spec.oneOf -%}
            {%- if variant.type == "object" -%}
                {%- set has_object_variants = true -%}
            {%- endif -%}
        {%- endfor -%}
        {%- if has_object_variants and param_spec.oneOf|length > 1 -%}
            {{- "any" }}
        {%- else -%}
            {%- for variant in param_spec.oneOf -%}
                {{- render_typescript_type(variant, required_params) -}}
                {%- if variant.description %}
                    {{- "// " + variant.description }}
                {%- endif -%}
                {%- if variant.default is defined %}
                    {{ "// default: " + variant.default|tojson }}
                {%- endif -%}
                {%- if not loop.last %}
                    {{- " | " }}
                {% endif -%}
            {%- endfor -%}
        {%- endif -%}
    {%- elif param_spec.type == "string" -%}
        {%- if param_spec.enum -%}
            {{- '"' + param_spec.enum|join('" | "') + '"' -}}
        {%- else -%}
            {{- "string" }}
            {%- if param_spec.nullable %}
                {{- " | null" }}
            {%- endif -%}
        {%- endif -%}
    {%- elif param_spec.type == "number" -%}
        {{- "number" }}
    {%- elif param_spec.type == "integer" -%}
        {{- "number" }}
    {%- elif param_spec.type == "boolean" -%}
        {{- "boolean" }}

    {%- elif param_spec.type == "object" -%}
        {%- if param_spec.properties -%}
            {{- "{\n" }}
            {%- for prop_name, prop_spec in param_spec.properties.items() -%}
                {{- prop_name -}}
                {%- if prop_name not in (param_spec.required or []) -%}
                    {{- "?" }}
                {%- endif -%}
                {{- ": " }}
                {{ render_typescript_type(prop_spec, param_spec.required or []) }}
                {%- if not loop.last -%}
                    {{-", " }}
                {%- endif -%}
            {%- endfor -%}
            {{- "}" }}
        {%- else -%}
            {{- "object" }}
        {%- endif -%}
    {%- else -%}
        {{- "any" }}
    {%- endif -%}
{%- endmacro -%}

{%- macro render_tool_namespace(namespace_name, tools) -%}
    {{- "## " + namespace_name + "\n\n" }}
    {{- "namespace " + namespace_name + " {\n\n" }}
    {%- for tool in tools %}
        {%- set tool = tool.function %}
        {{- "// " + tool.description + "\n" }}
        {{- "type "+ tool.name + " = " }}
        {%- if tool.parameters and tool.parameters.properties %}
            {{- "(_: {\n" }}
            {%- for param_name, param_spec in tool.parameters.properties.items() %}
                {%- if param_spec.description %}
                    {{- "// " + param_spec.description + "\n" }}
                {%- endif %}
                {{- param_name }}
                {%- if param_name not in (tool.parameters.required or []) -%}
                    {{- "?" }}
                {%- endif -%}
                {{- ": " }}
                {{- render_typescript_type(param_spec, tool.parameters.required or []) }}
                {%- if param_spec.default is defined -%}
                    {%- if param_spec.enum %}
                        {{- ", // default: " + param_spec.default }}
                    {%- elif param_spec.oneOf %}
                        {{- "// default: " + param_spec.default }}
                    {%- else %}
                        {{- ", // default: " + param_spec.default|tojson }}
                    {%- endif -%}
                {%- endif -%}
                {%- if not loop.last %}
                    {{- ",\n" }}
                {%- else %}
                    {{- ",\n" }}
                {%- endif -%}
            {%- endfor %}
            {{- "}) => any;\n\n" }}
        {%- else -%}
            {{- "() => any;\n\n" }}
        {%- endif -%}
    {%- endfor %}
    {{- "} // namespace " + namespace_name }}
{%- endmacro -%}

{%- macro render_builtin_tools(browser_tool, python_tool) -%}
    {%- if browser_tool %}
        {{- "## browser\n\n" }}
        {{- "// Tool for browsing.\n" }}
        {{- "// The ` + "`" + `cursor` + "`" + ` appears in brackets before each browsing display: ` + "`" + `[{cursor}]` + "`" + `.\n" }}
        {{- "// Cite information from the tool using the following format:\n" }}
        {{- "// ` + "`" + `【{cursor}†L{line_start}(-L{line_end})?】` + "`" + `, for example: ` + "`" + `【6†L9-L11】` + "`" + ` or ` + "`" + `【8†L3】` + "`" + `.\n" }}
        {{- "// Do not quote more than 10 words directly from the tool output.\n" }}
        {{- "// sources=web (default: web)\n" }}
        {{- "namespace browser {\n\n" }}
        {{- "// Searches for information related to ` + "`" + `query` + "`" + ` and displays ` + "`" + `topn` + "`" + ` results.\n" }}
        {{- "type search = (_: {\n" }}
        {{- "query: string,\n" }}
        {{- "topn?: number, // default: 10\n" }}
        {{- "source?: string,\n" }}
        {{- "}) => any;\n\n" }}
        {{- "// Opens the link ` + "`" + `id` + "`" + ` from the page indicated by ` + "`" + `cursor` + "`" + ` starting at line number ` + "`" + `loc` + "`" + `, showing ` + "`" + `num_lines` + "`" + ` lines.\n" }}
        {{- "// Valid link ids are displayed with the formatting: ` + "`" + `【{id}†.*】` + "`" + `.\n" }}
        {{- "// If ` + "`" + `cursor` + "`" + ` is not provided, the most recent page is implied.\n" }}
        {{- "// If ` + "`" + `id` + "`" + ` is a string, it is treated as a fully qualified URL associated with ` + "`" + `source` + "`" + `.\n" }}
        {{- "// If ` + "`" + `loc` + "`" + ` is not provided, the viewport will be positioned at the beginning of the document or centered on the most relevant passage, if available.\n" }}
        {{- "// Use this function without ` + "`" + `id` + "`" + ` to scroll to a new location of an opened page.\n" }}
        {{- "type open = (_: {\n" }}
        {{- "id?: number | string, // default: -1\n" }}
        {{- "cursor?: number, // default: -1\n" }}
        {{- "loc?: number, // default: -1\n" }}
        {{- "num_lines?: number, // default: -1\n" }}
        {{- "view_source?: boolean, // default: false\n" }}
        {{- "source?: string,\n" }}
        {{- "}) => any;\n\n" }}
        {{- "// Finds exact matches of ` + "`" + `pattern` + "`" + ` in the current page, or the page given by ` + "`" + `cursor` + "`" + `.\n" }}
        {{- "type find = (_: {\n" }}
        {{- "pattern: string,\n" }}
        {{- "cursor?: number, // default: -1\n" }}
        {{- "}) => any;\n\n" }}
        {{- "} // namespace browser\n\n" }}
    {%- endif -%}

    {%- if python_tool %}
        {{- "## python\n\n" }}
        {{- "Use this tool to execute Python code in your chain of thought. The code will not be shown to the user. This tool should be used for internal reasoning, but not for code that is intended to be visible to the user (e.g. when creating plots, tables, or files).\n\n" }}
        {{- "When you send a message containing Python code to python, it will be executed in a stateful Jupyter notebook environment. python will respond with the output of the execution or time out after 120.0 seconds. The drive at '/mnt/data' can be used to save and persist user files. Internet access for this session is UNKNOWN. Depends on the cluster.\n\n" }}
    {%- endif -%}
{%- endmacro -%}

{#- System Message Construction ============================================ #}
{%- macro build_system_message() -%}
    {%- if model_identity is not defined %}
        {%- set model_identity = "You are ChatGPT, a large language model trained by OpenAI." %}
    {%- endif %}
    {{- model_identity + "\n" }}
    {{- "Knowledge cutoff: 2024-06\n" }}
    {{- "Current date: " + strftime_now("%Y-%m-%d") + "\n\n" }}
    {%- if reasoning_effort is not defined %}
        {%- set reasoning_effort = "medium" %}
    {%- endif %}
    {{- "Reasoning: " + reasoning_effort + "\n\n" }}
    {%- if builtin_tools %}
        {{- "# Tools\n\n" }}
        {%- set available_builtin_tools = namespace(browser=false, python=false) %}
        {%- for tool in builtin_tools %}
            {%- if tool == "browser" %}
                {%- set available_builtin_tools.browser = true %}
            {%- elif tool == "python" %}
                {%- set available_builtin_tools.python = true %}
            {%- endif %}
        {%- endfor %}
        {{- render_builtin_tools(available_builtin_tools.browser, available_builtin_tools.python) }}
    {%- endif -%}
    {{- "# Valid channels: analysis, commentary, final. Channel must be included for every message." }}
    {%- if tools -%}
        {{- "\nCalls to these tools must go to the commentary channel: 'functions'." }}
    {%- endif -%}
{%- endmacro -%}

{#- Main Template Logic ================================================= #}
{#- Set defaults #}

{#- Render system message #}
{{- "<|start|>system<|message|>" }}
{{- build_system_message() }}
{{- "<|end|>" }}

{#- Extract developer message #}
{%- if messages[0].role == "developer" or messages[0].role == "system" %}
    {%- set developer_message = messages[0].content %}
    {%- set loop_messages = messages[1:] %}
{%- else %}
    {%- set developer_message = "" %}
    {%- set loop_messages = messages %}
{%- endif %}

{#- Render developer message #}
{%- if developer_message or tools %}
    {{- "<|start|>developer<|message|>" }}
    {%- if developer_message %}
        {{- "# Instructions\n\n" }}
        {{- developer_message }}
        {{- "\n\n" }}
    {%- endif %}
    {%- if tools -%}
        {{- "# Tools\n\n" }}
        {{- render_tool_namespace("functions", tools) }}
    {%- endif -%}
    {{- "<|end|>" }}
{%- endif %}

{#- Render messages #}
{%- set last_tool_call = namespace(name=none) %}
{%- for message in loop_messages -%}
    {#- At this point only assistant/user/tool messages should remain #}
    {%- if message.role == 'assistant' -%}
        {#- Checks to ensure the messages are being passed in the format we expect #}
        {%- if "content" in message %}
            {%- if "<|channel|>analysis<|message|>" in message.content or "<|channel|>final<|message|>" in message.content %}
                {{- raise_exception("You have passed a message containing <|channel|> tags in the content field. Instead of doing this, you should pass analysis messages (the string between '<|message|>' and '<|end|>') in the 'thinking' field, and final messages (the string between '<|message|>' and '<|end|>') in the 'content' field.") }}
            {%- endif %}
        {%- endif %}
        {%- if "thinking" in message %}
            {%- if "<|channel|>analysis<|message|>" in message.thinking or "<|channel|>final<|message|>" in message.thinking %}
                {{- raise_exception("You have passed a message containing <|channel|> tags in the thinking field. Instead of doing this, you should pass analysis messages (the string between '<|message|>' and '<|end|>') in the 'thinking' field, and final messages (the string between '<|message|>' and '<|end|>') in the 'content' field.") }}
            {%- endif %}
        {%- endif %}
        {%- if "tool_calls" in message %}
            {#- We need very careful handling here - we want to drop the tool call analysis message if the model #}
            {#- has output a later <|final|> message, but otherwise we want to retain it. This is the only case #}
            {#- when we render CoT/analysis messages in inference. #}
            {%- set future_final_message = namespace(found=false) %}
            {%- for future_message in loop_messages[loop.index:] %}
                {%- if future_message.role == 'assistant' and "tool_calls" not in future_message %}
                    {%- set future_final_message.found = true %}
                {%- endif %}
            {%- endfor %}
            {#- We assume max 1 tool call per message, and so we infer the tool call name #}
            {#- in "tool" messages from the most recent assistant tool call name #}
            {%- set tool_call = message.tool_calls[0] %}
            {%- if tool_call.function %}
                {%- set tool_call = tool_call.function %}
            {%- endif %}
            {%- if message.content and message.thinking %}
                {{- raise_exception("Cannot pass both content and thinking in an assistant message with tool calls! Put the analysis message in one or the other, but not both.") }}
            {%- elif message.content and not future_final_message.found %}
                {{- "<|start|>assistant<|channel|>analysis<|message|>" + message.content + "<|end|>" }}
            {%- elif message.thinking and not future_final_message.found %}
                {{- "<|start|>assistant<|channel|>analysis<|message|>" + message.thinking + "<|end|>" }}
            {%- endif %}
            {{- "<|start|>assistant to=" }}
            {{- "functions." + tool_call.name + "<|channel|>commentary " }}
            {{- (tool_call.content_type if tool_call.content_type is defined else "json") + "<|message|>" }}
            {{- tool_call.arguments|tojson }}
            {{- "<|call|>" }}
            {%- set last_tool_call.name = tool_call.name %}
        {%- elif loop.last and not add_generation_prompt %}
            {#- Only render the CoT if the final turn is an assistant turn and add_generation_prompt is false #}
            {#- This is a situation that should only occur in training, never in inference. #}
            {%- if "thinking" in message %}
                {{- "<|start|>assistant<|channel|>analysis<|message|>" + message.thinking + "<|end|>" }}
            {%- endif %}
            {#- <|return|> indicates the end of generation, but <|end|> does not #}
            {#- <|return|> should never be an input to the model, but we include it as the final token #}
            {#- when training, so the model learns to emit it. #}
            {{- "<|start|>assistant<|channel|>final<|message|>" + message.content + "<|return|>" }}
        {%- else %}
            {#- CoT is dropped during all previous turns, so we never render it for inference #}
            {{- "<|start|>assistant<|channel|>final<|message|>" + message.content + "<|end|>" }}
            {%- set last_tool_call.name = none %}
        {%- endif %}
    {%- elif message.role == 'tool' -%}
        {%- if last_tool_call.name is none %}
            {{- raise_exception("Message has tool role, but there was no previous assistant message with a tool call!") }}
        {%- endif %}
        {{- "<|start|>functions." + last_tool_call.name }}
        {{- " to=assistant<|channel|>commentary<|message|>" + message.content|tojson + "<|end|>" }}
    {%- elif message.role == 'user' -%}
        {{- "<|start|>user<|message|>" + message.content + "<|end|>" }}
    {%- endif -%}
{%- endfor -%}

{#- Generation prompt #}
{%- if add_generation_prompt -%}
<|start|>assistant
{%- endif -%}`,

	"Qwen3-VL-30B-A3B-Instruct-Q8_0": `{%- if tools %}
    {{- '<|im_start|>system\n' }}
    {%- if messages[0].role == 'system' %}
        {%- if messages[0].content is string %}
            {{- messages[0].content }}
        {%- else %}
            {%- for content in messages[0].content %}
                {%- if 'text' in content %}
                    {{- content.text }}
                {%- endif %}
            {%- endfor %}
        {%- endif %}
        {{- '\n\n' }}
    {%- endif %}
    {{- "# Tools\n\nYou may call one or more functions to assist with the user query.\n\nYou are provided with function signatures within <tools></tools> XML tags:\n<tools>" }}
    {%- for tool in tools %}
        {{- "\n" }}
        {{- tool | tojson }}
    {%- endfor %}
    {{- "\n</tools>\n\nFor each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:\n<tool_call>\n{\"name\": <function-name>, \"arguments\": <args-json-object>}\n</tool_call><|im_end|>\n" }}
{%- else %}
    {%- if messages[0].role == 'system' %}
        {{- '<|im_start|>system\n' }}
        {%- if messages[0].content is string %}
            {{- messages[0].content }}
        {%- else %}
            {%- for content in messages[0].content %}
                {%- if 'text' in content %}
                    {{- content.text }}
                {%- endif %}
            {%- endfor %}
        {%- endif %}
        {{- '<|im_end|>\n' }}
    {%- endif %}
{%- endif %}
{%- set image_count = namespace(value=0) %}
{%- set video_count = namespace(value=0) %}
{%- for message in messages %}
    {%- if message.role == "user" %}
        {{- '<|im_start|>' + message.role + '\n' }}
        {%- if message.content is string %}
            {{- message.content }}
        {%- else %}
            {%- for content in message.content %}
                {%- if content.type == 'image' or 'image' in content or 'image_url' in content %}
                    {%- set image_count.value = image_count.value + 1 %}
                    {%- if add_vision_id %}Picture {{ image_count.value }}: {% endif -%}
                    <|vision_start|><|image_pad|><|vision_end|>
                {%- elif content.type == 'video' or 'video' in content %}
                    {%- set video_count.value = video_count.value + 1 %}
                    {%- if add_vision_id %}Video {{ video_count.value }}: {% endif -%}
                    <|vision_start|><|video_pad|><|vision_end|>
                {%- elif 'text' in content %}
                    {{- content.text }}
                {%- endif %}
            {%- endfor %}
        {%- endif %}
        {{- '<|im_end|>\n' }}
    {%- elif message.role == "assistant" %}
        {{- '<|im_start|>' + message.role + '\n' }}
        {%- if message.content is string %}
            {{- message.content }}
        {%- else %}
            {%- for content_item in message.content %}
                {%- if 'text' in content_item %}
                    {{- content_item.text }}
                {%- endif %}
            {%- endfor %}
        {%- endif %}
        {%- if message.tool_calls %}
            {%- for tool_call in message.tool_calls %}
                {%- if (loop.first and message.content) or (not loop.first) %}
                    {{- '\n' }}
                {%- endif %}
                {%- if tool_call.function %}
                    {%- set tool_call = tool_call.function %}
                {%- endif %}
                {{- '<tool_call>\n{"name": "' }}
                {{- tool_call.name }}
                {{- '", "arguments": ' }}
                {%- if tool_call.arguments is string %}
                    {{- tool_call.arguments }}
                {%- else %}
                    {{- tool_call.arguments | tojson }}
                {%- endif %}
                {{- '}\n</tool_call>' }}
            {%- endfor %}
        {%- endif %}
        {{- '<|im_end|>\n' }}
    {%- elif message.role == "tool" %}
        {%- if loop.first or (messages[loop.index0 - 1].role != "tool") %}
            {{- '<|im_start|>user' }}
        {%- endif %}
        {{- '\n<tool_response>\n' }}
        {%- if message.content is string %}
            {{- message.content }}
        {%- else %}
            {%- for content in message.content %}
                {%- if content.type == 'image' or 'image' in content or 'image_url' in content %}
                    {%- set image_count.value = image_count.value + 1 %}
                    {%- if add_vision_id %}Picture {{ image_count.value }}: {% endif -%}
                    <|vision_start|><|image_pad|><|vision_end|>
                {%- elif content.type == 'video' or 'video' in content %}
                    {%- set video_count.value = video_count.value + 1 %}
                    {%- if add_vision_id %}Video {{ video_count.value }}: {% endif -%}
                    <|vision_start|><|video_pad|><|vision_end|>
                {%- elif 'text' in content %}
                    {{- content.text }}
                {%- endif %}
            {%- endfor %}
        {%- endif %}
        {{- '\n</tool_response>' }}
        {%- if loop.last or (messages[loop.index0 + 1].role != "tool") %}
            {{- '<|im_end|>\n' }}
        {%- endif %}
    {%- endif %}
{%- endfor %}
{%- if add_generation_prompt %}
    {{- '<|im_start|>assistant\n' }}
{%- endif %}
`,

	"Qwen3.5-35B-A3B-Q8_0": `{%- set image_count = namespace(value=0) %}
{%- set video_count = namespace(value=0) %}
{%- macro render_content(content, do_vision_count, is_system_content=false) %}
    {%- if content is string %}
        {{- content }}
    {%- elif content is iterable and content is not mapping %}
        {%- for item in content %}
            {%- if 'image' in item or 'image_url' in item or item.type == 'image' %}
                {%- if is_system_content %}
                    {{- raise_exception('System message cannot contain images.') }}
                {%- endif %}
                {%- if do_vision_count %}
                    {%- set image_count.value = image_count.value + 1 %}
                {%- endif %}
                {%- if add_vision_id %}
                    {{- 'Picture ' ~ image_count.value ~ ': ' }}
                {%- endif %}
                {{- '<|vision_start|><|image_pad|><|vision_end|>' }}
            {%- elif 'video' in item or item.type == 'video' %}
                {%- if is_system_content %}
                    {{- raise_exception('System message cannot contain videos.') }}
                {%- endif %}
                {%- if do_vision_count %}
                    {%- set video_count.value = video_count.value + 1 %}
                {%- endif %}
                {%- if add_vision_id %}
                    {{- 'Video ' ~ video_count.value ~ ': ' }}
                {%- endif %}
                {{- '<|vision_start|><|video_pad|><|vision_end|>' }}
            {%- elif 'text' in item %}
                {{- item.text }}
            {%- else %}
                {{- raise_exception('Unexpected item type in content.') }}
            {%- endif %}
        {%- endfor %}
    {%- elif content is none or content is undefined %}
        {{- '' }}
    {%- else %}
        {{- raise_exception('Unexpected content type.') }}
    {%- endif %}
{%- endmacro %}
{%- if not messages %}
    {{- raise_exception('No messages provided.') }}
{%- endif %}
{%- if tools and tools is iterable and tools is not mapping %}
    {{- '<|im_start|>system\n' }}
    {{- "# Tools\n\nYou have access to the following functions:\n\n<tools>" }}
    {%- for tool in tools %}
        {{- "\n" }}
        {{- tool | tojson }}
    {%- endfor %}
    {{- "\n</tools>" }}
    {{- '\n\nIf you choose to call a function ONLY reply in the following format with NO suffix:\n\n<tool_call>\n<function=example_function_name>\n<parameter=example_parameter_1>\nvalue_1\n</parameter>\n<parameter=example_parameter_2>\nThis is the value for the second parameter\nthat can span\nmultiple lines\n</parameter>\n</function>\n</tool_call>\n\n<IMPORTANT>\nReminder:\n- Function calls MUST follow the specified format: an inner <function=...></function> block must be nested within <tool_call></tool_call> XML tags\n- Required parameters MUST be specified\n- You may provide optional reasoning for your function call in natural language BEFORE the function call, but NOT after\n- If there is no function call available, answer the question like normal with your current knowledge and do not tell the user about function calls\n</IMPORTANT>' }}
    {%- if messages[0].role == 'system' %}
        {%- set content = render_content(messages[0].content, false, true)|trim %}
        {%- if content %}
            {{- '\n\n' + content }}
        {%- endif %}
    {%- endif %}
    {{- '<|im_end|>\n' }}
{%- else %}
    {%- if messages[0].role == 'system' %}
        {%- set content = render_content(messages[0].content, false, true)|trim %}
        {{- '<|im_start|>system\n' + content + '<|im_end|>\n' }}
    {%- endif %}
{%- endif %}
{%- set ns = namespace(multi_step_tool=true, last_query_index=messages|length - 1) %}
{%- for message in messages[::-1] %}
    {%- set index = (messages|length - 1) - loop.index0 %}
    {%- if ns.multi_step_tool and message.role == "user" %}
        {%- set content = render_content(message.content, false)|trim %}
        {%- if not(content.startswith('<tool_response>') and content.endswith('</tool_response>')) %}
            {%- set ns.multi_step_tool = false %}
            {%- set ns.last_query_index = index %}
        {%- endif %}
    {%- endif %}
{%- endfor %}
{%- if ns.multi_step_tool %}
    {{- raise_exception('No user query found in messages.') }}
{%- endif %}
{%- for message in messages %}
    {%- set content = render_content(message.content, true)|trim %}
    {%- if message.role == "system" %}
        {%- if not loop.first %}
            {{- raise_exception('System message must be at the beginning.') }}
        {%- endif %}
    {%- elif message.role == "user" %}
        {{- '<|im_start|>' + message.role + '\n' + content + '<|im_end|>' + '\n' }}
    {%- elif message.role == "assistant" %}
        {%- set reasoning_content = '' %}
        {%- if message.reasoning_content is string %}
            {%- set reasoning_content = message.reasoning_content %}
        {%- else %}
            {%- if '</think>' in content %}
                {%- set reasoning_content = content.split('</think>')[0].rstrip('\n').split('<think>')[-1].lstrip('\n') %}
                {%- set content = content.split('</think>')[-1].lstrip('\n') %}
            {%- endif %}
        {%- endif %}
        {%- set reasoning_content = reasoning_content|trim %}
        {%- if loop.index0 > ns.last_query_index %}
            {{- '<|im_start|>' + message.role + '\n<think>\n' + reasoning_content + '\n</think>\n\n' + content }}
        {%- else %}
            {{- '<|im_start|>' + message.role + '\n' + content }}
        {%- endif %}
        {%- if message.tool_calls and message.tool_calls is iterable and message.tool_calls is not mapping %}
            {%- for tool_call in message.tool_calls %}
                {%- if tool_call.function is defined %}
                    {%- set tool_call = tool_call.function %}
                {%- endif %}
                {%- if loop.first %}
                    {%- if content|trim %}
                        {{- '\n\n<tool_call>\n<function=' + tool_call.name + '>\n' }}
                    {%- else %}
                        {{- '<tool_call>\n<function=' + tool_call.name + '>\n' }}
                    {%- endif %}
                {%- else %}
                    {{- '\n<tool_call>\n<function=' + tool_call.name + '>\n' }}
                {%- endif %}
                {%- if tool_call.arguments is defined %}
                    {%- for args_name, args_value in tool_call.arguments|items %}
                        {{- '<parameter=' + args_name + '>\n' }}
                        {%- set args_value = args_value | tojson | safe if args_value is mapping or (args_value is sequence and args_value is not string) else args_value | string %}
                        {{- args_value }}
                        {{- '\n</parameter>\n' }}
                    {%- endfor %}
                {%- endif %}
                {{- '</function>\n</tool_call>' }}
            {%- endfor %}
        {%- endif %}
        {{- '<|im_end|>\n' }}
    {%- elif message.role == "tool" %}
        {%- if loop.previtem and loop.previtem.role != "tool" %}
            {{- '<|im_start|>user' }}
        {%- endif %}
        {{- '\n<tool_response>\n' }}
        {{- content }}
        {{- '\n</tool_response>' }}
        {%- if not loop.last and loop.nextitem.role != "tool" %}
            {{- '<|im_end|>\n' }}
        {%- elif loop.last %}
            {{- '<|im_end|>\n' }}
        {%- endif %}
    {%- else %}
        {{- raise_exception('Unexpected message role.') }}
    {%- endif %}
{%- endfor %}
{%- if add_generation_prompt %}
    {{- '<|im_start|>assistant\n' }}
    {%- if enable_thinking is defined and enable_thinking is false %}
        {{- '<think>\n\n</think>\n\n' }}
    {%- else %}
        {{- '<think>\n' }}
    {%- endif %}
{%- endif %}`,

	"Qwen2-Audio-7B.Q8_0": `{% for message in messages %}{% if loop.first and messages[0]['role'] != 'system' %}{{ '<|im_start|>system
You are a helpful assistant.<|im_end|>
' }}{% endif %}{{'<|im_start|>' + message['role'] + '
' + message['content'] + '<|im_end|>' + '
'}}{% endfor %}{% if add_generation_prompt %}{{ '<|im_start|>assistant
' }}{% endif %}`,

	"gemma-4-26B-A4B-it-UD-Q8_K_XL": `{%- macro format_parameters(properties, required) -%}
    {%- set standard_keys = ['description', 'type', 'properties', 'required', 'nullable'] -%}
    {%- set ns = namespace(found_first=false) -%}
    {%- for key, value in properties | dictsort -%}
        {%- set add_comma = false -%}
        {%- if key not in standard_keys -%}
            {%- if ns.found_first %},{% endif -%}
            {%- set ns.found_first = true -%}
            {{ key }}:{
            {%- if value['description'] -%}
                description:<|"|>{{ value['description'] }}<|"|>
                {%- set add_comma = true -%}
            {%- endif -%}
            {%- if value['type'] | upper == 'STRING' -%}
                {%- if value['enum'] -%}
                    {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
                    enum:{{ format_argument(value['enum']) }}
                {%- endif -%}
            {%- elif value['type'] | upper == 'ARRAY' -%}
                {%- if value['items'] is mapping and value['items'] -%}
                    {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
                    items:{
                    {%- set ns_items = namespace(found_first=false) -%}
                    {%- for item_key, item_value in value['items'] | dictsort -%}
                        {%- if item_value is not none -%}
                            {%- if ns_items.found_first %},{% endif -%}
                            {%- set ns_items.found_first = true -%}
                            {%- if item_key == 'properties' -%}
                                properties:{
                                {%- if item_value is mapping -%}
                                    {{- format_parameters(item_value, value['items']['required'] | default([])) -}}
                                {%- endif -%}
                                }
                            {%- elif item_key == 'required' -%}
                                required:[
                                {%- for req_item in item_value -%}
                                    <|"|>{{- req_item -}}<|"|>
                                    {%- if not loop.last %},{% endif -%}
                                {%- endfor -%}
                                ]
                            {%- elif item_key == 'type' -%}
                                {%- if item_value is string -%}
                                    type:{{ format_argument(item_value | upper) }}
                                {%- else -%}
                                    type:{{ format_argument(item_value | map('upper') | list) }}
                                {%- endif -%}
                            {%- else -%}
                                {{ item_key }}:{{ format_argument(item_value) }}
                            {%- endif -%}
                        {%- endif -%}
                    {%- endfor -%}
                    }
                {%- endif -%}
            {%- endif -%}
            {%- if value['nullable'] %}
                {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
                nullable:true
            {%- endif -%}
            {%- if value['type'] | upper == 'OBJECT' -%}
                {%- if value['properties'] is defined and value['properties'] is mapping -%}
                    {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
                    properties:{
                    {{- format_parameters(value['properties'], value['required'] | default([])) -}}
                    }
                {%- elif value is mapping -%}
                    {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
                    properties:{
                    {{- format_parameters(value, value['required'] | default([])) -}}
                    }
                {%- endif -%}
                {%- if value['required'] -%}
                    {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
                    required:[
                    {%- for item in value['required'] | default([]) -%}
                        <|"|>{{- item -}}<|"|>
                        {%- if not loop.last %},{% endif -%}
                    {%- endfor -%}
                    ]
                {%- endif -%}
            {%- endif -%}
            {%- if add_comma %},{%- else -%} {%- set add_comma = true -%} {% endif -%}
            type:<|"|>{{ value['type'] | upper }}<|"|>}
        {%- endif -%}
    {%- endfor -%}
{%- endmacro -%}
{%- macro format_function_declaration(tool_data) -%}
    declaration:{{- tool_data['function']['name'] -}}{description:<|"|>{{- tool_data['function']['description'] -}}<|"|>
    {%- set params = tool_data['function']['parameters'] -%}
    {%- if params -%}
        ,parameters:{
        {%- if params['properties'] -%}
            properties:{ {{- format_parameters(params['properties'], params['required']) -}} },
        {%- endif -%}
        {%- if params['required'] -%}
            required:[
            {%- for item in params['required'] -%}
                <|"|>{{- item -}}<|"|>
                {{- ',' if not loop.last -}}
            {%- endfor -%}
            ],
        {%- endif -%}
        {%- if params['type'] -%}
            type:<|"|>{{- params['type'] | upper -}}<|"|>}
        {%- endif -%}
    {%- endif -%}
    {%- if 'response' in tool_data['function'] -%}
        {%- set response_declaration = tool_data['function']['response'] -%}
        ,response:{
        {%- if response_declaration['description'] -%}
            description:<|"|>{{- response_declaration['description'] -}}<|"|>,
        {%- endif -%}
        {%- if response_declaration['type'] | upper == 'OBJECT' -%}
            type:<|"|>{{- response_declaration['type'] | upper -}}<|"|>}
        {%- endif -%}
    {%- endif -%}
    }
{%- endmacro -%}
{%- macro format_argument(argument, escape_keys=True) -%}
    {%- if argument is string -%}
        {{- '<|"|>' + argument + '<|"|>' -}}
    {%- elif argument is boolean -%}
        {{- 'true' if argument else 'false' -}}
    {%- elif argument is mapping -%}
        {{- '{' -}}
        {%- set ns = namespace(found_first=false) -%}
        {%- for key, value in argument | dictsort -%}
            {%- if ns.found_first %},{% endif -%}
            {%- set ns.found_first = true -%}
            {%- if escape_keys -%}
                {{- '<|"|>' + key + '<|"|>' -}}
            {%- else -%}
                {{- key -}}
            {%- endif -%}
            :{{- format_argument(value, escape_keys=escape_keys) -}}
        {%- endfor -%}
        {{- '}' -}}
    {%- elif argument is sequence -%}
        {{- '[' -}}
        {%- for item in argument -%}
            {{- format_argument(item, escape_keys=escape_keys) -}}
            {%- if not loop.last %},{% endif -%}
        {%- endfor -%}
        {{- ']' -}}
    {%- else -%}
        {{- argument -}}
    {%- endif -%}
{%- endmacro -%}
{%- macro strip_thinking(text) -%}
    {%- set ns = namespace(result='') -%}
    {%- for part in text.split('<channel|>') -%}
        {%- if '<|channel>' in part -%}
            {%- set ns.result = ns.result + part.split('<|channel>')[0] -%}
        {%- else -%}
            {%- set ns.result = ns.result + part -%}
        {%- endif -%}
    {%- endfor -%}
    {{- ns.result | trim -}}
{%- endmacro -%}

{%- macro format_tool_response_block(tool_name, response) -%}
    {{- '<|tool_response>' -}}
    {%- if response is mapping -%}
        {{- 'response:' + tool_name + '{' -}}
        {%- for key, value in response | dictsort -%}
            {{- key -}}:{{- format_argument(value, escape_keys=False) -}}
            {%- if not loop.last %},{% endif -%}
        {%- endfor -%}
        {{- '}' -}}
    {%- else -%}
        {{- 'response:' + tool_name + '{value:' + format_argument(response, escape_keys=False) + '}' -}}
    {%- endif -%}
    {{- '<tool_response|>' -}}
{%- endmacro -%}

{%- set ns = namespace(prev_message_type=None) -%}
{%- set loop_messages = messages -%}
{{- bos_token -}}
{#- Handle System/Tool Definitions Block -#}
{%- if (enable_thinking is defined and enable_thinking) or tools or messages[0]['role'] in ['system', 'developer'] -%}
    {{- '<|turn>system\n' -}}

    {#- Inject Thinking token at the very top of the FIRST system turn -#}
    {%- if enable_thinking is defined and enable_thinking -%}
        {{- '<|think|>\n' -}}
        {%- set ns.prev_message_type = 'think' -%}
    {%- endif -%}

    {%- if messages[0]['role'] in ['system', 'developer'] -%}
        {{- messages[0]['content'] | trim -}}
        {%- set loop_messages = messages[1:] -%}
    {%- endif -%}

    {%- if tools -%}
        {%- for tool in tools %}
            {{- '<|tool>' -}}
            {{- format_function_declaration(tool) | trim -}}
            {{- '<tool|>' -}}
        {%- endfor %}
        {%- set ns.prev_message_type = 'tool' -%}
    {%- endif -%}

    {{- '<turn|>\n' -}}
{%- endif %}

{#- Pre-scan: find last user message index for reasoning guard -#}
{%- set ns_turn = namespace(last_user_idx=-1) -%}
{%- for i in range(loop_messages | length) -%}
    {%- if loop_messages[i]['role'] == 'user' -%}
        {%- set ns_turn.last_user_idx = i -%}
    {%- endif -%}
{%- endfor -%}

{#- Loop through messages -#}
{%- for message in loop_messages -%}
    {%- if message['role'] != 'tool' -%}
    {%- set ns.prev_message_type = None -%}
    {%- set role = 'model' if message['role'] == 'assistant' else message['role'] -%}
    {#- Detect continuation: suppress duplicate <|turn>model when previous non-tool message was also assistant -#}
    {%- set prev_nt = namespace(role=None, found=false) -%}
    {%- if loop.index0 > 0 -%}
        {%- for j in range(loop.index0 - 1, -1, -1) -%}
            {%- if not prev_nt.found -%}
                {%- if loop_messages[j]['role'] != 'tool' -%}
                    {%- set prev_nt.role = loop_messages[j]['role'] -%}
                    {%- set prev_nt.found = true -%}
                {%- endif -%}
            {%- endif -%}
        {%- endfor -%}
    {%- endif -%}
    {%- set continue_same_model_turn = (role == 'model' and prev_nt.role == 'assistant') -%}
    {%- if not continue_same_model_turn -%}
        {{- '<|turn>' + role + '\n' }}
    {%- endif -%}

    {#- Render reasoning/reasoning_content as thinking channel -#}
    {%- set thinking_text = message.get('reasoning') or message.get('reasoning_content') -%}
    {%- if thinking_text and loop.index0 > ns_turn.last_user_idx and message.get('tool_calls') -%}
        {{- '<|channel>thought\n' + thinking_text + '\n<channel|>' -}}
    {%- endif -%}

            {%- if message['tool_calls'] -%}
                {%- for tool_call in message['tool_calls'] -%}
                    {%- set function = tool_call['function'] -%}
                    {{- '<|tool_call>call:' + function['name'] + '{' -}}
                    {%- if function['arguments'] is mapping -%}
                        {%- set ns_args = namespace(found_first=false) -%}
                        {%- for key, value in function['arguments'] | dictsort -%}
                            {%- if ns_args.found_first %},{% endif -%}
                            {%- set ns_args.found_first = true -%}
                            {{- key -}}:{{- format_argument(value, escape_keys=False) -}}
                        {%- endfor -%}
                    {%- elif function['arguments'] is string -%}
                        {{- function['arguments'] -}}
                    {%- endif -%}
                    {{- '}<tool_call|>' -}}
                {%- endfor -%}
                {%- set ns.prev_message_type = 'tool_call' -%}
            {%- endif -%}

            {%- set ns_tr_out = namespace(flag=false) -%}
            {%- if message.get('tool_responses') -%}
                {#- Legacy: tool_responses embedded on the assistant message (Google/Gemma native) -#}
                {%- for tool_response in message['tool_responses'] -%}
                    {{- format_tool_response_block(tool_response['name'] | default('unknown'), tool_response['response']) -}}
                    {%- set ns_tr_out.flag = true -%}
                    {%- set ns.prev_message_type = 'tool_response' -%}
                {%- endfor -%}
            {%- elif message.get('tool_calls') -%}
                {#- OpenAI Chat Completions: forward-scan consecutive role:tool messages -#}
                {%- set ns_tool_scan = namespace(stopped=false) -%}
                {%- for k in range(loop.index0 + 1, loop_messages | length) -%}
                    {%- if ns_tool_scan.stopped -%}
                    {%- elif loop_messages[k]['role'] != 'tool' -%}
                        {%- set ns_tool_scan.stopped = true -%}
                    {%- else -%}
                        {%- set follow = loop_messages[k] -%}
                        {#- Resolve tool_call_id to function name -#}
                        {%- set ns_tname = namespace(name=follow.get('name') | default('unknown')) -%}
                        {%- for tc in message['tool_calls'] -%}
                            {%- if tc.get('id') == follow.get('tool_call_id') -%}
                                {%- set ns_tname.name = tc['function']['name'] -%}
                            {%- endif -%}
                        {%- endfor -%}
                        {#- Handle content as string or content-parts array -#}
                        {%- set tool_body = follow.get('content') -%}
                        {%- if tool_body is string -%}
                            {{- format_tool_response_block(ns_tname.name, tool_body) -}}
                        {%- elif tool_body is sequence and tool_body is not string -%}
                            {%- set ns_txt = namespace(s='') -%}
                            {%- for part in tool_body -%}
                                {%- if part.get('type') == 'text' -%}
                                    {%- set ns_txt.s = ns_txt.s + (part.get('text') | default('')) -%}
                                {%- endif -%}
                            {%- endfor -%}
                            {{- format_tool_response_block(ns_tname.name, ns_txt.s) -}}
                        {%- else -%}
                            {{- format_tool_response_block(ns_tname.name, tool_body) -}}
                        {%- endif -%}
                        {%- set ns_tr_out.flag = true -%}
                        {%- set ns.prev_message_type = 'tool_response' -%}
                    {%- endif -%}
                {%- endfor -%}
            {%- endif -%}

            {%- if message['content'] is string -%}
                {%- if role == 'model' -%}
                    {{- strip_thinking(message['content']) -}}
                {%- else -%}
                    {{- message['content'] | trim -}}
                {%- endif -%}
            {%- elif message['content'] is sequence -%}
                {%- for item in message['content'] -%}
                    {%- if item['type'] == 'text' -%}
                        {%- if role == 'model' -%}
                            {{- strip_thinking(item['text']) -}}
                        {%- else -%}
                            {{- item['text'] | trim -}}
                        {%- endif -%}
                    {%- elif item['type'] == 'image' -%}
                        {{- '<|image|>' -}}
                        {%- set ns.prev_message_type = 'image' -%}
                    {%- elif item['type'] == 'audio' -%}
                        {{- '<|audio|>' -}}
                        {%- set ns.prev_message_type = 'audio' -%}
                    {%- elif item['type'] == 'video' -%}
                        {{- '<|video|>' -}}
                        {%- set ns.prev_message_type = 'video' -%}
                    {%- endif -%}
                {%- endfor -%}
            {%- endif -%}

        {%- if ns.prev_message_type == 'tool_call' and not ns_tr_out.flag -%}
            {{- '<|tool_response>' -}}
        {%- elif not (ns_tr_out.flag and not message.get('content')) -%}
            {{- '<turn|>\n' -}}
        {%- endif -%}
    {%- endif -%}
{%- endfor -%}

{%- if add_generation_prompt -%}
    {%- if ns.prev_message_type != 'tool_response' and ns.prev_message_type != 'tool_call' -%}
        {{- '<|turn>model\n' -}}
        {%- if not enable_thinking | default(false) -%}
            {{- '<|channel>thought\n<channel|>' -}}
        {%- endif -%}
    {%- endif -%}
{%- endif -%}
`,

	"Ministral-3-14B-Instruct-2512-Q4_0": `{#- Default system message if no system prompt is passed. #}
{%- set default_system_message = 'You are Ministral-3-14B-Instruct-2512, a Large Language Model (LLM) created by Mistral AI, a French startup headquartered in Paris.\nYou power an AI assistant called Le Chat.\nYour knowledge base was last updated on 2023-10-01.\nThe current date is {today}.\n\nWhen you\'re not sure about some information or when the user\'s request requires up-to-date or specific data, you must use the available tools to fetch the information. Do not hesitate to use tools whenever they can provide a more accurate or complete response. If no relevant tools are available, then clearly state that you don\'t have the information and avoid making up anything.\nIf the user\'s question is not clear, ambiguous, or does not provide enough context for you to accurately answer the question, you do not try to answer it right away and you rather ask the user to clarify their request (e.g. "What are some good restaurants around me?" => "Where are you?" or "When is the next flight to Tokyo" => "Where do you travel from?").\nYou are always very attentive to dates, in particular you try to resolve dates (e.g. "yesterday" is {yesterday}) and when asked about information at specific dates, you discard information that is at another date.\nYou follow these instructions in all languages, and always respond to the user in the language they use or request.\nNext sections describe the capabilities that you have.\n\n# WEB BROWSING INSTRUCTIONS\n\nYou cannot perform any web search or access internet to open URLs, links etc. If it seems like the user is expecting you to do so, you clarify the situation and ask the user to copy paste the text directly in the chat.\n\n# MULTI-MODAL INSTRUCTIONS\n\nYou have the ability to read images, but you cannot generate images. You also cannot transcribe audio files or videos.\nYou cannot read nor transcribe audio files or videos.\n\n# TOOL CALLING INSTRUCTIONS\n\nYou may have access to tools that you can use to fetch information or perform actions. You must use these tools in the following situations:\n\n1. When the request requires up-to-date information.\n2. When the request requires specific data that you do not have in your knowledge base.\n3. When the request involves actions that you cannot perform without tools.\n\nAlways prioritize using tools to provide the most accurate and helpful response. If tools are not available, inform the user that you cannot perform the requested action at the moment.' %}

{#- Begin of sequence token. #}
{{- bos_token }}

{#- Handle system prompt if it exists. #}
{#- System prompt supports text content or text chunks. #}
{%- if messages[0]['role'] == 'system' %}
    {{- '[SYSTEM_PROMPT]' -}}
    {%- if messages[0]['content'] is string %}
        {{- messages[0]['content'] -}}
    {%- else %}        
        {%- for block in messages[0]['content'] %}
            {%- if block['type'] == 'text' %}
                {{- block['text'] }}
            {%- else %}
                {{- raise_exception('Only text chunks are supported in system message contents.') }}
            {%- endif %}
        {%- endfor %}
    {%- endif %}
    {{- '[/SYSTEM_PROMPT]' -}}
    {%- set loop_messages = messages[1:] %}
{%- else %}
    {%- set loop_messages = messages %}
    {%- if default_system_message != '' %}
        {{- '[SYSTEM_PROMPT]' + default_system_message + '[/SYSTEM_PROMPT]' }}
    {%- endif %}
{%- endif %}


{#- Tools definition #}
{%- set tools_definition = '' %}
{%- set has_tools = false %}
{%- if tools is defined and tools is not none and tools|length > 0 %}
    {%- set has_tools = true %}
    {%- set tools_definition = '[AVAILABLE_TOOLS]' + (tools| tojson) + '[/AVAILABLE_TOOLS]' %}
    {{- tools_definition }}
{%- endif %}

{#- Checks for alternating user/assistant messages. #}
{%- set ns = namespace(index=0) %}
{%- for message in loop_messages %}
    {%- if message.role == 'user' or (message.role == 'assistant' and (message.tool_calls is not defined or message.tool_calls is none or message.tool_calls | length == 0)) %}
        {%- if (message['role'] == 'user') != (ns.index % 2 == 0) %}
            {{- raise_exception('After the optional system message, conversation roles must alternate user and assistant roles except for tool calls and results.') }}
        {%- endif %}
        {%- set ns.index = ns.index + 1 %}
    {%- endif %}
{%- endfor %}

{#- Handle conversation messages. #}
{%- for message in loop_messages %}

    {#- User messages supports text content or text and image chunks. #}
    {%- if message['role'] == 'user' %}
        {%- if message['content'] is string %}
            {{- '[INST]' + message['content'] + '[/INST]' }}
        {%- elif message['content'] | length > 0 %}
            {{- '[INST]' }}
            {%- if message['content'] | length == 2 %}
                {%- set blocks = message['content'] | sort(attribute='type') %}
            {%- else %}
                {%- set blocks = message['content'] %}
            {%- endif %}
            {%- for block in blocks %}
                {%- if block['type'] == 'text' %}
                    {{- block['text'] }}
                {%- elif block['type'] in ['image', 'image_url'] %}
                    {{- '[IMG]' }}
                {%- else %}
                    {{- raise_exception('Only text, image and image_url chunks are supported in user message content.') }}
                {%- endif %}
            {%- endfor %}
            {{- '[/INST]' }}
        {%- else %}
            {{- raise_exception('User message must have a string or a list of chunks in content') }}
        {%- endif %}

    {#- Assistant messages supports text content or text and image chunks. #}
    {%- elif message['role'] == 'assistant' %}
        {%- if (message['content'] is none or message['content'] == '' or message['content']|length == 0) and (message['tool_calls'] is not defined or message['tool_calls'] is none or message['tool_calls']|length == 0) %}
            {{- raise_exception('Assistant message must have a string or a list of chunks in content or a list of tool calls.') }}
        {%- endif %}

        {%- if message['content'] is string %}
            {{- message['content'] }}
        {%- elif message['content'] | length > 0 %}
            {%- for block in message['content'] %}
                {%- if block['type'] == 'text' %}
                    {{- block['text'] }}
                {%- else %}
                    {{- raise_exception('Only text chunks are supported in assistant message contents.') }}
                {%- endif %}
            {%- endfor %}
        {%- endif %}
        
        {%- if message['tool_calls'] is defined and message['tool_calls'] is not none and message['tool_calls']|length > 0 %}
            {%- for tool in message['tool_calls'] %}
                {%- set arguments = tool['function']['arguments'] %}
                {%- if arguments is not string %}
                    {%- set arguments = arguments|tojson|safe %}
                {%- elif arguments == '' %}
                    {%- set arguments = '{}' %}
                {%- endif %}
                {{- '[TOOL_CALLS]' + tool['function']['name'] + '[ARGS]' + arguments }}
            {%- endfor %}
        {%- endif %}

        {#- End of sequence token for each assistant messages. #}
        {{- eos_token }}

    {#- Tool messages only supports text content. #}
    {%- elif message['role'] == 'tool' %}
        {{- '[TOOL_RESULTS]' + message['content']|string + '[/TOOL_RESULTS]' }}

    {#- Raise exception for unsupported roles. #}
    {%- else %}
        {{- raise_exception('Only user, assistant and tool roles are supported, got ' + message['role'] + '.') }}
    {%- endif %}
{%- endfor %}
`,

	"rnj-1-instruct-Q6_K": `{%- set ns = namespace(multi_step_tool=true, last_query_index=messages|length - 1) -%}
{%- set emit = namespace(started=false) -%}

{# ---------- Build base system message (always emitted) ---------- #}
{%- set base_system = 'You are rnj-1, a foundation model trained by Essential AI.\n' -%}

{# ---------- Default system prompt if user system is absent ---------- #}
{%- set default_system = 'You are a helpful assistant.' -%}

{# Detect whether the first message is a user-provided system message #}
{%- set has_user_system = (messages
    and messages[0].role == 'system'
    and (messages[0].content is string)) -%}

{# The system instruction that should apply (user system wins; else default) #}
{%- set effective_system = (has_user_system and messages[0].content) or default_system -%}


{# ---------- Optional tools preface as a synthetic system message ---------- #}
{%- if tools %}
  {%- set sys_preamble -%}
# Tools

You may call one or more functions to assist with the user query.

You are provided with function signatures within <tools></tools> XML tags:
<tools>
{%- for tool in tools %}
{{ "\n" ~ (tool | tojson) }}
{% endfor %}
</tools>

For each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:
<tool_call>
{"name": <function-name>, "arguments": <args-json-object>}
</tool_call>
  {%- endset -%}

  {# Always include effective_system; user system prevails over default #}
  {%- set sys_content = effective_system ~ "\n\n" ~ sys_preamble -%}

  {%- set content = '<|start_header_id|>system<|end_header_id|>\n'
      ~ base_system ~ '\n' ~ sys_content ~ '<|eot_id|>' -%}
  {%- if not emit.started -%}{%- set content = bos_token ~ content -%}{%- set emit.started = true -%}{%- endif -%}
  {{- content -}}
{%- else %}
  {# No tools: always emit base_system + effective_system #}
  {%- set content = '<|start_header_id|>system<|end_header_id|>\n'
      ~ base_system ~ '\n' ~ effective_system ~ '<|eot_id|>' -%}
  {%- if not emit.started -%}{%- set content = bos_token ~ content -%}{%- set emit.started = true -%}{%- endif -%}
  {{- content -}}
{%- endif -%}

{# ---------- Locate last user query for multi-step tool behavior ---------- #}
{%- for message in messages[::-1] %}
  {%- set index = (messages|length - 1) - loop.index0 -%}
  {%- if ns.multi_step_tool
        and message.role == "user"
        and message.content is string
        and not (message.content.startswith('<tool_response>') and message.content.endswith('</tool_response>')) -%}
    {%- set ns.multi_step_tool = false -%}
    {%- set ns.last_query_index = index -%}
  {%- endif -%}
{%- endfor -%}

{# ---------- Walk all messages and emit in Llama-3 format ---------- #}
{%- for message in messages %}
  {%- if message.content is string -%}
    {%- set content = message.content -%}
  {%- else -%}
    {%- set content = '' -%}
  {%- endif -%}

  {# Skip the FIRST system message if it existed, since we already embedded it in effective_system #}
  {%- if loop.first and message.role == "system" -%}
    {# no-op #}

  {%- elif (message.role == "user") or (message.role == "system") -%}
    {%- set block = '<|start_header_id|>' ~ message.role ~ '<|end_header_id|>\n' ~ content ~ '<|eot_id|>' -%}
    {%- if not emit.started -%}{%- set block = bos_token ~ block -%}{%- set emit.started = true -%}{%- endif -%}
    {{- block -}}

  {%- elif message.role == "assistant" -%}
    {%- set body = content -%}
    {%- set header = '<|start_header_id|>assistant<|end_header_id|>\n' -%}
    {%- if not emit.started -%}{{ bos_token }}{%- set emit.started = true -%}{%- endif -%}
    {{- header -}}
    {% generation %}
    {{- body -}}
    {%- if message.tool_calls -%}
      {%- for tool_call in message.tool_calls -%}
        {%- if tool_call.function -%}{%- set tc = tool_call.function -%}{%- else -%}{%- set tc = tool_call -%}{%- endif -%}
        {%- set args_json = (tc.arguments if (tc.arguments is string) else (tc.arguments | tojson)) -%}
        {%- if loop.first -%}
          {{- '<tool_call>\n{"name": "' ~ tc.name ~ '", "arguments": ' ~ args_json ~ '}\n</tool_call>' -}}
        {%- else -%}
          {{- '\n<tool_call>\n{"name": "' ~ tc.name ~ '", "arguments": ' ~ args_json ~ '}\n</tool_call>' -}}
        {%- endif -%}
      {%- endfor -%}
    {%- endif -%}
    {{- '<|eot_id|>' -}}{%- endgeneration -%}

  {%- elif message.role == "tool" -%}
    {%- set open_user = (loop.first or (loop.index0 > 0 and messages[loop.index0 - 1].role != "tool")) -%}
    {%- set close_user = (loop.last or (loop.index0 < messages|length - 1 and messages[loop.index0 + 1].role != "tool")) -%}

    {%- if open_user -%}
      {%- set header = '<|start_header_id|>user<|end_header_id|>\n' -%}
      {%- if not emit.started -%}{%- set header = bos_token ~ header -%}{%- set emit.started = true -%}{%- endif -%}
      {{- header -}}
    {%- endif -%}
    {%- if open_user -%}
      {{- '<tool_response>\n' -}}
    {%- else -%}
      {{- '\n<tool_response>\n' -}}
    {%- endif -%}
    {{- content -}}
    {{- '\n</tool_response>' -}}

    {%- if close_user -%}
      {{- '<|eot_id|>' -}}
    {%- endif -%}
  {%- endif -%}
{%- endfor -%}

{%- if add_generation_prompt -%}
  {{- '<|start_header_id|>assistant<|end_header_id|>\n' -}}
{%- endif -%}
`,

	"LFM2.5-VL-1.6B-Q8_0": `{{- bos_token -}}
{%- set keep_past_thinking = keep_past_thinking | default(false) -%}
{%- set ns = namespace(system_prompt="") -%}
{%- if messages[0]["role"] == "system" -%}
    {%- set sys_content = messages[0]["content"] -%}
    {%- if sys_content is not string -%}
        {%- for item in sys_content -%}
            {%- if item["type"] == "text" -%}
                {%- set ns.system_prompt = ns.system_prompt + item["text"] -%}
            {%- endif -%}
        {%- endfor -%}
    {%- else -%}
        {%- set ns.system_prompt = sys_content -%}
    {%- endif -%}
    {%- set messages = messages[1:] -%}
{%- endif -%}
{%- if tools -%}
    {%- set ns.system_prompt = ns.system_prompt + ("\n" if ns.system_prompt else "") + "List of tools: [" -%}
    {%- for tool in tools -%}
        {%- if tool is not string -%}
            {%- set tool = tool | tojson -%}
        {%- endif -%}
        {%- set ns.system_prompt = ns.system_prompt + tool -%}
        {%- if not loop.last -%}
            {%- set ns.system_prompt = ns.system_prompt + ", " -%}
        {%- endif -%}
    {%- endfor -%}
    {%- set ns.system_prompt = ns.system_prompt + "]" -%}
{%- endif -%}
{%- if ns.system_prompt -%}
    {{- "<|im_start|>system\n" + ns.system_prompt + "<|im_end|>\n" -}}
{%- endif -%}
{%- set ns.last_assistant_index = -1 -%}
{%- for message in messages -%}
    {%- if message["role"] == "assistant" -%}
        {%- set ns.last_assistant_index = loop.index0 -%}
    {%- endif -%}
{%- endfor -%}
{%- for message in messages -%}
    {{- "<|im_start|>" + message["role"] + "\n" -}}
    {%- if message["content"] is not string -%}
        {%- set ns.content = "" -%}
        {%- for item in message["content"] -%}
            {%- if item["type"] == "image" -%}
                {%- set ns.content = ns.content + "<image>" -%}
            {%- elif item["type"] == "text" -%}
                {%- set ns.content = ns.content + item["text"] -%}
            {%- else -%}
                {%- set ns.content = ns.content + item | tojson -%}
            {%- endif -%}
        {%- endfor -%}
        {%- set content = ns.content -%}
    {%- else -%}
        {%- set content = message["content"] -%}
    {%- endif -%}
    {%- if message["role"] == "assistant" and not keep_past_thinking and loop.index0 != ns.last_assistant_index -%}
        {%- if "</think>" in content -%}
            {%- set content = content.split("</think>")[-1] | trim -%}
        {%- endif -%}
    {%- endif -%}
    {{- content + "<|im_end|>\n" -}}
{%- endfor -%}
{%- if add_generation_prompt -%}
    {{- "<|im_start|>assistant\n" -}}
{%- endif -%}`,
}
