package hotweb

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
)

func TestHotweb(t *testing.T) {
	existFile := []byte("foo")
	srcFile := []byte("<html></html>\n")
	f := afero.NewMemMapFs()
	if err := afero.WriteFile(f, "/root/exists.js", existFile, 0644); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(f, "/root/html.jsx", srcFile, 0644); err != nil {
		t.Fatal(err)
	}

	hw := New(f, "/root")

	t.Run("existing file, no proxy", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/exists.js?0", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		hw.ServeHTTP(rr, req)

		expected := string(existFile)
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

	t.Run("made file, no proxy", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/html.js?0", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		hw.ServeHTTP(rr, req)

		expected := "m(\"html\", null);\n"
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

}
