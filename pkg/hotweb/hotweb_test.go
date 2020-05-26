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
	if err := afero.WriteFile(f, "/root/sub/exists", existFile, 0644); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(f, "/root/exists.js", existFile, 0644); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(f, "/root/html.jsx", srcFile, 0644); err != nil {
		t.Fatal(err)
	}

	hw := New(Config{
		Filesystem: f,
		ServeRoot:  "/root",
	})

	t.Run("existing file in subdir, no proxy", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/sub/exists", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		match := hw.MatchHTTP(req)
		if !match {
			t.Fatal("no match")
		}

		hw.ServeHTTP(rr, req)
		expected := string(existFile)
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

	t.Run("existing file, no proxy", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/exists.js?0", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		match := hw.MatchHTTP(req)
		if !match {
			t.Fatal("no match")
		}

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
		match := hw.MatchHTTP(req)
		if !match {
			t.Fatal("no match")
		}

		hw.ServeHTTP(rr, req)
		expected := "m(\"html\", null);\n"
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

	hwp := New(Config{
		Filesystem: f,
		ServeRoot:  "/root",
		Prefix:     "/prefix",
	})

	t.Run("existing file, no proxy, prefixed", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/prefix/exists.js?0", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		match := hwp.MatchHTTP(req)
		if !match {
			t.Fatal("no match")
		}

		hwp.ServeHTTP(rr, req)
		expected := string(existFile)
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

	t.Run("made file, no proxy, prefixed", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/prefix/html.js?0", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		match := hwp.MatchHTTP(req)
		if !match {
			t.Fatal("no match")
		}

		hwp.ServeHTTP(rr, req)
		expected := "m(\"html\", null);\n"
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

	t.Run("existing file in subdir, no proxy, prefixed", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/prefix/sub/exists", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		match := hwp.MatchHTTP(req)
		if !match {
			t.Fatal("no match")
		}

		hwp.ServeHTTP(rr, req)
		expected := string(existFile)
		if rr.Body.String() != expected {
			t.Errorf("got %v want %v", rr.Body.String(), expected)
		}
	})

}
