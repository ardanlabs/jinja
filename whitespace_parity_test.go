package jinja_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ardanlabs/jinja"
)

// jinjaResult is one entry of the output of scripts/render_snippets.py.
type jinjaResult struct {
	Out   *string `json:"out"`
	Error string  `json:"error"`
}

// jinjaOutput is the output of scripts/render_snippets.py.
type jinjaOutput struct {
	Results    []jinjaResult `json:"results"`
	Whitespace []rune        `json:"whitespace"`
}

// renderWithJinja2 renders templates through real Jinja2 via `uv`.
func renderWithJinja2(templates []string) (jinjaOutput, error) {
	if _, err := exec.LookPath("uv"); err != nil {
		return jinjaOutput{}, errUVMissing
	}

	input, err := json.Marshal(templates)
	if err != nil {
		return jinjaOutput{}, err
	}

	scriptPath := filepath.Join("scripts", "render_snippets.py")
	cmd := exec.Command("uv", "run", "--quiet", "--with", "jinja2>=3.1", "python", scriptPath)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return jinjaOutput{}, fmt.Errorf("running %s failed: %v\nstderr:\n%s", scriptPath, err, stderr.String())
	}

	var output jinjaOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return jinjaOutput{}, fmt.Errorf("decoding %s output: %v", scriptPath, err)
	}
	if len(output.Results) != len(templates) {
		return jinjaOutput{}, fmt.Errorf("%s returned %d results for %d templates", scriptPath, len(output.Results), len(templates))
	}
	return output, nil
}

// adversarialCase is one entry of testdata/whitespace/adversarial.json.
type adversarialCase struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

// knownGaps lists adversarial cases where the engine still differs from Jinja2,
// for reasons unrelated to whitespace control. TestWhitespaceParity fails if one
// of them starts to agree, so that the entry gets removed.
var knownGaps = map[string]string{
	"err-empty-var-minus-both": "an empty expression is accepted",
	"near-200b-in-var":         "the expression tokenizer skips characters it does not know",
	"set-capture-filter":       "a filter on a {% set %} block is not supported",
	"wsrun-in-block":           "U+001C to U+001F are not whitespace inside a statement",
	"ws-001c-in-tags":          "U+001C to U+001F are not whitespace inside a statement",
	"cmt-eof-open":             "Jinja2 accepts a comment opened at the very end of the source",
	"cmt-eof-open-minus":       "Jinja2 accepts a comment opened at the very end of the source",
	"raw-eof-bare":             "Jinja2 accepts a raw block opened at the very end of the source",
	"raw-eof-minus-swallows":   "Jinja2 accepts a raw block opened at the very end of the source",
}

// parityGrid returns the part of the grid compared with Jinja2. TestWhitespaceGrid
// runs all of it against the model, but Python is too slow for that, so this keeps
// every tag pair and middle text and a few outer texts that cover what can differ:
// nothing, whitespace with a newline, and text.
func parityGrid() []string {
	outers := [][2]string{
		{"", ""},
		{" \n\t ", " \n\t "},
		{"\n  ", "\n"},
		{"a ", " b"},
	}

	var templates []string
	for _, pair := range gridPairs() {
		for _, outer := range outers {
			for _, mid := range gridTexts {
				c := gridCase{pre: outer[0], first: pair[0], mid: mid, second: pair[1], post: outer[1]}
				templates = append(templates, c.template())
			}
		}
	}
	return templates
}

