package jinja_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ardanlabs/jinja"
)

// TestRequestsAgainstTemplates renders every chat template under
// testdata/templates against every captured request body under
// testdata/requests and compares this engine's output to the canonical
// HuggingFace Jinja2 implementation byte-for-byte.
//
// The reference output is produced on demand by scripts/render_python.py,
// which uses the same `jinja2.sandbox.ImmutableSandboxedEnvironment` setup
// HuggingFace's `transformers.PreTrainedTokenizerBase.apply_chat_template`
// uses. There are no checked-in golden files: adding a template (drop it in
// testdata/templates/) or a captured request (drop it in testdata/requests/)
// just makes the next `go test` run pick it up automatically.
//
// The script is launched via `uv` so no Python venv setup is required. If `uv`
// is not on PATH the test is skipped with an explanatory message.
func TestRequestsAgainstTemplates(t *testing.T) {
	reference, err := pythonReference()
	if err != nil {
		if errors.Is(err, errUVMissing) {
			t.Skipf("skipping: `uv` not found on PATH (needed to run scripts/render_python.py for the reference Jinja2 output)")
		}
		t.Fatalf("could not obtain Python reference output: %v", err)
	}

	requests := loadRequests(t)
	reqNames := sortedKeys(requests)

	for _, templateName := range sortedTemplateNames() {
		content := chatTemplates[templateName]
		t.Run(templateName, func(t *testing.T) {
			tmpl, err := jinja.Compile(content)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}

			for _, reqName := range reqNames {
				req := requests[reqName]
				t.Run(reqName, func(t *testing.T) {
					data := buildRenderData(req)

					var got string
					result, err := tmpl.Render(data)
					if err != nil {
						got = "RENDER ERROR: " + err.Error() + "\n"
					} else {
						got = result
					}

					key := templateName + "/" + reqName
					want, ok := reference[key]
					if !ok {
						t.Fatalf("scripts/render_python.py did not produce output for key %q (template/request mismatch)", key)
					}

					if got != want {
						t.Errorf("rendered output diverges from canonical Python jinja2\n%s", diffSummary(got, want))
					}
				})
			}
		})
	}
}

// buildRenderData turns a captured request body into the variable map the chat
// template expects. The known chat templates in this repo reference messages,
// tools, and a handful of well-known control variables (bos_token,
// add_generation_prompt, enable_thinking, ...). We provide neutral defaults
// for those so every template can render without panicking on a missing name.
//
// IMPORTANT: this map must stay in sync with build_context() in
// scripts/render_python.py — both engines must see the exact same context.
func buildRenderData(req map[string]any) map[string]any {
	return map[string]any{
		"messages":              req["messages"],
		"tools":                 req["tools"],
		"add_generation_prompt": true,
		"bos_token":             "<s>",
		"eos_token":             "</s>",
		"enable_thinking":       false,
		"thinking":              false,
		"reasoning_effort":      "medium",
		"model_identity":        "You are a helpful assistant.",
		"builtin_tools":         []any{},
		"date":                  "2024-01-01",
		"date_string":           "2024-01-01",
		"tools_in_user_message": false,
	}
}

var errUVMissing = errors.New("uv binary not found")

// pythonReference invokes scripts/render_python.py via `uv` and returns its
// {key -> rendered text} map, where key is "<template>/<request>".
func pythonReference() (map[string]string, error) {
	if _, err := exec.LookPath("uv"); err != nil {
		return nil, errUVMissing
	}

	scriptPath := filepath.Join("scripts", "render_python.py")
	cmd := exec.Command("uv", "run", "--quiet", "--with", "jinja2>=3.1", "python", scriptPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s failed: %v\nstderr:\n%s", scriptPath, err, stderr.String())
	}

	var ref map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &ref); err != nil {
		return nil, fmt.Errorf("decoding %s output: %v\nfirst bytes: %q", scriptPath, err, head(stdout.Bytes(), 200))
	}
	return ref, nil
}

func head(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

var requestFiles map[string]map[string]any

func loadRequests(t testing.TB) map[string]map[string]any {
	if requestFiles != nil {
		return requestFiles
	}

	files, err := filepath.Glob("testdata/requests/*.json")
	if err != nil {
		t.Fatalf("glob requests: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no request files found in testdata/requests")
	}

	requestFiles = make(map[string]map[string]any, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %q: %v", f, err)
		}

		// Use json.Decoder with UseNumber so that JSON integers stay
		// distinguishable from floats — Python's json module preserves
		// that distinction and we need it to render large integers
		// (e.g. 9007199254740991) the same way the canonical engine does.
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()

		var req map[string]any
		if err := dec.Decode(&req); err != nil {
			t.Fatalf("unmarshal %q: %v", f, err)
		}
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		requestFiles[name] = req
	}
	return requestFiles
}

func sortedKeys(m map[string]map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTemplateNames() []string {
	keys := make([]string, 0, len(chatTemplates))
	for k := range chatTemplates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// diffSummary returns a compact, human-readable description of where two
// strings first differ. Rendered chat templates can be many KB long with
// extremely long single lines (e.g. tool-definition JSON), so dumping the full
// strings is unhelpful. Instead, we report sizes, the line/column of the first
// divergence, and a small character window around it.
func diffSummary(got, want string) string {
	if got == want {
		return "(no diff)"
	}

	n := min(len(want), len(got))
	i := 0
	for i < n && got[i] == want[i] {
		i++
	}

	line, col := lineCol(got, i)

	const window = 60
	gotCtx := contextAround(got, i, window)
	wantCtx := contextAround(want, i, window)

	return fmt.Sprintf(
		"first difference at byte %d (line %d, col %d); got=%dB want=%dB\n  got:  %q\n  want: %q",
		i, line, col, len(got), len(want), gotCtx, wantCtx,
	)
}

func contextAround(s string, i, window int) string {
	start := max(i-window, 0)
	end := min(i+window, len(s))
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	suffix := ""
	if end < len(s) {
		suffix = "…"
	}
	return prefix + s[start:end] + suffix
}

func lineCol(s string, i int) (int, int) {
	line, col := 1, 1
	for j := 0; j < i && j < len(s); j++ {
		if s[j] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
