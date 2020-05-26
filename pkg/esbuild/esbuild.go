package esbuild

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/progrium/esbuild/pkg/ast"
	"github.com/progrium/esbuild/pkg/bundler"
	"github.com/progrium/esbuild/pkg/fs"
	"github.com/progrium/esbuild/pkg/logging"
	"github.com/progrium/esbuild/pkg/parser"
	"github.com/progrium/esbuild/pkg/resolver"
	"github.com/spf13/afero"
)

var JsxFactory = "m"

func jsxFactory() string {
	if os.Getenv("JSX_FACTORY") != "" {
		return os.Getenv("JSX_FACTORY")
	}
	return JsxFactory
}

func BuildFile(fs afero.Fs, filepath string) ([]byte, error) {
	parseOptions := parser.ParseOptions{
		Defines: make(map[string]ast.E),
		JSX: parser.JSXOptions{
			Factory: []string{jsxFactory()},
		},
	}
	bundleOptions := bundler.BundleOptions{}
	logOptions := logging.StderrOptions{
		IncludeSource:      true,
		ErrorLimit:         10,
		ExitWhenLimitIsHit: true,
	}

	wrapfs := &FS{fs}
	resolver := resolver.NewResolver(wrapfs, []string{".jsx", ".js", ".mjs"})
	logger, join := logging.NewStderrLog(logOptions)
	bundle := bundler.ScanBundle(logger, wrapfs, resolver, []string{filepath}, parseOptions)
	if join().Errors != 0 {
		log.Println("[WARNING] ScanBundle failed")
		return nil, nil
	}
	result := bundle.Compile(logger, bundleOptions)

	for _, item := range result {
		if strings.Contains(item.JsAbsPath+"x", filepath) {
			return item.JsContents, nil
		}
	}
	return nil, fmt.Errorf("no result from esbuild")
}

type FS struct {
	afero.Fs
}

func (f *FS) ReadDirectory(path string) map[string]fs.Entry {
	dir, err := afero.ReadDir(f, path)
	if err != nil {
		return map[string]fs.Entry{}
	}
	m := make(map[string]fs.Entry)
	for _, fi := range dir {
		if fi.IsDir() {
			m[fi.Name()] = fs.DirEntry
		} else {
			m[fi.Name()] = fs.FileEntry
		}
	}
	return m
}

func (f *FS) ReadFile(path string) (string, bool) {
	buffer, err := afero.ReadFile(f, path)
	return string(buffer), err == nil
}

func (f *FS) Dir(path string) string {
	return filepath.Dir(path)
}

func (f *FS) Base(path string) string {
	return filepath.Base(path)
}

func (f *FS) Join(parts ...string) string {
	return filepath.Clean(filepath.Join(parts...))
}

func (f *FS) RelativeToCwd(path string) (string, bool) {
	return path, true
}