// TestWhitespaceParity renders the whitespace table, the grid and an adversarial
// corpus through both engines and compares them byte for byte. The reference comes
// from real Jinja2, so the test is skipped when `uv` is not on PATH.
func TestWhitespaceParity(t *testing.T) {
	var templates []string
	for _, tt := range whitespaceCases {
		templates = append(templates, tt.tmpl)
	}

	raw, err := os.ReadFile("testdata/whitespace/adversarial.json")
	if err != nil {
		t.Fatalf("read adversarial cases: %v", err)
	}
	var adversarial []adversarialCase
	if err := json.Unmarshal(raw, &adversarial); err != nil {
		t.Fatalf("decode adversarial cases: %v", err)
	}
	gaps := make(map[string]string, len(knownGaps))
	for _, c := range adversarial {
		if _, gap := knownGaps[c.Name]; gap {
			gaps[c.Template] = c.Name
		}
		templates = append(templates, c.Template)
	}
	if len(gaps) != len(knownGaps) {
		t.Errorf("knownGaps names %d cases but %d are in the corpus", len(knownGaps), len(gaps))
	}

	templates = append(templates, parityGrid()...)

	jinja2, err := renderWithJinja2(templates)
	if err != nil {
		if err == errUVMissing {
			t.Skipf("skipping: `uv` not found on PATH (needed to run scripts/render_snippets.py for the reference Jinja2 output)")
		}
		t.Fatalf("could not obtain Jinja2 output: %v", err)
	}

	if !slices.Equal(jinja2.Whitespace, pythonWhitespace) {
		t.Errorf("pythonWhitespace is out of date\n got: %x\nwant: %x", pythonWhitespace, jinja2.Whitespace)
	}

	const maxReported = 20
	failures := 0

	for i, source := range templates {
		problem := compareWithJinja2(source, jinja2.Results[i])

		if name, gap := gaps[source]; gap {
			if problem == "" {
				t.Errorf("%s now agrees with Jinja2: remove it from knownGaps", name)
			}
			continue
		}

		if problem == "" {
			continue
		}
		if failures++; failures <= maxReported {
			t.Errorf("template %q\n%s", source, problem)
		}
	}

	if failures > 0 {
		t.Errorf("%d of %d templates diverge from Jinja2", failures, len(templates))
	}
}

// compareWithJinja2 describes how the engine's handling of source differs from
// Jinja2's result, or returns "" if it does not.
func compareWithJinja2(source string, want jinjaResult) string {
	got, err := func() (string, error) {
		tmpl, err := jinja.Compile(source)
		if err != nil {
			return "", err
		}
		return tmpl.Render(whitespaceData())
	}()

	switch {
	case want.Out == nil && want.Error == "":
		return "Jinja2 returned neither output nor an error"
	case want.Out == nil && err == nil:
		return fmt.Sprintf("Jinja2 rejects it (%s) but this engine renders %q", want.Error, got)
	case want.Out == nil:
		return ""
	case err != nil:
		return fmt.Sprintf("this engine rejects it (%v) but Jinja2 renders %q", err, *want.Out)
	case got != *want.Out:
		return fmt.Sprintf(" got: %q\nwant: %q", got, *want.Out)
	}
	return ""
}

// glmGoldenRequest is a captured request with tools and a full history.
const glmGoldenRequest = "kronk-req-deepseek"

// renderGLM renders a request with the GLM-5.3-Flash chat
// template, which ships in the model's GGUF header. Its first line is
// `<|system|>Reasoning Effort: {{ effort | capitalize }}{%- endif -%}`, the shape
// that used to lose the space after the colon.
func renderGLM(t *testing.T, req map[string]any, reasoningEffort any) string {
	t.Helper()

	tmpl, err := jinja.Compile(chatTemplates["GLM-5.3-Flash"])
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	data := buildRenderData(req)
	if reasoningEffort == nil {
		delete(data, "reasoning_effort")
	} else {
		data["reasoning_effort"] = reasoningEffort
	}

	got, err := tmpl.Render(data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return got
}

func TestGLM53FlashReasoningEffort(t *testing.T) {
	req := map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "What is 2+2?"}},
	}

	tests := []struct {
		name   string
		effort any
		want   string
	}{
		{"unset defaults to max", nil, "Max"},
		{"unknown effort falls back to max", "medium", "Max"},
		{"low", "low", "Low"},
		{"high", "high", "High"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := "[gMASK]<sop><|system|>Reasoning Effort: " + tt.want + "<|user|>What is 2+2?<|assistant|><think>"
			if got := renderGLM(t, req, tt.effort); got != want {
				t.Errorf("\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

// TestGLM53FlashGolden pins the template on a request with tools and history at
// Reasoning Effort Max. The golden file was rendered by Jinja2 3.1.6 from the
// environment and context in scripts/render_python.py, and TestRequestsAgainstTemplates
// compares this template with live Jinja2 output on every captured request.
func TestGLM53FlashGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/golden/GLM-5.3-Flash/tools.rendered.txt")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	got := renderGLM(t, loadRequests(t)[glmGoldenRequest], nil)
	if got != string(want) {
		t.Errorf("rendered output diverges from Jinja2\n%s", diffSummary(got, string(want)))
	}
}
