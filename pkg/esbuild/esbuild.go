package esbuild

import (
	"fmt"
	"strings"

	"github.com/progrium/esbuild/pkg/ast"
	"github.com/progrium/esbuild/pkg/bundler"
	"github.com/progrium/esbuild/pkg/fs"
	"github.com/progrium/esbuild/pkg/logging"
	"github.com/progrium/esbuild/pkg/parser"
	"github.com/progrium/esbuild/pkg/resolver"
)

func BuildFile(filepath string) ([]byte, error) {
	parseOptions := parser.ParseOptions{
		Defines: make(map[string]ast.E),
		JSX: parser.JSXOptions{
			Factory: []string{"m"},
		},
	}
	bundleOptions := bundler.BundleOptions{}
	logOptions := logging.StderrOptions{
		IncludeSource:      true,
		ErrorLimit:         10,
		ExitWhenLimitIsHit: true,
	}

	fs := fs.RealFS()
	resolver := resolver.NewResolver(fs, []string{".jsx", ".js", ".mjs"})
	logger, _ := logging.NewStderrLog(logOptions)
	bundle := bundler.ScanBundle(logger, fs, resolver, []string{filepath}, parseOptions)
	result := bundle.Compile(logger, bundleOptions)
	//log.Println(filepath, result)

	for _, item := range result {
		if strings.Contains(item.JsAbsPath+"x", filepath) {
			return item.JsContents, nil
		}
	}
	return nil, fmt.Errorf("no result from esbuild")
}
