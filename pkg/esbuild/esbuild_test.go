package esbuild

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
)

const TestFile = "file.jsx"

func TestBuildFile(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{
			"const html = <html></html>;\n",
			"const html = m(\"html\", null);\n",
		},
	}
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("test%d", idx), func(t *testing.T) {
			fs := afero.NewMemMapFs()
			err := afero.WriteFile(fs, TestFile, []byte(tt.in), 0644)
			if err != nil {
				t.Fatal(err)
			}
			got, err := BuildFile(fs, TestFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.out {
				t.Errorf("got %q, want %q", got, tt.out)
			}
		})
	}
}
