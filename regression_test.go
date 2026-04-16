package jinja

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegression(t *testing.T) {
	files, err := filepath.Glob("testdata/templates/*.jinja")
	if err != nil {
		t.Fatalf("can't read testdata/templates - %v", err)
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("read - %v", err)
			}

			_, err = Compile(string(data))
			if err != nil {
				t.Fatalf("compile - %v", err)
			}
		})
	}
}
