package jinja

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"testing"
	"time"
)

type jinjaFixResult struct {
	ID     string `json:"id"`
	Stage  string `json:"stage"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func jinjaFixEvaluate(source string) (r jinjaFixResult) {
	defer func() {
		if p := recover(); p != nil {
			r = jinjaFixResult{Stage: "panic", Error: fmt.Sprint(p)}
		}
	}()
	template, err := Compile(source)
	if err != nil {
		return jinjaFixResult{Stage: "compile_error", Error: err.Error()}
	}
	output, err := template.Render(nil)
	if err != nil {
		return jinjaFixResult{Stage: "render_error", Error: err.Error()}
	}
	return jinjaFixResult{Stage: "render", Output: output}
}

// Child processes bound nontermination and stack exhaustion in broken revisions.
func TestJinjaFixWorker(t *testing.T) {
	if os.Getenv("JINJAFIX_WORKER") != "1" {
		return
	}
	debug.SetMaxStack(2 << 20)
	source, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	if json.NewEncoder(os.Stdout).Encode(jinjaFixEvaluate(string(source))) != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func jinjaFixReferenceMissing(found bool) bool {
	return !found
}

func TestJinjaFixRegression(t *testing.T) {
	var cases []struct {
		ID      string `json:"id"`
		Source  string `json:"source"`
		GoStage string `json:"go_stage,omitempty"`
		Reason  string `json:"reason,omitempty"`
	}
	var python struct {
		Cases []jinjaFixResult `json:"cases"`
	}
	read := func(path string, v any) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, v); err != nil {
			t.Fatal(err)
		}
	}
	read("testdata/jinjafix/cases.json", &cases)
	read("testdata/jinjafix/python-result.json", &python)
	expected := map[string]jinjaFixResult{}
	for _, r := range python.Cases {
		expected[r.ID] = r
	}
	var results []jinjaFixResult
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			want, ok := expected[c.ID]
			if jinjaFixReferenceMissing(ok) {
				t.Fatal("missing Python result")
			}
			if c.GoStage != "" {
				if c.Reason == "" {
					t.Fatal("undocumented parity exception")
				}
				want.Stage = c.GoStage
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJinjaFixWorker$")
			cmd.Env = append(os.Environ(), "JINJAFIX_WORKER=1", "GOMEMLIMIT=64MiB", "GOTRACEBACK=none")
			cmd.Stdin = bytes.NewBufferString(c.Source)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			var got jinjaFixResult
			switch {
			case ctx.Err() != nil:
				got = jinjaFixResult{Stage: "timeout", Error: ctx.Err().Error()}
			case err != nil:
				got = jinjaFixResult{Stage: "process_error", Error: err.Error()}
			default:
				if err = json.Unmarshal(stdout.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
			}
			got.ID = c.ID
			results = append(results, got)
			if jinjaFixResultsDiffer(got, want) {
				t.Errorf("Go: stage=%s output=%q; expected: stage=%s output=%q; error=%s", got.Stage, got.Output, want.Stage, want.Output, got.Error)
			}
		})
	}
	if path := os.Getenv("JINJAFIX_RESULT_PATH"); path != "" {
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func jinjaFixResultsDiffer(got, want jinjaFixResult) bool {
	if got.Stage != want.Stage {
		return true
	}
	return want.Stage == "render" && got.Output != want.Output
}
